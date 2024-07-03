// Copyright (c) 2021 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package sshlib

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/armon/go-socks5"
	xauth "github.com/blacknon/go-x11auth"
	"golang.org/x/crypto/ssh"
)

// x11 request data struct
type x11Request struct {
	SingleConnection bool
	AuthProtocol     string
	AuthCookie       string
	ScreenNumber     uint32
}

// X11Forward send x11-req to ssh server and do x11 forwarding.
// Since the display number of the transfer destination and the PATH of the socket communication file
// are checked from the local environmsdent variable DISPLAY, this does not work if it is not set.
//
// Also, the value of COOKIE transfers the local value as it is. This will be addressed in the future.
func (c *Connect) X11Forward(session *ssh.Session) (err error) {
	// get xauthority path
	xauthorityPath := os.Getenv("XAUTHORITY")
	if len(xauthorityPath) == 0 {
		home := os.Getenv("HOME")
		if len(home) == 0 {
			err = errors.New("Xauthority not found: $XAUTHORITY, $HOME not set")
			return err
		}
		xauthorityPath = home + "/.Xauthority"
	}

	xa := xauth.XAuth{}
	xa.Display = os.Getenv("DISPLAY")

	cookie, err := xa.GetXAuthCookie(xauthorityPath, c.ForwardX11Trusted)

	// set x11-req Payload
	payload := x11Request{
		SingleConnection: false,
		AuthProtocol:     string("MIT-MAGIC-COOKIE-1"),
		AuthCookie:       string(cookie),
		ScreenNumber:     uint32(0),
	}

	// Send x11-req Request
	ok, err := session.SendRequest("x11-req", true, ssh.Marshal(payload))
	if err == nil && !ok {
		return errors.New("ssh: x11-req failed")
	} else {
		// Open HandleChannel x11
		x11channels := c.Client.HandleChannelOpen("x11")

		go func() {
			for ch := range x11channels {
				channel, _, err := ch.Accept()
				if err != nil {
					continue
				}

				go x11forwarder(channel)
			}
		}()
	}

	return err
}

// x11Connect return net.Conn x11 socket.
func x11Connect(display string) (conn net.Conn, err error) {
	var conDisplay string

	protocol := "unix"

	if display[0] == '/' { // PATH type socket
		conDisplay = display
	} else if display[0] != ':' { // Forwarded display
		protocol = "tcp"
		if b, _, ok := strings.Cut(display, ":"); ok {
			conDisplay = fmt.Sprintf("%v:%v", b, getX11DisplayNumber(display)+6000)
		} else {
			conDisplay = display
		}
	} else { // /tmp/.X11-unix/X0
		conDisplay = fmt.Sprintf("/tmp/.X11-unix/X%v", getX11DisplayNumber(display))
	}

	return net.Dial(protocol, conDisplay)
}

