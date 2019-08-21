// Copyright (c) 2019 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package ssh

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	scplib "github.com/blacknon/go-scplib"
	"github.com/blacknon/go-sshlib"
	"github.com/blacknon/lssh/conf"
	"golang.org/x/crypto/ssh"
)

type RunScp struct {
	From       CopyConInfo
	To         CopyConInfo
	CopyData   *bytes.Buffer
	Permission bool
	Config     conf.Config
}

// TODO(blacknon): scp時のプログレスバーの表示について検討する(リモートについては、リモートで実行しているscpコマンドの出力をそのまま出力すればいけそうな気がする)
// TODO(blacknon): Reader/Writerでの処理に切り替えたほうが良さそう

// Start scp, switching process.
func (r *RunScp) Start() {
	// Create AuthMap
	slist := append(r.To.Server, r.From.Server...)
	run := new(Run)
	run.ServerList = slist
	run.Conf = r.Config
	run.createAuthMethodMap()
	authMap := run.serverAuthMethodMap

	switch {
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

// Run execute scp according to mode.
func (r *RunScp) run(mode string, authMap map[string][]ssh.AuthMethod) {
	finished := make(chan bool)

	// set target list
	targetList := []string{}
	switch mode {
	case "push":
		targetList = r.To.Server
	case "pull":
		targetList = r.From.Server
	}

	for _, value := range targetList {
		target := value

		go func() {
			// config
			config := r.Config.Server[target]

			// create ssh connect
			con := new(sshlib.Connect)
			addr := config.Addr
			port := config.Port
			user := config.User
			err := con.CreateClient(addr, port, user, authMap[target])
			if err != nil {
				fmt.Fprintf(os.Stderr, "cannot connect %v, %v \n", target, err)
				finished <- true
				return
			}

			// create ssh session
			session, err := con.CreateSession()
			if err != nil {
				fmt.Fprintf(os.Stderr, "cannot connect %v, %v \n", target, err)
				finished <- true
				return
			}
			defer session.Close()

			// create scp client
			scp := new(scplib.SCPClient)
			scp.Permission = r.Permission
			scp.Session = session

			switch mode {
			case "push":
				r.push(target, scp)
			case "pull":
				r.pull(target, scp)
			}

			fmt.Fprintf(os.Stderr, "%v(%v) is finished.\n", target, mode)
			finished <- true
		}()
	}

	for i := 1; i <= len(targetList); i++ {
		<-finished
	}
}

// push file scp
// TODO(blacknon): targetいらない気がする…
func (r *RunScp) push(target string, scp *scplib.SCPClient) {
	var err error
	if r.From.IsRemote && r.To.IsRemote {
		err = scp.PutData(r.CopyData, r.To.Path[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to run %v \n", err)
		}
	} else {
		err = scp.PutFile(r.From.Path, r.To.Path[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to run %v \n", err)
		}
	}
}

// pull file scp
func (r *RunScp) pull(target string, scp *scplib.SCPClient) {
	var err error

	// scp pull
	if r.From.IsRemote && r.To.IsRemote {
		r.CopyData, err = scp.GetData(r.From.Path)
	} else {
		toPath := createServersDir(target, r.From.Server, r.To.Path[0])
		err = scp.GetFile(r.From.Path, toPath)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to run %v \n", err.Error())
	}
}

func createServersDir(target string, serverList []string, toPath string) (path string) {
	if len(serverList) > 1 {
		toDir := filepath.Dir(toPath)
		toBase := filepath.Base(toPath)
		serverDir := toDir + "/" + target

		err := os.Mkdir(serverDir, os.FileMode(uint32(0755)))
		if err != nil {
			fmt.Fprintln(os.Stderr, "Failed to run: "+err.Error())
		}

		if toDir != toBase {
			toPath = serverDir + "/" + toBase
		} else {
			toPath = serverDir + "/"
		}
	}

	path = toPath
	return
}
