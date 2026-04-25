// Copyright (c) 2026 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package mux

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/acarl005/stripansi"
	sshlib "github.com/blacknon/go-sshlib"
	"github.com/blacknon/lssh/internal/common"
	conf "github.com/blacknon/lssh/internal/config"
	sshcmd "github.com/blacknon/lssh/internal/ssh"
	"github.com/blacknon/lssh/internal/termenv"
	"github.com/blacknon/lssh/providerapi"
	"github.com/blacknon/tvxterm"
	"github.com/creack/pty"
	"github.com/kballard/go-shellquote"
	"github.com/pkg/sftp"
)

// RemoteSession owns a mux pane SSH connection.
type RemoteSession struct {
	Server  string
	Config  conf.ServerConfig
	Notices []string

	Connect  *sshlib.Connect
	Terminal *sshlib.Terminal
	Input    io.WriteCloser
	Backend  *tvxterm.StreamBackend
	LogPath  string
}

// OpenSFTP opens an SFTP client that reuses the pane connection settings.
func (s *RemoteSession) OpenSFTP() (*sftp.Client, error) {
	if s == nil || s.Connect == nil {
		return nil, fmt.Errorf("sftp unavailable")
	}
	return s.Connect.OpenSFTP()
}

// SessionFactory creates remote sessions for panes.
type SessionFactory func(server string, cols, rows int) (*RemoteSession, error)

func NewSessionFactory(cfg conf.Config, command []string, options SessionOptions) SessionFactory {
	return func(server string, cols, rows int) (*RemoteSession, error) {
		run := &sshcmd.Run{
			ServerList:            []string{server},
			Conf:                  cfg,
			ControlMasterOverride: options.ControlMasterOverride,
			PortForward: append([]*conf.PortForward(nil),
				options.PortForward...,
			),
			ReverseDynamicPortForward:     options.ReverseDynamicPortForward,
			HTTPReverseDynamicPortForward: options.HTTPReverseDynamicPortForward,
			NFSReverseDynamicForwardPort:  options.NFSReverseDynamicForwardPort,
			NFSReverseDynamicForwardPath:  options.NFSReverseDynamicForwardPath,
			SMBReverseDynamicForwardPort:  options.SMBReverseDynamicForwardPort,
			SMBReverseDynamicForwardPath:  options.SMBReverseDynamicForwardPath,
			X11:                           options.X11,
			X11Trusted:                    options.X11Trusted,
			IsBashrc:                      options.IsBashrc,
			IsNotBashrc:                   options.IsNotBashrc,
		}
		run.CreateAuthMethodMap()
		serverConf := cfg.Server[server]
		if options.IsBashrc {
			serverConf.LocalRcUse = "yes"
		}
		if options.IsNotBashrc {
			serverConf.LocalRcUse = "no"
		}
		forwardConf := run.PrepareParallelForwardConfig(server)
		notices := []string{}
		if options.ParallelInfo != nil {
			notices = options.ParallelInfo(server)
		}

		if cfg.ServerUsesConnector(server) {
			return newConnectorRemoteSession(cfg, server, serverConf, notices, command, cols, rows)
		}
		connect, err := run.CreateSshConnect(server)
		if err != nil {
			return nil, err
		}
		return newSSHRemoteSession(cfg, server, serverConf, notices, connect, command, cols, rows, forwardConf)
	}
}

func writerWithLog(outputWriter *io.PipeWriter, logWriter *terminalLogWriter) io.Writer {
	if logWriter == nil {
		return outputWriter
	}
	return io.MultiWriter(outputWriter, logWriter)
}

type terminalLogWriter struct {
	mu         sync.Mutex
	file       *os.File
	timestamp  bool
	removeAnsi bool
	pending    string
}

type resizeDeduper struct {
	mu   sync.Mutex
	cols int
	rows int
	set  bool
}

