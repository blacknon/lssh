package ssh

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

func (r *Run) term() (err error) {
	server := r.ServerList[0]

	// print header
	r.printSelectServer()
	r.printProxy()
	fmt.Println() // print newline

	c := new(Connect)
	c.Server = server
	c.Conf = r.Conf

	// create ssh session
	session, err := c.CreateSession()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot connect %v, %v \n", c.Server, err)
		return err
	}

	// setup terminal log
	session, logPath, err := r.setTerminalLog(session, c.Server)
	if err != nil {
		fmt.Fprintf(os.Stderr, "setup terminal log error %v, %v \n", c.Server, err)
		return err
	}

	serverConf := c.Conf.Server[c.Server]
	preCmd := serverConf.PreCmd
	postCmd := serverConf.PostCmd

	// run pre local command
	if preCmd != "" {
		runCmdLocal(preCmd)
	}

	// Create and logging terminal log
	if r.Conf.Log.Enable {
		go func() {
			logWriter, _ := os.OpenFile(logPath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0600)
			preLine := []byte{}
			for {
				if r.OutputData.Len() > 0 {
					line, err := r.OutputData.ReadBytes('\n')

					if err == io.EOF {
						preLine = append(preLine, line...)
						continue
					} else {
						timestamp := time.Now().Format("2006/01/02 15:04:05 ")
						fmt.Fprintf(logWriter, timestamp+string(append(preLine, line...)))
						preLine = []byte{}
					}
				} else {
					time.Sleep(10 * time.Millisecond)
				}
			}
			logWriter.Close()
		}()
	}

	// Connect ssh terminal
	finished := make(chan bool)
	go func() {
		c.ConTerm(session)
		finished <- true
	}()
	<-finished

	// run post local command
	if postCmd != "" {
		runCmdLocal(postCmd)
	}

	return
}

func (r *Run) setTerminalLog(preSession *ssh.Session, server string) (session *ssh.Session, logPath string, err error) {
	session = preSession

	session.Stdin = os.Stdin
	session.Stdout = os.Stdout
	session.Stderr = os.Stderr

	if r.Conf.Log.Enable {
		// Generate logPath
		logDir := createLogDirPath(r.Conf.Log.Dir, server)
		logFile := time.Now().Format("20060102_150405") + "_" + server + ".log"
		logPath = logDir + "/" + logFile

		// mkdir logDir
		if err = os.MkdirAll(logDir, 0700); err != nil {
			return session, logPath, err
		}

		r.OutputData = new(bytes.Buffer)
		session.Stdout = io.MultiWriter(os.Stdout, r.OutputData)
		session.Stderr = io.MultiWriter(os.Stderr, r.OutputData)
	}
	return
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
