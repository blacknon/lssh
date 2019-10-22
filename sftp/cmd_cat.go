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

// cat is remote cat command
func (r *RunSftp) cat(args []string) {
	// create app
	app := cli.NewApp()
	// app.UseShortOptionHandling = true

	// set help message
	app.CustomAppHelpTemplate = helptext

	app.Name = "cat"
	app.Usage = "lsftp build-in command: cat [remote machine cat]"
	app.ArgsUsage = "[PATH]"
	app.HideHelp = true
	app.HideVersion = true
	app.EnableBashCompletion = true

	app.Action = func(c *cli.Context) error {
		argpath := c.Args().First()

		//
		fmt.Println(argpath)
		return nil
	}

	// parse short options
	args = common.ParseArgs(app.Flags, args)
	app.Run(args)
}

// lcat is local cat command
func (r *RunSftp) lcat(args []string) {
	// create app
	app := cli.NewApp()
	// app.UseShortOptionHandling = true

	// set help message
	app.CustomAppHelpTemplate = helptext

	app.Name = "lcat"
	app.Usage = "lsftp build-in command: lcat [local machine cat]"
	app.ArgsUsage = "[PATH]"
	app.HideHelp = true
	app.HideVersion = true
	app.EnableBashCompletion = true

	app.Action = func(c *cli.Context) error {
		argpath := c.Args().First()

		//
		fmt.Println(argpath)
		return nil
	}

	// parse short options
	args = common.ParseArgs(app.Flags, args)
	app.Run(args)

}