// x11forwarder forwarding socket x11 data.
func x11forwarder(channel ssh.Channel) {
	conn, err := x11Connect(os.Getenv("DISPLAY"))

	if err != nil {
		return
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		io.Copy(conn, channel)
		conn.Close()
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

// getX11Display return X11 display number from env $DISPLAY
func getX11DisplayNumber(display string) int {
	colonIdx := strings.LastIndex(display, ":")
	dotIdx := strings.LastIndex(display, ".")

	if colonIdx < 0 {
		return 0
	}

	if dotIdx < 0 {
		dotIdx = len(display)
	}

	i, err := strconv.Atoi(display[colonIdx+1 : dotIdx])
	if err != nil {
		return 0
	}

	return i
}

// TCPLocalForward forwarding tcp data. Like Local port forward (ssh -L).
// localAddr, remoteAddr is write as "address:port".
//
// example) "127.0.0.1:22", "abc.com:9977"
func (c *Connect) TCPLocalForward(localAddr, remoteAddr string) (err error) {
	// create listener
	listener, err := net.Listen("tcp", localAddr)
	if err != nil {
		return
	}

	// forwarding
	go func() {
		for {
			// local (type net.Conn)
			local, err := listener.Accept()
			if err != nil {
				return
			}

			// remote (type net.Conn)
			remote, err := c.Client.Dial("tcp", remoteAddr)

			// forward
			go c.forwarder(local, remote)
		}
	}()

	return
}

// TCPRemoteForward forwarding tcp data. Like Remote port forward (ssh -R).
// localAddr, remoteAddr is write as "address:port".
//
// example) "127.0.0.1:22", "abc.com:9977"
func (c *Connect) TCPRemoteForward(localAddr, remoteAddr string) (err error) {
	// create listener
	listener, err := c.Client.Listen("tcp", remoteAddr)
	if err != nil {
		return
	}

	// forwarding
	go func() {
		for {
			// local (type net.Conn)
			local, err := net.Dial("tcp", localAddr)
			if err != nil {
				return
			}

			// remote (type net.Conn)
			remote, err := listener.Accept()
			if err != nil {
				return
			}

			go c.forwarder(local, remote)
		}
	}()

	return
}

// forwarder tcp/udp port forward. dialType in `tcp` or `udp`.
// addr is remote port forward address (`localhost:80`, `192.168.10.100:443` etc...).
func (c *Connect) forwarder(local net.Conn, remote net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)

	// Copy local to remote
	go func() {
		io.Copy(remote, local)
		wg.Done()
	}()

	// Copy remote to local
	go func() {
		io.Copy(local, remote)
		wg.Done()
	}()

	wg.Wait()
	remote.Close()
	local.Close()
}

// socks5Resolver prevents DNS from resolving on the local machine, rather than over the SSH connection.
type socks5Resolver struct{}

// Resolve
func (socks5Resolver) Resolve(ctx context.Context, name string) (context.Context, net.IP, error) {
	return ctx, nil, nil
}

func (c *Connect) getDynamicForwardLogger() *log.Logger {
	if c.DynamicForwardLogger == nil {
		return log.New(io.Discard, "", log.LstdFlags)
	}
	return c.DynamicForwardLogger
}

// TCPDynamicForward forwarding tcp data. Like Dynamic forward (`ssh -D <port>`).
// listen port Socks5 proxy server.
func (c *Connect) TCPDynamicForward(address, port string) (err error) {
	// Create Socks5 config
	conf := &socks5.Config{
		Dial: func(ctx context.Context, n, addr string) (net.Conn, error) {
			return c.Client.Dial(n, addr)
		},
		Resolver: socks5Resolver{},
		Logger:   c.getDynamicForwardLogger(),
	}

	// Create Socks5 server
	s, err := socks5.New(conf)
	if err != nil {
		return
	}

	// Listen
	err = s.ListenAndServe("tcp", net.JoinHostPort(address, port))

	return
}

// TCPReverseDynamicForward reverse forwarding tcp data.
// Like Openssh Reverse Dynamic forward (`ssh -R <port>`).
func (c *Connect) TCPReverseDynamicForward(address, port string) (err error) {
	// Create Socks5 config
	conf := &socks5.Config{
		Dial: func(ctx context.Context, n, addr string) (net.Conn, error) {
			return net.Dial(n, addr)
		},
		Resolver: socks5Resolver{},
		Logger:   c.getDynamicForwardLogger(),
	}

	// create listener
	listener, err := c.Client.Listen("tcp", net.JoinHostPort(address, port))
	if err != nil {
		return
	}

	// Create Socks5 server
	s, err := socks5.New(conf)
	if err != nil {
		return
	}

	// Listen
	err = s.Serve(listener)
	return
}

// HTTPDynamicForward forwarding http data.
// Like Dynamic forward (`ssh -D <port>`). but use http proxy.
func (c *Connect) HTTPDynamicForward(address, port string) (err error) {
	// create dial
	dial := c.Client.Dial

	// create listener
	listener, err := net.Listen("tcp", net.JoinHostPort(address, port))
	if err != nil {
		return
	}
	defer listener.Close()

	// create proxy server.
	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodConnect {
				handleHTTPSProxy(dial, w, r)
			} else {
				handleHTTPProxy(dial, w, r)
			}
		}),
		ErrorLog: c.getDynamicForwardLogger(),
	}

	// listen
	err = server.Serve(listener)
	return
}

// HTTPReverseDynamicForward reverse forwarding http data.
// Like Reverse Dynamic forward (`ssh -R <port>`). but use http proxy.
func (c *Connect) HTTPReverseDynamicForward(address, port string) (err error) {
	// create dial
	dial := net.Dial

	// create listener
	listener, err := c.Client.Listen("tcp", net.JoinHostPort(address, port))
	if err != nil {
		return
	}
	defer listener.Close()

	// create proxy server.
	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodConnect {
				handleHTTPSProxy(dial, w, r)
			} else {
				handleHTTPProxy(dial, w, r)
			}
		}),
		ErrorLog: c.getDynamicForwardLogger(),
	}

	// listen
	err = server.Serve(listener)
	return
}
