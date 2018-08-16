package ssh

import (
	"bytes"
	"io/ioutil"
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

func (c *Connect) CreateSession() (session *ssh.Session, err error) {
	usr, _ := user.Current()
	conf := c.Conf.Server[c.Server]
	auth := []ssh.AuthMethod{}

	if conf.Key != "" {
		conf.Key = strings.Replace(conf.Key, "~", usr.HomeDir, 1)
		// Read PublicKey
		keyData, err := ioutil.ReadFile(conf.Key)
		if err != nil {
			return session, err
		}

		key, err := ssh.ParsePrivateKey(keyData)
		if err != nil {
			return session, err
		}

		auth = []ssh.AuthMethod{ssh.PublicKeys(key)}
	} else {
		auth = []ssh.AuthMethod{ssh.Password(conf.Pass)}
	}

	config := &ssh.ClientConfig{
		User:            conf.User,
		Auth:            auth,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         3600 * time.Second,
	}

	// New connect
	conn, err := ssh.Dial("tcp", conf.Addr+":"+conf.Port, config)
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

// func (c *Connect) CreateSession() (session *ssh.Session, err error) {
// 	usr, _ := user.Current()
// 	serverConf := c.Conf.Server[c.Server]

// 	ssh := &easyssh.MakeConfig{
// 		User:    serverConf.User,
// 		Server:  serverConf.Addr,
// 		Port:    serverConf.Port,
// 		Timeout: 3600 * time.Hour,
// 	}

// 	// auth
// 	if serverConf.Key != "" {
// 		serverConf.Key = strings.Replace(serverConf.Key, "~", usr.HomeDir, 1)
// 		ssh.KeyPath = serverConf.Key
// 	} else {
// 		ssh.Password = serverConf.Pass
// 	}

// 	// Proxy
// 	proxyServer := serverConf.ProxyServer
// 	if proxyServer != "" {
// 		proxyConf := c.Conf.Server[proxyServer]

// 		ssh.Proxy.User = proxyConf.User
// 		ssh.Proxy.Server = proxyConf.Addr
// 		ssh.Proxy.Port = proxyConf.Port
// 		if proxyConf.Key != "" {
// 			proxyConf.Key = strings.Replace(proxyConf.Key, "~", usr.HomeDir, 1)
// 			ssh.Proxy.Key = proxyConf.Key
// 		} else {
// 			ssh.Proxy.Password = proxyConf.Pass
// 		}
// 	}

// 	session, err = ssh.Connect()
// 	return
// }

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
		case <-time.After(1 * time.Millisecond):
			continue CheckCommandExit
		}
	}
	return
}

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
		case <-time.After(1 * time.Millisecond):
			continue
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
