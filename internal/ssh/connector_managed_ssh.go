package ssh

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/blacknon/go-sshlib"
	conf "github.com/blacknon/lssh/internal/config"
	"github.com/pkg/sftp"
	"golang.org/x/net/proxy"
)

type connectorContextDialer struct {
	dial func(ctx context.Context, network, addr string) (net.Conn, error)
}

type connectorManagedSSHNetConn struct {
	net.Conn
	closeResource io.Closer
}

func (c *connectorManagedSSHNetConn) Close() error {
	var closeErr error
	if c.Conn != nil {
		closeErr = c.Conn.Close()
	}
	if c.closeResource != nil {
		if err := c.closeResource.Close(); closeErr == nil {
			closeErr = err
		}
		c.closeResource = nil
	}
	return closeErr
}

func (d connectorContextDialer) Dial(network, addr string) (net.Conn, error) {
	return d.DialContext(context.Background(), network, addr)
}

func (d connectorContextDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	return d.dial(ctx, network, addr)
}

func IsConnectorManagedSSHRuntime(prepared conf.PreparedConnector) bool {
	return prepared.ManagedSSH != nil
}

func (r *Run) connectorManagedDialer(server string) (proxy.ContextDialer, error) {
	return connectorContextDialer{
		dial: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return r.dialConnectorTarget(ctx, server, network, addr)
		},
	}, nil
}

func (r *Run) createConnectorManagedSSHConnect(server string, config conf.ServerConfig) (*sshlib.Connect, error) {
	dialer, err := r.connectorManagedDialer(server)
	if err != nil {
		return nil, err
	}

	if r.agent == nil {
		r.agent = sshlib.ConnectSshAgent()
	}

	connect := &sshlib.Connect{
		ProxyDialer:           dialer,
		ForwardAgent:          config.SSHAgentUse,
		Agent:                 r.agent,
		ForwardX11:            config.X11 || r.X11,
		ForwardX11Trusted:     config.X11Trusted || r.X11Trusted,
		TTY:                   r.IsTerm,
		ConnectTimeout:        config.ConnectTimeout,
		SendKeepAliveMax:      config.ServerAliveCountMax,
		SendKeepAliveInterval: config.ServerAliveCountInterval,
		CheckKnownHosts:       config.CheckKnownHosts,
		KnownHostsFiles:       config.KnownHostsFiles,
		OverwriteKnownHosts:   true,
		ControlMaster:         "no",
	}
	if r.EnableStdoutMutex {
		connect.StdoutMutex = &r.stdoutMutex
	}

	port := strings.TrimSpace(config.Port)
	if port == "" {
		port = "22"
	}

	if err := connect.CreateClient(config.Addr, port, config.User, r.serverAuthMethodMap[server]); err != nil {
		return nil, err
	}

	return connect, nil
}

func (r *Run) dialConnectorManagedSSHTarget(ctx context.Context, server, network, addr string) (net.Conn, error) {
	config := r.effectiveServerConfig(server, true)
	connect, err := r.createConnectorManagedSSHConnect(server, config)
	if err != nil {
		return nil, err
	}

	conn, err := connect.Client.DialContext(ctx, network, addr)
	if err != nil {
		_ = connect.Close()
		return nil, err
	}

	return &connectorManagedSSHNetConn{
		Conn:          conn,
		closeResource: connect,
	}, nil
}

func (r *Run) CreateConnectorManagedSSHConnectDirect(server string) (*sshlib.Connect, error) {
	connectorName := r.Conf.ServerConnectorName(server)
	if connectorName == "" || connectorName == "ssh" {
		return nil, fmt.Errorf("server %q does not use an external connector", server)
	}

	prepared, err := r.prepareConnectorOperation(server, conf.ConnectorOperation{Name: "shell"})
	if err != nil {
		return nil, err
	}
	if !prepared.Supported {
		return nil, fmt.Errorf("server %q connector %q: %v", server, prepared.ConnectorName, prepared.Reason)
	}
	if prepared.ManagedSSH == nil {
		return nil, fmt.Errorf("server %q connector %q does not provide managed ssh transport", server, prepared.ConnectorName)
	}

	return r.createConnectorManagedSSHConnect(server, r.effectiveServerConfig(server, true))
}

func (r *Run) runConnectorManagedSSHTransportShell(server string, config conf.ServerConfig) error {
	connect, err := r.createConnectorManagedSSHConnect(server, config)
	if err != nil {
		return err
	}
	defer connect.Close()

	if config.SSHAgentUse {
		connect.Agent = r.agent
	}

	if connectorLocalRCEnabled(r, config) {
		return localrcShell(connect, nil, config.LocalRcPath, config.LocalRcDecodeCmd, config.LocalRcCompress, config.LocalRcUncompressCmd)
	}

	return connect.Shell(nil)
}

func (r *Run) runConnectorManagedSSHTransportExec(server string, config conf.ServerConfig, command []string, commandLine string, stdin io.Reader, stdout, stderr io.Writer, tty bool) (int, error) {
	connect, err := r.createConnectorManagedSSHConnect(server, config)
	if err != nil {
		return 0, err
	}
	defer connect.Close()

	if strings.TrimSpace(commandLine) == "" {
		commandLine = strings.Join(command, " ")
	}
	if commandLine == "" {
		return 0, fmt.Errorf("connector managed ssh exec requires a command")
	}

	connect.Stdin = stdin
	connect.Stdout = stdout
	connect.Stderr = stderr
	connect.TTY = tty

	return 0, connect.Command(commandLine)
}

func (r *Run) runConnectorManagedSSHLocalPortForward(server string, config conf.ServerConfig) error {
	connect, err := r.createConnectorManagedSSHConnect(server, config)
	if err != nil {
		return err
	}
	defer connect.Close()

	for _, fw := range config.Forwards {
		if err := r.startPortForward(connect, fw); err != nil {
			return err
		}
	}
	notifyParentReady()

	interrupts := make(chan os.Signal, 1)
	signal.Notify(interrupts, os.Interrupt)
	defer signal.Stop(interrupts)

	for {
		select {
		case <-interrupts:
			return nil
		case <-time.After(500 * time.Millisecond):
			if err := connect.CheckClientAlive(); err != nil {
				return err
			}
		}
	}
}

func (r *Run) createConnectorManagedSFTPHandle(server string) (*SFTPClientHandle, error) {
	config := r.Conf.Server[server]
	connect, err := r.createConnectorManagedSSHConnect(server, config)
	if err != nil {
		return nil, err
	}

	client, err := sftp.NewClient(connect.Client)
	if err != nil {
		_ = connect.Close()
		return nil, err
	}

	return &SFTPClientHandle{
		Client:     client,
		Closer:     multiCloser{closers: []io.Closer{client, connect}},
		SSHConnect: connect,
	}, nil
}
