package ssh

import (
	"fmt"
	"io"
	"net"
	"os"
)

// @brief:
func (c *Connect) forward(localConn net.Conn) {
	// Create ssh connect
	sshConn, err := c.Client.Dial("tcp", c.ForwardRemote)

	// Copy localConn.Reader to sshConn.Writer
	go func() {
		_, err = io.Copy(sshConn, localConn)
		if err != nil {
			fmt.Println("Port forward local to remote failed: %v\n", err)
		}
	}()

	// Copy sshConn.Reader to localConn.Writer
	go func() {
		_, err = io.Copy(localConn, sshConn)
		if err != nil {
			fmt.Println("Port forward remote to local failed: %v\n", err)
		}
	}()
}

// @brief:
func (c *Connect) PortForwarder() {
	// Open local port.
	localListener, err := net.Listen("tcp", c.ForwardLocal)

	if err != nil {
		// error local port open.
		fmt.Fprintf(os.Stdout, "local port listen failed: %v\n", err)
	} else {
		// start port forwarding.
		go func() {
			for {
				// Setup localConn (type net.Conn)
				localConn, err := localListener.Accept()
				if err != nil {
					fmt.Println("listen.Accept failed: %v\n", err)
				}
				go c.forward(localConn)
			}
		}()
	}
}
