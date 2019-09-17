// Copyright (c) 2019 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

// This file describes the code of the built-in command used by lsftp.
// It is quite big in that relationship. Maybe it will be separated or repaired soon.

package sftp

import (
	"fmt"

	"github.com/blacknon/lssh/common"
	"github.com/urfave/cli"
)

//
func (r *RunSftp) ln(args []string) (err error) {
	// create app
	app := cli.NewApp()
	// app.UseShortOptionHandling = true

	// set help message
	app.CustomAppHelpTemplate = helptext
	app.Name = "ln"
	app.Usage = "lsftp build-in command: ln [remote machine ln]"
	app.ArgsUsage = "[PATH PATH]"
	app.HideHelp = true
	app.HideVersion = true
	app.EnableBashCompletion = true

	// action
	app.Action = func(c *cli.Context) error {
		if len(c.Args()) != 2 {
			fmt.Println("Requires two arguments")
			fmt.Println("ln [option] source target")
			return nil
		}

		// for s, cl := range r.Client {
		// 	server := s
		// 	client := cl

		// 	go func() {
		// 		// get writer
		// 		client.Output.Create(server)
		// 		w := client.Output.NewWriter()
		// 	}()
		// }

		return nil
	}

	// parse short options
	args = common.ParseArgs(app.Flags, args)
	app.Run(args)

	return
}