func (d *resizeDeduper) ShouldSend(cols, rows int) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.set && d.cols == cols && d.rows == rows {
		return false
	}

	d.cols = cols
	d.rows = rows
	d.set = true
	return true
}

func dedupeResizeFunc(initialCols, initialRows int, resize func(cols, rows int) error) func(cols, rows int) error {
	state := &resizeDeduper{}
	if initialCols > 0 && initialRows > 0 {
		_ = state.ShouldSend(initialCols, initialRows)
	}

	return func(cols, rows int) error {
		cols = maxInt(cols, 1)
		rows = maxInt(rows, 1)
		if !state.ShouldSend(cols, rows) {
			return nil
		}
		return resize(cols, rows)
	}
}

func newTerminalLogWriter(path string, timestamp, removeAnsi bool) (*terminalLogWriter, error) {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0600)
	if err != nil {
		return nil, err
	}
	return &terminalLogWriter{
		file:       file,
		timestamp:  timestamp,
		removeAnsi: removeAnsi,
	}, nil
}

func (w *terminalLogWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file == nil {
		return 0, os.ErrClosed
	}

	if !w.timestamp && !w.removeAnsi {
		_, err := w.file.Write(p)
		return len(p), err
	}

	chunk := string(p)
	if w.removeAnsi {
		chunk = stripansi.Strip(chunk)
	}

	chunk = w.pending + chunk
	lines := strings.SplitAfter(chunk, "\n")
	if len(lines) > 0 && !strings.HasSuffix(chunk, "\n") {
		w.pending = lines[len(lines)-1]
		lines = lines[:len(lines)-1]
	} else {
		w.pending = ""
	}

	for _, line := range lines {
		if line == "" {
			continue
		}
		if w.timestamp {
			line = time.Now().Format("2006/01/02 15:04:05 ") + line
		}
		if _, err := io.WriteString(w.file, line); err != nil {
			return len(p), err
		}
	}

	return len(p), nil
}

func (w *terminalLogWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file == nil {
		return nil
	}
	if w.pending != "" {
		line := w.pending
		if w.timestamp {
			line = time.Now().Format("2006/01/02 15:04:05 ") + line
		}
		if _, err := io.WriteString(w.file, line); err != nil {
			_ = w.file.Close()
			w.file = nil
			w.pending = ""
			return err
		}
		w.pending = ""
	}
	err := w.file.Close()
	w.file = nil
	return err
}

type nopWriteCloser struct {
	io.Writer
}

func (n nopWriteCloser) Close() error {
	return nil
}

func newConnectorRemoteSession(cfg conf.Config, server string, serverConf conf.ServerConfig, notices []string, command []string, cols, rows int) (*RemoteSession, error) {
	operation := conf.ConnectorOperation{Name: "shell"}
	if len(command) > 0 {
		operation = conf.ConnectorOperation{
			Name:    "exec_pty",
			Command: append([]string(nil), command...),
		}
	}

	prepared, err := cfg.PrepareConnectorRuntime(server, operation)
	if err != nil {
		return nil, err
	}
	if len(command) > 0 && !prepared.Supported {
		fallbackPrepared, fallbackErr := cfg.PrepareConnectorRuntime(server, conf.ConnectorOperation{
			Name:    "exec",
			Command: append([]string(nil), command...),
		})
		if fallbackErr != nil {
			return nil, fallbackErr
		}
		prepared = fallbackPrepared
	}
	if !prepared.Supported {
		reason := prepared.Reason
		if reason == "" {
			reason = "connector operation is not supported"
		}
		return nil, fmt.Errorf("%s", reason)
	}

	switch {
	case prepared.Command != nil:
		if len(command) == 0 {
			return newCommandShellRemoteSession(cfg, server, serverConf, notices, *prepared.Command, cols, rows)
		}
		return newCommandExecRemoteSession(cfg, server, serverConf, notices, *prepared.Command)
	case prepared.ManagedSSH != nil:
		run := &sshcmd.Run{
			ServerList: []string{server},
			Conf:       cfg,
		}
		run.CreateAuthMethodMap()
		connect, err := run.CreateConnectorManagedSSHConnectDirect(server)
		if err != nil {
			return nil, err
		}
		return newSSHRemoteSession(cfg, server, serverConf, notices, connect, command, cols, rows, conf.ServerConfig{})
	case prepared.ProviderManagedPlan != nil:
		if len(command) == 0 {
			return newProviderManagedShellRemoteSession(cfg, server, serverConf, notices, *prepared.ProviderManagedPlan, cols, rows)
		}
		return newProviderManagedExecRemoteSession(cfg, server, serverConf, notices, *prepared.ProviderManagedPlan, cols, rows)
	default:
		return nil, fmt.Errorf("server %q connector %q returned unsupported plan kind %q", server, prepared.ConnectorName, prepared.PlanKind)
	}
}

