package sshlib

import (
	"encoding/binary"
	"encoding/gob"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"golang.org/x/crypto/ssh"
)

const (
	controlRequestPing     = "ping"
	controlRequestShell    = "shell"
	controlRequestCmdShell = "cmdshell"
	controlRequestCommand  = "command"
	controlRequestTunnel   = "tunnel"
)

const (
	streamFrameStdin byte = iota + 1
	streamFrameStdout
	streamFrameStderr
	streamFrameCloseStdin
	streamFrameExit
	streamFrameError
	streamFrameWindowChange
)

type controlRequest struct {
	Type    string
	Command string
	Options controlSessionOptions
	Tunnel  controlTunnelOptions
}

type controlResponse struct {
	OK         bool
	Error      string
	StreamPath string
}

type controlSessionOptions struct {
	TTY               bool
	Term              string
	Width             int
	Height            int
	ForwardX11        bool
	ForwardX11Trusted bool
	ForwardAgent      bool
}

type controlTunnelOptions struct {
	Mode TunnelMode
	Unit int
}

type controlMaster struct {
	connect        *Connect
	path           string
	listener       net.Listener
	closeOnce      sync.Once
	sessionMu      sync.Mutex
	sessionCond    *sync.Cond
	activeSessions int
	onActivity     func()
}

type controlClient struct {
	path string
}

type lockedFrameWriter struct {
	w  io.Writer
	mu sync.Mutex
}

func (w *lockedFrameWriter) WriteFrame(frameType byte, payload []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return writeStreamFrame(w.w, frameType, payload)
}

func newControlMaster(c *Connect, path string) (*controlMaster, error) {
	if err := ensureControlPath(path); err != nil {
		return nil, err
	}

	if err := cleanupStaleControlSocket(path); err != nil {
		return nil, err
	}

	listener, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}

	_ = os.Chmod(path, 0600)

	m := &controlMaster{
		connect:  c,
		path:     path,
		listener: listener,
	}
	m.sessionCond = sync.NewCond(&m.sessionMu)

	go m.acceptLoop()

	return m, nil
}

func (m *controlMaster) acceptLoop() {
	for {
		conn, err := m.listener.Accept()
		if err != nil {
			return
		}

		go m.handleControlConn(conn)
	}
}

func (m *controlMaster) handleControlConn(conn net.Conn) {
	defer conn.Close()
	m.touch()

	decoder := gob.NewDecoder(conn)
	encoder := gob.NewEncoder(conn)

	var req controlRequest
	if err := decoder.Decode(&req); err != nil {
		_ = encoder.Encode(controlResponse{OK: false, Error: err.Error()})
		return
	}

	switch req.Type {
	case controlRequestPing:
		_ = encoder.Encode(controlResponse{OK: true})
	case controlRequestShell, controlRequestCmdShell, controlRequestCommand, controlRequestTunnel:
		resp, err := m.prepareSession(req)
		if err != nil {
			_ = encoder.Encode(controlResponse{OK: false, Error: err.Error()})
			return
		}
		_ = encoder.Encode(resp)
	default:
		_ = encoder.Encode(controlResponse{OK: false, Error: fmt.Sprintf("unknown control request: %s", req.Type)})
	}
}

func (m *controlMaster) prepareSession(req controlRequest) (controlResponse, error) {
	m.touch()

	streamPath, err := m.newStreamPath()
	if err != nil {
		return controlResponse{}, err
	}

	listener, err := net.Listen("unix", streamPath)
	if err != nil {
		return controlResponse{}, err
	}

	_ = os.Chmod(streamPath, 0600)

	go func() {
		defer listener.Close()
		defer os.Remove(streamPath)

		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		if req.Type == controlRequestTunnel {
			m.serveTunnelStream(req, conn)
			return
		}

		m.serveStream(req, conn)
	}()

	return controlResponse{OK: true, StreamPath: streamPath}, nil
}

func (m *controlMaster) newStreamPath() (string, error) {
	dir := filepath.Dir(m.path)
	base := shortSocketToken(m.path)

	for i := 0; i < 32; i++ {
		candidate := filepath.Join(dir, fmt.Sprintf("gs-%s-%x.sock", base, time.Now().UnixNano()+int64(i)))
		if err := cleanupStaleControlSocket(candidate); err != nil {
			return "", err
		}
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate, nil
		}
	}

	return "", errors.New("sshlib: failed to allocate a stream socket path")
}

