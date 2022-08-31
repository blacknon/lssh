// Copyright (c) 2022 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

// This file describes the code of the built-in command used by lsftp.

package sftp

import (
	"fmt"
	"os"
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
	app.ArgsUsage = "perm path..."
	app.HideHelp = true
	app.HideVersion = true
	app.EnableBashCompletion = true

	// action
	app.Action = func(c *cli.Context) error {
		if len(c.Args()) <= 1 {
			fmt.Println("Requires over two arguments")
			fmt.Println("chmod mode path...")
			return nil
		}

		mode := c.Args()[0]
		pathlist := c.Args()[1:]

		targetmap := map[string]*TargetConnectMap{}
		for _, p := range pathlist {
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

				// get mode
				modeint, err := strconv.ParseUint(mode, 8, 32)
				if err != nil {
					fmt.Fprintf(w, "%s\n", err)
					exit <- true
					return
				}
				filemode := os.FileMode(modeint)

				for _, path := range client.Path {
					// set arg path
					pathList, err := ExpandRemotePath(client, path)
					if err != nil {
						fmt.Fprintf(w, "%s\n", err)
						exit <- true
						return
					}

					for _, p := range pathList {
						// set filemode
						err = client.Connect.Chmod(p, filemode)
						if err != nil {
							fmt.Fprintf(w, "%s\n", err)
							exit <- true
							return
						}

						fmt.Fprintf(w, "chmod: set %s's permission as %o(%s)\n", p, filemode.Perm(), filemode.String())
					}
				}

				exit <- true
				return
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
