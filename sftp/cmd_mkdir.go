// Copyright (c) 2021 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

// This file describes the code of the built-in command used by lsftp.
// It is quite big in that relationship. Maybe it will be separated or repaired soon.

package sftp

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/blacknon/lssh/common"
	"github.com/urfave/cli"
)

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
	app.ArgsUsage = "path..."
	app.HideHelp = true
	app.HideVersion = true
	app.EnableBashCompletion = true

	// action
	app.Action = func(c *cli.Context) error {
		// TODO(blacknon): 複数のディレクトリ受付(v0.6.2以降)
		if len(c.Args()) <= 1 {
			fmt.Println("Requires one arguments")
			fmt.Println("mkdir path...")
			return nil
		}

		targetmap := map[string]*TargetConnectMap{}
		for _, p := range c.Args() {
			targetmap = r.createTargetMap(targetmap, p)
		}

		exit := make(chan bool)
		for s, cl := range targetmap {
			server := s
			client := cl

			go func() {
				// get writer
				client.Output.Create(server)
				w := client.Output.NewWriter()

				for _, path := range client.Path {
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
				}

				exit <- true
			}()
		}

		for i := 0; i < len(targetmap); i++ {
			<-exit
		}

		return nil
	}

	// parse short options
	args = common.ParseArgs(app.Flags, args)
	app.Run(args)

	return
}

//
func (r *RunSftp) lmkdir(args []string) {
	// create app
	app := cli.NewApp()
	// app.UseShortOptionHandling = true

	// set parameter
	app.Flags = []cli.Flag{
		cli.BoolFlag{Name: "p", Usage: "no error if existing, make parent directories as needed"},
	}

	// set help message
	app.CustomAppHelpTemplate = helptext
	app.Name = "lmkdir"
	app.Usage = "lsftp build-in command: lmkdir [local machine mkdir]"
	app.ArgsUsage = "[path]"
	app.HideHelp = true
	app.HideVersion = true
	app.EnableBashCompletion = true

	// action
	app.Action = func(c *cli.Context) error {
		// TODO(blacknon): 複数のディレクトリ受付(v0.6.2以降)
		if len(c.Args()) != 1 {
			fmt.Println("Requires one arguments")
			fmt.Println("lmkdir [path]")
			return nil
		}

		pathlist := c.Args()
		var err error

		for _, path := range pathlist {
			if c.Bool("p") {
				err = os.MkdirAll(path, 0755)
			} else {
				err = os.Mkdir(path, 0755)
			}

			if err != nil {
				fmt.Fprintf(os.Stderr, "%s\n", err)
			}
		}

		return nil
	}

	// parse short options
	args = common.ParseArgs(app.Flags, args)
	app.Run(args)

	return
}
