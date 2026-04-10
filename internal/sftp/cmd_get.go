// Copyright (c) 2022 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package sftp

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/blacknon/lssh/internal/common"
	"github.com/blacknon/lssh/internal/output"
	"github.com/urfave/cli"
	"github.com/vbauerster/mpb"
)

type pullTask struct {
	remotePath string
	localPath  string
	size       int64
	mode       fs.FileMode
}

// get
func (r *RunSftp) get(args []string) {
	// create app
	app := cli.NewApp()

	// set help message
	app.CustomAppHelpTemplate = helptext

	// set parameter
	app.Name = "get"
	app.Usage = "lsftp build-in command: get"
	app.ArgsUsage = "source(remote)... target(local)"
	app.HideHelp = true
	app.HideVersion = true
	app.EnableBashCompletion = true
	app.Flags = []cli.Flag{
		cli.IntFlag{Name: "parallel,P", Value: 1, Usage: "parallel file copy count per host"},
		cli.BoolFlag{Name: "dry-run", Usage: "show get actions without modifying files"},
	}

	// action
	app.Action = func(c *cli.Context) error {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		if len(c.Args()) < 2 {
			fmt.Println("Requires over two arguments")
			fmt.Println("get source(remote)... target(local)")
			return nil
		}

		// Create Progress
		r.ProgressWG = new(sync.WaitGroup)
		r.Progress = mpb.New(mpb.WithWaitGroup(r.ProgressWG))
		r.DryRun = c.Bool("dry-run")

		// set pathlist
		argsSize := len(c.Args()) - 1
		source := c.Args()[:argsSize]
		destination := c.Args()[argsSize]

		// get destination directory abs
		destinationList, err := ExpandLocalPath(destination)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			return nil
		}

		// check destination count.
		if len(destinationList) != 1 {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			return nil
		}
		destination = destinationList[0]

		targetmap := map[string]*TargetConnectMap{}
		for _, spath := range source {
			targetmap = r.createTargetMap(targetmap, spath)
		}

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

		isMultiServer := len(targetmap) > 1
		destinationIsDir := shouldTreatLocalGetDestinationAsDir(destination, isMultiServer)
		if destinationIsDir {
			if stat, statErr := os.Stat(destination); statErr == nil && !stat.IsDir() {
				fmt.Fprintf(os.Stderr, "Error: destination must be directory when getting from multiple servers: %s\n", destination)
				return nil
			}

			// 複数サーバから取得する場合、出力先はディレクトリとして扱う.
			err = os.MkdirAll(destination, 0755)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %s\n", err)
				return nil
			}
		}

		// get directory data, copy remote to local
		exit := make(chan bool, len(targetmap))
		for s, c := range targetmap {
			// go funcに食わせるための変数宣言箇所
			server := s
			client := c
			targetDestinationPath := destination

			// 複数サーバが指定されていた場合、各ホストごとの保存先ディレクトリを作成する.
			if isMultiServer {
				targetDestinationPath = filepath.Join(destination, server)

				// make target dir at local machine.
				err = os.MkdirAll(targetDestinationPath, 0755)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: %s\n", err)
					return nil
				}
			}

			// pullDataをホスト台数分だけ並列実行する.
			go func() {
				tasks := make(chan pullTask)
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
							case path, ok := <-tasks:
								if !ok {
									if workerClient.Connect != client.Connect {
										workerClient.Connect.Close()
									}
									workerExit <- true
									return
								}

								r.pullData(ctx, workerClient, path)
							}
						}
					}(workerClient)
				}

				for _, path := range client.Path {
					err := r.enqueuePullTasks(ctx, client, path, targetDestinationPath, destinationIsDir, func(task pullTask) error {
						select {
						case <-ctx.Done():
							return ctx.Err()
						case tasks <- task:
							return nil
						}
					})
					if err != nil {
						close(tasks)
						for i := 0; i < workerCount; i++ {
							<-workerExit
						}
						if err != context.Canceled {
							fmt.Fprintf(os.Stderr, "Error: %s\n", err)
						}
						exit <- true
						return
					}
				}
				close(tasks)

				for i := 0; i < workerCount; i++ {
					<-workerExit
				}

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

