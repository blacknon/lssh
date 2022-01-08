// Copyright (c) 2021 Blacknon. All rights reserved.
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

	"golang.org/x/crypto/ssh/terminal"
)

// getAbsPath return absolute path convert.
// Replace `~` with your home directory.
func getAbsPath(path string) string {
	// Replace home directory
	usr, _ := user.Current()
	path = strings.Replace(path, "~", usr.HomeDir, 1)

	path, _ = filepath.Abs(path)
	return path
}

// getPassphrase gets the passphrase from virtual terminal input and returns the result. Works only on UNIX-based OS.
func getPassphrase(msg string) (input string, err error) {
	fmt.Fprintf(os.Stderr, msg)

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
