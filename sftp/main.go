// Copyright (c) 2019 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package sftp

import (
	"fmt"
	"os"
	"sync"

	"github.com/blacknon/lssh/conf"
	"github.com/blacknon/lssh/output"
	sshl "github.com/blacknon/lssh/ssh"
	"github.com/c-bata/go-prompt"
	"github.com/pkg/sftp"
	"github.com/vbauerster/mpb"
)

type RunSftp struct {
	// select server
	SelectServer []string

	// config
	Config conf.Config

	// Client
	Client map[string]*SftpConnect

	// ssh Run
	Run *sshl.Run

	// now not use. delete at 0.6.1
	Permission bool

	// progress bar
	Progress   *mpb.Progress
	ProgressWG *sync.WaitGroup

	// PathComplete
	RemoteComplete []prompt.Suggest
	LocalComplete  []prompt.Suggest
}

type SftpConnect struct {
	// ssh connect
	Connect *sftp.Client

	// Output
	Output *output.Output

	// Current Directory
	Pwd string
}

type PathSet struct {
	Base      string
	PathSlice []string
}

var (
	oprompt = "${SERVER} :: "
)

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
