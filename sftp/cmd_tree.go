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
		cli.BoolFlag{Name: "s", Usage: "list one file per line"},
		cli.BoolFlag{Name: "h", Usage: "do not ignore entries starting with"},
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

		pathList := []string{"./"}

		if len(argData) > 0 {
			pathList = []string{}
			for _, arg := range argData {
				// sftp target host
				pathList = append(pathList, arg)
			}
		}

		for _, path := range pathList {
			// get dirctory tree data.
			dirTree, err := buildDirTree(nil, path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s\n", err)
				return nil
			}

			fmt.Println(dirTree.Print())
		}

		return nil
	}

	// parse short options
	args = common.ParseArgs(app.Flags, args)
	app.Run(args)

	return
}

// ltree is local tree command
func (r *RunSftp) ltree(args []string) (err error) {
	// create app
	app := cli.NewApp()

	// set help message
	app.CustomAppHelpTemplate = helptext

	// set parameter
	app.Flags = []cli.Flag{
		cli.BoolFlag{Name: "s", Usage: "list one file per line"},
		cli.BoolFlag{Name: "h", Usage: "do not ignore entries starting with"},
	}
	app.Name = "ltree"
	app.Usage = "lsftp build-in command: ltree [local machine tree]"
	app.ArgsUsage = "[PATH]..."
	app.HideHelp = true
	app.HideVersion = true
	app.EnableBashCompletion = true

	// action
	app.Action = func(c *cli.Context) error {
		// argpath
		argData := c.Args()

		pathList := []string{"./"}

		if len(argData) > 0 {
			pathList = []string{}
			for _, arg := range argData {
				// sftp target host
				pathList = append(pathList, arg)
			}
		}

		for _, path := range pathList {
			// get dirctory tree data.
			dirTree, err := buildDirTree(nil, path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s\n", err)
				return nil
			}

			fmt.Println(dirTree.Print())
		}

		return nil
	}

	// parse short options
	args = common.ParseArgs(app.Flags, args)
	app.Run(args)

	return
}

//
func buildDirTree(client *TargetConnectMap, path string) (tree gotree.Tree, err error) {
	if client == nil {
		// is localhost
		// get file stat
		stat, err := os.Stat(path)
		if err != nil {
			return tree, err
		}

		// create tree
		tree = gotree.New(path)

		// check is directory
		if stat.IsDir() {
			// create directory tree
			tree = buildLocalDirTree(path)
		}
	} else {
		// is remotehost

	}

	return
}

//
func buildRemoteDirTree(client *TargetConnectMap, dir string) gotree.Tree {
	return nil
}

func buildLocalDirTree(dir string) (dirTree gotree.Tree) {
	// add a slash at the end of dir.
	dirName := filepath.Base(dir)

	// create dirTree
	dirTree = gotree.New(dirName + "/")

	// Check the path directly under the directory.
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		// if directory, step into and build tree
		if info.IsDir() && dirName != info.Name() {
			dirTree.AddTree(buildLocalDirTree(path))
			return filepath.SkipDir
		}

		// only add nodes to tree with the same depth
		if len(strings.Split(dir, "/"))+1 == len(strings.Split(path, "/")) &&
			info.Name() != dirName &&
			!info.IsDir() {
			dirTree.Add(info.Name())
		}

		return nil
	})

	if err != nil {
		return nil
	}

	return
}