func newSSHRemoteSession(cfg conf.Config, server string, serverConf conf.ServerConfig, notices []string, connect *sshlib.Connect, command []string, cols, rows int, forwardConf conf.ServerConfig) (*RemoteSession, error) {
	if len(forwardConf.Forwards) > 0 || forwardConf.DynamicPortForward != "" || forwardConf.HTTPDynamicPortForward != "" || forwardConf.ReverseDynamicPortForward != "" || forwardConf.HTTPReverseDynamicPortForward != "" || forwardConf.NFSReverseDynamicForwardPort != "" || forwardConf.SMBReverseDynamicForwardPort != "" {
		if err := sshcmd.StartParallelForwards(connect, forwardConf); err != nil {
			if connect.Client != nil {
				_ = connect.Client.Close()
			}
			return nil, err
		}
	}

	opts := sshlib.TerminalOptions{
		Term: termenv.Current(),
		Cols: maxInt(cols, 80),
		Rows: maxInt(rows, 24),
	}
	if len(command) == 0 {
		opts.StartShell = true
	} else {
		opts.Command = termenv.WrapShellExec(shellquote.Join(command...))
	}
	if len(command) == 0 && serverConf.LocalRcUse == "yes" {
		opts.StartShell = false
		opts.Command = buildLocalRcCommand(
			serverConf.LocalRcPath,
			serverConf.LocalRcDecodeCmd,
			serverConf.LocalRcCompress,
			serverConf.LocalRcUncompressCmd,
		)
	}

	terminal, err := connect.OpenTerminal(opts)
	if err != nil {
		if connect.Client != nil {
			_ = connect.Client.Close()
		}
		return nil, err
	}

	outputReader, outputWriter := io.Pipe()
	var logWriter *terminalLogWriter
	logPath := ""
	if cfg.Log.Enable {
		logPath, err = buildMuxLogPath(cfg.Log, server)
		if err != nil {
			_ = outputWriter.CloseWithError(err)
			_ = terminal.Close()
			if connect.Client != nil {
				_ = connect.Client.Close()
			}
			return nil, err
		}
		logWriter, err = newTerminalLogWriter(logPath, cfg.Log.Timestamp, cfg.Log.RemoveAnsiCode)
		if err != nil {
			_ = outputWriter.CloseWithError(err)
			_ = terminal.Close()
			if connect.Client != nil {
				_ = connect.Client.Close()
			}
			return nil, err
		}
	}
	var copyWG sync.WaitGroup
	copyWG.Add(2)

	go copyPipe(&copyWG, writerWithLog(outputWriter, logWriter), terminal.Stdout)
	go copyPipe(&copyWG, writerWithLog(outputWriter, logWriter), terminal.Stderr)

	go func() {
		copyWG.Wait()
		_ = outputWriter.Close()
	}()

	var closeOnce sync.Once
	closeFn := func() error {
		var closeErr error
		closeOnce.Do(func() {
			_ = outputWriter.Close()
			if logWriter != nil {
				_ = logWriter.Close()
			}
			if err := terminal.Close(); err != nil {
				closeErr = err
			}
			if connect.Client != nil {
				if err := connect.Client.Close(); closeErr == nil && err != nil {
					closeErr = err
				}
			}
		})
		return closeErr
	}

	return &RemoteSession{
		Server:   server,
		Config:   serverConf,
		Notices:  append([]string(nil), notices...),
		Connect:  connect,
		Terminal: terminal,
		LogPath:  logPath,
		Backend: tvxterm.NewStreamBackend(
			outputReader,
			terminal.Stdin,
			dedupeResizeFunc(opts.Cols, opts.Rows, func(cols, rows int) error {
				return terminal.Resize(cols, rows)
			}),
			closeFn,
		),
	}, nil
}

