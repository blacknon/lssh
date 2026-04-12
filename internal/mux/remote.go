// Copyright (c) 2026 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package mux

import (
	"fmt"
	"io"
	"os"
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
	"github.com/blacknon/tvxterm"
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
		forwardConf := run.PrepareParallelForwardConfig(server)
		notices := []string{}
		if options.ParallelInfo != nil {
			notices = options.ParallelInfo(server)
		}

		connect, err := run.CreateSshConnect(server)
		if err != nil {
			return nil, err
		}
		if err := sshcmd.StartParallelForwards(connect, forwardConf); err != nil {
			if connect.Client != nil {
				_ = connect.Client.Close()
			}
			return nil, err
		}

		opts := sshlib.TerminalOptions{
			Term: "xterm-256color",
			Cols: maxInt(cols, 80),
			Rows: maxInt(rows, 24),
		}
		if len(command) == 0 {
			opts.StartShell = true
		} else {
			opts.Command = shellquote.Join(command...)
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
				func(cols, rows int) error {
					return terminal.Resize(cols, rows)
				},
				closeFn,
			),
		}, nil
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
	if len(localrcPath) == 0 {
		localrcPath = []string{"~/.bashrc"}
	}

	rcData, _ := common.GetFilesBase64(localrcPath, localrcArchiveMode(compress))

	if uncompress == "" {
		uncompress = "gzip -d"
	}

	switch {
	case !compress && decoder != "":
		return fmt.Sprintf("bash --noprofile --rcfile <(echo %s | %s); exit 0", rcData, decoder)
	case !compress && decoder == "":
		return fmt.Sprintf("bash --noprofile --rcfile <(echo %s | ( (base64 --help | grep -q coreutils) && base64 -d <(cat) || base64 -D <(cat) ) ); exit 0", rcData)
	case compress && decoder != "":
		return fmt.Sprintf("bash --noprofile --rcfile <(echo %s | %s | %s); exit 0", rcData, decoder, uncompress)
	default:
		return fmt.Sprintf("bash --noprofile --rcfile <(echo %s | ( (base64 --help | grep -q coreutils) && base64 -d <(cat) || base64 -D <(cat) ) | %s); exit 0", rcData, uncompress)
	}
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
