// Copyright (c) 2022 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package ssh

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/user"
	"regexp"
	"strings"
	"time"

	"github.com/blacknon/go-sshlib"
	"github.com/blacknon/lssh/internal/common"
	"golang.org/x/crypto/ssh"
)

// run shell
func (r *Run) shell() (err error) {
	// server config
	server := r.ServerList[0]
	config := r.resolveShellConfig(server)

	if r.usesConnector(server) {
		return r.runConnectorShellWithConfig(server, config)
	}

	// check count AuthMethod
	if len(r.serverAuthMethodMap[server]) == 0 {
		msg := fmt.Sprintf("Error: %s is No AuthMethod.\n", server)
		err = errors.New(msg)
		return
	}

	// header
	r.PrintSelectServer()
	for _, fw := range config.Forwards {
		r.printPortForward(fw.Mode, fw.Local, fw.Remote)
	}
	r.printDynamicPortForward(config.DynamicPortForward)
	r.printReverseDynamicPortForward(config.ReverseDynamicPortForward)
	r.printHTTPDynamicPortForward(config.HTTPDynamicPortForward)
	r.printNFSDynamicForward(config.NFSDynamicForwardPort, config.NFSDynamicForwardPath)
	r.printNFSReverseDynamicForward(config.NFSReverseDynamicForwardPort, config.NFSReverseDynamicForwardPath)
	r.printSMBDynamicForward(config.SMBDynamicForwardPort, config.SMBDynamicForwardPath)
	r.printSMBReverseDynamicForward(config.SMBReverseDynamicForwardPort, config.SMBReverseDynamicForwardPath)
	r.printProxy(server)

	connect, err := r.CreateSshConnect(server)
	if err != nil {
		return
	}

	if connect.IsControlClient() {
		if validateErr := validateControlMasterClient(connect); validateErr != nil {
			if isControlMasterRemoteExit255(validateErr) {
				if connect.ControlPath != "" {
					_ = os.Remove(connect.ControlPath)
				}
				connect, err = r.CreateSshConnect(server)
				if err != nil {
					return
				}
			} else {
				return validateErr
			}
		}
	}

	// Print connection info (Local rc, ControlMaster state, etc.).
	r.PrintConnectInfo(server, connect, config)

	// Record whether this is a ControlMaster client. We still need to run
	// pre/post commands and logging; only session creation differs later.
	isControlClient := connect.IsControlClient()

	var session *ssh.Session
	if !isControlClient {
		// Create session
		session, err = connect.CreateSession()
		if err != nil {
			return
		}

		// ssh-agent
		if config.SSHAgentUse {
			connect.Agent = r.agent
			connect.ForwardSshAgent(session)
		}
	}

	// Local/Remote Port Forwarding
	for _, fw := range config.Forwards {
		err = r.startPortForward(connect, fw)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}

	// Dynamic Port Forwarding
	if config.DynamicPortForward != "" {
		go connect.TCPDynamicForward("localhost", config.DynamicPortForward)
	}

	// Reverse Dynamic Port Forwarding
	if config.ReverseDynamicPortForward != "" {
		go connect.TCPReverseDynamicForward("localhost", config.ReverseDynamicPortForward)
	}

	// HTTP Dynamic Port Forwarding
	if config.HTTPDynamicPortForward != "" {
		go connect.HTTPDynamicForward("localhost", config.HTTPDynamicPortForward)
	}

	// HTTP Reverse Dynamic Port Forwarding
	if config.HTTPReverseDynamicPortForward != "" {
		go connect.HTTPReverseDynamicForward("localhost", config.HTTPReverseDynamicPortForward)
	}

	// NFS Dynamic Forwarding
	if config.NFSDynamicForwardPort != "" && config.NFSDynamicForwardPath != "" {
		go connect.NFSForward("localhost", config.NFSDynamicForwardPort, config.NFSDynamicForwardPath)
	}

	// NFS Reverse Dynamic Forwarding
	if config.NFSReverseDynamicForwardPort != "" && config.NFSReverseDynamicForwardPath != "" {
		go connect.NFSReverseForward("localhost", config.NFSReverseDynamicForwardPort, config.NFSReverseDynamicForwardPath)
	}

	if config.SMBDynamicForwardPort != "" && config.SMBDynamicForwardPath != "" {
		go connect.SMBForward("localhost", config.SMBDynamicForwardPort, "", config.SMBDynamicForwardPath)
	}

	if config.SMBReverseDynamicForwardPort != "" && config.SMBReverseDynamicForwardPath != "" {
		go connect.SMBReverseForward("localhost", config.SMBReverseDynamicForwardPort, "", config.SMBReverseDynamicForwardPath)
	}

	// If started as daemonized child, notify parent that forwarding is ready
	notifyParentReady()

	// switch check Not-execute flag
	// TODO(blacknon): Backgroundフラグを実装したら追加
	switch {
	case r.IsNone:
		r.noneExecute(connect)

	default:
		// run pre local command
		if config.PreCmd != "" {
			execLocalCommand(config.PreCmd)
		}

		// defer run post local command
		if config.PostCmd != "" {
			defer execLocalCommand(config.PostCmd)
		}

		// if terminal log enable
		logConf := r.Conf.Log
		if logConf.Enable {
			logPath := r.getLogPath(server)

			// Check logging with remove ANSI code flag.
			if logConf.RemoveAnsiCode {
				connect.SetLogWithRemoveAnsiCode(logPath, logConf.Timestamp)
			} else {
				connect.SetLog(logPath, logConf.Timestamp)
			}
		}

		// TODO(blacknon): local rc file add
		// No special handling for ControlMaster: allow agent/X11 forwarding to proceed normally.

		if config.LocalRcUse == "yes" {
			err = localrcShell(connect, session, config.LocalRcPath, config.LocalRcDecodeCmd, config.LocalRcCompress, config.LocalRcUncompressCmd)
		} else {
			// Connect shell
			err = connect.Shell(session)
		}

		// No special handling for ControlMaster: allow agent/X11 forwarding to proceed normally.
	}

	return
}

