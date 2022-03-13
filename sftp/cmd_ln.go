// Copyright (c) 2022 Blacknon. All rights reserved.
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

// TODO(blacknon): sftpライブラリ側で対応するようになったら開発する

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

		// parse old path, with server...
		source := c.Args()[0]
		targetmap := map[string]*TargetConnectMap{}
		targetmap = r.createTargetMap(targetmap, source)
		target := c.Args()[1]

		exit := make(chan bool)
		for s, cl := range targetmap {
			server := s
			client := cl

			go func() {
				// get writer
				client.Output.Create(server)
				w := client.Output.NewWriter()

				source := client.Path[0]

				// set arg path
				if !filepath.IsAbs(source) {
					source = filepath.Join(client.Pwd, source)
				}

				if !filepath.IsAbs(target) {
					target = filepath.Join(client.Pwd, target)
				}

				err := client.Connect.Link(source, target)
				if err != nil {
					fmt.Fprintf(w, "%s\n", err)
					exit <- true
					return
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
