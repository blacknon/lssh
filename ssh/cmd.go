// Copyright (c) 2022 Blacknon. All rights reserved.
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
	"sync"
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

	// set enable stdoutMutex
	r.EnableStdoutMutex = true

	var wg sync.WaitGroup
	var mu sync.Mutex // Mutex to safely update connmap

	// Create sshlib.Connect to connmap
	// TODO: go-sshlib側でstdout用にMutexを渡して、それで重複出力を排除する感じにすればいいかも？？？
	//       → もしかしたら、これでlsshellの並列outputも処理できるかも？？？…と思ったが、parallelのときは握りっぱなしになるので、間でうまいこと処理するナニカ(writer)がいないとダメかも？
	for _, server := range r.ServerList {
		server := server // Capture range variable for goroutine
		wg.Add(1)        // Increment WaitGroup counter

		go func() {
			defer wg.Done() // Decrement counter when goroutine completes

			// check count AuthMethod
			if len(r.serverAuthMethodMap[server]) == 0 {
				fmt.Fprintf(os.Stderr, "Error: %s is No AuthMethod.\n", server)
				return
			}

			// Create sshlib.Connect
			conn, err := r.CreateSshConnect(server)
			if err != nil {
				log.Printf("Error: %s:%s\n", server, err)
				return
			}

			// centralized informational output
			r.PrintConnectInfo(server, conn, r.Conf.Server[server])

			mu.Lock()
			connmap[server] = conn
			mu.Unlock()
		}()
	}

	// Wait for all goroutines to finish
	wg.Wait()

	// Run command and print loop
	writers := []io.WriteCloser{}
	for s, c := range connmap {
		// Get server config
		config := r.Conf.Server[s]

		// Agent will be set by CreateSshConnect; ForwardAgent is configured there.

		// If this connection is NOT a control client, create a session now
		if !c.IsControlClient() {
			c.Session, _ = c.CreateSession()
		}

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

			// OverWrite dynamic port forwarding
			if r.DynamicPortForward != "" {
				config.DynamicPortForward = r.DynamicPortForward
			}

			// OverWrite reverse dynamic port forwarding
			if r.ReverseDynamicPortForward != "" {
				config.ReverseDynamicPortForward = r.ReverseDynamicPortForward
			}

			// OverWrite http dynamic port forwarding
			if r.HTTPDynamicPortForward != "" {
				config.HTTPDynamicPortForward = r.HTTPDynamicPortForward
			}

			// OverWrite reverse http dynamic port forwarding
			if r.HTTPReverseDynamicPortForward != "" {
				config.HTTPReverseDynamicPortForward = r.HTTPReverseDynamicPortForward
			}

			// OverWrite local bashrc use
			if r.IsBashrc {
				config.LocalRcUse = "yes"
			}

			// OverWrite local bashrc not use
			if r.IsNotBashrc {
				config.LocalRcUse = "no"
			}

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

			// Reverse Dynamic Port Forwarding
			if config.ReverseDynamicPortForward != "" {
				r.printReverseDynamicPortForward(config.ReverseDynamicPortForward)
				go c.TCPReverseDynamicForward("localhost", config.ReverseDynamicPortForward)
			}

			// HTTP Dynamic Port Forwarding
			if config.HTTPDynamicPortForward != "" {
				r.printHTTPDynamicPortForward(config.HTTPDynamicPortForward)
				go c.HTTPDynamicForward("localhost", config.HTTPDynamicPortForward)
			}

			// HTTP Reverse Dynamic Port Forwarding
			if config.HTTPReverseDynamicPortForward != "" {
				r.printHTTPReverseDynamicPortForward(config.HTTPReverseDynamicPortForward)
				go c.HTTPReverseDynamicForward("localhost", config.HTTPReverseDynamicPortForward)
			}

			// NFS Dynamic Forward
			if r.NFSDynamicForwardPort != "" && r.NFSDynamicForwardPath != "" {
				config.NFSDynamicForwardPort = r.NFSDynamicForwardPort
				config.NFSDynamicForwardPath = r.NFSDynamicForwardPath
				go c.NFSForward("localhost", config.NFSDynamicForwardPort, config.NFSDynamicForwardPath)
			}

			// NFS Reverse Dynamic Forward
			if r.NFSReverseDynamicForwardPort != "" && r.NFSReverseDynamicForwardPath != "" {
				config.NFSReverseDynamicForwardPort = r.NFSReverseDynamicForwardPort
				config.NFSReverseDynamicForwardPath = r.NFSReverseDynamicForwardPath
				go c.NFSReverseForward("localhost", config.NFSReverseDynamicForwardPort, config.NFSReverseDynamicForwardPath)
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

	// If started as daemonized child, notify parent that forwarding is ready
	notifyParentReady()

	var stdinData []byte
	switch {
	case r.IsParallel && len(r.ServerList) > 1:
		go output.PushInput(exitInput, writers, os.Stdin)

	case !r.IsParallel && len(r.ServerList) > 1:
		if r.IsStdinPipe {
			stdinData, _ = ioutil.ReadAll(os.Stdin)
		}
	}

	// run command
	for _, c := range connmap {
		conn := c
		if r.IsParallel {
			go func() {
				// When control client, Command handles control path internally.
				conn.Command(command)
				finished <- true
			}()
		} else {
			if len(stdinData) > 0 {
				// If control client, set conn.Stdin so runControlCommand reads it.
				if conn.IsControlClient() {
					conn.Stdin = bytes.NewReader(stdinData)

					// run command (synchronous)
					conn.Command(command)
					finished <- true
				} else {
					// get stdin via session pipe for non-control client
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
				}
			} else {
				// run command (both control and direct handled by Command)
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
