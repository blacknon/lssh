// Copyright (c) 2021 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

// This file describes the code of the built-in command used by lsftp.

package sftp

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/blacknon/lssh/common"
	"github.com/urfave/cli"
)

// chmod
func (r *RunSftp) chmod(args []string) {
	// create app
	app := cli.NewApp()
	// app.UseShortOptionHandling = true

	// set help message
	app.CustomAppHelpTemplate = helptext
	app.Name = "chmod"
	app.Usage = "lsftp build-in command: chmod [remote machine chmod]"
	app.ArgsUsage = "[perm path]"
	app.HideHelp = true
	app.HideVersion = true
	app.EnableBashCompletion = true

	// action
	app.Action = func(c *cli.Context) error {
		if len(c.Args()) != 2 {
			fmt.Println("Requires two arguments")
			fmt.Println("chmod mode path")
			return nil
		}

		exit := make(chan bool)
		for s, cl := range r.Client {
			server := s
			client := cl

			mode := c.Args()[0]
			path := c.Args()[1]

			go func() {
				// get writer
				client.Output.Create(server)
				w := client.Output.NewWriter()

				// set arg path
				if !filepath.IsAbs(path) {
					path = filepath.Join(client.Pwd, path)
				}

				// get mode
				modeint, err := strconv.ParseUint(mode, 8, 32)
				if err != nil {
					fmt.Fprintf(w, "%s\n", err)
					exit <- true
					return
				}
				filemode := os.FileMode(modeint)

				// set filemode
				err = client.Connect.Chmod(path, filemode)
				if err != nil {
					fmt.Fprintf(w, "%s\n", err)
					exit <- true
					return
				}

				fmt.Fprintf(w, "chmod: set %s's permission as %o(%s)\n", path, filemode.Perm(), filemode.String())
				exit <- true
				return
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
