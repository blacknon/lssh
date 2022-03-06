// Copyright (c) 2021 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

// This file describes the code of the built-in command used by lsftp.

package sftp

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
)

// NOTE: カレントディレクトリの移動の仕組みを別途作成すること(保持する仕組みがないので)
// cd change remote machine current directory
func (r *RunSftp) cd(args []string) {
	path := "./"
	// cd command only
	if len(args) == 1 {
		// set pwd
		for _, c := range r.Client {
			c.Pwd = path
		}

		return
	}

	targetmap := map[string]*TargetConnectMap{}
	targetmap = r.createTargetMap(targetmap, args[1])

	// check directory
	var okcounter int
	for server, client := range targetmap {
		// get output
		client.Output.Create(server)
		w := client.Output.NewWriter()

		// set arg path
		path = client.Path[0]
		var err error
		if !filepath.IsAbs(path) {
			path = filepath.Join(client.Pwd, path)
		}

		// get symlink
		p, err := client.Connect.ReadLink(path)
		if err == nil {
			path = p
		}

		// get stat
		stat, err := client.Connect.Lstat(path)
		if err != nil {
			fmt.Fprintf(w, "Error: %s\n", err)
			continue
		}

		if !stat.IsDir() {
			fmt.Fprintf(w, "Error: %s\n", "is not directory")
			continue
		}

		// set pwd
		r.Client[server].Pwd = path

		// add count
		okcounter++
	}

	// check count okcounter
	if okcounter != len(targetmap) {
		return
	}

	return
}

// lcd
func (r *RunSftp) lcd(args []string) {
	// get user home directory path
	usr, _ := user.Current()

	path := usr.HomeDir
	if len(args) > 1 {
		path = args[1]
	}

	err := os.Chdir(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}
