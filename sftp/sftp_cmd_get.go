// Copyright (c) 2019 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package sftp

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/blacknon/lssh/common"
	"github.com/urfave/cli"
	"github.com/vbauerster/mpb"
)

// TODO(blacknon): リファクタリング(v0.6.1)

//
func (r *RunSftp) get(args []string) {
	// create app
	app := cli.NewApp()
	// app.UseShortOptionHandling = true

	// set help message
	app.CustomAppHelpTemplate = helptext

	// set parameter
	app.Name = "get"
	app.Usage = "lsftp build-in command: get"
	app.ArgsUsage = "[source(remote) target(local)]"
	app.HideHelp = true
	app.HideVersion = true
	app.EnableBashCompletion = true

	// action
	app.Action = func(c *cli.Context) error {
		if len(c.Args()) != 2 {
			fmt.Println("Requires two arguments")
			fmt.Println("get source(remote) target(local)")
			return nil
		}

		// Create Progress
		r.ProgressWG = new(sync.WaitGroup)
		r.Progress = mpb.New(mpb.WithWaitGroup(r.ProgressWG))

		// set path
		source := c.Args()[0]
		target := c.Args()[1]

		// get target directory abs
		target, err := filepath.Abs(target)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			return nil
		}

		// mkdir local target directory
		err = os.MkdirAll(target, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			return nil
		}

		// get directory data, copy remote to local
		exit := make(chan bool)
		for s, c := range r.Client {
			server := s
			client := c

			targetdir := target
			// TODO: ホストを複数台指定している場合、ホスト名でディレクトリを掘る
			if len(r.Client) > 1 {
				targetdir = filepath.Join(target, server)
			}

			go func() {
				// set Progress
				client.Output.Progress = r.Progress
				client.Output.ProgressWG = r.ProgressWG

				// create output
				client.Output.Create(server)

				// local target
				target, _ = filepath.Abs(target)

				err = r.pullPath(client, source, client.Pwd, targetdir)

				exit <- true
			}()
		}

		// wait exit
		for i := 0; i < len(r.Client); i++ {
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

//
func (r *RunSftp) pullPath(client *SftpConnect, path, base, target string) (err error) {
	// set arg path
	rpath, _ := filepath.Rel(base, path)
	switch {
	case filepath.IsAbs(target):
		rpath = filepath.Join(target, rpath)
	case !filepath.IsAbs(target):
		target = filepath.Join(client.Pwd, target)
		rpath = filepath.Join(target, rpath)
	}

	// get writer
	ow := client.Output.NewWriter()

	// expantion path
	epath, _ := client.Connect.Glob(rpath)

	// for walk
	for _, ep := range epath {
		walker := client.Connect.Walk(ep)

		fmt.Println(walker)
		for walker.Step() {
			err := walker.Err()
			if err != nil {
				fmt.Fprintf(ow, "Error: %s\n", err)
				continue
			}

			p := walker.Path()
			stat := walker.Stat()

			localpath := filepath.Join(target, p)

			//
			if stat.IsDir() { // is directory
				os.Mkdir(localpath, 0755)
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
				localfile, err := os.OpenFile(localpath, os.O_RDWR|os.O_CREATE, 0644)
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

	// TODO: ftpクライアントを使ってディレクトリをwalk(+ワイルドカードか？)

	// TODO: walk後、forで回してgetしていく

}
