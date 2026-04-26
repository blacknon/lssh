// Copyright (c) 2022 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package ssh

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/blacknon/go-sshlib"
	"github.com/blacknon/lssh/internal/output"
)

var cmdOPROMPT = "${SERVER} :: "

// cmd is run command.
func (r *Run) cmd() (err error) {
	// command
	command := strings.Join(r.ExecCmd, " ")

	// create connect map
	connmap := map[string]*sshlib.Connect{}
	connectorServers := map[string]bool{}

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

			if r.usesConnector(server) {
				mu.Lock()
				connectorServers[server] = true
				mu.Unlock()
				return
			}

			// check count AuthMethod
			if len(r.serverAuthMethodMap[server]) == 0 {
				fmt.Fprintf(os.Stderr, "Error: %s is No AuthMethod.\n", server)
				return
			}

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

			// OverWrite nfs dynamic forwarding
			if r.NFSDynamicForwardPort != "" {
				config.NFSDynamicForwardPort = r.NFSDynamicForwardPort
			}

			// OverWrite nfs dynamic path
			if r.NFSDynamicForwardPath != "" {
				config.NFSDynamicForwardPath = r.NFSDynamicForwardPath
			}
			if r.SMBDynamicForwardPort != "" {
				config.SMBDynamicForwardPort = r.SMBDynamicForwardPort
			}
			if r.SMBDynamicForwardPath != "" {
				config.SMBDynamicForwardPath = r.SMBDynamicForwardPath
			}

			// OverWrite nfs reverse dynamic forwarding
			if r.NFSReverseDynamicForwardPort != "" {
				config.NFSReverseDynamicForwardPort = r.NFSReverseDynamicForwardPort
			}

			// OverWrite nfs reverse dynamic path
			if r.NFSReverseDynamicForwardPath != "" {
				config.NFSReverseDynamicForwardPath = r.NFSReverseDynamicForwardPath
			}
			if r.SMBReverseDynamicForwardPort != "" {
				config.SMBReverseDynamicForwardPort = r.SMBReverseDynamicForwardPort
			}
			if r.SMBReverseDynamicForwardPath != "" {
				config.SMBReverseDynamicForwardPath = r.SMBReverseDynamicForwardPath
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
				err = r.startPortForward(c, fw)
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
			if config.NFSDynamicForwardPort != "" && config.NFSDynamicForwardPath != "" {
				r.printNFSDynamicForward(config.NFSDynamicForwardPort, config.NFSDynamicForwardPath)
				go c.NFSForward("localhost", config.NFSDynamicForwardPort, config.NFSDynamicForwardPath)
			}

			// NFS Reverse Dynamic Forward
			if config.NFSReverseDynamicForwardPort != "" && config.NFSReverseDynamicForwardPath != "" {
				r.printNFSReverseDynamicForward(config.NFSReverseDynamicForwardPort, config.NFSReverseDynamicForwardPath)
				go c.NFSReverseForward("localhost", config.NFSReverseDynamicForwardPort, config.NFSReverseDynamicForwardPath)
			}

			if config.SMBDynamicForwardPort != "" && config.SMBDynamicForwardPath != "" {
				r.printSMBDynamicForward(config.SMBDynamicForwardPort, config.SMBDynamicForwardPath)
				go c.SMBForward("localhost", config.SMBDynamicForwardPort, "", config.SMBDynamicForwardPath)
			}

			if config.SMBReverseDynamicForwardPort != "" && config.SMBReverseDynamicForwardPath != "" {
				r.printSMBReverseDynamicForward(config.SMBReverseDynamicForwardPort, config.SMBReverseDynamicForwardPath)
				go c.SMBReverseForward("localhost", config.SMBReverseDynamicForwardPort, "", config.SMBReverseDynamicForwardPath)
			}

			// if tty
			if r.IsTerm {
				c.Stdin = os.Stdin
				c.Stdout = os.Stdout
				c.Stderr = os.Stderr
			}
		} else {
			if r.IsParallel {
				// For parallel mode, prepare writers to send stdin to each host.
				// - For non-control clients: use session.StdinPipe()
				// - For control clients: create an io.Pipe(), set read-side to c.Stdin
				//   so Command() will read from it, and append write-side to writers.
				if c.Session != nil {
					w, _ := c.Session.StdinPipe()
					writers = append(writers, w)
				} else if c.IsControlClient() {
					pr, pw := io.Pipe()
					c.Stdin = pr
					writers = append(writers, pw)
				}
			}
		}
	}

	connectorFinished := 0
	for _, server := range r.ServerList {
		if !connectorServers[server] {
			continue
		}
		connectorFinished++

		stdoutWriter, stderrWriter := connectorOutputWriters(r, server, len(r.ServerList) == 1)
		if r.IsParallel {
			go func(server string, stdoutWriter, stderrWriter io.Writer) {
				if _, runErr := r.runConnectorCommand(server, stdoutWriter, stderrWriter); runErr != nil {
					fmt.Fprintln(os.Stderr, connectorErrorString(server, runErr))
				}
				finished <- true
			}(server, stdoutWriter, stderrWriter)
			continue
		}

		if _, runErr := r.runConnectorCommand(server, stdoutWriter, stderrWriter); runErr != nil {
			fmt.Fprintln(os.Stderr, connectorErrorString(server, runErr))
		}
		go func() { finished <- true }()
	}

	// if parallel flag true, and select server is not single,
	// set send stdin.

	// If started as daemonized child, notify parent that forwarding is ready
	notifyParentReady()

	var stdinData []byte
	var stdinBuffer bytes.Buffer
	stdinCaptured := false
	switch {
	case r.IsParallel && len(r.ServerList) > 1:
		go output.PushInput(exitInput, writers, os.Stdin)
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
			if !stdinCaptured && r.IsStdinPipe && len(r.ServerList) > 1 {
				stdinCaptured = true
				reader := io.TeeReader(os.Stdin, &stdinBuffer)

				// If control client, set conn.Stdin so runControlCommand reads it.
				if conn.IsControlClient() {
					conn.Stdin = reader

					// run command (synchronous)
					conn.Command(command)
					finished <- true
				} else {
					// get stdin via session pipe for non-control client
					w, _ := conn.Session.StdinPipe()

					// run command
					go func() {
						conn.Command(command)
						finished <- true
					}()

					// send stdin while caching it for subsequent hosts
					io.Copy(w, reader)
					w.Close()
				}

				stdinData = stdinBuffer.Bytes()
			} else if len(stdinData) > 0 {
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
	for i := 0; i < len(connmap)+connectorFinished; i++ {
		<-finished
	}

	close(exitInput)

	// sleep
	time.Sleep(300 * time.Millisecond)

	return
}

func (r *Run) createCommandOutput(server string) *output.Output {
	o := &output.Output{
		Templete:      cmdOPROMPT,
		Count:         0,
		ServerList:    r.ServerList,
		Conf:          r.Conf.Server[server],
		EnableHeader:  r.EnableHeader,
		DisableHeader: r.DisableHeader,
		AutoColor:     true,
	}
	o.Create(server)
	return o
}
