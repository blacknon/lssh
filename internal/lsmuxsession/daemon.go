//go:build !windows

package lsmuxsession

import (
	"bytes"
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/creack/pty"
)

const maxBufferedOutput = 1 << 20

type Daemon struct {
	Name       string
	ConfigPath string
	SocketPath string
	Exe        string
	Args       []string
	Env        []string

	listener net.Listener
	cmd      *exec.Cmd
	ptyFile  *os.File

	mu       sync.Mutex
	client   *attachedClient
	buffer   bytes.Buffer
	shutdown bool
}

type attachedClient struct {
	conn net.Conn
	enc  *json.Encoder
	mu   sync.Mutex
}

func (c *attachedClient) send(msg Message) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.enc.Encode(msg)
}

func (d *Daemon) Run(ready func()) error {
	network, address, resolvedSocket, err := listenerSpec(d.Name, d.SocketPath)
	if err != nil {
		return err
	}
	if network == "unix" {
		_ = os.Remove(address)
	}
	listener, err := net.Listen(network, address)
	if err != nil {
		return err
	}
	d.listener = listener
	d.SocketPath = resolvedSocket
	if network == "tcp" {
		address = listener.Addr().String()
	}

	cmd := exec.Command(d.Exe, d.Args...)
	cmd.Env = append([]string(nil), d.Env...)
	ptyFile, err := pty.Start(cmd)
	if err != nil {
		_ = listener.Close()
		return err
	}
	d.cmd = cmd
	d.ptyFile = ptyFile

	if err := SaveSession(Session{
		Name:         d.Name,
		PID:          cmd.Process.Pid,
		Network:      network,
		Address:      address,
		SocketPath:   d.SocketPath,
		ConfigPath:   d.ConfigPath,
		CreatedAt:    time.Now(),
		LastAttached: time.Now(),
	}); err != nil {
		_ = ptyFile.Close()
		_ = listener.Close()
		return err
	}

	go d.readPTY()
	go d.waitChild()

	if ready != nil {
		ready()
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			d.mu.Lock()
			shutdown := d.shutdown
			d.mu.Unlock()
			if shutdown {
				return nil
			}
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				time.Sleep(100 * time.Millisecond)
				continue
			}
			return err
		}
		go d.handleConn(conn)
	}
}

func (d *Daemon) readPTY() {
	buf := make([]byte, 8192)
	for {
		n, err := d.ptyFile.Read(buf)
		if n > 0 {
			d.broadcastOutput(buf[:n])
		}
		if err != nil {
			return
		}
	}
}

func (d *Daemon) broadcastOutput(data []byte) {
	d.mu.Lock()
	client := d.client
	if client == nil {
		d.appendBufferedOutput(data)
		d.mu.Unlock()
		return
	}
	d.mu.Unlock()
	if err := client.send(Message{Type: "output", Data: append([]byte(nil), data...)}); err != nil {
		d.detachClient(client)
	}
}

func (d *Daemon) appendBufferedOutput(data []byte) {
	if len(data) == 0 {
		return
	}
	if d.buffer.Len()+len(data) > maxBufferedOutput {
		trim := d.buffer.Len() + len(data) - maxBufferedOutput
		if trim > d.buffer.Len() {
			d.buffer.Reset()
		} else {
			remaining := append([]byte(nil), d.buffer.Bytes()[trim:]...)
			d.buffer.Reset()
			_, _ = d.buffer.Write(remaining)
		}
	}
	_, _ = d.buffer.Write(data)
}

func (d *Daemon) waitChild() {
	err := d.cmd.Wait()
	d.mu.Lock()
	client := d.client
	d.mu.Unlock()
	if client != nil {
		msg := Message{Type: "exit", Message: "session exited"}
		if err != nil {
			msg.Message = err.Error()
		}
		_ = client.send(msg)
	}
	_ = d.Close()
}

func (d *Daemon) handleConn(conn net.Conn) {
	defer conn.Close()
	dec := json.NewDecoder(conn)
	enc := json.NewEncoder(conn)

	var hello Message
	if err := dec.Decode(&hello); err != nil {
		return
	}
	switch hello.Type {
	case "ping":
		_ = enc.Encode(Message{Type: "pong"})
	case "attach":
		d.attachConn(conn, dec, enc)
	default:
		_ = enc.Encode(Message{Type: "error", Message: "unknown action"})
	}
}

func (d *Daemon) attachConn(conn net.Conn, dec *json.Decoder, enc *json.Encoder) {
	client := &attachedClient{conn: conn, enc: enc}

	d.mu.Lock()
	if d.client != nil {
		d.mu.Unlock()
		_ = enc.Encode(Message{Type: "error", Message: "session is already attached"})
		return
	}
	d.client = client
	buffered := append([]byte(nil), d.buffer.Bytes()...)
	d.buffer.Reset()
	d.mu.Unlock()

	if len(buffered) > 0 {
		_ = client.send(Message{Type: "output", Data: buffered})
	}

	session, err := LoadSession(d.Name)
	if err == nil {
		session.LastAttached = time.Now()
		_ = SaveSession(session)
	}

	for {
		var msg Message
		if err := dec.Decode(&msg); err != nil {
			d.detachClient(client)
			return
		}
		switch msg.Type {
		case "input":
			if len(msg.Data) > 0 {
				if _, err := d.ptyFile.Write(msg.Data); err != nil {
					d.detachClient(client)
					return
				}
			}
		case "resize":
			_ = pty.Setsize(d.ptyFile, &pty.Winsize{
				Cols: uint16(maxInt(msg.Cols, 1)),
				Rows: uint16(maxInt(msg.Rows, 1)),
			})
		case "detach":
			d.detachClient(client)
			return
		}
	}
}

func (d *Daemon) detachClient(client *attachedClient) {
	d.mu.Lock()
	if d.client == client {
		d.client = nil
	}
	d.mu.Unlock()
	_ = client.conn.Close()
}

func (d *Daemon) Close() error {
	d.mu.Lock()
	if d.shutdown {
		d.mu.Unlock()
		return nil
	}
	d.shutdown = true
	listener := d.listener
	client := d.client
	ptyFile := d.ptyFile
	cmd := d.cmd
	d.mu.Unlock()

	if client != nil {
		_ = client.conn.Close()
	}
	if listener != nil {
		_ = listener.Close()
	}
	if ptyFile != nil {
		_ = ptyFile.Close()
	}
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	return RemoveSession(d.Name)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
