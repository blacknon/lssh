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
	"github.com/dustin/go-humanize"
	"github.com/urfave/cli"
)

// ltree is local tree command
func (r *RunSftp) ltree(args []string) (err error) {
	// create app
	app := cli.NewApp()

	// set help message
	app.CustomAppHelpTemplate = helptext

	// set parameter
	app.Flags = []cli.Flag{
		cli.BoolFlag{Name: "s", Usage: "print the size in bytes of each file."},
		cli.BoolFlag{Name: "h", Usage: "print the size in a more human readable way."},
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
			// get file stat
			stat, err := os.Stat(path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s\n", err)
				return nil
			}

			// create tree
			// get dirctory tree data.
			tree := gotree.New(path)

			// check is directory
			if stat.IsDir() {
				// create directory tree
				tree = r.buildLocalDirTree(path, c)
			}

			fmt.Println(tree.Print())
		}

		return nil
	}

	// parse short options
	args = common.ParseArgs(app.Flags, args)
	app.Run(args)

	return
}

// buildLocalDirTree is create goTree object at local machine.
func (r *RunSftp) buildLocalDirTree(dir string, options *cli.Context) (dirTree gotree.Tree) {
	// add a slash at the end of dir.
	dirName := filepath.Base(dir)

	// set printout text
	dirNameText := dirName + "/"

	// size options
	// h takes precedence over s.
	switch {
	case options.Bool("h"):
		stat, _ := os.Stat(dir)
		size := humanize.Bytes(uint64(stat.Size()))
		dirNameText = fmt.Sprintf("[%10s] %s", size, dirNameText)
	case options.Bool("s"):
		stat, _ := os.Stat(dir)
		dirNameText = fmt.Sprintf("[%10d] %s", stat.Size(), dirNameText)
	}

	// create dirTree
	dirTree = gotree.New(dirNameText)

	// Check the path directly under the directory.
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		// if directory, step into and build tree
		if info.IsDir() && dirName != info.Name() {
			dirTree.AddTree(r.buildLocalDirTree(path, options))
			return filepath.SkipDir
		}

		// only add nodes to tree with the same depth
		if len(strings.Split(dir, "/"))+1 == len(strings.Split(path, "/")) &&
			info.Name() != dirName &&
			!info.IsDir() {
			// set printout text
			fileNameText := info.Name()

			// size options
			// h takes precedence over s.
			switch {
			case options.Bool("h"):
				size := humanize.Bytes(uint64(info.Size()))
				fileNameText = fmt.Sprintf("[%10s] %s", size, fileNameText)

			case options.Bool("s"):
				fileNameText = fmt.Sprintf("[%10d] %s", info.Size(), fileNameText)
			}

			dirTree.Add(fileNameText)
		}

		return nil
	})

	if err != nil {
		return nil
	}

	return
}
