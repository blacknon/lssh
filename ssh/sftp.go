// Copyright (c) 2019 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package ssh

import (
	"github.com/blacknon/lssh/conf"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type RunSftp struct {
	From       CopyConInfo
	To         CopyConInfo
	Permission bool
	Config     conf.Config
	IsShell    bool
}

func (r *RunSftp) Start() {
	// Create AuthMap
	slist := append(r.To.Server, r.From.Server...)
	run := new(Run)
	run.ServerList = slist
	run.Conf = r.Config
	run.createAuthMethodMap()
	authMap := run.serverAuthMethodMap

	// Create Connection

	switch {
	case r.IsShell:
		r.shell(authMap)

	// remote to remote
	case r.From.IsRemote && r.To.IsRemote:
		r.run("pull", authMap)
		r.run("push", authMap)

	// remote to local
	case r.From.IsRemote && !r.To.IsRemote:
		r.run("pull", authMap)

	// local to remote
	case !r.From.IsRemote && r.To.IsRemote:
		r.run("push", authMap)
	}
}

// sftp Shell mode function
func (r *RunSftp) shell(authMap map[string][]ssh.AuthMethod) {

}

// sftp put/pull function
func (r *RunSftp) run(mode string, authMap map[string][]ssh.AuthMethod) {
	// finished := make(chan bool)

	// // set target list
	// targetList := []string{}
	// switch mode {
	// case "push":
	// 	targetList = r.To.Server
	// case "pull":
	// 	targetList = r.From.Server
	// }

	// for _, value := range targetList {
	// 	target := value
	// }
}

func (r *RunSftp) list(target, path string, sftp *sftp.Client) {
	walker := sftp.Walk(path)

}

func (r *RunSftp) push(target, path string, sftp *sftp.Client) {

}

func (r *RunSftp) pull(target, path string, sftp *sftp.Client) {

}