func shortSocketToken(path string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(path))
	return fmt.Sprintf("%08x", h.Sum32())
}

func (m *controlMaster) serveStream(req controlRequest, conn net.Conn) {
	m.acquireSession()
	defer m.releaseSession()
	m.touch()

	writer := &lockedFrameWriter{w: conn}

	session, err := m.connect.Client.NewSession()
	if err != nil {
		_ = writer.WriteFrame(streamFrameError, []byte(err.Error()))
		_ = writer.WriteFrame(streamFrameExit, encodeExitStatus(255))
		return
	}
	defer session.Close()

	if req.Options.TTY {
		if err := requestRemotePty(session, req.Options.Term, req.Options.Width, req.Options.Height); err != nil {
			_ = writer.WriteFrame(streamFrameError, []byte(err.Error()))
			_ = writer.WriteFrame(streamFrameExit, encodeExitStatus(255))
			return
		}
	}

	if req.Options.ForwardX11 {
		prevTrusted := m.connect.ForwardX11Trusted
		m.connect.ForwardX11Trusted = req.Options.ForwardX11Trusted
		err := m.connect.X11Forward(session)
		m.connect.ForwardX11Trusted = prevTrusted
		if err != nil {
			_ = writer.WriteFrame(streamFrameError, []byte(err.Error()))
			_ = writer.WriteFrame(streamFrameExit, encodeExitStatus(255))
			return
		}
	}

	if req.Options.ForwardAgent {
		m.connect.ForwardSshAgent(session)
	}
	stdin, err := session.StdinPipe()
	if err != nil {
		_ = writer.WriteFrame(streamFrameError, []byte(err.Error()))
		_ = writer.WriteFrame(streamFrameExit, encodeExitStatus(255))
		return
	}

	stdout, err := session.StdoutPipe()
	if err != nil {
		_ = writer.WriteFrame(streamFrameError, []byte(err.Error()))
		_ = writer.WriteFrame(streamFrameExit, encodeExitStatus(255))
		return
	}

	stderr, err := session.StderrPipe()
	if err != nil {
		_ = writer.WriteFrame(streamFrameError, []byte(err.Error()))
		_ = writer.WriteFrame(streamFrameExit, encodeExitStatus(255))
		return
	}

	var streamWG sync.WaitGroup
	streamWG.Add(2)

	go func() {
		defer streamWG.Done()
		pipeStreamToConn(stdout, writer, streamFrameStdout, session)
	}()
	go func() {
		defer streamWG.Done()
		pipeStreamToConn(stderr, writer, streamFrameStderr, session)
	}()
	go func() {
		readClientStream(conn, stdin, session)
	}()

	switch req.Type {
	case controlRequestShell:
		err = session.Shell()
	case controlRequestCmdShell, controlRequestCommand:
		err = session.Start(req.Command)
	}
	if err != nil {
		_ = writer.WriteFrame(streamFrameError, []byte(err.Error()))
		_ = writer.WriteFrame(streamFrameExit, encodeExitStatus(exitStatusFromError(err)))
		return
	}

	stopKeepAlive := m.connect.startSessionKeepAlive(session)
	defer stopKeepAlive()

	err = session.Wait()
	_ = stdin.Close()
	_ = session.Close()
	streamWG.Wait()
	m.touch()

	if err != nil && !shouldSuppressControlStreamError(req, err) {
		_ = writer.WriteFrame(streamFrameError, []byte(err.Error()))
	}
	_ = writer.WriteFrame(streamFrameExit, encodeExitStatus(exitStatusFromError(err)))
}