// getLogPath return log file path.
func (r *Run) getLogPath(server string) (logPath string) {
	// check regex
	// if ~/.ssh/config, in ":"
	reg := regexp.MustCompile(`:`)

	if reg.MatchString(server) {
		slice := strings.SplitN(server, ":", 2)
		server = slice[1]
	}

	dir, err := r.getLogDirPath(server)
	if err != nil {
		log.Println(err)
	}

	file := time.Now().Format("20060102_150405") + "_" + server + ".log"
	logPath = dir + "/" + file

	return
}

// getLogDirPath return log directory path
func (r *Run) getLogDirPath(server string) (dir string, err error) {
	u, _ := user.Current()
	logConf := r.Conf.Log

	// expantion variable
	dir = logConf.Dir
	dir = strings.Replace(dir, "~", u.HomeDir, 1)
	dir = strings.Replace(dir, "<Date>", time.Now().Format("20060102"), 1)
	dir = strings.Replace(dir, "<Hostname>", server, 1)

	// create directory
	err = os.MkdirAll(dir, 0700)

	return
}

// noneExecute is not execute command and shell.
func (r *Run) noneExecute(con *sshlib.Connect) (err error) {
loop:
	for {
		select {
		case <-time.After(500 * time.Millisecond):
			// 接続状況チェック
			err = con.CheckClientAlive()
			if err != nil {
				// error
				fmt.Fprintf(os.Stderr, "Exit Connect, Error: %s\n", err)

				// close sftp client
				con.Client.Close()

				break loop
			}

			continue loop
		}
	}

	return
}

// localRcShell connect to remote shell using local bashrc
func localrcShell(connect *sshlib.Connect, session *ssh.Session, localrcPath []string, decoder string, compress bool, uncompress string) (err error) {
	return connect.CmdShell(session, BuildLocalRCShellCommand(localrcPath, decoder, compress, uncompress))
}

func BuildLocalRCShellCommand(localrcPath []string, decoder string, compress bool, uncompress string) string {
	return buildLocalRCShellCommand(localrcPath, decoder, compress, uncompress, false)
}

func BuildInteractiveLocalRCShellCommand(localrcPath []string, decoder string, compress bool, uncompress string) string {
	return buildPortableInteractiveLocalRCShellCommand(localrcPath, decoder, compress, uncompress)
}

func InteractiveLocalRCStartupMarker() string {
	return "__LSSH_LOCALRC_READY__"
}

