// Copyright (c) 2019 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package ssh

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"

	"github.com/blacknon/go-sshlib"
	"github.com/blacknon/lssh/output"
)

var cmdOPROMPT = "${SERVER} :: "

// cmd is run command.
func (r *Run) cmd() (err error) {
	// command
	command := strings.Join(r.ExecCmd, " ")

	// create connect map
	connmap := map[string]*sshlib.Connect{}

	// make channel
	finished := make(chan bool)
	exitInput := make(chan bool)

	// print header
	r.PrintSelectServer()
	r.printRunCommand()
	if len(r.ServerList) == 1 {
		r.printProxy(r.ServerList[0])
	}

	// Create sshlib.Connect to connmap
	for _, server := range r.ServerList {
		// check count AuthMethod
		if len(r.serverAuthMethodMap[server]) == 0 {
			fmt.Fprintf(os.Stderr, "Error: %s is No AuthMethod.\n", server)
			continue
		}

		// Create sshlib.Connect
		conn, err := r.CreateSshConnect(server)
		if err != nil {
			log.Printf("Error: %s:%s\n", server, err)
			continue
		}

		connmap[server] = conn
	}

	// Run command and print loop
	writers := []io.WriteCloser{}
	for s, c := range connmap {
		// set session
		c.Session, _ = c.CreateSession()

		// Get server config
		config := r.Conf.Server[s]

		// create Output
		o := &output.Output{
			Templete:      cmdOPROMPT,
			Count:         0,
			ServerList:    r.ServerList,
			Conf:          r.Conf.Server[s],
			EnableHeader:  r.EnableHeader,
			DisableHeader: r.DisableHeader,
			AutoColor:     true,
		}
		o.Create(s)

		// set output
		c.Stdout = o.NewWriter()
		c.Stderr = o.NewWriter()

		// if single server, setup port forwarding.
		if len(r.ServerList) == 1 {
			// set port forwarding
			config = r.setPortForwards(s, config)

			// print header
			for _, fw := range config.Forwards {
				r.printPortForward(fw.Mode, fw.Local, fw.Remote)
			}

			// Port Forwarding
			for _, fw := range config.Forwards {
				// port forwarding
				switch fw.Mode {
				case "L", "":
					err = c.TCPLocalForward(fw.Local, fw.Remote)
				case "R":
					err = c.TCPRemoteForward(fw.Local, fw.Remote)
				}

				if err != nil {
					fmt.Fprintln(os.Stderr, err)
				}
			}

			// Dynamic Port Forwarding
			if config.DynamicPortForward != "" {
				r.printDynamicPortForward(config.DynamicPortForward)
				go c.TCPDynamicForward("localhost", config.DynamicPortForward)
			}

			// if tty
			if r.IsTerm {
				c.Stdin = os.Stdin
				c.Stdout = os.Stdout
				c.Stderr = os.Stderr
			}
		} else {
			if r.IsParallel {
				w, _ := c.Session.StdinPipe()
				writers = append(writers, w)
			}
		}
	}

	// if parallel flag true, and select server is not single,
	// set send stdin.
	var stdinData []byte
	switch {
	case r.IsParallel && len(r.ServerList) > 1:
		if r.isStdinPipe {
			go output.PushPipeWriter(exitInput, writers, os.Stdin)
		} else {
			go output.PushInput(exitInput, writers, byte('\n'))
		}
	case !r.IsParallel && len(r.ServerList) > 1:
		if r.isStdinPipe {
			stdinData, _ = ioutil.ReadAll(os.Stdin)
		}
	}

	// run command
	for _, c := range connmap {
		conn := c
		if r.IsParallel {
			go func() {
				conn.Command(command)
				finished <- true
			}()
		} else {
			if len(stdinData) > 0 {
				// get stdin
				rd := bytes.NewReader(stdinData)
				w, _ := conn.Session.StdinPipe()

				// run command
				go func() {
					conn.Command(command)
					finished <- true
				}()

				// send stdin
				io.Copy(w, rd)
				w.Close()
			} else {
				// run command
				conn.Command(command)
				go func() { finished <- true }()
			}
		}
	}

	// wait
	for i := 0; i < len(connmap); i++ {
		<-finished
	}

	close(exitInput)

	// sleep
	time.Sleep(300 * time.Millisecond)

	return
}
