// Copyright (c) 2022 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package sftp

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/blacknon/lssh/internal/common"
	"github.com/blacknon/lssh/internal/output"
	"github.com/urfave/cli"
	"github.com/vbauerster/mpb"
)

func (r *RunSftp) put(args []string) {
	// create app
	app := cli.NewApp()

	// set help message
	app.CustomAppHelpTemplate = helptext

	// set parameter
	app.Name = "put"
	app.Usage = "lsftp build-in command: put"
	app.ArgsUsage = "source(local)... target(remote)"
	app.HideHelp = true
	app.HideVersion = true
	app.EnableBashCompletion = true
	app.Flags = []cli.Flag{
		cli.IntFlag{Name: "parallel,P", Value: 1, Usage: "parallel file copy count per host"},
	}

	// action
	app.Action = func(c *cli.Context) error {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		if len(c.Args()) < 2 {
			fmt.Println("Requires over two arguments")
			fmt.Println("put source(local)... target(remote)")
			return nil
		}

		// Create Progress
		r.ProgressWG = new(sync.WaitGroup)
		r.Progress = mpb.New(mpb.WithWaitGroup(r.ProgressWG))

		// set path
		argsSize := len(c.Args()) - 1
		source := c.Args()[:argsSize]
		destination := c.Args()[argsSize]

		pathset := []PathSet{}

		for _, l := range source {
			epath, err := ExpandLocalPath(l)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s\n", err)
				return nil
			}

			for _, p := range epath {
				// get local host directory walk data
				data, err := common.WalkDir(p)
				if err != nil {
					fmt.Fprintf(os.Stderr, "%s\n", err)
					return nil
				}

				sort.Strings(data)
				dataset := PathSet{
					Base:      filepath.Dir(p),
					PathSlice: data,
				}
				pathset = append(pathset, dataset)
			}
		}

		targetmap := map[string]*TargetConnectMap{}
		targetmap = r.createTargetMap(targetmap, destination)

		parallelNum := c.Int("parallel")
		if parallelNum < 1 {
			parallelNum = 1
		}

		var cancelMu sync.Mutex
		cancelExtraClients := map[*TargetConnectMap]struct{}{}
		registerExtraClient := func(client *TargetConnectMap) {
			if client == nil || client.Connect == nil {
				return
			}

			cancelMu.Lock()
			cancelExtraClients[client] = struct{}{}
			cancelMu.Unlock()
		}
		closeExtraClients := func() {
			cancelMu.Lock()
			clients := make([]*TargetConnectMap, 0, len(cancelExtraClients))
			for client := range cancelExtraClients {
				clients = append(clients, client)
			}
			cancelMu.Unlock()

			for _, client := range clients {
				client.Connect.Close()
			}
		}

		go func() {
			<-ctx.Done()
			closeExtraClients()
		}()

		newWorkerOutput := func(base *output.Output, server string) *output.Output {
			if base == nil {
				return nil
			}

			o := &output.Output{
				Templete:      base.Templete,
				ServerList:    append([]string(nil), base.ServerList...),
				Conf:          base.Conf,
				Progress:      r.Progress,
				ProgressWG:    r.ProgressWG,
				EnableHeader:  base.EnableHeader,
				DisableHeader: base.DisableHeader,
				AutoColor:     base.AutoColor,
			}
			o.Create(server)

			return o
		}

		// parallel push data
		exit := make(chan bool, len(targetmap))
		for s, c := range targetmap {
			server := s
			client := c
			go func() {
				type pushTask struct {
					base string
					path string
				}

				tasks := make(chan pushTask)
				workerExit := make(chan bool, parallelNum)
				workerCount := 0

				workers := make([]*TargetConnectMap, 0, parallelNum)
				client.Output = newWorkerOutput(client.Output, server)
				workers = append(workers, client)

				if parallelNum > 1 {
					for i := 1; i < parallelNum; i++ {
						for _, extraClient := range r.createSftpConnect([]string{server}) {
							worker := &TargetConnectMap{
								SftpConnect: *extraClient,
								Path:        client.Path,
							}
							worker.Output = newWorkerOutput(client.Output, server)
							workers = append(workers, worker)
							registerExtraClient(worker)
						}
					}
				}

				for _, workerClient := range workers {
					workerCount++
					go func(workerClient *TargetConnectMap) {
						for {
							select {
							case <-ctx.Done():
								if workerClient.Connect != client.Connect {
									workerClient.Connect.Close()
								}

								workerExit <- true
								return
							case task, ok := <-tasks:
								if !ok {
									if workerClient.Connect != client.Connect {
										workerClient.Connect.Close()
									}

									workerExit <- true
									return
								}

								r.pushData(workerClient, len(pathset) > 1, task.base, task.path)
							}
						}
					}(workerClient)
				}

				for _, p := range pathset {
					base := p.Base
					data := p.PathSlice
					for _, path := range data {
						select {
						case <-ctx.Done():
							close(tasks)
							for i := 0; i < workerCount; i++ {
								<-workerExit
							}

							exit <- true
							return
						case tasks <- pushTask{base: base, path: path}:
						}
					}
				}
				close(tasks)

				for i := 0; i < workerCount; i++ {
					<-workerExit
				}

				// exit
				exit <- true
			}()
		}

		// wait exit
		for i := 0; i < len(targetmap); i++ {
			<-exit
		}
		close(exit)

		// wait Progress
		r.Progress.Wait()

		// wait 0.3 sec
		time.Sleep(300 * time.Millisecond)

		return nil
	}

	// parse short options
	args = common.ParseArgs(app.Flags, args)
	app.Run(args)

	return
}

