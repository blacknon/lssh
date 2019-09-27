// Copyright (c) 2019 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

// This file describes the code of the built-in command used by lsftp.
// It is quite big in that relationship. Maybe it will be separated or repaired soon.

package sftp

import (
	"fmt"
	"path/filepath"

	"github.com/blacknon/lssh/common"
	"github.com/urfave/cli"
)

//
//
func (r *RunSftp) mkdir(args []string) {
	// create app
	app := cli.NewApp()
	// app.UseShortOptionHandling = true

	// set parameter
	app.Flags = []cli.Flag{
		cli.BoolFlag{Name: "p", Usage: "no error if existing, make parent directories as needed"},
	}

	// set help message
	app.CustomAppHelpTemplate = helptext
	app.Name = "mkdir"
	app.Usage = "lsftp build-in command: mkdir [remote machine mkdir]"
	app.ArgsUsage = "[path]"
	app.HideHelp = true
	app.HideVersion = true
	app.EnableBashCompletion = true

	// action
	app.Action = func(c *cli.Context) error {
		// TODO(blacknon): 複数のディレクトリ受付(v0.6.1)
		if len(c.Args()) != 1 {
			fmt.Println("Requires one arguments")
			fmt.Println("mkdir [path]")
			return nil
		}

		exit := make(chan bool)
		for s, cl := range r.Client {
			server := s
			client := cl
			path := c.Args()[0]

			go func() {
				// get writer
				client.Output.Create(server)
				w := client.Output.NewWriter()

				// set arg path
				if !filepath.IsAbs(path) {
					path = filepath.Join(client.Pwd, path)
				}

				// create directory
				var err error
				if c.Bool("p") {
					err = client.Connect.MkdirAll(path)
				} else {
					err = client.Connect.Mkdir(path)
				}

				// check error
				if err != nil {
					fmt.Fprintf(w, "%s\n", err)
				}

				fmt.Fprintf(w, "make directory: %s\n", path)
				exit <- true
			}()
		}

		for i := 0; i < len(r.Client); i++ {
			<-exit
		}

		return nil
	}

	// parse short options
	args = common.ParseArgs(app.Flags, args)
	app.Run(args)

	return
}
