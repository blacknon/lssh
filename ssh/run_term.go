package ssh

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/user"
	"strings"
	"time"

	"github.com/blacknon/lssh/common"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

func (r *Run) term() (err error) {
	server := r.ServerList[0]
	c := new(Connect)
	c.Server = server
	c.Conf = r.Conf
	serverConf := c.Conf.Server[c.Server]

	// print header
	r.printSelectServer()
	r.printProxy()

	// create ssh session
	session, err := c.CreateSession()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot connect %v, %v \n", c.Server, err)
		return err
	}

	// setup terminal log
	session, err = r.setTerminalLog(session, c.Server)
	if err != nil {
		fmt.Fprintf(os.Stderr, "setup terminal log error %v, %v \n", c.Server, err)
		return err
	}

	preCmd := serverConf.PreCmd
	postCmd := serverConf.PostCmd

	// if use local bashrc file.
	switch serverConf.LocalRcUse {
	case "yes", "Yes", "YES", "y":
		c.IsLocalRc = true
	default:
		c.IsLocalRc = false
	}

	if c.IsLocalRc {
		fmt.Fprintf(os.Stderr, "Infomation    :This connect use local bashrc. \n")
		if len(serverConf.LocalRcPath) > 0 {
			c.LocalRcData, err = common.GetFilesBase64(serverConf.LocalRcPath)
			if err != nil {
				return err
			}
		} else {
			rcfile := []string{"~/.bashrc"}
			c.LocalRcData, err = common.GetFilesBase64(rcfile)
			if err != nil {
				return err
			}
		}
		c.LocalRcDecodeCmd = serverConf.LocalRcDecodeCmd
	}

	// run pre local command
	if preCmd != "" {
		runCmdLocal(preCmd)
	}

	// defer run post local command
	if postCmd != "" {
		defer runCmdLocal(postCmd)
	}

	// Overwrite port forward option.
	if len(r.PortForwardLocal) > 0 {
		serverConf.PortForwardLocal = r.PortForwardLocal
	}
	if len(r.PortForwardRemote) > 0 {
		serverConf.PortForwardRemote = r.PortForwardRemote
	}

	// Port Forwarding
	if len(serverConf.PortForwardLocal) > 0 && len(serverConf.PortForwardRemote) > 0 {
		c.ForwardLocal = serverConf.PortForwardLocal
		c.ForwardRemote = serverConf.PortForwardRemote

		r.printPortForward(c.ForwardLocal, c.ForwardRemote)

		go func() {
			c.PortForwarder()
		}()
	}

	// ssh-agent
	if serverConf.SSHAgentUse {
		fmt.Fprintf(os.Stderr, "Infomation    :This connect use ssh agent. \n")

		// forward agent
		_, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK"))
		if err != nil {
			agent.ForwardToAgent(c.sshClient, c.sshAgent)
		} else {
			agent.ForwardToAgent(c.sshClient, c.sshExtendedAgent)
		}
		agent.RequestAgentForwarding(session)
	}

	// print newline
	fmt.Println("------------------------------")

	// Connect ssh terminal
	finished := make(chan bool)
	go func() {
		c.ConTerm(session)
		finished <- true
	}()
	<-finished

	return
}

func (r *Run) setTerminalLog(preSession *ssh.Session, server string) (session *ssh.Session, err error) {
	session = preSession

	session.Stdin = os.Stdin
	session.Stdout = os.Stdout
	session.Stderr = os.Stderr

	logConf := r.Conf.Log
	if logConf.Enable {
		// Generate logPath
		logDir := createLogDirPath(r.Conf.Log.Dir, server)
		logFile := time.Now().Format("20060102_150405") + "_" + server + ".log"
		logPath := logDir + "/" + logFile

		// mkdir logDir
		if err = os.MkdirAll(logDir, 0700); err != nil {
			return session, err
		}

		// log enable/disable timestamp
		if logConf.Timestamp {
			r.OutputData = new(bytes.Buffer)
			session.Stdout = io.MultiWriter(os.Stdout, r.OutputData)
			session.Stderr = io.MultiWriter(os.Stderr, r.OutputData)

			// log writer
			go r.writeTimestampTerminalLog(logPath)
		} else {
			logWriter, _ := os.OpenFile(logPath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0600)
			session.Stdout = io.MultiWriter(os.Stdout, logWriter)
			session.Stderr = io.MultiWriter(os.Stderr, logWriter)
		}
	}

	return
}

func (r *Run) writeTimestampTerminalLog(logPath string) {
	logWriter, _ := os.OpenFile(logPath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0600)
	defer logWriter.Close()

	preLine := []byte{}
	for {
		if r.OutputData.Len() > 0 {
			line, err := r.OutputData.ReadBytes('\n')

			if err == io.EOF {
				preLine = append(preLine, line...)
				continue
			} else {
				timestamp := time.Now().Format("2006/01/02 15:04:05 ") // yyyy/mm/dd HH:MM:SS
				fmt.Fprintf(logWriter, timestamp+string(append(preLine, line...)))
				preLine = []byte{}
			}
		} else {
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func runCmdLocal(cmd string) {
	out, _ := exec.Command("sh", "-c", cmd).CombinedOutput()
	fmt.Printf(string(out))
}

func createLogDirPath(dirPath string, server string) string {
	currentUser, _ := user.Current()

	dirPath = strings.Replace(dirPath, "~", currentUser.HomeDir, 1)
	dirPath = strings.Replace(dirPath, "<Date>", time.Now().Format("20060102"), 1)
	dirPath = strings.Replace(dirPath, "<Hostname>", server, 1)

	return dirPath
}
