// Copyright (c) 2021 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

// This file describes the code of the built-in command used by lsftp.
// It is quite big in that relationship. Maybe it will be separated or repaired soon.

package sftp

import (
	"fmt"
	"os"
	"path/filepath"
)

// pwd
func (r *RunSftp) pwd(args []string) {
	exit := make(chan bool)

	go func() {
		for server, client := range r.Client {
			// get writer
			client.Output.Create(server)
			w := client.Output.NewWriter()

			// get current directory
			pwd, _ := client.Connect.Getwd()
			if len(client.Pwd) != 0 {
				if filepath.IsAbs(client.Pwd) {
					pwd = client.Pwd
				} else {
					pwd = filepath.Join(pwd, client.Pwd)
				}

			}

			fmt.Fprintf(w, "%s\n", pwd)

			exit <- true
		}
	}()

	for i := 0; i < len(r.Client); i++ {
		<-exit
	}

	return
}

// lpwd
func (r *RunSftp) lpwd(args []string) {
	pwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}

	fmt.Println(pwd)
}
