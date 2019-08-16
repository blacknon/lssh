// Copyright (c) 2019 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package ssh

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/blacknon/go-sshlib"
	"golang.org/x/crypto/ssh"
)

var cmdOPROMPT = "${SERVER} :: "

// cmd
// TODO(blacknon): リファクタリング(v0.6.0)
// TODO(blacknon): ダブルクォーテーションなどで囲った文字列を渡した際、単純に結合するだけだとうまく行かないので、ちゃんと囲み直してあげる必要がある
func (r *Run) cmd() (err error) {
	// command
	command := strings.Join(r.ExecCmd, " ")

	// connect map
	connmap := map[string]*sshlib.Connect{}

	// make channel
	finished := make(chan bool)
	exitInput := make(chan bool)

	// print header
	r.printSelectServer()
	r.printRunCommand()
	if len(r.ServerList) == 1 {
		r.printProxy(r.ServerList[0])
	}

	// Create sshlib.Connect loop
	for _, server := range r.ServerList {
		// check count AuthMethod
		if len(r.serverAuthMethodMap[server]) == 0 {
			fmt.Fprintf(os.Stderr, "Error: %s is No AuthMethod.\n", server)
			continue
		}

		// Create sshlib.Connect
		conn, err := r.createSshConnect(server)
		if err != nil {
			log.Printf("Error: %s:%s\n", server, err)
			continue
		}

		connmap[server] = conn
	}

	// Run command and print loop
	writers := []io.WriteCloser{}
	sessions := []*ssh.Session{}
	for s, c := range connmap {
		session, _ := c.CreateSession()

		// Get server config
		config := r.Conf.Server[s]

		// create Output
		o := &Output{
			Templete:   cmdOPROMPT,
			Count:      0,
			ServerList: r.ServerList,
			Conf:       r.Conf.Server[s],
			AutoColor:  true,
		}
		o.Create(s)

		// if single server
		if len(r.ServerList) == 1 {
			session.Stdout = os.Stdout
			session.Stderr = os.Stderr
			session.Stdin = os.Stdin
			// switch {
			// case r.IsTerm && len(r.stdinData) == 0:

			// case len(r.stdinData) > 0:
			// 	session.Stdin = bytes.NewReader(r.stdinData)
			// }

			// OverWrite port forward mode
			if r.PortForwardMode != "" {
				config.PortForwardMode = r.PortForwardMode
			}

			// Overwrite port forward address
			if r.PortForwardLocal != "" && r.PortForwardRemote != "" {
				config.PortForwardLocal = r.PortForwardLocal
				config.PortForwardRemote = r.PortForwardRemote
			}

			// print header
			r.printPortForward(config.PortForwardMode, config.PortForwardLocal, config.PortForwardRemote)

			// Port Forwarding
			switch config.PortForwardMode {
			case "L", "":
				c.TCPLocalForward(config.PortForwardLocal, config.PortForwardRemote)
			case "R":
				c.TCPRemoteForward(config.PortForwardLocal, config.PortForwardRemote)
			}

			// Dynamic Port Forwarding
			if config.DynamicPortForward != "" {
				r.printDynamicPortForward(config.DynamicPortForward)
				go c.TCPDynamicForward("localhost", config.DynamicPortForward)
			}
		} else {
			session.Stdout = o.NewWriter()
			session.Stderr = o.NewWriter()
			w, _ := session.StdinPipe()
			writers = append(writers, w)
		}
		sessions = append(sessions, session)
	}

	// if parallel flag true, and select server is not single,
	// set send stdin.
	switch {
	case r.IsParallel && len(r.ServerList) > 1:
		if r.isStdinPipe {
			go pushPipeWriter(exitInput, writers, os.Stdin)
		} else {
			go pushInput(exitInput, writers)
		}
	}

	// run command
	for _, s := range sessions {
		session := s
		if r.IsParallel {
			go func() {
				session.Run(command)
				finished <- true
			}()
		} else {
			session.Run(command)
			go func() { finished <- true }()
		}
	}

	for i := 0; i < len(connmap); i++ {
		<-finished
	}

	close(exitInput)

	return
}
