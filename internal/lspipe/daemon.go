// Copyright (c) 2026 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package lspipe

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	sshlib "github.com/blacknon/go-sshlib"
	conf "github.com/blacknon/lssh/internal/config"
	lssh "github.com/blacknon/lssh/internal/ssh"
	gossh "golang.org/x/crypto/ssh"
)

type Daemon struct {
	Name                  string
	Config                conf.Config
	ConfigPath            string
	Hosts                 []string
	ControlMasterOverride *bool

	mu       sync.RWMutex
	conns    map[string]*sshlib.Connect
	health   map[string]HostHealth
	listener net.Listener
}

func NewDaemon(name, configPath string, config conf.Config, hosts []string, controlMasterOverride *bool) *Daemon {
	hosts = append([]string(nil), hosts...)
	sort.Strings(hosts)
	return &Daemon{
		Name:                  normalizeSessionName(name),
		Config:                config,
		ConfigPath:            configPath,
		Hosts:                 hosts,
		ControlMasterOverride: controlMasterOverride,
		conns:                 map[string]*sshlib.Connect{},
		health:                map[string]HostHealth{},
	}
}

func (d *Daemon) Run(ready func()) error {
	network, address, err := listenerSpec(d.Name)
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

	if network == "tcp" {
		address = listener.Addr().String()
	}

	if err := d.connectAll(); err != nil {
		return err
	}

	if err := SaveSession(Session{
		Name:       d.Name,
		Hosts:      d.Hosts,
		PID:        os.Getpid(),
		Network:    network,
		Address:    address,
		ConfigPath: d.ConfigPath,
		CreatedAt:  time.Now(),
		LastUsedAt: time.Now(),
		HostHealth: d.snapshotHealth(),
	}); err != nil {
		_ = listener.Close()
		return err
	}

	if ready != nil {
		ready()
	}

	stopSignals := make(chan os.Signal, 1)
	signal.Notify(stopSignals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(stopSignals)

	go func() {
		<-stopSignals
		_ = d.Close()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				time.Sleep(100 * time.Millisecond)
				continue
			}
			return nil
		}
		go d.handleConn(conn)
	}
}

func (d *Daemon) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.listener != nil {
		_ = d.listener.Close()
		d.listener = nil
	}

	for host, conn := range d.conns {
		_ = conn.Close()
		delete(d.conns, host)
	}

	return RemoveSession(d.Name)
}

func (d *Daemon) handleConn(conn net.Conn) {
	defer conn.Close()

	dec := json.NewDecoder(conn)
	enc := json.NewEncoder(conn)

	var req Request
	if err := dec.Decode(&req); err != nil {
		_ = enc.Encode(Event{Type: "error", Message: err.Error()})
		return
	}

	switch req.Action {
	case actionPing:
		_ = enc.Encode(Event{Type: "pong"})
	case actionExec:
		if err := d.execRequest(enc, req); err != nil {
			_ = enc.Encode(Event{Type: "error", Message: err.Error()})
		}
	default:
		_ = enc.Encode(Event{Type: "error", Message: "unknown action"})
	}
}

func (d *Daemon) execRequest(enc *json.Encoder, req Request) error {
	targets, err := d.resolveTargets(req.Hosts)
	if err != nil {
		return err
	}
	if req.Raw && len(targets) != 1 {
		return fmt.Errorf("--raw requires exactly one host after resolution")
	}

	var encMu sync.Mutex
	sendEvent := func(event Event) {
		encMu.Lock()
		defer encMu.Unlock()
		_ = enc.Encode(event)
	}

	var wg sync.WaitGroup
	results := make(chan int, len(targets))

	for _, host := range targets {
		host := host
		wg.Add(1)
		go func() {
			defer wg.Done()
			code, runErr := d.runCommand(host, req, sendEvent)
			if runErr != nil {
				sendEvent(Event{Type: "stderr", Host: host, Stream: "stderr", Data: []byte(fmt.Sprintf("%s :: %v\n", host, runErr))})
			}
			results <- code
		}()
	}

	wg.Wait()
	close(results)

	exitCode := 0
	for code := range results {
		if code != 0 {
			exitCode = code
			if exitCode == 0 {
				exitCode = 1
			}
		}
	}

	_ = TouchSession(d.Name)
	session, err := LoadSession(d.Name)
	if err == nil {
		session.HostHealth = d.snapshotHealth()
		session.LastUsedAt = time.Now()
		_ = SaveSession(session)
	}

	sendEvent(Event{Type: "done", ExitCode: exitCode})
	return nil
}