func buildLocalRCShellCommand(localrcPath []string, decoder string, compress bool, uncompress string, interactive bool) string {
	var cmd string
	bashCommand := "bash"
	interactiveSuffix := ""
	exitSuffix := "; exit 0"
	prefixCommand := ""

	if len(localrcPath) == 0 {
		localrcPath = []string{"~/.bashrc"}
	}

	if interactive {
		bashCommand = "bash"
		interactiveSuffix = " -i"
		exitSuffix = ""
		prefixCommand = fmt.Sprintf("export TERM=%s; ", shellSingleQuote(interactiveShellTerm()))
	}

	rcData, _ := common.GetFilesBase64(localrcPath, localrcArchiveMode(compress))

	if uncompress == "" {
		uncompress = "gzip -d"
	}

	switch {
	case !compress && decoder != "":
		cmd = fmt.Sprintf("%s%s --noprofile --rcfile <(echo %s | %s)%s%s", prefixCommand, bashCommand, rcData, decoder, interactiveSuffix, exitSuffix)
	case !compress && decoder == "":
		cmd = fmt.Sprintf("%s%s --noprofile --rcfile <(echo %s | ( (base64 --help | grep -q coreutils) && base64 -d <(cat) || base64 -D <(cat) ) )%s%s", prefixCommand, bashCommand, rcData, interactiveSuffix, exitSuffix)
	case compress && decoder != "":
		cmd = fmt.Sprintf("%s%s --noprofile --rcfile <(echo %s | %s | %s)%s%s", prefixCommand, bashCommand, rcData, decoder, uncompress, interactiveSuffix, exitSuffix)
	case compress && decoder == "":
		cmd = fmt.Sprintf("%s%s --noprofile --rcfile <(echo %s | ( (base64 --help | grep -q coreutils) && base64 -d <(cat) || base64 -D <(cat) ) | %s)%s%s", prefixCommand, bashCommand, rcData, uncompress, interactiveSuffix, exitSuffix)
	}

	return cmd
}

func buildPortableInteractiveLocalRCShellCommand(localrcPath []string, decoder string, compress bool, uncompress string) string {
	if len(localrcPath) == 0 {
		localrcPath = []string{"~/.bashrc"}
	}

	rcData, _ := common.GetFilesBase64(localrcPath, localrcArchiveMode(compress))
	if uncompress == "" {
		uncompress = "gzip -d"
	}
	decodeCommand := decoder
	if decodeCommand == "" {
		decodeCommand = "( (base64 --help | grep -q coreutils) && base64 -d || base64 -D )"
	}
	markerLine := fmt.Sprintf("\nprintf '%%s\\n' %s\n", shellSingleQuote(InteractiveLocalRCStartupMarker()))
	markerEncoded := base64.StdEncoding.EncodeToString([]byte(markerLine))

	var pipeline string
	switch {
	case !compress:
		pipeline = fmt.Sprintf("printf %%s %s | %s", shellSingleQuote(rcData), decodeCommand)
	default:
		pipeline = fmt.Sprintf("printf %%s %s | %s | %s", shellSingleQuote(rcData), decodeCommand, uncompress)
	}

	rcStream := fmt.Sprintf("{ %s; printf %%s %s | ( (base64 --help | grep -q coreutils) && base64 -d || base64 -D ); }",
		pipeline,
		shellSingleQuote(markerEncoded),
	)
	bashScript := fmt.Sprintf("exec bash --noprofile --rcfile <(%s) -i", rcStream)

	return fmt.Sprintf(
		"export TERM=%s; exec bash -lc %s",
		shellSingleQuote(interactiveShellTerm()),
		shellSingleQuote(bashScript),
	)
}

func interactiveShellTerm() string {
	term := strings.TrimSpace(os.Getenv("TERM"))
	if term == "" {
		return "xterm-256color"
	}

	return term
}

func shellSingleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

func localrcArchiveMode(compress bool) int {
	if compress {
		return common.ARCHIVE_GZIP
	}

	return common.ARCHIVE_NONE
}

func validateControlMasterClient(connect *sshlib.Connect) error {
	if connect == nil || !connect.IsControlClient() {
		return nil
	}

	clone := *connect
	clone.Stdin = bytes.NewReader(nil)
	clone.Stdout = io.Discard
	clone.Stderr = io.Discard
	clone.TTY = false

	return clone.Command("true")
}

func isControlMasterRemoteExit255(err error) bool {
	return err != nil && strings.Contains(err.Error(), "sshlib: remote command exited with status 255")
}
