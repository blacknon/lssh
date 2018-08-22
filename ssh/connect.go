package ssh

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/signal"
	"os/user"
	"regexp"
	"strings"
	"syscall"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"

	"github.com/blacknon/lssh/conf"
)

type Connect struct {
	Server     string
	Conf       conf.Config
	IsTerm     bool
	IsParallel bool
}

// @brief: create ssh session
func (c *Connect) CreateSession() (session *ssh.Session, err error) {
	// New connect
	conn, err := c.createSshClient()
	if err != nil {
		return session, err
	}

	// New session
	session, err = conn.NewSession()
	if err != nil {
		return session, err
	}

	return
}

func (c *Connect) createSshClient() (client *ssh.Client, err error) {
	// New ClientConfig
	conf := c.Conf.Server[c.Server]
	sshConf, err := c.createSshClientConfig(c.Server)
	if err != nil {
		return client, err
	}

	// not use proxy
	if conf.Proxy == "" {
		client, err = ssh.Dial("tcp", net.JoinHostPort(conf.Addr, conf.Port), sshConf)
		if err != nil {
			return client, err
		}

		return client, err
	}

	// get proxy slice
	proxyList := c.getProxyList()

	// var
	var proxyClient *ssh.Client
	var proxyConn net.Conn

	for i, proxy := range proxyList {
		// New Proxy ClientConfig
		proxyConf := c.Conf.Server[proxy]
		proxySshConf, err := c.createSshClientConfig(proxy)
		if err != nil {
			return client, err
		}

		if i == 0 {
			// first proxy
			proxyClient, err = ssh.Dial("tcp", net.JoinHostPort(proxyConf.Addr, proxyConf.Port), proxySshConf)
			if err != nil {
				return client, err
			}

		} else {
			// after second proxy
			proxyConn, err = proxyClient.Dial("tcp", net.JoinHostPort(proxyConf.Addr, proxyConf.Port))
			if err != nil {
				return client, err
			}

			pConnect, pChans, pReqs, err := ssh.NewClientConn(proxyConn, net.JoinHostPort(proxyConf.Addr, proxyConf.Port), proxySshConf)
			if err != nil {
				fmt.Println(err)
				fmt.Println("proxyConnect,Chans,proxyReqs")
				return client, err
			}

			proxyClient = ssh.NewClient(pConnect, pChans, pReqs)
		}

		// target server connect over last proxy
		proxyConn, err = proxyClient.Dial("tcp", net.JoinHostPort(conf.Addr, conf.Port))
		if err != nil {
			return client, err
		}

		clientConnect, clientChans, clientReqs, err := ssh.NewClientConn(proxyConn, net.JoinHostPort(conf.Addr, conf.Port), sshConf)
		if err != nil {
			return client, err
		}

		client = ssh.NewClient(clientConnect, clientChans, clientReqs)
	}

	return client, err
}

// 	// first proxy
// 	proxyConf := c.Conf.Server[proxySlice[0]]
// 	proxyClientConfig, err := c.createSshClientConfig(proxySlice[0])
// 	if err != nil {
// 		return client, err
// 	}

// 	proxyClient, err := ssh.Dial("tcp", net.JoinHostPort(proxyConf.Addr, proxyConf.Port), proxyClientConfig)
// 	if err != nil {
// 		return client, err
// 	}

// 	// proxy2Conf := c.Conf.Server[proxySlice[1]]
// 	// proxy2ClientConfig, err := c.createSshClientConfig(proxySlice[1])
// 	// if err != nil {
// 	// 	fmt.Println("proxy2ClientConfig")
// 	// 	return client, err
// 	// }

// 	// // connect ssh client
// 	// clientConnect, clientChans, clientReqs, err := ssh.NewClientConn(proxyConn, net.JoinHostPort(conf.Addr, conf.Port), clientConfig)
// 	// if err != nil {
// 	// 	return client, err
// 	// }
// 	// client := ssh.NewClient(clientConnect, clientChans, clientReqs)

// 	// second proxy

// 	// test!!
// 	fmt.Println(proxySlice)
// 	if len(proxySlice) > 1 {
// 		pserver := proxySlice[1]
// 		// pserver := c.Server
// 		fmt.Println("pserver is ...")
// 		fmt.Println(pserver)

