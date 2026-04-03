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
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/blacknon/lssh/internal/common"
	"github.com/urfave/cli"
	"github.com/vbauerster/mpb"
)

// TODO: getでのパラレル取得が動いていない。putのコードを元に、getのコードを見直す必要がある。
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

		isMultiServer := len(targetmap) > 1
		destinationIsDir := isMultiServer
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
				tasks := make(chan string)
				workerExit := make(chan bool, parallelNum)
				workerCount := 0

				workers := make([]*TargetConnectMap, 0, parallelNum)
				workers = append(workers, client)

				if parallelNum > 1 {
					for i := 1; i < parallelNum; i++ {
						for _, extraClient := range r.createSftpConnect([]string{server}) {
							worker := &TargetConnectMap{
								SftpConnect: *extraClient,
								Path:        client.Path,
							}
							workers = append(workers, worker)
							registerExtraClient(worker)
						}
					}
				}

				for _, workerClient := range workers {
					workerClient.Output.Progress = r.Progress
					workerClient.Output.ProgressWG = r.ProgressWG
					workerClient.Output.Create(server)

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

								taskClient := &TargetConnectMap{
									SftpConnect: workerClient.SftpConnect,
									Path:        []string{path},
								}
								r.pullData(ctx, taskClient, targetDestinationPath, destinationIsDir)
							}
						}
					}(workerClient)
				}

				for _, path := range client.Path {
					select {
					case <-ctx.Done():
						close(tasks)
						for i := 0; i < workerCount; i++ {
							<-workerExit
						}
						exit <- true
						return
					case tasks <- path:
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

// pullData
func (r *RunSftp) pullData(ctx context.Context, client *TargetConnectMap, targetpath string, isdir bool) (err error) {
	// set pullfile Permission.
	filePerm := GeneratePermWithUmask([]string{"0", "6", "6", "6"}, r.LocalUmask)
	ow := client.Output.NewWriter()
	defer ow.Close()

	for _, path := range client.Path {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// set arg path and targetdir
		var rpath string

		// set base dir
		switch {
		case filepath.IsAbs(path):
			rpath = path
		case !filepath.IsAbs(path):
			rpath = filepath.Join(client.Pwd, path)
		}
		base := filepath.Dir(rpath)

		// expantion path
		epath, eerr := ExpandRemotePath(client, rpath)
		if len(epath) == 0 {
			// glob展開したpathが0の場合、取得対象がなかったものとみなしerror.
			fmt.Fprintf(ow, "Error: File Not founds.\n")
			return
		} else if len(epath) > 1 {
			// glob展開したpathが1より大きい場合、複数ファイルを取得するものとみなしてtargetpathはディレクトリを指定されたものとみなす.
			isdir = true
		}

		if eerr != nil {
			fmt.Fprintf(ow, "Error: %s\n", eerr)
			return
		}

		// TODO: 取得元がディレクトリかどうかを識別して、isdirフラグの挙動を決める. 1個以上だったら問答無用でisdirが有効になるはずなので、1個目だけを見てやればいい？
		//       その後は、isDirだった場合にはpathを足していくような処理をwalkerのforで定義してやればいい。。。はず？

		checkStat, eerr := client.Connect.Stat(epath[0])
		if eerr != nil {
			fmt.Fprintf(ow, "Error: %s\n", eerr)
			return
		}
		if checkStat.IsDir() {
			isdir = true
		}

		// for walk
		for _, ep := range epath {
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

				err := walker.Err()
				if err != nil {
					fmt.Fprintf(ow, "Error: %s\n", err)
					continue
				}
				var localpath string

				p := walker.Path()
				stat := walker.Stat()

				if isdir {
					relpath, _ := filepath.Rel(base, p)
					relpath = strings.Replace(relpath, "../", "", 1)
					if strings.Contains(relpath, "/") {
						os.MkdirAll(filepath.Join(targetpath, filepath.Dir(relpath)), 0755)
					}
					localpath = filepath.Join(targetpath, relpath)

				} else {
					localpath = targetpath
				}

				//
				if stat.IsDir() { // is directory
					os.MkdirAll(localpath, 0755)
				} else { // is not directory
					// get size
					size := stat.Size()

					// open remote file
					remotefile, err := client.Connect.Open(p)
					if err != nil {
						fmt.Fprintf(ow, "Error: %s\n", err)
						continue
					}

					// open local file
					localfile, err := os.OpenFile(localpath, os.O_RDWR|os.O_CREATE, filePerm)
					if err != nil {
						fmt.Fprintf(ow, "Error: %s\n", err)
						continue
					}

					// empty the file
					err = localfile.Truncate(0)
					if err != nil {
						fmt.Fprintf(ow, "Error: %s\n", err)
						continue
					}

					// set tee reader
					rd := io.TeeReader(remotefile, localfile)

					r.ProgressWG.Add(1)
					client.Output.ProgressPrinter(size, rd, p)
				}

				// set mode
				if r.Permission {
					os.Chmod(localpath, stat.Mode())
				}
			}
		}
	}

	return
}
