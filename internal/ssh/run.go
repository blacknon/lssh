// Copyright (c) 2022 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package ssh

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"

	"github.com/blacknon/go-sshlib"
	"github.com/blacknon/lssh/internal/common"
	conf "github.com/blacknon/lssh/internal/config"
	"github.com/blacknon/lssh/internal/connectorruntime"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"
)

// TODO(blacknon): Parallel ssh接続時に1ホストづつ接続しているので、goroutineで並列接続させるようにする(v0.7.0)

// TOOD(blacknon): なんかProxyのポートが表示おかしいので、修正する(v0.7.0)

// Run
type Run struct {
	ServerList []string
	Conf       conf.Config

	// ControlMasterOverride temporarily overrides the config setting for
	// this run. nil means use the config value as-is.
	ControlMasterOverride *bool

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

	// x11 forwarding (-X option)
	X11 bool

	// Trusted X11 flag (-Y option)
	X11Trusted bool

	// use or not-use local bashrc.
	// IsNotBashrc takes precedence.
	IsBashrc    bool
	IsNotBashrc bool

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

	// Exec command
	ExecCmd []string

	// ConnectorAttachSession resumes a connector-managed shell session by id.
	ConnectorAttachSession string

	// ConnectorDetach starts a connector-managed shell session without attaching.
	ConnectorDetach bool

	// enable/disable print header in command mode
	EnableHeader  bool
	DisableHeader bool

	// Agent is ssh-agent.
	// In agent.Agent or agent.ExtendedAgent.
	agent interface{}

	// Enable/Disable stdoutMutex
	EnableStdoutMutex bool

	// Mutex for parallel execution of output to stdout with goroutine
	stdoutMutex sync.Mutex

	// StdinData from pipe flag
	IsStdinPipe bool

	// ShareConnect reuses the monitor SSH connection for interactive terminals.
	ShareConnect bool

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

// PrintSelectServer is printout select server.
// use ssh login header.
func (r *Run) PrintSelectServer() {
	serverListStr := strings.Join(r.ServerList, ",")
	fmt.Fprintf(os.Stderr, "Select Server :%s\n", serverListStr)
}

// printRunCommand is printout run command.
// use ssh command run header.
func (r *Run) printRunCommand() {
	runCmdStr := strings.Join(r.ExecCmd, " ")
	fmt.Fprintf(os.Stderr, "Run Command   :%s\n", runCmdStr)
}

// printPortForward is printout port forwarding.
// use ssh command run header. only use shell().
func (r *Run) printPortForward(m, forwardLocal, forwardRemote string) {
	if forwardLocal != "" && forwardRemote != "" {
		var mode, arrow string
		switch m {
		case "L", "":
			mode = "LOCAL "
			arrow = " =>"
		case "R":
			mode = "REMOTE"
			arrow = "<= "
		}

		fmt.Fprintf(os.Stderr, "Port Forward  :%s\n", mode)
		fmt.Fprintf(os.Stderr, "               local[%s] %s remote[%s]\n", forwardLocal, arrow, forwardRemote)
	}
}

// printDynamicPortForward is printout port forwarding.
// use ssh command run header. only use shell().
func (r *Run) printDynamicPortForward(port string) {
	if port != "" {
		fmt.Fprintf(os.Stderr, "DynamicForward:%s\n", port)
		fmt.Fprintf(os.Stderr, "               %s\n", "connect Socks5.")
	}
}

// printReverseDynamicPortForward is printout port forwarding.
// use ssh command run header. only use shell().
func (r *Run) printReverseDynamicPortForward(port string) {
	if port != "" {
		fmt.Fprintf(os.Stderr, "ReverseDynamicForward:%s\n", port)
		fmt.Fprintf(os.Stderr, "                      %s\n", "connect Socks5.")
	}
}

// printPortForward is printout port forwarding.
// use ssh command run header. only use shell().
func (r *Run) printHTTPDynamicPortForward(port string) {
	if port != "" {
		fmt.Fprintf(os.Stderr, "HTTPDynamicForward:%s\n", port)
		fmt.Fprintf(os.Stderr, "                   %s\n", "connect http.")
	}
}

// printHTTPReverseDynamicPortForward is printout port forwarding.
// use ssh command run header. only use shell().
func (r *Run) printHTTPReverseDynamicPortForward(port string) {
	if port != "" {
		fmt.Fprintf(os.Stderr, "HTTPReverseDynamicForward:%s\n", port)
		fmt.Fprintf(os.Stderr, "                        %s\n", "connect http.")
	}
}

// printNFSDynamicForward is printout forwarding.
// use ssh command run header. only use shell().
func (r *Run) printNFSDynamicForward(port, path string) {
	if port != "" {
		fmt.Fprintf(os.Stderr, "NFSDynamicForward:%s:%s\n", port, path)
		fmt.Fprintf(os.Stderr, "                 %s\n", "connect NFS.")
	}
}

// printNFSReverseDynamicForward is printout forwarding.
// use ssh command run header. only use shell().
func (r *Run) printNFSReverseDynamicForward(port, path string) {
	if port != "" {
		fmt.Fprintf(os.Stderr, "NFSReverseDynamicForward:%s:%s\n", port, path)
		fmt.Fprintf(os.Stderr, "                      %s\n", "connect NFS.")
	}
}

func (r *Run) printSMBDynamicForward(port, path string) {
	if port != "" {
		fmt.Fprintf(os.Stderr, "SMBDynamicForward:%s:%s\n", port, path)
		fmt.Fprintf(os.Stderr, "                 %s\n", "connect SMB.")
	}
}

func (r *Run) printSMBReverseDynamicForward(port, path string) {
	if port != "" {
		fmt.Fprintf(os.Stderr, "SMBReverseDynamicForward:%s:%s\n", port, path)
		fmt.Fprintf(os.Stderr, "                      %s\n", "connect SMB.")
	}
}

// printProxy is printout proxy route.
// use ssh command run header. only use shell().
func (r *Run) printProxy(server string) {
	array := []string{}

	proxyRoute, err := getProxyRoute(server, r.Conf)
	if err != nil || len(proxyRoute) == 0 {
		return
	}

	// set localhost
	localhost := "localhost"

	// set target host
	targethost := server

	// add localhost
	array = append(array, localhost)

	for _, pxy := range proxyRoute {
		// seprator
		var sep string
		if pxy.Type == "command" {
			sep = ":"
		} else {
			sep = "://"
		}

		// setup string
		str := "[" + pxy.Type + sep + pxy.Name
		if pxy.Port != "" {
			str = str + ":" + pxy.Port
		}
		str = str + "]"

		array = append(array, str)
	}

	// add target
	array = append(array, targethost)

	// print header
	header := strings.Join(array, " => ")
	fmt.Fprintf(os.Stderr, "Proxy         :%s\n", header)
}

// setPortForwards is Add local/remote port forward to Run.PortForward
func (r *Run) setPortForwards(server string, config conf.ServerConfig) (c conf.ServerConfig) {
	// set config
	c = config

	// append single port forward settings (Backward compatibility).
	if c.PortForwardLocal != "" && c.PortForwardRemote != "" {
		fw := new(conf.PortForward)
		fw.Mode = c.PortForwardMode
		fw.Local = c.PortForwardLocal
		fw.Remote = c.PortForwardRemote

		c.Forwards = append(c.Forwards, fw)
	}

	// append port forwards from c, to r.PortForward
	for _, f := range c.PortForwards {
		var err error

		// create forward
		fw := new(conf.PortForward)

		// split config forward settings
		farray := strings.SplitN(f, ":", 2)

		// check array count
		if len(farray) == 1 {
			fmt.Fprintf(os.Stderr, "port forward format is incorrect: %s: \"%s\"", server, f)
			continue
		}

		//
		mode := strings.ToLower(farray[0])
		switch mode {
		// local/remote port forward
		case "local", "l":
			fw.Mode = "L"
			fw.Local, fw.Remote, err = common.ParseForwardPort(farray[1])

		case "remote", "r":
			fw.Mode = "R"
			fw.Local, fw.Remote, err = common.ParseForwardPort(farray[1])

		// other
		default:
			fmt.Fprintf(os.Stderr, "port forward format is incorrect: %s: \"%s\"", server, f)
			continue
		}

		if err != nil {
			fmt.Fprintf(os.Stderr, "port forward format is incorrect: %s: \"%s\"", server, f)
			continue
		}

		c.Forwards = append(c.Forwards, fw)
	}

	// append r.PortForward to c.Forwards
	c.Forwards = append(c.Forwards, r.PortForward...)

	return
}

func (r *Run) startPortForward(connect *sshlib.Connect, fw *conf.PortForward) error {
	if fw == nil {
		return nil
	}

	localNetwork := fw.LocalNetwork
	if localNetwork == "" {
		localNetwork = "tcp"
	}
	remoteNetwork := fw.RemoteNetwork
	if remoteNetwork == "" {
		remoteNetwork = "tcp"
	}

	if strings.ToUpper(fw.Mode) == "R" {
		return connect.RemoteForward(localNetwork, fw.Local, remoteNetwork, fw.Remote)
	}

	return connect.LocalForward(localNetwork, fw.Local, remoteNetwork, fw.Remote)
}

// ParallelIgnoredFeatures lists forwarding settings that are intentionally
// skipped for per-host parallel sessions because they require local listeners.
func (r *Run) ParallelIgnoredFeatures(server string) []string {
	config := r.Conf.Server[server]
	config = r.setPortForwards(server, config)

	if r.DynamicPortForward != "" {
		config.DynamicPortForward = r.DynamicPortForward
	}
	if r.HTTPDynamicPortForward != "" {
		config.HTTPDynamicPortForward = r.HTTPDynamicPortForward
	}
	if r.NFSDynamicForwardPort != "" {
		config.NFSDynamicForwardPort = r.NFSDynamicForwardPort
	}
	if r.NFSDynamicForwardPath != "" {
		config.NFSDynamicForwardPath = r.NFSDynamicForwardPath
	}
	if r.SMBDynamicForwardPort != "" {
		config.SMBDynamicForwardPort = r.SMBDynamicForwardPort
	}
	if r.SMBDynamicForwardPath != "" {
		config.SMBDynamicForwardPath = r.SMBDynamicForwardPath
	}

	notices := []string{}
	for _, fw := range config.Forwards {
		if fw == nil || strings.ToUpper(fw.Mode) == "R" {
			continue
		}
		notices = append(notices, fmt.Sprintf("-L %s:%s", fw.Local, fw.Remote))
	}
	if config.DynamicPortForward != "" {
		notices = append(notices, fmt.Sprintf("-D %s", config.DynamicPortForward))
	}
	if config.HTTPDynamicPortForward != "" {
		notices = append(notices, fmt.Sprintf("-d %s", config.HTTPDynamicPortForward))
	}
	if config.NFSDynamicForwardPort != "" && config.NFSDynamicForwardPath != "" {
		notices = append(notices, fmt.Sprintf("-M %s:%s", config.NFSDynamicForwardPort, config.NFSDynamicForwardPath))
	}
	if config.SMBDynamicForwardPort != "" && config.SMBDynamicForwardPath != "" {
		notices = append(notices, fmt.Sprintf("-S %s:%s", config.SMBDynamicForwardPort, config.SMBDynamicForwardPath))
	}
	if r.TunnelEnabled {
		notices = append(notices, fmt.Sprintf("--tunnel %d:%d", r.TunnelLocal, r.TunnelRemote))
	}

	return notices
}

// PrepareParallelForwardConfig returns only the forwarding settings that are safe
// to apply independently to each parallel connection.
func (r *Run) PrepareParallelForwardConfig(server string) (c conf.ServerConfig) {
	c = r.Conf.Server[server]
	c = r.setPortForwards(server, c)

	if r.ReverseDynamicPortForward != "" {
		c.ReverseDynamicPortForward = r.ReverseDynamicPortForward
	}
	if r.HTTPReverseDynamicPortForward != "" {
		c.HTTPReverseDynamicPortForward = r.HTTPReverseDynamicPortForward
	}
	if r.NFSReverseDynamicForwardPort != "" {
		c.NFSReverseDynamicForwardPort = r.NFSReverseDynamicForwardPort
	}
	if r.NFSReverseDynamicForwardPath != "" {
		c.NFSReverseDynamicForwardPath = r.NFSReverseDynamicForwardPath
	}
	if r.SMBReverseDynamicForwardPort != "" {
		c.SMBReverseDynamicForwardPort = r.SMBReverseDynamicForwardPort
	}
	if r.SMBReverseDynamicForwardPath != "" {
		c.SMBReverseDynamicForwardPath = r.SMBReverseDynamicForwardPath
	}

	forwards := make([]*conf.PortForward, 0, len(c.Forwards))
	for _, fw := range c.Forwards {
		if fw == nil {
			continue
		}
		if strings.ToUpper(fw.Mode) != "R" {
			continue
		}
		forwards = append(forwards, fw)
	}
	c.Forwards = forwards

	c.DynamicPortForward = ""
	c.HTTPDynamicPortForward = ""
	c.NFSDynamicForwardPort = ""
	c.NFSDynamicForwardPath = ""
	c.SMBDynamicForwardPort = ""
	c.SMBDynamicForwardPath = ""

	return
}

// StartParallelForwards starts only the reverse-side forwards that can be
// applied independently per connection in parallel workflows.
func StartParallelForwards(connect *sshlib.Connect, config conf.ServerConfig) error {
	var errs []error

	for _, fw := range config.Forwards {
		if fw == nil {
			continue
		}
		if err := (&Run{}).startPortForward(connect, fw); err != nil {
			errs = append(errs, err)
		}
	}

	if config.ReverseDynamicPortForward != "" {
		go connect.TCPReverseDynamicForward("localhost", config.ReverseDynamicPortForward)
	}

	if config.HTTPReverseDynamicPortForward != "" {
		go connect.HTTPReverseDynamicForward("localhost", config.HTTPReverseDynamicPortForward)
	}

	if config.NFSReverseDynamicForwardPort != "" && config.NFSReverseDynamicForwardPath != "" {
		go connect.NFSReverseForward("localhost", config.NFSReverseDynamicForwardPort, config.NFSReverseDynamicForwardPath)
	}

	if config.SMBReverseDynamicForwardPort != "" && config.SMBReverseDynamicForwardPath != "" {
		go connect.SMBReverseForward("localhost", config.SMBReverseDynamicForwardPort, "", config.SMBReverseDynamicForwardPath)
	}

	return errors.Join(errs...)
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
