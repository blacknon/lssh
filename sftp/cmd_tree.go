// Copyright (c) 2022 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

// This file describes the code of the built-in command used by lsftp.
// It is quite big in that relationship. Maybe it will be separated or repaired soon.

package sftp

import (
	"fmt"
	"path/filepath"
	"sync"

	"github.com/blacknon/lssh/common"
	"github.com/disiqueira/gotree"
	"github.com/urfave/cli"
)

// TODO(blacknon):
//   とりあえず、↓のライブラリ使って開発を実施する。
//     - https://github.com/DiSiqueira/GoTree

// tree is remote tree command
func (r *RunSftp) tree(args []string) (err error) {
	// create app
	app := cli.NewApp()

	// set help message
	app.CustomAppHelpTemplate = helptext

	// set parameter
	app.Flags = []cli.Flag{
		cli.BoolFlag{Name: "s", Usage: "print the size in bytes of each file."},
		cli.BoolFlag{Name: "h", Usage: "print the size in a more human readable way."},
	}
	app.Name = "tree"
	app.Usage = "lsftp build-in command: ltree [remote machine tree]"
	app.ArgsUsage = "[host,host...:][PATH]..."
	app.HideHelp = true
	app.HideVersion = true
	app.EnableBashCompletion = true

	// action
	app.Action = func(c *cli.Context) error {
		// argpath
		argData := c.Args()

		// set default pathList.
		// Specifies the default pathList.
		// This value is used if there is no destination specified in the argument.
		pathList := []string{"./"}

		// create empty targetmap
		targetmap := map[string]*TargetConnectMap{}

		// add path to targetmap
		if len(argData) > 0 {
			// If there is an argument specification.
			pathList = []string{}

			for _, arg := range argData {
				// sftp target host
				targetmap = r.createTargetMap(targetmap, arg)

			}
		} else {
			for server, client := range r.Client {
				// sftp target host
				targetmap[server] = &TargetConnectMap{}
				targetmap[server].SftpConnect = *client
			}

			pathList = []string{}
			for _, arg := range argData {
				// sftp target host
				pathList = append(pathList, arg)
			}

		}

		r.executeRemoteTree(c, targetmap)

		return nil
	}

	// parse short options
	args = common.ParseArgs(app.Flags, args)
	app.Run(args)

	return
}

//
func (r *RunSftp) executeRemoteTree(c *cli.Context, clients map[string]*TargetConnectMap) {
	treeData := map[string]gotree.Tree{}
	exit := make(chan bool)
	m := new(sync.Mutex)

	for s, cl := range clients {
		server := s
		client := cl

		// Get required data at tree, is obtained in parallel from each server.
		go func() {
			// get output
			client.Output.Create(server)
			w := client.Output.NewWriter()

			// set target directory
			if len(client.Path) > 0 {
				for i, path := range client.Path {
					if !filepath.IsAbs(path) {
						client.Path[i] = filepath.Join(client.Pwd, path)
					}
				}
			} else {
				client.Path = append(client.Path, client.Pwd)
			}

			// get tree data
			data, err := r.buildRemoteDirTree(client, c)
			if err != nil {
				fmt.Fprintf(w, "Error: %s\n", err)
				exit <- true
				return
			}

			// write lsdata
			m.Lock()
			treeData[server] = data
			m.Unlock()
		}()

		// wait get directory data
		for i := 0; i < len(clients); i++ {
			<-exit
		}

	}

	// TODO: 各targetへの処理をかく(lsとかと同じ)

	// TODO: 各ホストで実行する処理になるので、あとでこの関数からは消す？
	// for _, path := range pathList {
	// // get dirctory tree data.
	// dirTree, err := buildDirTree(nil, path, c)
	// 	if err != nil {
	// 		fmt.Fprintf(os.Stderr, "%s\n", err)
	// 		return nil
	// 	}

	// fmt.Println(dirTree.Print())
	// }
}

//
func (r *RunSftp) buildRemoteDirTree(client *TargetConnectMap, options *cli.Context) (tree gotree.Tree, err error) {
	// w := client.Output.NewWriter()

	// for _, ep := range client.Path {

	// }

	return
}
