package ssh

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/blacknon/go-sshlib"
)

var cmdOPROMPT = "${SERVER} :: "

func (r *Run) cmd() (err error) {
	// command
	command := strings.Join(r.ExecCmd, " ")

	// connect map
	connmap := map[string]*sshlib.Connect{}

	// make channel
	finished := make(chan bool)
	input := make(chan io.Writer)
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
		if len(r.stdinData) > 0 {
			conn.Stdin = r.stdinData
		}

		connmap[server] = conn
	}

	// Run command and print loop
	for s, c := range connmap {
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

		// create channel
		output := make(chan []byte)

		// Overwrite port forwarding.
		// Valid only when one server is specified
		if len(r.ServerList) == 1 {
			// if select server is single, Force a value such as.
			//     session.Stdout = os.Stdout
			//     session.Stderr = os.Stderr
			c.ForceStd = true

			// overwrite port forward address
			if r.PortForwardLocal != "" && r.PortForwardRemote != "" {
				config.PortForwardLocal = r.PortForwardLocal
				config.PortForwardRemote = r.PortForwardRemote
			}

			// print header
			r.printPortForward(config.PortForwardLocal, config.PortForwardRemote)

			// local port forwarding
			c.TCPLocalForward(config.PortForwardLocal, config.PortForwardRemote)
		}

		// run command
		// if parallel flag true, and select server is not single,
		// os.Stdin to multiple server.
		go func() {
			if r.IsParallel && len(r.ServerList) > 1 {
				c.CmdWriter(command, output, input)
			} else {
				c.Cmd(command, output)
			}
			close(output)
			finished <- true
		}()

		if r.IsParallel {
			go printOutput(o, output)
		} else {
			printOutput(o, output)
		}

	}

	// if parallel flag true, and select server is not single,
	// create io.MultiWriter and send input.
	if r.IsParallel && len(r.ServerList) > 1 {
		writers := []io.Writer{}
		for i := 0; i < len(r.ServerList); i++ {
			w := <-input
			writers = append(writers, w)
		}
		writer := io.MultiWriter(writers...)
		go pushInput(exitInput, writer)
	}

	for i := 0; i < len(connmap); i++ {
		<-finished
	}

	close(exitInput)
	return
}