func newCommandShellRemoteSession(cfg conf.Config, server string, serverConf conf.ServerConfig, notices []string, plan conf.ConnectorCommandPlan, cols, rows int) (*RemoteSession, error) {
	if plan.Program == "" {
		return nil, fmt.Errorf("connector command plan is missing program")
	}

	cmd := exec.Command(plan.Program, plan.Args...)
	cmd.Env = mergedCommandPlanEnv(plan.Env)
	tty, err := pty.StartWithSize(cmd, &pty.Winsize{
		Cols: uint16(maxInt(cols, 80)),
		Rows: uint16(maxInt(rows, 24)),
	})
	if err != nil {
		return nil, err
	}

	outputReader, outputWriter := io.Pipe()
	var logWriter *terminalLogWriter
	logPath := ""
	if cfg.Log.Enable {
		logPath, err = buildMuxLogPath(cfg.Log, server)
		if err != nil {
			_ = outputWriter.CloseWithError(err)
			_ = tty.Close()
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			return nil, err
		}
		logWriter, err = newTerminalLogWriter(logPath, cfg.Log.Timestamp, cfg.Log.RemoveAnsiCode)
		if err != nil {
			_ = outputWriter.CloseWithError(err)
			_ = tty.Close()
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			return nil, err
		}
	}

	var copyWG sync.WaitGroup
	copyWG.Add(1)
	go copyPipe(&copyWG, writerWithLog(outputWriter, logWriter), tty)

	var waitErr error
	waitDone := make(chan struct{})
	go func() {
		waitErr = cmd.Wait()
		close(waitDone)
	}()
	go func() {
		defer func() {
			copyWG.Wait()
		}()
		<-waitDone
		if waitErr != nil {
			_ = outputWriter.CloseWithError(waitErr)
			return
		}
		_ = outputWriter.Close()
	}()

	var closeOnce sync.Once
	closeFn := func() error {
		var closeErr error
		closeOnce.Do(func() {
			_ = outputWriter.Close()
			if logWriter != nil {
				_ = logWriter.Close()
			}
			if err := tty.Close(); closeErr == nil && err != nil {
				closeErr = err
			}
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
			<-waitDone
			if closeErr == nil && waitErr != nil && !isExpectedProcessExit(waitErr) {
				closeErr = waitErr
			}
		})
		return closeErr
	}

	return &RemoteSession{
		Server:  server,
		Config:  serverConf,
		Notices: append([]string(nil), notices...),
		Input:   tty,
		LogPath: logPath,
		Backend: tvxterm.NewStreamBackend(
			outputReader,
			tty,
			dedupeResizeFunc(maxInt(cols, 80), maxInt(rows, 24), func(cols, rows int) error {
				return pty.Setsize(tty, &pty.Winsize{
					Cols: uint16(cols),
					Rows: uint16(rows),
				})
			}),
			closeFn,
		),
	}, nil
}

