// Copyright (c) 2019 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package sftp

import (
	"fmt"
	"io"
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

		// get directory data, copy remote to local
		exit := make(chan bool)
		for s, c := range r.Client {
			server := s
			client := c

			go func() {
				// set Progress
				client.Output.Progress = r.Progress
				client.Output.ProgressWG = r.ProgressWG

				// set ftp client
				ftp := client.Connect

				// get output writer
				client.Output.Create(server)
				ow := client.Output.NewWriter()

				// local target
				target, _ = filepath.Abs(target)

				err = pullPath(ftp, ow, client.Output, source, client.Pwd, target)

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
func (r *RunSftp) pullPath(client *SftpConnect, target, base, path string) (err error) {

}

// いらないかも？
func (r *RunSftp) pullFile(client *SftpConnect, remote io.Reader, path string, size int64) (err error) {

}
