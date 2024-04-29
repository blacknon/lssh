// Copyright (c) 2022 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package sftp

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/blacknon/lssh/common"
	"github.com/urfave/cli"
	"github.com/vbauerster/mpb"
)

// TODO: 複数サーバからの取得と個別サーバからの取得の処理分岐(出力先PATH指定)がうまくいってないので修正する.

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

	// action
	app.Action = func(c *cli.Context) error {
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

		// 複数ホストに接続して取得する場合、destinationはディレクトリ指定と仮定して動作させる.
		isDir := false
		if len(targetmap) > 1 {
			isDir = true
			// mkdir local destination directory
			err = os.MkdirAll(destination, 0755)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %s\n", err)
				return nil
			}
		}

		// get directory data, copy remote to local
		exit := make(chan bool)
		for s, c := range targetmap {
			// go funcに食わせるための変数宣言箇所
			server := s
			client := c
			targetDestinationPath := destination

			// 複数サーバが指定されていた場合、targetDestinationPath(取得先のPath)はディレクトリとみなし、複数ホスト別ディレクトリを作成する.
			if len(targetmap) > 1 {
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
				// set Progress
				client.Output.Progress = r.Progress
				client.Output.ProgressWG = r.ProgressWG

				// create output
				client.Output.Create(server)

				err = r.pullData(client, targetDestinationPath, isDir)

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
func (r *RunSftp) pullData(client *TargetConnectMap, targetpath string, isdir bool) (err error) {
	// set pullfile Permission.
	filePerm := GeneratePermWithUmask([]string{"0", "6", "6", "6"}, r.LocalUmask)

	for _, path := range client.Path {
		// get writer
		ow := client.Output.NewWriter()

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

			walker := client.Connect.Walk(ep)

			for walker.Step() {
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