func newCommandExecRemoteSession(cfg conf.Config, server string, serverConf conf.ServerConfig, notices []string, plan conf.ConnectorCommandPlan) (*RemoteSession, error) {
	if plan.Program == "" {
		return nil, fmt.Errorf("connector command plan is missing program")
	}

	cmd := exec.Command(plan.Program, plan.Args...)
	cmd.Env = mergedCommandPlanEnv(plan.Env)
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	outputReader, outputWriter := io.Pipe()
	var logWriter *terminalLogWriter
	logPath := ""
	if cfg.Log.Enable {
		logPath, err = buildMuxLogPath(cfg.Log, server)
		if err != nil {
			_ = outputWriter.CloseWithError(err)
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			return nil, err
		}
		logWriter, err = newTerminalLogWriter(logPath, cfg.Log.Timestamp, cfg.Log.RemoveAnsiCode)
		if err != nil {
			_ = outputWriter.CloseWithError(err)
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			return nil, err
		}
	}

	var copyWG sync.WaitGroup
	copyWG.Add(2)
	go copyPipe(&copyWG, writerWithLog(outputWriter, logWriter), stdoutPipe)
	go copyPipe(&copyWG, writerWithLog(outputWriter, logWriter), stderrPipe)

	var waitErr error
	waitDone := make(chan struct{})
	go func() {
		waitErr = cmd.Wait()
		close(waitDone)
	}()
	go func() {
		defer func() {
			copyWG.Wait()
		}()
		<-waitDone
		if waitErr != nil {
			_ = outputWriter.CloseWithError(waitErr)
			return
		}
		_ = outputWriter.Close()
	}()

	var closeOnce sync.Once
	closeFn := func() error {
		var closeErr error
		closeOnce.Do(func() {
			_ = outputWriter.Close()
			if logWriter != nil {
				_ = logWriter.Close()
			}
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
			<-waitDone
			if closeErr == nil && waitErr != nil && !isExpectedProcessExit(waitErr) {
				closeErr = waitErr
			}
		})
		return closeErr
	}

	return &RemoteSession{
		Server:  server,
		Config:  serverConf,
		Notices: append([]string(nil), notices...),
		Input:   nopWriteCloser{Writer: io.Discard},
		LogPath: logPath,
		Backend: tvxterm.NewStreamBackend(
			outputReader,
			nopWriteCloser{Writer: io.Discard},
			func(cols, rows int) error { return nil },
			closeFn,
		),
	}, nil
}

func newProviderManagedShellRemoteSession(cfg conf.Config, server string, serverConf conf.ServerConfig, notices []string, plan providerapi.ConnectorPlan, cols, rows int) (*RemoteSession, error) {
	startupCommand := ""
	startupMarker := ""
	if serverConf.LocalRcUse == "yes" {
		startupCommand = buildLocalRcCommand(
			serverConf.LocalRcPath,
			serverConf.LocalRcDecodeCmd,
			serverConf.LocalRcCompress,
			serverConf.LocalRcUncompressCmd,
		)
		startupMarker = sshcmd.InteractiveLocalRCStartupMarker()
	}

	cmd, resultPath, err := cfg.PrepareProviderRuntimeCommand(context.Background(), server, plan, providerapi.MethodConnectorShell, startupCommand, startupMarker, false)
	if err != nil {
		return nil, err
	}
	return newProviderRuntimePTYRemoteSession(cfg, server, serverConf, notices, cmd, resultPath, cols, rows)
}

func newProviderManagedExecRemoteSession(cfg conf.Config, server string, serverConf conf.ServerConfig, notices []string, plan providerapi.ConnectorPlan, cols, rows int) (*RemoteSession, error) {
	cmd, resultPath, err := cfg.PrepareProviderRuntimeCommand(context.Background(), server, plan, providerapi.MethodConnectorExec, "", "", true)
	if err != nil {
		return nil, err
	}
	return newProviderRuntimePTYRemoteSession(cfg, server, serverConf, notices, cmd, resultPath, cols, rows)
}

