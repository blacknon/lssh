// Copyright (c) 2022 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package ssh

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sync"

	"github.com/blacknon/go-sshlib"
	conf "github.com/blacknon/lssh/internal/config"
	"github.com/blacknon/lssh/internal/connectorruntime"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"
)

// TODO(blacknon): Parallel ssh接続時に1ホストづつ接続しているので、goroutineで並列接続させるようにする(v0.7.0)

// TOOD(blacknon): なんかProxyのポートが表示おかしいので、修正する(v0.7.0)

type RunSessionConfig struct {
	// ControlMasterOverride temporarily overrides the config setting for
	// this run. nil means use the config value as-is.
	ControlMasterOverride *bool

	// x11 forwarding (-X option)
	X11 bool

	// Trusted X11 flag (-Y option)
	X11Trusted bool

	// use or not-use local bashrc.
	// IsNotBashrc takes precedence.
	IsBashrc    bool
	IsNotBashrc bool

	// ConnectorAttachSession resumes a connector-managed shell session by id.
	ConnectorAttachSession string

	// ConnectorDetach starts a connector-managed shell session without attaching.
	ConnectorDetach bool

	// ShareConnect reuses the monitor SSH connection for interactive terminals.
	ShareConnect bool
}

type RunCommandConfig struct {
	// Mode value in
	//     - shell
	//     - cmd
	Mode string

	// tty use (-t option)
	IsTerm bool

	// parallel connect (-p option)
	IsParallel bool

	// not run (-N option)
	IsNone bool

	// Exec command
	ExecCmd []string

	// enable/disable print header in command mode
	EnableHeader  bool
	DisableHeader bool

	// StdinData from pipe flag
	IsStdinPipe bool
}

type RunForwardConfig struct {
	// local/remote Port Forwarding
	PortForward []*conf.PortForward

	// TODO(blacknon): Delete old keys
	// L or R
	PortForwardMode string

	// local port num (ex. 11080).
	PortForwardLocal string

	// remote host and port (ex. localhost:11080).
	PortForwardRemote string

	// Dynamic Port Forwarding
	// set localhost port num (ex. 11080).
	DynamicPortForward string

	// HTTP Dynamic Port Forwarding
	// set localhost port num (ex. 11080).
	HTTPDynamicPortForward string

	// Reverse Dynamic Port Forwarding
	// set remotehost port num (ex. 11080).
	ReverseDynamicPortForward string

	// HTTP Reverse Dynamic Port Forwarding
	// set remotehost port num (ex. 11080).
	HTTPReverseDynamicPortForward string

	// NFS Dynamic Forward
	// set localhost port num (ex. 12049).
	NFSDynamicForwardPort string

	// NFS Dynamic Forward Path
	// set remotehost path (ex. /path/to/remote).
	NFSDynamicForwardPath string

	// NFS Reverse Dynamic Forward
	// set remotehost port num (ex. 12049).
	NFSReverseDynamicForwardPort string

	// NFS Reverse Dynamic Forward Path
	// set localhost path (ex. /path/to/local).
	NFSReverseDynamicForwardPath string

	// SMB Dynamic Forward
	// set localhost port num (ex. 12445).
	SMBDynamicForwardPort string

	// SMB Dynamic Forward Path
	// set remotehost path (ex. /path/to/remote).
	SMBDynamicForwardPath string

	// SMB Reverse Dynamic Forward
	// set remotehost port num (ex. 12445).
	SMBReverseDynamicForwardPort string

	// SMB Reverse Dynamic Forward Path
	// set localhost path (ex. /path/to/local).
	SMBReverseDynamicForwardPath string

	// Tunnel device (-w equivalent). Enable and units.
	TunnelEnabled bool
	TunnelLocal   int
	TunnelRemote  int
}

// Run
type Run struct {
	ServerList []string
	Conf       conf.Config

	RunSessionConfig
	RunCommandConfig
	RunForwardConfig

	// Agent is ssh-agent.
	// In agent.Agent or agent.ExtendedAgent.
	agent interface{}

	// Enable/Disable stdoutMutex
	EnableStdoutMutex bool

	// Mutex for parallel execution of output to stdout with goroutine
	stdoutMutex sync.Mutex

	// AuthMethodMap is
	// map of AuthMethod summarized in Run overall
	authMethodMap map[AuthKey][]ssh.AuthMethod

	// ServerAuthMethodMap is
	// Map of AuthMethod used by target server
	serverAuthMethodMap map[string][]ssh.AuthMethod

	// donedPKCS11 is　the value of panic measures (v0.6.2-).
	// If error occurs and pkcs11 processing occurs more than once, the library will keep the token and Panic will occur.
	// this value is so for countermeasures.
	donedPKCS11 bool

	// ActiveTunnel holds the active tunnel created for this Run (if any)
	ActiveTunnel *sshlib.Tunnel

	// ConnectorRuntime executes provider-managed connector plans.
	ConnectorRuntime connectorruntime.Executor
}

// AuthKey Auth map key struct.
type AuthKey struct {
	// auth type:
	//   - password
	//   - agent
	//   - key
	//   - cert
	//   - pkcs11
	Type string

	// auth type value:
	//   - key(path)
	//     ex.) ~/.ssh/id_rsa
	//   - cert(path)
	//     ex.) ~/.ssh/id_rsa.crt
	//   - pkcs11(libpath)
	//     ex.) /usr/local/lib/opensc-pkcs11.so
	Value string
}

// use scp,sftp
type PathSet struct {
	Base      string
	PathSlice []string
}

const (
	AUTHKEY_PASSWORD = "password"
	AUTHKEY_AGENT    = "agent"
	AUTHKEY_KEY      = "key"
	AUTHKEY_CERT     = "cert"
	AUTHKEY_PKCS11   = "pkcs11"
)

// Start ssh connect
func (r *Run) Start() {
	var err error

	// Ensure any active tunnel is closed when Run exits
	defer func() {
		if r.ActiveTunnel != nil {
			_ = r.ActiveTunnel.Close()
			_ = r.ActiveTunnel.Wait()
		}
	}()

	if runtime.GOOS != "windows" {
		stdin := 0
		if !terminal.IsTerminal(stdin) {
			r.IsStdinPipe = true
		}
	}

	// create AuthMap
	r.CreateAuthMethodMap()

	// connect
	switch {
	case len(r.ExecCmd) > 0 && r.Mode == "cmd":
		// connect and run command
		err = r.cmd()

	case r.Mode == "shell":
		// connect remote shell
		err = r.shell()

	default:
		return
	}

	if err != nil {
		fmt.Println(err)
	}
}

// runCmdLocal exec command local machine.
// Mainly used in r.shell().
func execLocalCommand(cmd string) {
	out, _ := exec.Command("sh", "-c", cmd).CombinedOutput()
	fmt.Print(string(out))
}

// notifyParentReady writes readiness to fd 3 if child was started by parent with ExtraFiles.
func notifyParentReady() {
	if os.Getenv("_LSSH_DAEMON") != "1" {
		return
	}

	// ExtraFiles in exec become fd starting at 3. We expect the parent passed a pipe as fd 3.
	f := os.NewFile(uintptr(3), "lssh_ready")
	if f == nil {
		return
	}
	defer f.Close()

	_, _ = f.Write([]byte("OK\n"))
}
