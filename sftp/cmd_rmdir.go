// Copyright (c) 2021 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package sftp

import (
	"fmt"

	"github.com/blacknon/lssh/common"
	"github.com/urfave/cli"
)

//
func (r *RunSftp) rmdir(args []string) {
	// create app
	app := cli.NewApp()
	// app.UseShortOptionHandling = true

	// set help message
	app.CustomAppHelpTemplate = helptext
	app.Name = "rmdir"
	app.Usage = "lsftp build-in command: rmdir [remote machine rmdir]"
	app.HideHelp = true
	app.HideVersion = true
	app.EnableBashCompletion = true

	// action
	app.Action = func(c *cli.Context) error {
		if len(c.Args()) < 1 {
			fmt.Println("Requires one arguments")
			fmt.Println("rmdir path...")
			return nil
		}

		targetmap := map[string]*TargetConnectMap{}
		for _, p := range c.Args() {
			targetmap = r.createTargetMap(targetmap, p)
		}

		for server, client := range targetmap {
			// get writer
			client.Output.Create(server)
			w := client.Output.NewWriter()

			// remove directory
			for _, path := range client.Path {
				err := client.Connect.RemoveDirectory(path)
				if err != nil {
					fmt.Fprintf(w, "%s\n", err)
					continue
				}
				fmt.Fprintf(w, "remove dir: %s\n", path)
			}
		}

		return nil
	}

	// parse short options
	args = common.ParseArgs(app.Flags, args)
	app.Run(args)

	return
}