func newProviderRuntimePTYRemoteSession(cfg conf.Config, server string, serverConf conf.ServerConfig, notices []string, cmd *exec.Cmd, resultPath string, cols, rows int) (*RemoteSession, error) {
	tty, err := pty.StartWithSize(cmd, &pty.Winsize{
		Cols: uint16(maxInt(cols, 80)),
		Rows: uint16(maxInt(rows, 24)),
	})
	if err != nil {
		if resultPath != "" {
			_ = os.Remove(resultPath)
		}
		return nil, err
	}

	outputReader, outputWriter := io.Pipe()
	var logWriter *terminalLogWriter
	logPath := ""
	if cfg.Log.Enable {
		logPath, err = buildMuxLogPath(cfg.Log, server)
		if err != nil {
			_ = outputWriter.CloseWithError(err)
			_ = tty.Close()
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
			_ = cmd.Wait()
			if resultPath != "" {
				_ = os.Remove(resultPath)
			}
			return nil, err
		}
		logWriter, err = newTerminalLogWriter(logPath, cfg.Log.Timestamp, cfg.Log.RemoveAnsiCode)
		if err != nil {
			_ = outputWriter.CloseWithError(err)
			_ = tty.Close()
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
			_ = cmd.Wait()
			if resultPath != "" {
				_ = os.Remove(resultPath)
			}
			return nil, err
		}
	}

	var copyWG sync.WaitGroup
	copyWG.Add(1)
	go copyPipe(&copyWG, writerWithLog(outputWriter, logWriter), tty)

	var waitErr error
	waitDone := make(chan struct{})
	go func() {
		waitErr = cmd.Wait()
		close(waitDone)
	}()
	go func() {
		defer func() {
			copyWG.Wait()
		}()
		<-waitDone
		if waitErr != nil {
			_ = outputWriter.CloseWithError(waitErr)
			return
		}
		_ = outputWriter.Close()
	}()

	var closeOnce sync.Once
	closeFn := func() error {
		var closeErr error
		closeOnce.Do(func() {
			_ = outputWriter.Close()
			if logWriter != nil {
				_ = logWriter.Close()
			}
			if err := tty.Close(); closeErr == nil && err != nil {
				closeErr = err
			}
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
			<-waitDone
			if resultPath != "" {
				_ = os.Remove(resultPath)
			}
			if closeErr == nil && waitErr != nil && !isExpectedProcessExit(waitErr) {
				closeErr = waitErr
			}
		})
		return closeErr
	}

	return &RemoteSession{
		Server:  server,
		Config:  serverConf,
		Notices: append([]string(nil), notices...),
		Input:   tty,
		LogPath: logPath,
		Backend: tvxterm.NewStreamBackend(
			outputReader,
			tty,
			dedupeResizeFunc(maxInt(cols, 80), maxInt(rows, 24), func(cols, rows int) error {
				return pty.Setsize(tty, &pty.Winsize{
					Cols: uint16(cols),
					Rows: uint16(rows),
				})
			}),
			closeFn,
		),
	}, nil
}

