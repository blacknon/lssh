package ssh

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"

	"golang.org/x/crypto/ssh"
)

// TODO(blacknon):
//     socket forwardについても実装する

func readAuthority(hostname, display string) (
	name string, data []byte, err error) {

	// b is a scratch buffer to use and should be at least 256 bytes long
	// (i.e. it should be able to hold a hostname).
	b := make([]byte, 256)

	// As per /usr/include/X11/Xauth.h.
	const familyLocal = 256

	if len(hostname) == 0 || hostname == "localhost" {
		hostname, err = os.Hostname()
		if err != nil {
			return "", nil, err
		}
	}

	fname := os.Getenv("XAUTHORITY")
	if len(fname) == 0 {
		home := os.Getenv("HOME")
		if len(home) == 0 {
			err = errors.New("Xauthority not found: $XAUTHORITY, $HOME not set")
			return "", nil, err
		}
		fname = home + "/.Xauthority"
	}

	r, err := os.Open(fname)
	if err != nil {
		return "", nil, err
	}
	defer r.Close()

	for {
		var family uint16
		if err := binary.Read(r, binary.BigEndian, &family); err != nil {
			return "", nil, err
		}

		addr, err := getString(r, b)
		if err != nil {
			return "", nil, err
		}

		disp, err := getString(r, b)
		if err != nil {
			return "", nil, err
		}

		name0, err := getString(r, b)
		if err != nil {
			return "", nil, err
		}

		data0, err := getBytes(r, b)
		if err != nil {
			return "", nil, err
		}

		if family == familyLocal && addr == hostname && disp == display {
			return name0, data0, nil
		}
	}
	panic("unreachable")
}

func getBytes(r io.Reader, b []byte) ([]byte, error) {
	var n uint16
	if err := binary.Read(r, binary.BigEndian, &n); err != nil {
		return nil, err
	} else if n > uint16(len(b)) {
		return nil, errors.New("bytes too long for buffer")
	}

	if _, err := io.ReadFull(r, b[0:n]); err != nil {
		return nil, err
	}
	return b[0:n], nil
}

func getString(r io.Reader, b []byte) (string, error) {
	b, err := getBytes(r, b)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

type x11request struct {
	SingleConnection bool
	AuthProtocol     string
	AuthCookie       string
	ScreenNumber     uint32
}

func x11ConnectDisplay() (conn net.Conn, err error) {
	display := os.Getenv("DISPLAY")
	display0 := display
	colonIdx := strings.LastIndex(display, ":")
	dotIdx := strings.LastIndex(display, ".")

	if colonIdx < 0 {
		err = errors.New("bad display string: " + display0)
		return
	}

	var conDisplay string
	if display[0] == '/' { // PATH type socket
		conDisplay = display
	} else { // /tmp/.X11-unix/X0
		conDisplay = "/tmp/.X11-unix/X" + display[colonIdx+1:dotIdx]
	}

	// fmt.Println(conDisplay)
	conn, err = net.Dial("unix", conDisplay)
	return
}

func x11SocketForward(channel ssh.Channel) {
	// TODO(blacknon): Socket通信しか考慮されていないので、TCP通信での指定もできるようにする
	conn, err := x11ConnectDisplay()

	if err != nil {
		return
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		io.Copy(conn, channel)
		conn.(*net.UnixConn).CloseWrite()
		wg.Done()
	}()
	go func() {
		io.Copy(channel, conn)
		channel.CloseWrite()
		wg.Done()
	}()

	wg.Wait()
	conn.Close()
	channel.Close()
}

func (c *Connect) X11Forwarder(session *ssh.Session) {
	// xgbConn, err := xgb.NewConn()
	// cookie := xgbConn.NewCookie(true, true)

	// TODO(blacknon): DISPLAY関数のパース処理用の関数を別途作成し、それを呼び出してDISPLAY番号を指定させる。
	xAuthName, xAuthData, err := readAuthority("", "0")
	if err != nil {
		os.Exit(1)
	}

	var cookie string
	fmt.Println(xAuthName)
	for _, d := range xAuthData {
		cookie = cookie + fmt.Sprintf("%02x", d)
	}

	// set x11-req Payload
	payload := x11request{
		SingleConnection: false,
		AuthProtocol:     string("MIT-MAGIC-COOKIE-1"),
		// AuthCookie:       string(common.NewSHA1Hash()),
		AuthCookie:   string(cookie),
		ScreenNumber: uint32(0),
	}

	// Send x11-req Request
	ok, err := session.SendRequest("x11-req", true, ssh.Marshal(payload))
	if err == nil && !ok {
		fmt.Println(errors.New("ssh: x11-req failed"))
	} else {
		// Open HandleChannel x11
		x11channels := c.Client.HandleChannelOpen("x11")

		go func() {
			for ch := range x11channels {
				channel, _, err := ch.Accept()
				if err != nil {
					continue
				}

				go x11SocketForward(channel)
			}
		}()
	}
}

// forward function to do port io.Copy with goroutine
func (c *Connect) portForward(localConn net.Conn) {
	// TODO(blacknon): 関数名等をちゃんと考える

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
	// TODO(blacknon):
	// 現在の方式だと、クライアント側で無理やりポートフォワーディングをしている状態なので、RFCに沿ってport forwardさせる処理についても追加する
	//
	// 【参考】
	//     - https://github.com/maxhawkins/easyssh/blob/a4ce364b6dd8bf2433a0d67ae76cf1d880c71d75/tcpip.go
	//     - https://www.unixuser.org/~haruyama/RFC/ssh/rfc4254.txt
	//
	// TODO(blacknon): 関数名等をちゃんと考える

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
				go c.portForward(localConn)
			}
		}()
	}
}
