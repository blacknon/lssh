package monitor

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
	"github.com/blacknon/lssh/internal/mux"
	"github.com/blacknon/tvxterm"
)

func (m *Monitor) createSharedTopTerminalSession(server string, cols, rows int) (*mux.RemoteSession, error) {
	node := m.GetNode(server)
	if node == nil {
		return nil, fmt.Errorf("monitor node not found: %s", server)
	}
	if node.con == nil || node.con.Connect == nil {
		return nil, fmt.Errorf("monitor connection unavailable: %s", server)
	}
	if !node.CheckClientAlive() {
		return nil, fmt.Errorf("monitor connection is not alive: %s", server)
	}

	return newSharedMonitorRemoteSession(server, m.r.Conf, node.con.Connect, cols, rows)
}

func newSharedMonitorRemoteSession(server string, cfg conf.Config, connect *sshlib.Connect, cols, rows int) (*mux.RemoteSession, error) {
	if connect == nil {
		return nil, fmt.Errorf("ssh connect is nil")
	}

	serverConf := cfg.Server[server]
	opts := sshlib.TerminalOptions{
		Term:       "xterm-256color",
		Cols:       sharedTerminalMaxInt(cols, 80),
		Rows:       sharedTerminalMaxInt(rows, 24),
		StartShell: true,
	}

	if serverConf.LocalRcUse == "yes" {
		opts.StartShell = false
		opts.Command = buildSharedLocalRcCommand(
			serverConf.LocalRcPath,
			serverConf.LocalRcDecodeCmd,
			serverConf.LocalRcCompress,
			serverConf.LocalRcUncompressCmd,
		)
	}

	terminal, err := connect.OpenTerminal(opts)
	if err != nil {
		return nil, err
	}

	outputReader, outputWriter := io.Pipe()
	var logWriter *sharedTerminalLogWriter
	logPath := ""
	if cfg.Log.Enable {
		logPath, err = buildSharedTerminalLogPath(cfg.Log, server)
		if err != nil {
			_ = outputWriter.CloseWithError(err)
			_ = terminal.Close()
			return nil, err
		}
		logWriter, err = newSharedTerminalLogWriter(logPath, cfg.Log.Timestamp, cfg.Log.RemoveAnsiCode)
		if err != nil {
			_ = outputWriter.CloseWithError(err)
			_ = terminal.Close()
			return nil, err
		}
	}

	var copyWG sync.WaitGroup
	copyWG.Add(2)

	go sharedTerminalCopyPipe(&copyWG, sharedTerminalWriterWithLog(outputWriter, logWriter), terminal.Stdout)
	go sharedTerminalCopyPipe(&copyWG, sharedTerminalWriterWithLog(outputWriter, logWriter), terminal.Stderr)

	go func() {
		copyWG.Wait()
		_ = outputWriter.Close()
	}()

	closeFn := newSharedTerminalCloseFunc(outputWriter, logWriter, terminal, nil, false)

	return &mux.RemoteSession{
		Server:   server,
		Config:   serverConf,
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

func newSharedTerminalCloseFunc(outputWriter *io.PipeWriter, logWriter io.Closer, terminal io.Closer, client io.Closer, closeClient bool) func() error {
	var closeOnce sync.Once

	return func() error {
		var closeErr error
		closeOnce.Do(func() {
			if outputWriter != nil {
				_ = outputWriter.Close()
			}
			if logWriter != nil {
				if err := logWriter.Close(); closeErr == nil && err != nil {
					closeErr = err
				}
			}
			if terminal != nil {
				if err := terminal.Close(); closeErr == nil && err != nil {
					closeErr = err
				}
			}
			if closeClient && client != nil {
				if err := client.Close(); closeErr == nil && err != nil {
					closeErr = err
				}
			}
		})
		return closeErr
	}
}

func sharedTerminalWriterWithLog(outputWriter *io.PipeWriter, logWriter *sharedTerminalLogWriter) io.Writer {
	if logWriter == nil {
		return outputWriter
	}
	return io.MultiWriter(outputWriter, logWriter)
}

type sharedTerminalLogWriter struct {
	mu         sync.Mutex
	file       *os.File
	timestamp  bool
	removeAnsi bool
	pending    string
}

func newSharedTerminalLogWriter(path string, timestamp, removeAnsi bool) (*sharedTerminalLogWriter, error) {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0600)
	if err != nil {
		return nil, err
	}
	return &sharedTerminalLogWriter{
		file:       file,
		timestamp:  timestamp,
		removeAnsi: removeAnsi,
	}, nil
}

func (w *sharedTerminalLogWriter) Write(p []byte) (int, error) {
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

func (w *sharedTerminalLogWriter) Close() error {
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

func buildSharedTerminalLogPath(logConf conf.LogConfig, server string) (string, error) {
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

func buildSharedLocalRcCommand(localrcPath []string, decoder string, compress bool, uncompress string) string {
	if len(localrcPath) == 0 {
		localrcPath = []string{"~/.bashrc"}
	}

	rcData, _ := common.GetFilesBase64(localrcPath, sharedLocalrcArchiveMode(compress))

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

func sharedLocalrcArchiveMode(compress bool) int {
	if compress {
		return common.ARCHIVE_GZIP
	}
	return common.ARCHIVE_NONE
}

func sharedTerminalCopyPipe(wg *sync.WaitGroup, writer io.Writer, reader io.Reader) {
	defer wg.Done()
	_, _ = io.Copy(writer, reader)
}

func sharedTerminalMaxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
