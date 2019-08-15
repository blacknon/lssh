package ssh

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/blacknon/go-sshlib"
)

var cmdOPROMPT = "${SERVER} :: "

// cmd
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

		// stdin data check
		// if len(r.stdinData) > 0 {
		// 	conn.Stdin = r.stdinData
		// }

		connmap[server] = conn
	}

	// Run command and print loop
	writers := []io.WriteCloser{}
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
			switch {
			case r.IsTerm && len(r.stdinData) == 0:
				session.Stdin = os.Stdin
			case len(r.stdinData) > 0:
				session.Stdin = bytes.NewReader(r.stdinData)
			}

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
			w, _ := session.StdinPipe()
			writers = append(writers, w)
		}

		// run command
		go func() {
			session.Run(command)
			finished <- true
		}()
	}

	// if parallel flag true, and select server is not single,
	// set send stdin.
	if r.IsParallel && len(r.ServerList) > 1 {
		if len(r.stdinData) > 0 {
			ws := []io.Writer{}
			for _, w := range writers {
				var ww io.Writer
				ww = w
				ws = append(ws, ww)
			}

			writer := io.MultiWriter(ws...)
			go func() {
				io.Copy(writer, bytes.NewReader(r.stdinData))
				for _, w := range writers {
					w.Close()
				}
			}()
		} else {
			go pushInput(exitInput, writers)
		}
	}

	for i := 0; i < len(connmap); i++ {
		<-finished
	}

	close(exitInput)

	return
}