// 		proxy2Conf := c.Conf.Server[pserver]
// 		proxy2ClientConfig, err := c.createSshClientConfig(pserver)
// 		if err != nil {
// 			fmt.Println("proxy2ClientConfig")
// 			return client, err
// 		}

// 		proxyConn, err := proxyClient.Dial("tcp", net.JoinHostPort(proxy2Conf.Addr, proxy2Conf.Port))
// 		if err != nil {
// 			fmt.Println("proxyConn")
// 			return client, err
// 		}

// 		// proxy2Conf := c.Conf.Server[pserver]
// 		// proxy2ClientConfig, err := c.createSshClientConfig(pserver)
// 		// if err != nil {
// 		// 	fmt.Println("proxy2ClientConfig")
// 		// 	return client, err
// 		// }

// 		fmt.Println(proxy2Conf)

// 		// proxy2Conf := c.Conf.Server[c.Server]
// 		// proxy2ClientConfig, err := c.createSshClientConfig(c.Server)
// 		// if err != nil {
// 		// 	fmt.Println("proxy2ClientConfig")
// 		// 	return client, err
// 		// }

// 		fmt.Println(proxy2Conf.Addr, proxy2Conf.Port)
// 		pConnect, pChans, pReqs, err := ssh.NewClientConn(proxyConn, net.JoinHostPort(proxy2Conf.Addr, proxy2Conf.Port), proxy2ClientConfig)
// 		if err != nil {
// 			fmt.Println(err)
// 			fmt.Println("proxyConnect,Chans,proxyReqs")
// 			return client, err
// 		}
// 		proxy2Client := ssh.NewClient(pConnect, pChans, pReqs)
// 		// client = ssh.NewClient(pConnect, pChans, pReqs)

// 		// client = proxy2Client

// 		// proxyConnect, proxyChans, proxyReqs, err := ssh.NewClientConn(proxyConn, net.JoinHostPort(conf.Addr, conf.Port), clientConfig)
// 		// if err != nil {
// 		// 	fmt.Println("proxyConnect,Chans,proxyReqs")
// 		// 	return client, err
// 		// }
// 		// client = ssh.NewClient(proxyConnect, proxyChans, proxyReqs)

// 		proxy2Conn, err := proxy2Client.Dial("tcp", net.JoinHostPort(conf.Addr, conf.Port))
// 		if err != nil {
// 			fmt.Println("proxy2Conn")
// 			return client, err
// 		}

// 		clientConnect, clientChans, clientReqs, err := ssh.NewClientConn(proxy2Conn, net.JoinHostPort(conf.Addr, conf.Port), sshConf)
// 		if err != nil {
// 			return client, err
// 		}
// 		client = ssh.NewClient(clientConnect, clientChans, clientReqs)
// 	} else {
// 		proxyConn, err := proxyClient.Dial("tcp", net.JoinHostPort(conf.Addr, conf.Port))
// 		if err != nil {
// 			fmt.Println("proxyConn")
// 			return client, err
// 		}

// 		fmt.Println("debug len(slice) == 1")
// 		clientConnect, clientChans, clientReqs, err := ssh.NewClientConn(proxyConn, net.JoinHostPort(conf.Addr, conf.Port), sshConf)
// 		if err != nil {
// 			return client, err
// 		}
// 		client = ssh.NewClient(clientConnect, clientChans, clientReqs)
// 	}

// 	return client, err
// }

// func (c *Connect) createSshConnectProxy(server string) (conn *net.Conn, err error) {
// 	proxyConf := c.Conf.Server[server]
// 	proxyClientConfig, err := c.createSshClientConfig(server)
// 	if err != nil {
// 		return conn, err
// 	}

// 	proxyClient, err := ssh.Dial("tcp", net.JoinHostPort(proxyConf.Addr, proxyConf.Port), proxyClientConfig)
// 	if err != nil {
// 		return conn, err
// 	}

// 	proxyConn, err := proxyClient.Dial("tcp", net.JoinHostPort(conf.Addr, conf.Port))
// 	if err != nil {
// 		return conn, err
// 	}

// 	clientConnect, clientChans, clientReqs, err := ssh.NewClientConn(proxyConn, net.JoinHostPort(conf.Addr, conf.Port), clientConfig)
// 	if err != nil {
// 		return conn, err
// 	}

