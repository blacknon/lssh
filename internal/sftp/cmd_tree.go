// Copyright (c) 2022 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

// This file describes the code of the built-in command used by lsftp.
// It is quite big in that relationship. Maybe it will be separated or repaired soon.

package sftp

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/blacknon/lssh/internal/common"
	"github.com/disiqueira/gotree"
	"github.com/dustin/go-humanize"
	"github.com/urfave/cli"
)

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

func (r *RunSftp) executeRemoteTree(c *cli.Context, clients map[string]*TargetConnectMap) {
	treeData := map[string][]gotree.Tree{}
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
				w.Close()
				exit <- true
				return
			}

			// write lsdata
			m.Lock()
			treeData[server] = data
			m.Unlock()

			w.Close()
			exit <- true
		}()
	}

	for i := 0; i < len(clients); i++ {
		<-exit
	}

	for server, trees := range treeData {
		clients[server].Output.Create(server)
		w := clients[server].Output.NewWriter()

		for _, tree := range trees {
			fmt.Fprintln(w, tree.Print())
		}

		w.Close()
	}
}

func (r *RunSftp) buildRemoteDirTree(client *TargetConnectMap, options *cli.Context) (trees []gotree.Tree, err error) {
	for _, path := range client.Path {
		stat, statErr := client.Connect.Stat(path)
		if statErr != nil {
			return nil, statErr
		}

		tree := gotree.New(path)
		if stat.IsDir() {
			tree = r.buildRemoteDirNode(client, path, options)
		}

		trees = append(trees, tree)
	}

	return
}

func (r *RunSftp) buildRemoteDirNode(client *TargetConnectMap, dir string, options *cli.Context) (dirTree gotree.Tree) {
	dirName := filepath.Base(dir)
	if dirName == "." || dirName == "/" || dirName == string(os.PathSeparator) {
		dirName = dir
	}

	dirNameText := dirName + "/"

	if stat, err := client.Connect.Stat(dir); err == nil {
		switch {
		case options.Bool("h"):
			size := humanize.Bytes(uint64(stat.Size()))
			dirNameText = fmt.Sprintf("[%10s] %s", size, dirNameText)
		case options.Bool("s"):
			dirNameText = fmt.Sprintf("[%10d] %s", stat.Size(), dirNameText)
		}
	}

	dirTree = gotree.New(dirNameText)

	files, err := client.Connect.ReadDir(dir)
	if err != nil {
		return gotree.New(dirNameText)
	}

	for _, info := range files {
		fullPath := filepath.Join(dir, info.Name())

		if info.IsDir() {
			dirTree.AddTree(r.buildRemoteDirNode(client, fullPath, options))
			continue
		}

		fileNameText := info.Name()
		switch {
		case options.Bool("h"):
			size := humanize.Bytes(uint64(info.Size()))
			fileNameText = fmt.Sprintf("[%10s] %s", size, fileNameText)
		case options.Bool("s"):
			fileNameText = fmt.Sprintf("[%10d] %s", info.Size(), fileNameText)
		}

		if !strings.Contains(fileNameText, "\n") {
			dirTree.Add(fileNameText)
		}
	}

	return
}
