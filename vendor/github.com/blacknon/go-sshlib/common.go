// Copyright (c) 2026 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package sshlib

import (
	"fmt"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	terminal "golang.org/x/term"
)

// getAbsPath return absolute path convert.
// Replace `~` with your home directory.
func getAbsPath(path string) string {
	// Replace a leading home-directory marker only.
	// On Windows, paths may legitimately contain `~` as part of an 8.3 short name
	// such as `RUNNERC~1`, so replacing arbitrary `~` characters corrupts the path.
	if path == "~" || strings.HasPrefix(path, "~"+string(filepath.Separator)) || strings.HasPrefix(path, "~/") || strings.HasPrefix(path, "~\\") {
		usr, _ := user.Current()
		if usr != nil {
			switch {
			case path == "~":
				path = usr.HomeDir
			case strings.HasPrefix(path, "~/"), strings.HasPrefix(path, "~\\"):
				path = filepath.Join(usr.HomeDir, path[2:])
			default:
				path = usr.HomeDir + path[1:]
			}
		}
	}

	path, _ = filepath.Abs(path)
	return path
}

// getPassphrase gets the passphrase from virtual terminal input and returns the result. Works only on UNIX-based OS.
func getPassphrase(msg string) (input string, err error) {
	fmt.Fprint(os.Stderr, msg)

	// Open /dev/tty
	tty, err := os.Open("/dev/tty")
	if err != nil {
		log.Fatal(err)
	}
	defer tty.Close()

	// get input
	result, err := terminal.ReadPassword(int(tty.Fd()))

	if len(result) == 0 {
		err = fmt.Errorf("err: input is empty")
		return
	}

	input = string(result)
	fmt.Println()
	return
}