// 	client = ssh.NewClient(clientConnect, clientChans, clientReqs)

// }

// @brief: Create ssh Client
func (c *Connect) createSshClientConfig(server string) (clientConfig *ssh.ClientConfig, err error) {
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
		Timeout:         3600 * time.Hour,
	}
	return clientConfig, err
}

// @brief: create ssh session auth
func (c *Connect) createSshAuth(server string) (auth []ssh.AuthMethod, err error) {
	usr, _ := user.Current()
	conf := c.Conf.Server[server]

	if conf.Key != "" {
		conf.Key = strings.Replace(conf.Key, "~", usr.HomeDir, 1)

		// Read PrivateKey file
		keyData, err := ioutil.ReadFile(conf.Key)
		if err != nil {
			return auth, err
		}

		// Read PrivateKey data
		key, err := ssh.ParsePrivateKey(keyData)
		if err != nil {
			return auth, err
		}

		auth = []ssh.AuthMethod{ssh.PublicKeys(key)}
	} else {
		auth = []ssh.AuthMethod{ssh.Password(conf.Pass)}
	}

	return auth, err
}

// @brief: get ssh proxy server slice
func (c *Connect) getProxyList() (proxyServers []string) {
	targetServer := c.Server
	for {
		serverConf := c.Conf.Server[targetServer]
		if serverConf.Proxy == "" {
			break
		}
		proxyServers = append(proxyServers, serverConf.Proxy)
		targetServer = serverConf.Proxy
	}

	// reverse proxyServers slice
	for i, j := 0, len(proxyServers)-1; i < j; i, j = i+1, j-1 {
		proxyServers[i], proxyServers[j] = proxyServers[j], proxyServers[i]
	}

	return proxyServers
}

// @brief: run command over ssh
func (c *Connect) RunCmd(session *ssh.Session, command []string) (err error) {
	defer session.Close()

	// set timeout
	go func() {
		time.Sleep(2419200 * time.Second)
		session.Close()
	}()

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
		time.Sleep(100 * time.Millisecond)
		select {
		case <-isExit:
			break CheckCommandExit
		case <-time.After(100 * time.Millisecond):
			continue CheckCommandExit
		}
	}

	return
}

// @brief: run command over ssh, output to gochannel
func (c *Connect) RunCmdGetOutput(session *ssh.Session, command []string, outputChan chan string) {
	var outputBuf bytes.Buffer
	session.Stdout = &outputBuf
	session.Stderr = &outputBuf

	// run command
	isExit := make(chan bool)
	go func() {
		c.RunCmd(session, command)
		isExit <- true
	}()

	readedLineBytes := 0

GetOutputLoop:
	for {
		time.Sleep(100 * time.Millisecond)

		outputBufStr := outputBuf.String()
		if len(outputBufStr) == 0 {
			continue
		}

		outputBufByte := []byte(outputBufStr)
		outputBufSlice := regexp.MustCompile("\r\n|\n\r|\n|\r").Split(string(outputBufByte[readedLineBytes:]), -1)

		readedLineBytes = len(outputBufByte)

		for i, outputLine := range outputBufSlice {
			if i == len(outputBufSlice)-1 {
				break
			}
			outputChan <- outputLine
		}

		select {
		case <-isExit:
			break GetOutputLoop
		case <-time.After(100 * time.Millisecond):
			continue GetOutputLoop
		}
	}

	// last check
	outputBufByte := []byte(outputBuf.String())
	if len(outputBufByte) > readedLineBytes {
		outputBufSlice := regexp.MustCompile("\r\n|\n\r|\n|\r").Split(string(outputBufByte[readedLineBytes:]), -1)
		for i, outputLine := range outputBufSlice {
			if i == len(outputBufSlice)-1 {
				break
			}
			outputChan <- outputLine
		}
	}
}

// @brief: connect ssh terminal
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

	err = session.RequestPty("xterm", height, width, modes)
	if err != nil {
		return
	}

	err = session.Shell()
	if err != nil {
		return
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

// @brief: set pesudo (run command only)
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

		if err = preSession.RequestPty("xterm", hight, width, modes); err != nil {
			preSession.Close()
			return session, err
		}
	}
	session = preSession
	return
}