func (m *controlMaster) serveTunnelStream(req controlRequest, conn net.Conn) {
	m.acquireSession()
	defer m.releaseSession()
	m.touch()

	writer := &lockedFrameWriter{w: conn}

	msg := tunnelOpenChannelMessage{
		Mode: uint32(req.Tunnel.Mode),
		Unit: normalizeTunnelUnit(req.Tunnel.Unit),
	}

	channel, reqs, err := m.connect.Client.OpenChannel("tun@openssh.com", ssh.Marshal(&msg))
	if err != nil {
		_ = writer.WriteFrame(streamFrameError, []byte(err.Error()))
		_ = writer.WriteFrame(streamFrameExit, encodeExitStatus(255))
		return
	}
	defer channel.Close()

	go ssh.DiscardRequests(reqs)
	go readControlTunnelStream(conn, channel, channel)

	buf := make([]byte, 64*1024)
	for {
		n, err := channel.Read(buf)
		if n > 0 {
			if werr := writer.WriteFrame(streamFrameStdout, buf[:n]); werr != nil {
				_ = channel.Close()
				return
			}
			m.touch()
		}
		if err != nil {
			if !errors.Is(err, io.EOF) {
				_ = writer.WriteFrame(streamFrameError, []byte(err.Error()))
				_ = writer.WriteFrame(streamFrameExit, encodeExitStatus(255))
			} else {
				_ = writer.WriteFrame(streamFrameExit, encodeExitStatus(0))
			}
			return
		}
	}
}

func shouldSuppressControlStreamError(req controlRequest, err error) bool {
	if err == nil {
		return false
	}

	if !isInteractiveControlRequest(req.Type) {
		return false
	}

	var exitErr *ssh.ExitError
	if errors.As(err, &exitErr) {
		return true
	}

	var exitMissingErr *ssh.ExitMissingError
	if errors.As(err, &exitMissingErr) {
		return true
	}

	return exitStatusFromError(err) == 130
}

func isInteractiveControlRequest(requestType string) bool {
	switch requestType {
	case controlRequestShell, controlRequestCmdShell:
		return true
	default:
		return false
	}
}

func (m *controlMaster) touch() {
	if m.onActivity != nil {
		m.onActivity()
	}
}

func pipeStreamToConn(r io.Reader, writer *lockedFrameWriter, frameType byte, session *ssh.Session) {
	buf := make([]byte, 32*1024)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			if werr := writer.WriteFrame(frameType, buf[:n]); werr != nil {
				_ = session.Close()
				return
			}
		}
		if err != nil {
			return
		}
	}
}

func readClientStream(conn net.Conn, stdin io.WriteCloser, session *ssh.Session) {
	defer stdin.Close()

	for {
		frameType, payload, err := readStreamFrame(conn)
		if err != nil {
			if !errors.Is(err, io.EOF) {
				_ = session.Close()
			}
			return
		}

		switch frameType {
		case streamFrameStdin:
			if len(payload) == 0 {
				continue
			}
			if _, err := stdin.Write(payload); err != nil {
				return
			}
		case streamFrameCloseStdin:
			return
		case streamFrameWindowChange:
			if len(payload) != 8 {
				continue
			}
			width := int(binary.BigEndian.Uint32(payload[:4]))
			height := int(binary.BigEndian.Uint32(payload[4:]))
			_ = session.WindowChange(height, width)
		}
	}
}

func readControlTunnelStream(conn net.Conn, dst io.WriteCloser, closer io.Closer) {
	defer dst.Close()

	for {
		frameType, payload, err := readStreamFrame(conn)
		if err != nil {
			if !errors.Is(err, io.EOF) {
				_ = closer.Close()
			}
			return
		}

		switch frameType {
		case streamFrameStdin:
			if len(payload) == 0 {
				continue
			}
			if _, err := dst.Write(payload); err != nil {
				_ = closer.Close()
				return
			}
		case streamFrameCloseStdin:
			return
		}
	}
}

func (m *controlMaster) Close() error {
	var err error
	m.closeOnce.Do(func() {
		m.waitForSessionsToDrain()
		if m.listener != nil {
			err = m.listener.Close()
		}
		if m.connect.Client != nil {
			closeErr := m.connect.Client.Close()
			if err == nil {
				err = closeErr
			}
			m.connect.Client = nil
		}
		closeErr := m.connect.closeProxyConnects()
		if err == nil {
			err = closeErr
		}
		_ = os.Remove(m.path)
	})
	return err
}

func (m *controlMaster) acquireSession() {
	m.sessionMu.Lock()
	m.activeSessions++
	m.sessionMu.Unlock()
}

