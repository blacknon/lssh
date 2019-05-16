package ssh

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/terminal"
	"golang.org/x/net/proxy"

	"github.com/blacknon/lssh/conf"
)

type Connect struct {
	Server           string
	Conf             conf.Config
	Client           *ssh.Client
	sshAgent         agent.Agent
	sshExtendedAgent agent.ExtendedAgent
	IsTerm           bool
	IsParallel       bool
	IsLocalRc        bool
	LocalRcData      string
	LocalRcDecodeCmd string
	ForwardLocal     string
	ForwardRemote    string
}

type Proxy struct {
	Name string
	Type string
}

// @brief: create ssh session
func (c *Connect) CreateSession() (session *ssh.Session, err error) {
	// New connect
	if c.Client == nil {
		err = c.CreateClient()
		if err != nil {
			return session, err
		}
	}

	// New session
	session, err = c.Client.NewSession()

	if err != nil {
		return session, err
	}

	return
}

// @brief: create ssh client
// @note:
//     support multiple proxy connect.
func (c *Connect) CreateClient() (err error) {
	// New ClientConfig
	serverConf := c.Conf.Server[c.Server]

	// if use ssh-agent
	if serverConf.SSHAgentUse || serverConf.AgentAuth {
		err := c.CreateSshAgent()
		if err != nil {
			return err
		}
	}

	sshConf, err := c.createClientConfig(c.Server)
	if err != nil {
		return err
	}

	// set default port 22
	if serverConf.Port == "" {
		serverConf.Port = "22"
	}

	// not use proxy
	if serverConf.Proxy == "" {
		client, err := ssh.Dial("tcp", net.JoinHostPort(serverConf.Addr, serverConf.Port), sshConf)
		if err != nil {
			return err
		}

		// set client
		c.Client = client
	} else {
		err := c.createClientOverProxy(serverConf, sshConf)
		if err != nil {
			return err
		}
	}

	return err
}

// @brief:
//     Create ssh client via proxy
func (c *Connect) createClientOverProxy(serverConf conf.ServerConfig, sshConf *ssh.ClientConfig) (err error) {
	// get proxy slice
	proxyList, proxyType, err := GetProxyList(c.Server, c.Conf)
	if err != nil {
		return err
	}

	// var
	var proxyClient *ssh.Client
	var proxyDialer proxy.Dialer

	for _, proxy := range proxyList {
		switch proxyType[proxy] {
		case "http", "https":
			proxyConf := c.Conf.Proxy[proxy]
			proxyDialer, err = createProxyDialerHttp(proxyConf)

		case "socks5":
			proxyConf := c.Conf.Proxy[proxy]
			proxyDialer, err = createProxyDialerSocks5(proxyConf)

		default:
			proxyConf := c.Conf.Server[proxy]
			proxySshConf, err := c.createClientConfig(proxy)
			if err != nil {
				return err
			}
			proxyClient, err = createClientViaProxy(proxyConf, proxySshConf, proxyClient, proxyDialer)

		}

		if err != nil {
			return err
		}
	}

	client, err := createClientViaProxy(serverConf, sshConf, proxyClient, proxyDialer)
	if err != nil {
		return err
	}

	// set c.client
	c.Client = client

	return
}

// @brief:
//     Create ssh Client
func (c *Connect) createClientConfig(server string) (clientConfig *ssh.ClientConfig, err error) {
	conf := c.Conf.Server[server]

	auth, err := c.createSshAuth(server)
	if err != nil {
		return clientConfig, err
	}

	// create ssh ClientConfig
	clientConfig = &ssh.ClientConfig{
		User:            conf.User,
		Auth:            auth,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         0,
	}
	return clientConfig, err
}

// @brief:
//    run command over ssh
func (c *Connect) RunCmd(session *ssh.Session, command []string) (err error) {
	defer session.Close()

	// set timeout
	// go func() {
	// 	time.Sleep(2419200 * time.Second)
	// 	session.Close()
	// }()

	// set TerminalModes
	if session, err = c.setIsTerm(session); err != nil {
		return
	}

	// join command
	execCmd := strings.Join(command, " ")

	// run command
	isExit := make(chan bool)
	go func() {
		err = session.Run(execCmd)
		isExit <- true
	}()

	// check command exit
CheckCommandExit:
	for {
		// time.Sleep(100 * time.Millisecond)
		select {
		case <-isExit:
			break CheckCommandExit
		case <-time.After(100 * time.Millisecond):
			continue CheckCommandExit
		}
	}

	return
}