func (r *RunSftp) enqueuePullTasks(ctx context.Context, client *TargetConnectMap, path, targetpath string, isdir bool, enqueue func(task pullTask) error) error {
	ow := client.Output.NewWriter()
	defer ow.Close()

	rpath := path
	if !filepath.IsAbs(rpath) {
		rpath = filepath.Join(client.Pwd, rpath)
	}

	epath, err := ExpandRemotePath(client, rpath)
	if err != nil {
		fmt.Fprintf(ow, "Error: %s\n", err)
		return err
	}
	if len(epath) == 0 {
		fmt.Fprintf(ow, "Error: File Not founds.\n")
		return fmt.Errorf("file not found: %s", path)
	}

	taskIsDir := isdir
	if len(epath) > 1 {
		taskIsDir = true
	}

	checkStat, err := client.Connect.Stat(epath[0])
	if err != nil {
		fmt.Fprintf(ow, "Error: %s\n", err)
		return err
	}
	if checkStat.IsDir() {
		taskIsDir = true
	}

	for _, ep := range epath {
		preserveSourceName := isdir || len(epath) > 1
		base := copySourceBase(ep, checkStat.IsDir(), preserveSourceName)

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		walker := client.Connect.Walk(ep)
		for walker.Step() {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			if err := walker.Err(); err != nil {
				fmt.Fprintf(ow, "Error: %s\n", err)
				continue
			}

			p := walker.Path()
			stat := walker.Stat()
			relpath, _ := filepath.Rel(base, p)
			relpath = strings.Replace(relpath, "../", "", 1)
			localpath := resolveLocalGetDestinationPath(targetpath, relpath, taskIsDir)

			if stat.IsDir() {
				if r.DryRun {
					r.printAction(client.Output, "mkdir", fmt.Sprintf("local:%s", localpath))
					if r.Permission {
						r.printAction(client.Output, "chmod", fmt.Sprintf("local:%s", localpath))
					}
				} else {
					err := os.MkdirAll(localpath, 0755)
					if err != nil {
						fmt.Fprintf(ow, "Error: %s\n", err)
						continue
					}
					if r.Permission {
						_ = os.Chmod(localpath, stat.Mode())
					}
				}
				continue
			}

			if err := enqueue(pullTask{
				remotePath: p,
				localPath:  localpath,
				size:       stat.Size(),
				mode:       stat.Mode(),
			}); err != nil {
				return err
			}
		}
	}

	return nil
}

// pullData
func (r *RunSftp) pullData(ctx context.Context, client *TargetConnectMap, task pullTask) (err error) {
	filePerm := GeneratePermWithUmask([]string{"0", "6", "6", "6"}, r.LocalUmask)
	ow := client.Output.NewWriter()
	defer ow.Close()

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if r.DryRun {
		r.printAction(client.Output, "copy", fmt.Sprintf("%s:%s -> local:%s", client.Output.Server, task.remotePath, task.localPath))
		if r.Permission {
			r.printAction(client.Output, "chmod", fmt.Sprintf("local:%s", task.localPath))
		}
		return nil
	}

	err = os.MkdirAll(filepath.Dir(task.localPath), 0755)
	if err != nil {
		fmt.Fprintf(ow, "Error: %s\n", err)
		return err
	}

	remotefile, err := client.Connect.Open(task.remotePath)
	if err != nil {
		fmt.Fprintf(ow, "Error: %s\n", err)
		return err
	}
	defer remotefile.Close()

	localfile, err := os.OpenFile(task.localPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, filePerm)
	if err != nil {
		fmt.Fprintf(ow, "Error: %s\n", err)
		return err
	}
	defer localfile.Close()

	rd := io.TeeReader(remotefile, localfile)

	r.ProgressWG.Add(1)
	client.Output.ProgressPrinter(task.size, rd, fmt.Sprintf("%s:%s -> local:%s", client.Output.Server, task.remotePath, task.localPath))

	if r.Permission {
		_ = os.Chmod(task.localPath, task.mode)
	}

	return nil
}
