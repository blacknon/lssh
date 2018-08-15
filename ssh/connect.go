package ssh

import (
	"bytes"
	"os"
	"os/user"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"

	"github.com/appleboy/easyssh-proxy"
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
	serverConf := c.Conf.Server[c.Server]

	ssh := &easyssh.MakeConfig{
		User:   serverConf.User,
		Server: serverConf.Addr,
		Port:   serverConf.Port,
	}

	// auth
	if serverConf.Key != "" {
		serverConf.Key = strings.Replace(serverConf.Key, "~", usr.HomeDir, 1)
		ssh.KeyPath = serverConf.Key
	} else {
		ssh.Password = serverConf.Pass
	}

	// Proxy
	proxyServer := serverConf.ProxyServer
	if proxyServer != "" {
		proxyConf := c.Conf.Server[proxyServer]

		ssh.Proxy.User = proxyConf.User
		ssh.Proxy.Server = proxyConf.Addr
		ssh.Proxy.Port = proxyConf.Port
		if proxyConf.Key != "" {
			proxyConf.Key = strings.Replace(proxyConf.Key, "~", usr.HomeDir, 1)
			ssh.Proxy.Key = proxyConf.Key
		} else {
			ssh.Proxy.Password = proxyConf.Pass
		}
	}

	session, err = ssh.Connect()
	return
}

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

	err = session.Wait()
	if err != nil {
		return
	}

	return
}

func (c *Connect) setIsTerm(befSession *ssh.Session) (aftSession *ssh.Session, err error) {
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
			befSession.Close()
			return aftSession, err
		}

		if err = befSession.RequestPty("xterm", hight, width, modes); err != nil {
			befSession.Close()
			return aftSession, err
		}
	}
	aftSession = befSession
	return
}