// @brief:
//     Run command over ssh, output to gochannel
// func (c *Connect) RunCmdWithOutput(session *ssh.Session, command []string, outputChan chan string) {
func (c *Connect) RunCmdWithOutput(session *ssh.Session, command []string, outputChan chan []byte) {
	outputBuf := new(bytes.Buffer)
	session.Stdout = io.MultiWriter(outputBuf)
	session.Stderr = io.MultiWriter(outputBuf)

	// run command
	isExit := make(chan bool)
	go func() {
		c.RunCmd(session, command)
		isExit <- true
	}()

	// preLine := []byte{}

GetOutputLoop:
	for {
		if outputBuf.Len() > 0 {
			line, _ := outputBuf.ReadBytes('\n')
			outputChan <- line
		} else {
			select {
			case <-isExit:
				break GetOutputLoop
			case <-time.After(1000 * time.Millisecond):
				continue GetOutputLoop
			}
		}
	}

	// last check
	if outputBuf.Len() > 0 {
		for {
			line, err := outputBuf.ReadBytes('\n')
			if err != io.EOF {
				outputChan <- line
			} else {
				break
			}
		}
	}
}

// @brief:
//     connect ssh terminal
func (c *Connect) ConTerm(session *ssh.Session) (err error) {
	// defer session.Close()
	fd := int(os.Stdin.Fd())
	state, err := terminal.MakeRaw(fd)
	if err != nil {
		return
	}
	defer terminal.Restore(fd, state)

	// get terminal size
	width, height, err := terminal.GetSize(fd)
	if err != nil {
		return
	}

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}

	term := os.Getenv("TERM")
	err = session.RequestPty(term, height, width, modes)
	if err != nil {
		return
	}

	// start shell
	if c.IsLocalRc {
		session, err = c.runLocalRcShell(session)
		if err != nil {
			return
		}
	} else {
		err = session.Shell()
		if err != nil {
			return
		}
	}

	// Terminal resize
	signal_chan := make(chan os.Signal, 1)
	signal.Notify(signal_chan, syscall.SIGWINCH)
	go func() {
		for {
			s := <-signal_chan
			switch s {
			case syscall.SIGWINCH:
				fd := int(os.Stdout.Fd())
				width, height, _ = terminal.GetSize(fd)
				session.WindowChange(height, width)
			}
		}
	}()

	// keep alive packet
	go func() {
		for {
			_, _ = session.SendRequest("keepalive@golang.org", true, nil)
			time.Sleep(15 * time.Second)
		}
	}()

	err = session.Wait()
	if err != nil {
		return
	}

	return
}

// @brief:
//     set pesudo (run command only)
func (c *Connect) setIsTerm(preSession *ssh.Session) (session *ssh.Session, err error) {
	if c.IsTerm {
		modes := ssh.TerminalModes{
			ssh.ECHO:          0,
			ssh.TTY_OP_ISPEED: 14400,
			ssh.TTY_OP_OSPEED: 14400,
		}

		// Get terminal window size
		fd := int(os.Stdin.Fd())
		width, hight, err := terminal.GetSize(fd)
		if err != nil {
			preSession.Close()
			return session, err
		}

		term := os.Getenv("TERM")
		if err = preSession.RequestPty(term, hight, width, modes); err != nil {
			preSession.Close()
			return session, err
		}
	}
	session = preSession
	return
}

// @brief:
//     get ssh proxy server slice
func GetProxyList(server string, config conf.Config) (proxyList []string, proxyType map[string]string, err error) {
	var targetType string
	var preProxy, preProxyType string

	targetServer := server
	proxyType = map[string]string{}

	for {
		isOk := false

		switch targetType {
		case "http", "https", "socks5":
			_, isOk = config.Proxy[targetServer]
			preProxy = ""
			preProxyType = ""

		default:
			var preProxyConf conf.ServerConfig
			preProxyConf, isOk = config.Server[targetServer]
			preProxy = preProxyConf.Proxy
			preProxyType = preProxyConf.ProxyType
		}

		// not use pre proxy
		if preProxy == "" {
			break
		}

		if !isOk {
			err = fmt.Errorf("Not Found proxy : %s", targetServer)
			return nil, nil, err
		}

		// set proxy info
		proxy := new(Proxy)
		proxy.Name = preProxy

		switch preProxyType {
		case "http", "https", "socks5":
			proxy.Type = preProxyType
		default:
			proxy.Type = "ssh"
		}

		proxyList = append(proxyList, proxy.Name)
		proxyType[proxy.Name] = proxy.Type

		targetServer = proxy.Name
		targetType = proxy.Type
	}

	// reverse proxyServers slice
	for i, j := 0, len(proxyList)-1; i < j; i, j = i+1, j-1 {
		proxyList[i], proxyList[j] = proxyList[j], proxyList[i]
	}

	return
}

// @brief:
//    run shell use local rc file.
func (c *Connect) runLocalRcShell(preSession *ssh.Session) (session *ssh.Session, err error) {
	session = preSession

	// command
	cmd := fmt.Sprintf("bash --rcfile <(echo %s|((base64 --help | grep -q coreutils) && base64 -d <(cat) || base64 -D <(cat) ))", c.LocalRcData)
	if len(c.LocalRcDecodeCmd) > 0 {

		cmd = fmt.Sprintf("bash --rcfile <(echo %s | %s)", c.LocalRcData, c.LocalRcDecodeCmd)
	}

	err = session.Start(cmd)

	return session, err
}
