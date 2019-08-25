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
	// From       CopyConInfo
	// To         CopyConInfo
	Permission bool
	Config     conf.Config
	IsShell    bool
}

// TODO(blacknon): 転送時の進捗状況を表示するプログレスバーの表示はさせること

func (r *RunSftp) Start() {
	// // Create AuthMap
	// slist := append(r.To.Server, r.From.Server...)
	// run := new(Run)
	// run.ServerList = slist
	// run.Conf = r.Config
	// run.createAuthMethodMap()
	// authMap := run.serverAuthMethodMap

	// // Create Connection

	// switch {
	// case r.IsShell:
	// 	r.shell(authMap)

	// // remote to remote
	// case r.From.IsRemote && r.To.IsRemote:
	// 	r.copy("pull", authMap)
	// 	r.copy("push", authMap)

	// // remote to local
	// case r.From.IsRemote && !r.To.IsRemote:
	// 	r.copy("pull", authMap)

	// // local to remote
	// case !r.From.IsRemote && r.To.IsRemote:
	// 	r.copy("push", authMap)
	// }
}

// sftp Shell mode function
func (r *RunSftp) shell(authMap map[string][]ssh.AuthMethod) {

}

// sftp put/pull function
func (r *RunSftp) copy(mode string, authMap map[string][]ssh.AuthMethod) {
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

// list is stfp ls command.
func (r *RunSftp) list(target, path string, sftp *sftp.Client) (err error) {
	// // get directory files
	// data, err := sftp.ReadDir(path)
	// if err != nil {
	// 	return
	// }

	// // TODO(blacknon): lsのオプション相当のものを受け付けるようにする必要がある。
	// //                 それとも、単にdataを返すだけになるかも？？
	// for _, f := range data {
	// 	name := f.Name()
	// 	fmt.Println(name)
	// }
	return
}

//
func (r *RunSftp) push(target, path string, sftp *sftp.Client) {
	// pathがディレクトリかどうかのチェックが必要
	// remoteFile, err := sftp.Create("hello.txt")
	// localFile, err := os.Open("hello.txt")
	// io.Copy(remoteFile, localFile)

	// f, err := sftp.Create("hello.txt")
	// TODO(blacknon): io.Copy使うとよさそう？？
}

//
func (r *RunSftp) pull(target, path string, sftp *sftp.Client) {
	// pathがディレクトリかどうかのチェックが必要
	// f, err := sftp.Open(path)
	// TODO(blacknon): io.Copy使うとよさそう？？
}