func newReaderBackedRemoteSession(cfg conf.Config, server string, serverConf conf.ServerConfig, notices []string, stdin io.Writer, stdout, stderr io.Reader, waitFn func() error, closeResource func() error) (*RemoteSession, error) {
	outputReader, outputWriter := io.Pipe()
	var logWriter *terminalLogWriter
	var err error
	logPath := ""
	if cfg.Log.Enable {
		logPath, err = buildMuxLogPath(cfg.Log, server)
		if err != nil {
			_ = outputWriter.CloseWithError(err)
			if closeResource != nil {
				_ = closeResource()
			}
			return nil, err
		}
		logWriter, err = newTerminalLogWriter(logPath, cfg.Log.Timestamp, cfg.Log.RemoveAnsiCode)
		if err != nil {
			_ = outputWriter.CloseWithError(err)
			if closeResource != nil {
				_ = closeResource()
			}
			return nil, err
		}
	}

	var copyWG sync.WaitGroup
	if stdout != nil {
		copyWG.Add(1)
		go copyPipe(&copyWG, writerWithLog(outputWriter, logWriter), stdout)
	}
	if stderr != nil {
		copyWG.Add(1)
		go copyPipe(&copyWG, writerWithLog(outputWriter, logWriter), stderr)
	}

	var waitErr error
	waitDone := make(chan struct{})
	go func() {
		waitErr = waitFn()
		close(waitDone)
	}()
	go func() {
		defer func() {
			copyWG.Wait()
		}()
		<-waitDone
		if waitErr != nil {
			_ = outputWriter.CloseWithError(waitErr)
			return
		}
		_ = outputWriter.Close()
	}()

	var closeOnce sync.Once
	closeFn := func() error {
		var closeErr error
		closeOnce.Do(func() {
			_ = outputWriter.Close()
			if logWriter != nil {
				_ = logWriter.Close()
			}
			if closeResource != nil {
				if err := closeResource(); closeErr == nil && err != nil {
					closeErr = err
				}
			}
			<-waitDone
			if closeErr == nil && waitErr != nil && !isExpectedProcessExit(waitErr) {
				closeErr = waitErr
			}
		})
		return closeErr
	}

	input := nopWriteCloser{Writer: io.Discard}
	if stdin != nil {
		input = nopWriteCloser{Writer: stdin}
	}

	return &RemoteSession{
		Server:  server,
		Config:  serverConf,
		Notices: append([]string(nil), notices...),
		Input:   input,
		LogPath: logPath,
		Backend: tvxterm.NewStreamBackend(
			outputReader,
			input,
			func(cols, rows int) error { return nil },
			closeFn,
		),
	}, nil
}

func detailString(details map[string]interface{}, key string) string {
	if details == nil {
		return ""
	}
	if value, ok := details[key]; ok && value != nil {
		return fmt.Sprint(value)
	}
	return ""
}

func detailStringSlice(details map[string]interface{}, key string) []string {
	if details == nil {
		return nil
	}
	raw, ok := details[key]
	if !ok || raw == nil {
		return nil
	}
	switch typed := raw.(type) {
	case []string:
		return append([]string(nil), typed...)
	case []interface{}:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if item == nil {
				continue
			}
			out = append(out, fmt.Sprint(item))
		}
		return out
	default:
		return []string{fmt.Sprint(typed)}
	}
}

func mergedCommandPlanEnv(env map[string]string) []string {
	return termenv.MergeEnv(env)
}

func isExpectedProcessExit(err error) bool {
	if err == nil {
		return true
	}
	if exitErr, ok := err.(*exec.ExitError); ok && exitErr != nil {
		return true
	}
	return false
}

func buildMuxLogPath(logConf conf.LogConfig, server string) (string, error) {
	name := server
	if regexp.MustCompile(`:`).MatchString(name) {
		slice := strings.SplitN(name, ":", 2)
		name = slice[1]
	}

	u, _ := user.Current()
	dir := logConf.Dir
	dir = strings.Replace(dir, "~", u.HomeDir, 1)
	dir = strings.Replace(dir, "<Date>", time.Now().Format("20060102"), 1)
	dir = strings.Replace(dir, "<Hostname>", name, 1)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}

	file := time.Now().Format("20060102_150405") + "_" + name + ".log"
	return dir + "/" + file, nil
}

func buildLocalRcCommand(localrcPath []string, decoder string, compress bool, uncompress string) string {
	return fmt.Sprintf(
		"export TERM=%s; %s",
		shellquote.Join(termenv.Current()),
		sshcmd.BuildInteractiveLocalRCShellCommand(localrcPath, decoder, compress, uncompress),
	)
}

func localrcArchiveMode(compress bool) int {
	if compress {
		return common.ARCHIVE_GZIP
	}
	return common.ARCHIVE_NONE
}

func copyPipe(wg *sync.WaitGroup, writer io.Writer, reader io.Reader) {
	defer wg.Done()
	_, _ = io.Copy(writer, reader)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
