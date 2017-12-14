package ssh

import (
	"bufio"
	"fmt"
	"os"
	"os/signal"
	"os/user"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/blacknon/gexpect"
	"github.com/blacknon/lssh/conf"
)

type ConInfoTerm struct {
	Log     bool
	LogDir  string
	Server  string
	Addr    string
	Port    string
	User    string
	Pass    string
	KeyPath string
}

func (c *ConInfoTerm) Connect() int {
	// ssh command Args
	// "/usr/bin/ssh -o 'StrictHostKeyChecking no' -o 'NumberOfPasswordPrompts 1' connectUser@connectAddr -p connectPort"
	sshCmd := []string{"/usr/bin/ssh", "-o", "StrictHostKeyChecking no", "-o", "NumberOfPasswordPrompts 1", c.User + "@" + c.Addr, "-p", c.Port}
	if c.KeyPath != "" {
		// "/usr/bin/ssh -o 'StrictHostKeyChecking no' -o 'NumberOfPasswordPrompts 1' -i connectKey connectUser@connectAddr -p connectPort"
		sshCmd = []string{"/usr/bin/ssh", "-o", "StrictHostKeyChecking no", "-o", "NumberOfPasswordPrompts 1", "-i", c.KeyPath, c.User + "@" + c.Addr, "-p", c.Port}
	}

	// exec ssh command
	child, _ := gexpect.NewSubProcess(sshCmd[0], sshCmd[1:]...)
	if err := child.Start(); err != nil {
		fmt.Println(err)
		return 1
	}
	defer child.Close()

	// Log Enable
	if c.Log == true {
		logDirPath := c.LogDir
		usr, _ := user.Current()
		logDirPath = strings.Replace(logDirPath, "~", usr.HomeDir, 1)

		// mkdir logDIr
		if err := os.MkdirAll(logDirPath, 0700); err != nil {
			fmt.Println(err)
			return 1
		}

		// Golang time.format YYYYmmdd_HHMMSS = "20060102_150405".(https://golang.org/src/time/format.go)
		logFile := time.Now().Format("20060102_150405") + "_" + c.Server + ".log"
		logFilePATH := logDirPath + "/" + logFile
		fifoPATH := logDirPath + "/." + logFile + ".fifo"

		// Create FIFO
		syscall.Mknod(fifoPATH, syscall.S_IFIFO|0600, 0)
		defer os.Remove(fifoPATH)

		// log write FIFO
		fifoWrite, _ := os.OpenFile(fifoPATH, os.O_RDWR|os.O_APPEND, 0600)
		child.Term.Log = fifoWrite

		// Read FiIFO write LogFile Add Timestamp
		go func() {
			// Open FIFO
			openFIFO, err := os.Open(fifoPATH)
			if err != nil {
				fmt.Println(err)
			}
			scanner := bufio.NewScanner(openFIFO)

			// Open Logfile
			wirteLog, err := os.OpenFile(logFilePATH, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0600)
			if err != nil {
				fmt.Println(err)
			}
			defer wirteLog.Close()

			// for loop Add timestamp log
			for scanner.Scan() {
				fmt.Fprintln(wirteLog, time.Now().Format("2006/01/02 15:04:05 ")+scanner.Text())
			}
		}()
	}

	// Terminal Size Change Trap
	signal_chan := make(chan os.Signal, 1)
	signal.Notify(signal_chan, syscall.SIGWINCH)
	go func() {
		for {
			s := <-signal_chan
			switch s {
			case syscall.SIGWINCH:
				child.Term.ResetWinSize()
			}
		}
	}()

	// Password Input
	if c.Pass != "" {
		pwPrompt := "word:"
		idx, _ := child.ExpectTimeout(20*time.Second, regexp.MustCompile(pwPrompt))
		if idx >= 0 {
			child.SendLine(c.Pass)

		} else {
			fmt.Println("Not Connected")
			return 1
		}
	}

	// timeout
	child.InteractTimeout(2419200 * time.Second)
	return 0
}

func SshTerm(cServer string, cList conf.Config) (c *ConInfoTerm, err error) {
	c = new(ConInfoTerm)

	c.Log = cList.Log.Enable
	c.LogDir = cList.Log.Dir

	c.Server = cServer
	c.Addr = cList.Server[cServer].Addr
	c.Port = "22"
	if cList.Server[cServer].Port != "" {
		c.Port = cList.Server[cServer].Port
	}
	c.User = cList.Server[cServer].User

	c.Pass = ""
	if cList.Server[cServer].Pass != "" {
		c.Pass = cList.Server[cServer].Pass
	}

	c.KeyPath = cList.Server[cServer].Key
	if cList.Server[cServer].Key != "" {
		c.KeyPath = cList.Server[cServer].Key
	}
	return
}