func (m *controlMaster) releaseSession() {
	m.sessionMu.Lock()
	defer m.sessionMu.Unlock()

	if m.activeSessions > 0 {
		m.activeSessions--
	}
	if m.activeSessions == 0 {
		m.sessionCond.Broadcast()
	}
}

func (m *controlMaster) waitForIdle() {
	m.sessionMu.Lock()
	defer m.sessionMu.Unlock()

	for m.activeSessions > 0 {
		m.sessionCond.Wait()
	}
}

func (m *controlMaster) waitForSessionsToDrain() {
	persist := m.connect.ControlPersist
	if persist <= 0 {
		m.waitForIdle()
		return
	}

	deadline := time.Now().Add(persist)
	for {
		m.waitForIdle()

		remaining := time.Until(deadline)
		if remaining <= 0 {
			return
		}

		sleepFor := 200 * time.Millisecond
		if remaining < sleepFor {
			sleepFor = remaining
		}
		time.Sleep(sleepFor)

		m.sessionMu.Lock()
		active := m.activeSessions
		m.sessionMu.Unlock()
		if active == 0 {
			if time.Now().After(deadline) || time.Now().Equal(deadline) {
				return
			}
			continue
		}

		deadline = time.Now().Add(persist)
	}
}

func dialControlClient(path string) (*controlClient, error) {
	conn, err := net.DialTimeout("unix", path, 2*time.Second)
	if err != nil {
		return nil, err
	}
	conn.Close()

	return &controlClient{path: path}, nil
}

func (c *controlClient) Close() error {
	return nil
}

func (c *controlClient) Ping() error {
	_, err := c.request(controlRequest{Type: controlRequestPing})
	return err
}

func (c *controlClient) request(req controlRequest) (controlResponse, error) {
	conn, err := net.DialTimeout("unix", c.path, 5*time.Second)
	if err != nil {
		return controlResponse{}, err
	}
	defer conn.Close()

	encoder := gob.NewEncoder(conn)
	decoder := gob.NewDecoder(conn)

	if err := encoder.Encode(req); err != nil {
		return controlResponse{}, err
	}

	var resp controlResponse
	if err := decoder.Decode(&resp); err != nil {
		return controlResponse{}, err
	}
	if !resp.OK {
		return controlResponse{}, errors.New(resp.Error)
	}

	return resp, nil
}

func ensureControlPath(path string) error {
	dir := filepath.Dir(path)
	return os.MkdirAll(dir, 0700)
}

func cleanupStaleControlSocket(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	if info.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("sshlib: control path exists and is not a socket: %s", path)
	}

	conn, err := net.DialTimeout("unix", path, time.Second)
	if err == nil {
		conn.Close()
		return nil
	}

	if errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, syscall.ENOENT) {
		return os.Remove(path)
	}

	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return os.Remove(path)
	}

	return nil
}

func writeStreamFrame(w io.Writer, frameType byte, payload []byte) error {
	header := make([]byte, 5)
	header[0] = frameType
	binary.BigEndian.PutUint32(header[1:], uint32(len(payload)))

	if _, err := w.Write(header); err != nil {
		return err
	}
	if len(payload) == 0 {
		return nil
	}
	_, err := w.Write(payload)
	return err
}

func readStreamFrame(r io.Reader) (byte, []byte, error) {
	header := make([]byte, 5)
	if _, err := io.ReadFull(r, header); err != nil {
		return 0, nil, err
	}

	size := binary.BigEndian.Uint32(header[1:])
	payload := make([]byte, size)
	if size > 0 {
		if _, err := io.ReadFull(r, payload); err != nil {
			return 0, nil, err
		}
	}

	return header[0], payload, nil
}

func encodeExitStatus(code int) []byte {
	payload := make([]byte, 4)
	binary.BigEndian.PutUint32(payload, uint32(code))
	return payload
}

func exitStatusFromError(err error) int {
	if err == nil {
		return 0
	}

	var exitErr *ssh.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitStatus()
	}

	return 255
}

func requestRemotePty(session *ssh.Session, term string, width, height int) error {
	if term == "" {
		term = "xterm"
	}
	if width <= 0 {
		width = 80
	}
	if height <= 0 {
		height = 24
	}

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}

	return session.RequestPty(term, height, width, modes)
}
