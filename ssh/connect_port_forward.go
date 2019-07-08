package ssh

import (
	"fmt"
	"io"
	"net"
	"os"
)

// TODO(blacknon): x11 forwardingの実装
// 【参考】
//       - https://ttssh2.osdn.jp/manual/ja/reference/sourcecode.html
//       - https://godoc.org/golang.org/x/crypto/ssh#Conn
//

// forward function to do port io.Copy with goroutine
func (c *Connect) forward(localConn net.Conn) {
	// Create ssh connect
	sshConn, err := c.Client.Dial("tcp", c.ForwardRemote)

	// Copy localConn.Reader to sshConn.Writer
	go func() {
		_, err = io.Copy(sshConn, localConn)
		if err != nil {
			fmt.Printf("Port forward local to remote failed: %v\n", err)
		}
	}()

	// Copy sshConn.Reader to localConn.Writer
	go func() {
		_, err = io.Copy(localConn, sshConn)
		if err != nil {
			fmt.Printf("Port forward remote to local failed: %v\n", err)
		}
	}()
}

// PortForwarder port forwarding based on the value of Connect
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
					fmt.Printf("listen.Accept failed: %v\n", err)
				}
				go c.forward(localConn)
			}
		}()
	}
}
