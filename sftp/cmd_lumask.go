// Copyright (c) 2022 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

// NOTE:
//
package sftp

import (
	"fmt"
	"regexp"
	"strings"
)

// TODO(blacknon): sftpのlumaskをそのまま実装する. 実装的には、structの変数に設定しておくだけな気がする(あとは各コマンドでそのstruct変数を読み込んでパーミッションを定義させる)

// ls exec and print out remote ls data.
func (r *RunSftp) lumask(args []string) (err error) {
	helpMessage := "You must supply a numeric argument to the lumask command.\nlumask [000-777]"

	// check args number
	switch len(args) {
	case 1:
		// printout now local umask.
		umaskString := strings.Join(r.LocalUmask, "")
		fmt.Printf("now local umask: %s.\n", umaskString)

	case 2:
		// set local umask.
		re := regexp.MustCompile(`^[0-7]{3}$`)

		if re.Match([]byte(args[1])) {
			// set umask
			r.LocalUmask = strings.Split(args[1], "")
			fmt.Printf("set local umask: %s.\n", args[1])
		} else {
			// printout error message...
			fmt.Println(helpMessage)
		}

	default:
		// if one or more arguments are specified.
		// printout error message...
		fmt.Println(helpMessage)
	}

	return nil
}
