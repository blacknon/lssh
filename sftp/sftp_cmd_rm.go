// Copyright (c) 2019 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package sftp

import (
	"fmt"

	"github.com/blacknon/lssh/common"
	"github.com/urfave/cli"
)

//
func (r *RunSftp) rm(args []string) {
	// create app
	app := cli.NewApp()
	// app.UseShortOptionHandling = true

	// set help message
	app.CustomAppHelpTemplate = helptext

	// set parameter
	// TODO(blacknon): walkerでPATHを取得して各個削除する
	app.Flags = []cli.Flag{
		cli.BoolFlag{Name: "r", Usage: "remove directories and their contents recursively"},
	}

	app.Name = "rm"
	app.Usage = "lsftp build-in command: rm [remote machine rm]"
	app.HideHelp = true
	app.HideVersion = true
	app.EnableBashCompletion = true

	// action
	app.Action = func(c *cli.Context) error {
		if len(c.Args()) != 1 {
			fmt.Println("Requires one arguments")
			fmt.Println("rm [path]")
			return nil
		}

		for s, cl := range r.Client {
			server := s
			client := cl
			path := c.Args()[0]

			go func() {
				// get writer
				client.Output.Create(server)
				w := client.Output.NewWriter()

				// get current directory
				if c.Bool("r") {
					// create walker
					walker := client.Connect.Walk(c.Args()[0])

					var data []string
					for walker.Step() {
						err := walker.Err()
						if err != nil {
							fmt.Fprintf(w, "Error: %s\n", err)
							continue
						}

						p := walker.Path()
						data = append(data, p)
					}

					// reverse slice
					for i, j := 0, len(data)-1; i < j; i, j = i+1, j-1 {
						data[i], data[j] = data[j], data[i]
					}

					for _, p := range data {
						err := client.Connect.Remove(p)
						if err != nil {
							fmt.Fprintf(w, "%s\n", err)
							return
						}
					}

				} else {
					err := client.Connect.Remove(path)
					if err != nil {
						fmt.Fprintf(w, "%s\n", err)
						return
					}
				}

				fmt.Fprintf(w, "remove: %s\n", path)
			}()
		}

		return nil
	}

	// parse short options
	args = common.ParseArgs(app.Flags, args)
	app.Run(args)

	return
}
