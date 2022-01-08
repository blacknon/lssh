// Copyright (c) 2021 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

// This file describes the code of the built-in command used by lsftp.

package sftp

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strconv"

	"github.com/blacknon/lssh/common"
	"github.com/pkg/sftp"
	"github.com/urfave/cli"
)

// chown
func (r *RunSftp) chown(args []string) {
	// create app
	app := cli.NewApp()
	// app.UseShortOptionHandling = true

	// set help message
	app.CustomAppHelpTemplate = helptext
	app.Name = "chown"
	app.Usage = "lsftp build-in command: chown [remote machine chown]"
	app.ArgsUsage = "[user path]"
	app.HideHelp = true
	app.HideVersion = true
	app.EnableBashCompletion = true

	// action
	app.Action = func(c *cli.Context) error {
		if len(c.Args()) != 2 {
			fmt.Println("Requires two arguments")
			fmt.Println("chown group path")
			return nil
		}

		exit := make(chan bool)
		for s, cl := range r.Client {
			server := s
			client := cl

			user := c.Args()[0]
			path := c.Args()[1]

			go func() {
				// get writer
				client.Output.Create(server)
				w := client.Output.NewWriter()

				// set arg path
				if !filepath.IsAbs(path) {
					path = filepath.Join(client.Pwd, path)
				}

				//
				userid, err := strconv.Atoi(user)
				var gid, uid int
				if err != nil {
					// read /etc/passwd
					passwdFile, err := client.Connect.Open("/etc/passwd")
					if err != nil {
						fmt.Fprintf(w, "%s\n", err)
						exit <- true
						return
					}
					passwdByte, err := ioutil.ReadAll(passwdFile)
					if err != nil {
						fmt.Fprintf(w, "%s\n", err)
						exit <- true
						return
					}
					passwd := string(passwdByte)

					// get gid
					uid32, err := common.GetIdFromName(passwd, user)
					if err != nil {
						fmt.Fprintf(w, "%s\n", err)
						exit <- true
						return
					}

					uid = int(uid32)
				} else {
					uid = int(userid)
				}

				// get current uid
				stat, err := client.Connect.Lstat(path)
				if err != nil {
					fmt.Fprintf(w, "%s\n", err)
					exit <- true
					return
				}

				sys := stat.Sys()
				if fstat, ok := sys.(*sftp.FileStat); ok {
					gid = int(fstat.GID)
				}

				// set gid
				err = client.Connect.Chown(path, uid, gid)
				if err != nil {
					fmt.Fprintf(w, "%s\n", err)
					exit <- true
					return
				}

				fmt.Fprintf(w, "chown: set %s's user as %s\n", path, user)
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