func (d *Daemon) resolveTargets(requested []string) ([]string, error) {
	if len(requested) == 0 {
		return append([]string(nil), d.Hosts...), nil
	}

	allowed := map[string]struct{}{}
	for _, host := range d.Hosts {
		allowed[host] = struct{}{}
	}

	targets := make([]string, 0, len(requested))
	for _, host := range requested {
		host = strings.TrimSpace(host)
		if host == "" {
			continue
		}
		if _, ok := allowed[host]; !ok {
			return nil, fmt.Errorf("host %q is not part of session %q", host, d.Name)
		}
		targets = append(targets, host)
	}
	sort.Strings(targets)
	return targets, nil
}

func (d *Daemon) runCommand(host string, req Request, sendEvent func(Event)) (int, error) {
	conn, err := d.getOrReconnect(host)
	if err != nil {
		return 1, err
	}

	var stdoutWriter io.Writer
	var stderrWriter io.Writer
	if req.Raw {
		stdoutWriter = &eventWriter{host: host, stream: "stdout", raw: true, send: sendEvent}
		stderrWriter = &eventWriter{host: host, stream: "stderr", raw: true, send: sendEvent}
	} else {
		stdout := &eventWriter{host: host, stream: "stdout", send: sendEvent}
		stderr := &eventWriter{host: host, stream: "stderr", send: sendEvent}
		stdoutWriter = stdout
		stderrWriter = stderr
		defer stdout.Flush()
		defer stderr.Flush()
	}

	err = runSessionCommand(conn, req.Command, req.Stdin, stdoutWriter, stderrWriter)
	code := exitCode(err)

	if err != nil {
		d.setHealth(host, HostHealth{Connected: false, Error: err.Error()})
	} else {
		d.setHealth(host, HostHealth{Connected: true})
	}

	return code, nil
}

func runSessionCommand(conn *sshlib.Connect, command string, stdin []byte, stdout io.Writer, stderr io.Writer) error {
	session, err := conn.CreateSession()
	if err != nil {
		return err
	}
	defer session.Close()

	stdoutPipe, err := session.StdoutPipe()
	if err != nil {
		return err
	}
	stderrPipe, err := session.StderrPipe()
	if err != nil {
		return err
	}

	var stdinPipe io.WriteCloser
	if len(stdin) > 0 {
		stdinPipe, err = session.StdinPipe()
		if err != nil {
			return err
		}
	}

	if err := session.Start(command); err != nil {
		if stdinPipe != nil {
			_ = stdinPipe.Close()
		}
		return err
	}

	var wg sync.WaitGroup
	if stdout != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = io.Copy(stdout, stdoutPipe)
		}()
	}
	if stderr != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = io.Copy(stderr, stderrPipe)
		}()
	}
	if stdinPipe != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = io.Copy(stdinPipe, bytes.NewReader(stdin))
			_ = stdinPipe.Close()
		}()
	}

	waitErr := session.Wait()
	wg.Wait()
	return waitErr
}

func (d *Daemon) connectAll() error {
	for _, host := range d.Hosts {
		if _, err := d.connect(host); err != nil {
			d.setHealth(host, HostHealth{Connected: false, Error: err.Error()})
		}
	}
	return nil
}

func (d *Daemon) getOrReconnect(host string) (*sshlib.Connect, error) {
	d.mu.RLock()
	conn := d.conns[host]
	d.mu.RUnlock()

	if conn != nil {
		if err := conn.CheckClientAlive(); err == nil {
			return conn, nil
		}
		_ = conn.Close()
	}

	return d.connect(host)
}

