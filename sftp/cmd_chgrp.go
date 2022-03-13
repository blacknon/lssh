// Copyright (c) 2022 Blacknon. All rights reserved.
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

// chgrp
func (r *RunSftp) chgrp(args []string) {
	// create app
	app := cli.NewApp()
	// app.UseShortOptionHandling = true

	// set help message
	app.CustomAppHelpTemplate = helptext
	app.Name = "chgrp"
	app.Usage = "lsftp build-in command: chgrp [remote machine chgrp]"
	app.ArgsUsage = "group path..."
	app.HideHelp = true
	app.HideVersion = true
	app.EnableBashCompletion = true

	// action
	app.Action = func(c *cli.Context) error {
		if len(c.Args()) <= 1 {
			fmt.Println("Requires over two arguments")
			fmt.Println("chgrp group path...")
			return nil
		}

		group := c.Args()[0]
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

				//
				groupid, err := strconv.Atoi(group)
				var gid, uid int
				if err != nil {
					// read /etc/group
					groupFile, err := client.Connect.Open("/etc/group")
					if err != nil {
						fmt.Fprintf(w, "%s\n", err)
						exit <- true
						return
					}
					groupByte, err := ioutil.ReadAll(groupFile)
					if err != nil {
						fmt.Fprintf(w, "%s\n", err)
						exit <- true
						return
					}
					groups := string(groupByte)

					// get gid
					gid32, err := common.GetIdFromName(groups, group)
					if err != nil {
						fmt.Fprintf(w, "%s\n", err)
						exit <- true
						return
					}

					gid = int(gid32)
				} else {
					gid = int(groupid)
				}

				for _, path := range client.Path {
					// set arg path
					if !filepath.IsAbs(path) {
						path = filepath.Join(client.Pwd, path)
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
						uid = int(fstat.UID)
					}

					// set gid
					err = client.Connect.Chown(path, uid, gid)
					if err != nil {
						fmt.Fprintf(w, "%s\n", err)
						exit <- true
						return
					}

					fmt.Fprintf(w, "chgrp: set %s's group as %s\n", path, group)
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
