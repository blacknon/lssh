// Copyright (c) 2021 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package sftp

import (
	"fmt"
	"os"
	"sync"

	"github.com/blacknon/lssh/common"
	"github.com/blacknon/lssh/conf"
	"github.com/blacknon/lssh/output"
	sshl "github.com/blacknon/lssh/ssh"
	"github.com/c-bata/go-prompt"
	"github.com/pkg/sftp"
	"github.com/vbauerster/mpb"
)

// TODO(blacknon): Ctrl + Cでコマンドの処理をキャンセルできるようにする

// RunSftp struct sftp run
type RunSftp struct {
	// select server
	SelectServer []string

	// config
	Config conf.Config

	// Client
	Client map[string]*SftpConnect

	// Complete select client
	TargetClient map[string]*SftpConnect

	// ssh Run
	Run *sshl.Run

	//
	Permission bool

	// progress bar
	Progress   *mpb.Progress
	ProgressWG *sync.WaitGroup

	// PathComplete
	RemoteComplete []prompt.Suggest
	LocalComplete  []prompt.Suggest
}

// SftpConnect struct at sftp client
type SftpConnect struct {
	// ssh connect
	Connect *sftp.Client

	// Output
	Output *output.Output

	// Current Directory
	Pwd string
}

type TargetConnectMap struct {
	SftpConnect

	// Target Path list
	Path []string
}

// PathSet struct at path data
type PathSet struct {
	Base      string
	PathSlice []string
}

var (
	oprompt = "${SERVER} :: "
)

// Start sftp shell
func (r *RunSftp) Start() {
	// Create AuthMap
	r.Run = new(sshl.Run)
	r.Run.ServerList = r.SelectServer
	r.Run.Conf = r.Config
	r.Run.CreateAuthMethodMap()

	// Create Sftp Connect
	r.Client = r.createSftpConnect(r.Run.ServerList)

	// Start sftp shell
	r.shell()
}

//
func (r *RunSftp) createSftpConnect(targets []string) (result map[string]*SftpConnect) {
	// init
	result = map[string]*SftpConnect{}

	ch := make(chan bool)
	m := new(sync.Mutex)
	for _, target := range targets {
		server := target
		go func() {
			// ssh connect
			conn, err := r.Run.CreateSshConnect(server)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s connect error: %s\n", server, err)
				ch <- true
				return
			}

			// create sftp client
			ftp, err := sftp.NewClient(conn.Client)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s create client error: %s\n", server, err)
				ch <- true
				return
			}

			// create output
			o := &output.Output{
				Templete:   oprompt,
				ServerList: targets,
				Conf:       r.Config.Server[server],
				AutoColor:  true,
			}

			// create SftpConnect
			sftpCon := &SftpConnect{
				Connect: ftp,
				Output:  o,
				Pwd:     "./",
			}

			// append result
			m.Lock()
			result[server] = sftpCon
			m.Unlock()

			ch <- true
		}()
	}

	// wait
	for i := 0; i < len(targets); i++ {
		<-ch
	}

	return result
}

//
func (r *RunSftp) createTargetMap(srcTargetMap map[string]*TargetConnectMap, pathline string) (targetMap map[string]*TargetConnectMap) {
	// sftp target host
	targetMap = srcTargetMap

	// get r.Client keys
	servers := make([]string, 0, len(r.Client))
	for k := range r.Client {
		servers = append(servers, k)
	}

	// parse pathline
	targetList, path := common.ParseHostPath(pathline)

	if len(targetList) == 0 {
		targetList = servers
	}

	// check exist server.
	for _, t := range targetList {
		if !common.Contains(servers, t) {
			fmt.Fprintf(os.Stderr, "Error: host %s not found.\n", t)
			continue
		}
	}

	// create targetMap
	for server, client := range r.Client {
		if common.Contains(targetList, server) {
			if _, ok := targetMap[server]; !ok {
				targetMap[server] = &TargetConnectMap{}
				targetMap[server].SftpConnect = *client
			}

			// append path
			targetMap[server].Path = append(targetMap[server].Path, path)
		}
	}

	return targetMap
}
