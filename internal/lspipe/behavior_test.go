package lspipe

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	conf "github.com/blacknon/lssh/internal/config"
)

type fakeConn struct {
	session    sessionRunner
	createErr  error
	aliveErr   error
	closeCount int
	closed     bool
	checks     int
	mu         sync.Mutex
}

func (f *fakeConn) CreateSession() (sessionRunner, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	return f.session, nil
}

func (f *fakeConn) CheckClientAlive() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.checks++
	return f.aliveErr
}

func (f *fakeConn) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	f.closeCount++
	return nil
}

type fakeSession struct {
	stdoutData []byte
	stderrData []byte
	waitErr    error
	started    string
	stdinBuf   bytes.Buffer
	stdoutR    *io.PipeReader
	stdoutW    *io.PipeWriter
	stderrR    *io.PipeReader
	stderrW    *io.PipeWriter
}

func newFakeSession(stdoutData, stderrData string, waitErr error) *fakeSession {
	stdoutR, stdoutW := io.Pipe()
	stderrR, stderrW := io.Pipe()
	return &fakeSession{
		stdoutData: []byte(stdoutData),
		stderrData: []byte(stderrData),
		waitErr:    waitErr,
		stdoutR:    stdoutR,
		stdoutW:    stdoutW,
		stderrR:    stderrR,
		stderrW:    stderrW,
	}
}

func (f *fakeSession) StdoutPipe() (io.Reader, error) { return f.stdoutR, nil }
func (f *fakeSession) StderrPipe() (io.Reader, error) { return f.stderrR, nil }
func (f *fakeSession) StdinPipe() (io.WriteCloser, error) {
	return nopWriteCloser{Writer: &f.stdinBuf}, nil
}
func (f *fakeSession) Start(command string) error {
	f.started = command
	go func() {
		if len(f.stdoutData) > 0 {
			_, _ = f.stdoutW.Write(f.stdoutData)
		}
		_ = f.stdoutW.Close()
	}()
	go func() {
		if len(f.stderrData) > 0 {
			_, _ = f.stderrW.Write(f.stderrData)
		}
		_ = f.stderrW.Close()
	}()
	return nil
}
func (f *fakeSession) Wait() error  { return f.waitErr }
func (f *fakeSession) Close() error { return nil }

type nopWriteCloser struct{ io.Writer }

func (n nopWriteCloser) Close() error { return nil }

func decodeEvents(t *testing.T, data []byte) []Event {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(data))
	var out []Event
	for dec.More() {
		var event Event
		if err := dec.Decode(&event); err != nil {
			t.Fatalf("decode event: %v", err)
		}
		out = append(out, event)
	}
	return out
}

func TestRunSessionCommandBroadcastsStdinAndStreams(t *testing.T) {
	session := newFakeSession("out\n", "err\n", nil)
	conn := &fakeConn{session: session}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := runSessionCommand(conn, "hostname", []byte("input\n"), &stdout, &stderr)
	if err != nil {
		t.Fatalf("runSessionCommand() error = %v", err)
	}
	if session.started != "hostname" {
		t.Fatalf("started command = %q", session.started)
	}
	if got := session.stdinBuf.String(); got != "input\n" {
		t.Fatalf("stdin = %q", got)
	}
	if got := stdout.String(); got != "out\n" {
		t.Fatalf("stdout = %q", got)
	}
	if got := stderr.String(); got != "err\n" {
		t.Fatalf("stderr = %q", got)
	}
}