func (r *RunSftp) pushData(client *TargetConnectMap, isMultiple bool, base, path string) (err error) {
	// set arg relpath
	relpath, _ := filepath.Rel(base, path)

	for _, target := range client.Path {
		target = strings.TrimSpace(target)
		targetList := []string{}

		if strings.ContainsAny(target, "*?[") {
			targetList, err = ExpandRemotePath(client, target)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %s\n", err)
				return err
			}
		} else {
			switch {
			case target == "~":
				target, err = client.Connect.Getwd()
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: %s\n", err)
					return err
				}
			case strings.HasPrefix(target, "~/"):
				home, homeErr := client.Connect.Getwd()
				if homeErr != nil {
					fmt.Fprintf(os.Stderr, "Error: %s\n", homeErr)
					return homeErr
				}
				target = filepath.Join(home, target[2:])
			case !filepath.IsAbs(target):
				target = filepath.Join(client.Pwd, target)
			}

			targetList = append(targetList, target)
		}

		for _, t := range targetList {
			rpath := ""

			// set rpath
			lstat, err := client.Connect.Lstat(t)
			if err == nil {
				if lstat.IsDir() {
					rpath = filepath.Join(t, relpath)
				}
			}

			if len(rpath) == 0 {
				dInfo, _ := os.Lstat(path)
				if dInfo.IsDir() || isMultiple {
					rpath = filepath.Join(t, relpath)
					client.Connect.Mkdir(t)
				} else {
					rpath = filepath.Clean(t)
				}
			}

			// get local file info
			fInfo, _ := os.Lstat(path)
			if fInfo.IsDir() { // directory
				client.Connect.Mkdir(rpath)
			} else { //file
				// open local file
				localfile, err := os.Open(path)
				if err != nil {
					return err
				}
				defer localfile.Close()

				// get file size
				lstat, _ := os.Lstat(path)
				size := lstat.Size()

				// copy file
				err = r.pushFile(client, localfile, rpath, size)
				if err != nil {
					return err
				}
			}

			// set mode
			if r.Permission {
				client.Connect.Chmod(rpath, fInfo.Mode())
			}
		}
	}

	return
}

// pushfile put file to path.
func (r *RunSftp) pushFile(client *TargetConnectMap, localfile io.Reader, path string, size int64) (err error) {
	// mkdir all
	dir := filepath.Dir(path)
	err = client.Connect.MkdirAll(dir)
	if err != nil {
		return
	}

	// open remote file
	remotefile, err := client.Connect.OpenFile(path, os.O_RDWR|os.O_CREATE)
	if err != nil {
		return
	}

	// empty the file
	err = remotefile.Truncate(0)
	if err != nil {
		return
	}

	// set tee reader
	rd := io.TeeReader(localfile, remotefile)

	// copy to data
	r.ProgressWG.Add(1)
	client.Output.ProgressPrinter(size, rd, path)

	return
}