func (d *Daemon) connect(host string) (*sshlib.Connect, error) {
	run := &lssh.Run{
		ServerList:            []string{host},
		Conf:                  d.Config,
		ControlMasterOverride: d.ControlMasterOverride,
		EnableStdoutMutex:     false,
		EnableHeader:          false,
		DisableHeader:         true,
	}
	run.CreateAuthMethodMap()

	conn, err := run.CreateSshConnectDirect(host)
	if err != nil {
		d.setHealth(host, HostHealth{Connected: false, Error: err.Error()})
		return nil, err
	}

	d.mu.Lock()
	d.conns[host] = conn
	d.health[host] = HostHealth{Connected: true}
	d.mu.Unlock()
	return conn, nil
}

func (d *Daemon) setHealth(host string, health HostHealth) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.health[host] = health
	if !health.Connected {
		if conn, ok := d.conns[host]; ok {
			_ = conn.Close()
			delete(d.conns, host)
		}
	}
}

func (d *Daemon) snapshotHealth() map[string]HostHealth {
	d.mu.RLock()
	defer d.mu.RUnlock()

	out := make(map[string]HostHealth, len(d.health))
	for host, health := range d.health {
		out[host] = health
	}
	return out
}

type eventWriter struct {
	host   string
	stream string
	raw    bool
	send   func(Event)
	mu     sync.Mutex
	buf    bytes.Buffer
}

func (w *eventWriter) Write(p []byte) (int, error) {
	if w.raw {
		if len(p) > 0 {
			data := append([]byte(nil), p...)
			if w.stream == "stderr" {
				data = filterSttyNoiseBytes(data)
			}
			if len(data) > 0 {
				w.send(Event{Type: w.stream, Host: w.host, Stream: w.stream, Data: data})
			}
		}
		return len(p), nil
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	reader := bufio.NewReader(bytes.NewReader(p))
	for {
		chunk, err := reader.ReadBytes('\n')
		if len(chunk) > 0 {
			w.buf.Write(chunk)
			if chunk[len(chunk)-1] == '\n' {
				w.flushLocked(true)
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return 0, err
		}
	}
	return len(p), nil
}

func (w *eventWriter) Flush() {
	if w.raw {
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	w.flushLocked(false)
}

func (w *eventWriter) flushLocked(hadNewline bool) {
	if w.buf.Len() == 0 {
		return
	}

	data := w.buf.String()
	w.buf.Reset()
	if !hadNewline {
		data = strings.TrimSuffix(data, "\n")
	}
	if data == "" {
		return
	}

	lines := strings.SplitAfter(data, "\n")
	var out strings.Builder
	for _, line := range lines {
		if line == "" {
			continue
		}
		if w.stream == "stderr" && shouldSuppressSttyNoise(line) {
			continue
		}
		out.WriteString(w.host)
		out.WriteString(" :: ")
		out.WriteString(line)
		if !strings.HasSuffix(line, "\n") {
			out.WriteByte('\n')
		}
	}

	w.send(Event{Type: w.stream, Host: w.host, Stream: w.stream, Data: []byte(out.String())})
}

func filterSttyNoiseBytes(data []byte) []byte {
	lines := strings.SplitAfter(string(data), "\n")
	var out strings.Builder
	for _, line := range lines {
		if line == "" || shouldSuppressSttyNoise(line) {
			continue
		}
		out.WriteString(line)
	}
	return []byte(out.String())
}

func shouldSuppressSttyNoise(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false
	}

	if !strings.HasPrefix(trimmed, "stty:") {
		return false
	}

	noisePatterns := []string{
		"inappropriate ioctl for device",
		"standard input: Inappropriate ioctl for device",
		"標準入力: デバイスに対する不適切なioctlです",
		"'stdin': Inappropriate ioctl for device",
	}
	for _, pattern := range noisePatterns {
		if strings.Contains(trimmed, pattern) {
			return true
		}
	}

	return false
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}

	var exitErr *gossh.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitStatus()
	}

	return 1
}