func TestExecuteRoutesStdoutAndStderrAndUsesDoneExitCode(t *testing.T) {
	originalDial := dialSession
	t.Cleanup(func() { dialSession = originalDial })

	var dialCount int
	dialSession = func(session Session) (net.Conn, error) {
		server, client := net.Pipe()
		dialCount++
		go func(call int) {
			defer server.Close()
			enc := json.NewEncoder(server)
			if call == 1 {
				var req Request
				_ = json.NewDecoder(server).Decode(&req)
				_ = enc.Encode(Event{Type: "pong"})
				return
			}
			dec := json.NewDecoder(server)
			var req Request
			_ = dec.Decode(&req)
			_ = enc.Encode(Event{Type: "stdout", Data: []byte("hello\n")})
			_ = enc.Encode(Event{Type: "stderr", Data: []byte("warn\n")})
			_ = enc.Encode(Event{Type: "done", ExitCode: 7})
		}(dialCount)
		return client, nil
	}

	cacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)
	if err := SaveSession(Session{Name: "default", Network: "test", Address: "pipe"}); err != nil {
		t.Fatalf("SaveSession() error = %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Execute(ExecOptions{Name: "default", Command: "hostname", Stdout: &stdout, Stderr: &stderr})
	if err == nil || !strings.Contains(err.Error(), "exit code 7") {
		t.Fatalf("Execute() error = %v, want exit code 7", err)
	}
	if got := stdout.String(); got != "hello\n" {
		t.Fatalf("stdout = %q", got)
	}
	if got := stderr.String(); got != "warn\n" {
		t.Fatalf("stderr = %q", got)
	}
}

func TestExecuteResolvesOneBasedHostIndexes(t *testing.T) {
	originalDial := dialSession
	t.Cleanup(func() { dialSession = originalDial })

	var gotHosts []string
	var dialCount int
	dialSession = func(session Session) (net.Conn, error) {
		server, client := net.Pipe()
		dialCount++
		go func() {
			defer server.Close()
			dec := json.NewDecoder(server)
			enc := json.NewEncoder(server)

			var req Request
			_ = dec.Decode(&req)
			if dialCount == 1 {
				_ = enc.Encode(Event{Type: "pong"})
				return
			}

			gotHosts = append([]string{}, req.Hosts...)
			_ = enc.Encode(Event{Type: "done", ExitCode: 0})
		}()
		return client, nil
	}

	cacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)
	if err := SaveSession(Session{Name: "default", Network: "test", Address: "pipe", Hosts: []string{"web01", "web02", "db01"}}); err != nil {
		t.Fatalf("SaveSession() error = %v", err)
	}

	if err := Execute(ExecOptions{Name: "default", Command: "hostname", Hosts: []string{"2", "db01"}}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	want := []string{"web01", "db01"}
	if !reflect.DeepEqual(gotHosts, want) {
		t.Fatalf("resolved hosts = %#v, want %#v", gotHosts, want)
	}
}

func TestPingSessionSuccessAndFailure(t *testing.T) {
	originalDial := dialSession
	t.Cleanup(func() { dialSession = originalDial })

	t.Run("success", func(t *testing.T) {
		server, client := net.Pipe()
		dialSession = func(session Session) (net.Conn, error) {
			return client, nil
		}
		go func() {
			defer server.Close()
			dec := json.NewDecoder(server)
			var req Request
			_ = dec.Decode(&req)
			_ = json.NewEncoder(server).Encode(Event{Type: "pong"})
		}()
		if !PingSession(Session{Name: "x", Network: "test", Address: "ok"}) {
			t.Fatal("PingSession() = false, want true")
		}
	})

	t.Run("failure", func(t *testing.T) {
		dialSession = func(session Session) (net.Conn, error) {
			return nil, errors.New("dial failed")
		}
		if PingSession(Session{Name: "x", Network: "test", Address: "bad"}) {
			t.Fatal("PingSession() = true, want false")
		}
	})
}

func TestMarkSessionAliveReflectsPingResult(t *testing.T) {
	originalDial := dialSession
	t.Cleanup(func() { dialSession = originalDial })

	dialSession = func(session Session) (net.Conn, error) {
		server, client := net.Pipe()
		go func() {
			defer server.Close()
			var req Request
			_ = json.NewDecoder(server).Decode(&req)
			_ = json.NewEncoder(server).Encode(Event{Type: "pong"})
		}()
		return client, nil
	}
	session := Session{Name: "ok", Network: "test", Address: "addr"}
	MarkSessionAlive(&session)
	if !session.AliveChecked || session.Stale {
		t.Fatalf("session = %#v", session)
	}

	dialSession = func(session Session) (net.Conn, error) {
		return nil, errors.New("down")
	}
	session = Session{Name: "bad", Network: "test", Address: "addr"}
	MarkSessionAlive(&session)
	if !session.AliveChecked || !session.Stale {
		t.Fatalf("session = %#v", session)
	}
}

func TestDaemonExecRequestAggregatesExitCodeAndPrefixesErrors(t *testing.T) {
	daemon := NewDaemon("default", "", conf.Config{}, []string{"web01", "web02"}, nil)
	daemon.runCommandFn = func(host string, req Request, sendEvent func(Event)) (int, error) {
		if host == "web01" {
			sendEvent(Event{Type: "stdout", Host: host, Data: []byte(host + " ok\n")})
			return 0, nil
		}
		return 9, errors.New("boom")
	}

	var buf bytes.Buffer
	err := daemon.execRequest(json.NewEncoder(&buf), Request{Action: actionExec, Command: "hostname"})
	if err != nil {
		t.Fatalf("execRequest() error = %v", err)
	}

	events := decodeEvents(t, buf.Bytes())
	if len(events) == 0 {
		t.Fatal("no events")
	}
	foundDone := false
	foundPrefixedErr := false
	for _, event := range events {
		if event.Type == "done" && event.ExitCode == 9 {
			foundDone = true
		}
		if event.Type == "stderr" && strings.Contains(string(event.Data), "web02 :: boom") {
			foundPrefixedErr = true
		}
	}
	if !foundDone {
		t.Fatalf("done event missing: %#v", events)
	}
	if !foundPrefixedErr {
		t.Fatalf("prefixed stderr missing: %#v", events)
	}
}

func TestDaemonExecRequestSingleTargetDoesNotPrefixErrors(t *testing.T) {
	daemon := NewDaemon("default", "", conf.Config{}, []string{"web01"}, nil)
	daemon.runCommandFn = func(host string, req Request, sendEvent func(Event)) (int, error) {
		return 9, errors.New("boom")
	}

	var buf bytes.Buffer
	err := daemon.execRequest(json.NewEncoder(&buf), Request{Action: actionExec, Command: "hostname", Hosts: []string{"web01"}})
	if err != nil {
		t.Fatalf("execRequest() error = %v", err)
	}

	events := decodeEvents(t, buf.Bytes())
	foundPlainErr := false
	for _, event := range events {
		if event.Type == "stderr" && string(event.Data) == "boom\n" {
			foundPlainErr = true
		}
		if event.Type == "stderr" && strings.Contains(string(event.Data), "web01 :: boom") {
			t.Fatalf("unexpected prefixed stderr: %#v", events)
		}
	}
	if !foundPlainErr {
		t.Fatalf("plain stderr missing: %#v", events)
	}
}

func TestDaemonExecRequestRejectsRawMultiHost(t *testing.T) {
	daemon := NewDaemon("default", "", conf.Config{}, []string{"web01", "web02"}, nil)
	var buf bytes.Buffer
	err := daemon.execRequest(json.NewEncoder(&buf), Request{Action: actionExec, Command: "hostname", Raw: true})
	if err == nil || !strings.Contains(err.Error(), "exactly one host") {
		t.Fatalf("execRequest() error = %v", err)
	}
}

func TestDaemonGetOrReconnectReusesAliveConnection(t *testing.T) {
	conn := &fakeConn{}
	daemon := NewDaemon("default", "", conf.Config{}, []string{"web01"}, nil)
	daemon.conns["web01"] = conn
	daemon.connectFn = func(host string) (sessionConn, error) {
		return nil, errors.New("should not reconnect")
	}

	got, err := daemon.getOrReconnect("web01")
	if err != nil {
		t.Fatalf("getOrReconnect() error = %v", err)
	}
	if got != conn {
		t.Fatalf("getOrReconnect() returned unexpected conn")
	}
	if conn.checks != 1 {
		t.Fatalf("CheckClientAlive() count = %d", conn.checks)
	}
}

func TestDaemonGetOrReconnectReconnectsAfterAliveFailure(t *testing.T) {
	oldConn := &fakeConn{aliveErr: errors.New("dead")}
	newConn := &fakeConn{}
	daemon := NewDaemon("default", "", conf.Config{}, []string{"web01"}, nil)
	daemon.conns["web01"] = oldConn
	daemon.connectFn = func(host string) (sessionConn, error) {
		return newConn, nil
	}

	got, err := daemon.getOrReconnect("web01")
	if err != nil {
		t.Fatalf("getOrReconnect() error = %v", err)
	}
	if got != newConn {
		t.Fatalf("getOrReconnect() returned unexpected conn")
	}
	if !oldConn.closed {
		t.Fatal("old connection was not closed")
	}
}

func TestDaemonSetHealthDisconnectRemovesConnection(t *testing.T) {
	conn := &fakeConn{}
	daemon := NewDaemon("default", "", conf.Config{}, []string{"web01"}, nil)
	daemon.conns["web01"] = conn

	daemon.setHealth("web01", HostHealth{Connected: false, Error: "down"})
	if _, ok := daemon.conns["web01"]; ok {
		t.Fatal("connection still present after disconnect")
	}
	if !conn.closed {
		t.Fatal("connection was not closed")
	}
}

func TestExitCodeUsesGenericFallback(t *testing.T) {
	if got := exitCode(errors.New("failed")); got != 1 {
		t.Fatalf("exitCode() = %d", got)
	}
}

func TestEventWriterPrefixesNonRawOutput(t *testing.T) {
	var events []Event
	writer := &eventWriter{
		host:   "web01",
		stream: "stderr",
		send: func(event Event) {
			events = append(events, event)
		},
	}

	_, _ = writer.Write([]byte("warning\n"))
	writer.Flush()

	if len(events) != 1 || !strings.Contains(string(events[0].Data), "web01 :: warning") {
		t.Fatalf("events = %#v", events)
	}
}

func TestEventWriterSuppressesHeaderForSingleTarget(t *testing.T) {
	var events []Event
	writer := &eventWriter{
		host:           "web01",
		stream:         "stdout",
		suppressHeader: true,
		send: func(event Event) {
			events = append(events, event)
		},
	}

	_, _ = writer.Write([]byte("warning\n"))
	writer.Flush()

	if len(events) != 1 || string(events[0].Data) != "warning\n" {
		t.Fatalf("events = %#v", events)
	}
}

func TestEventWriterRawSuppressesSttyNoise(t *testing.T) {
	var events []Event
	writer := &eventWriter{
		host:   "web01",
		stream: "stderr",
		raw:    true,
		send: func(event Event) {
			events = append(events, event)
		},
	}

	_, _ = writer.Write([]byte("stty: standard input: Inappropriate ioctl for device\nreal\n"))
	if len(events) != 1 || string(events[0].Data) != "real\n" {
		t.Fatalf("events = %#v", events)
	}
}

func TestExecuteSendsBroadcastStdinInRequest(t *testing.T) {
	originalDial := dialSession
	t.Cleanup(func() { dialSession = originalDial })

	reqCh := make(chan Request, 1)
	var dialCount int
	dialSession = func(session Session) (net.Conn, error) {
		server, client := net.Pipe()
		dialCount++
		go func(call int) {
			defer server.Close()
			var req Request
			_ = json.NewDecoder(server).Decode(&req)
			if call == 1 {
				_ = json.NewEncoder(server).Encode(Event{Type: "pong"})
				return
			}
			reqCh <- req
			_ = json.NewEncoder(server).Encode(Event{Type: "done", ExitCode: 0})
		}(dialCount)
		return client, nil
	}

	cacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)
	if err := SaveSession(Session{Name: "default", Network: "test", Address: "pipe"}); err != nil {
		t.Fatalf("SaveSession() error = %v", err)
	}

	if err := Execute(ExecOptions{Name: "default", Command: "cat", Stdin: []byte("hello")}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	select {
	case req := <-reqCh:
		if string(req.Stdin) != "hello" {
			t.Fatalf("stdin = %q", string(req.Stdin))
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for request")
	}
}
