// Copyright (c) 2021 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package sftp

import (
	"fmt"
	"path/filepath"

	"github.com/blacknon/lssh/common"
	"github.com/urfave/cli"
)

// TODO(blacknon): 転送時の進捗状況を表示するプログレスバーの表示はさせること
func (r *RunSftp) symlink(args []string) {
	// create app
	app := cli.NewApp()
	// app.UseShortOptionHandling = true

	// set help message
	app.CustomAppHelpTemplate = helptext

	app.Name = "symlink"
	app.Usage = "lsftp build-in command: symlink [remote machine symlink]"
	app.ArgsUsage = "[source target]"
	app.HideHelp = true
	app.HideVersion = true
	app.EnableBashCompletion = true

	// action
	app.Action = func(c *cli.Context) error {
		if len(c.Args()) != 2 {
			fmt.Println("Requires two arguments")
			fmt.Println("symlink source target")
			return nil
		}

		exit := make(chan bool)
		for s, cl := range r.Client {
			server := s
			client := cl

			source := c.Args()[0]
			target := c.Args()[1]

			go func() {
				// get writer
				client.Output.Create(server)
				w := client.Output.NewWriter()

				// set arg path
				if !filepath.IsAbs(source) {
					source = filepath.Join(client.Pwd, source)
				}

				if !filepath.IsAbs(target) {
					target = filepath.Join(client.Pwd, target)
				}

				err := client.Connect.Symlink(source, target)
				if err != nil {
					fmt.Fprintf(w, "%s\n", err)
					exit <- true
					return
				}

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
