package ssh

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"regexp"
	"strings"
	"syscall"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"

	"github.com/blacknon/lssh/conf"
	termbox "github.com/nsf/termbox-go"
)

var (
	stdin []byte
)

type RunInfoCmd struct {
	ServerList []string
	ConfList   conf.Config
	Tflag      bool
	Pflag      bool
	ExecCmd    []string
}

type ConInfoCmd struct {
	Index           int
	Count           int
	Cmd             []string
	Server          string
	ServerMaxLength int
	Addr            string
	Port            string
	User            string
	Pass            string
	KeyPath         string
	Flag            ConInfoCmdFlag
	Connect         *ssh.Client

	StdinTempPath string
}

type ConInfoCmdFlag struct {
	PesudoTerm bool
	Parallel   bool
}

func outColorStrings(num int, inStrings string) (str string) {
	color := num % 5
	switch color {
	// Red
	case 1:
		str = fmt.Sprintf("\x1b[31m%s\x1b[0m", inStrings)
	// Yellow
	case 2:
		str = fmt.Sprintf("\x1b[33m%s\x1b[0m", inStrings)
	// Blue
	case 3:
		str = fmt.Sprintf("\x1b[34m%s\x1b[0m", inStrings)
	// Magenta
	case 4:
		str = fmt.Sprintf("\x1b[35m%s\x1b[0m", inStrings)
	// Cyan
	case 0:
		str = fmt.Sprintf("\x1b[36m%s\x1b[0m", inStrings)
	}
	return
}

func (c *ConInfoCmd) CreateConnect() (conn *ssh.Client, err error) {
	usr, _ := user.Current()
	auth := []ssh.AuthMethod{}
	if c.KeyPath != "" {
		c.KeyPath = strings.Replace(c.KeyPath, "~", usr.HomeDir, 1)
		// Read PublicKey
		buffer, b_err := ioutil.ReadFile(c.KeyPath)
		if b_err != nil {
			err = b_err
			return
		}
		key, b_err := ssh.ParsePrivateKey(buffer)
		if b_err != nil {
			err = b_err
			return
		}
		auth = []ssh.AuthMethod{ssh.PublicKeys(key)}
	} else {
		auth = []ssh.AuthMethod{ssh.Password(c.Pass)}
	}

	config := &ssh.ClientConfig{
		User:            c.User,
		Auth:            auth,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         60 * time.Second,
	}

	// New connect
	conn, err = ssh.Dial("tcp", c.Addr+":"+c.Port, config)
	return
}

// exec ssh command function
func (c *ConInfoCmd) Run(conn *ssh.Client) int {
	defer conn.Close()

	// New Session
	session, err := conn.NewSession()
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect error %v,cannot open new session: %v \n", c.Server, err)
		return 1
	}
	defer session.Close()

	go func() {
		time.Sleep(2419200 * time.Second)
		conn.Close()
	}()

	// if PesudoTerm Enable
	if c.Flag.PesudoTerm == true {
		modes := ssh.TerminalModes{
			ssh.ECHO:          0,
			ssh.TTY_OP_ISPEED: 14400,
			ssh.TTY_OP_OSPEED: 14400,
		}

		// Get terminal window size
		if err := termbox.Init(); err != nil {
			panic(err)
		}
		width, hight := termbox.Size()
		termbox.Close()

		if err := session.RequestPty("xterm", hight, width, modes); err != nil {
			session.Close()
			fmt.Errorf("request for pseudo terminal failed: %s \n", err)
		}
	}

	// get stdin
	session.Stdin = bytes.NewReader(stdin)

	// exec command join
	execCmd := strings.Join(c.Cmd, " ")

	if c.Count == 1 {
		session.Stdout = os.Stdout
		session.Stderr = os.Stderr

		err = session.Run(execCmd)

		if err != nil {
			fmt.Fprintf(os.Stderr, "%v Error: %v\n", c.Server, err)
			if ee, ok := err.(*ssh.ExitError); ok {
				return ee.ExitStatus()
			}
		}
		return 0
	} else if c.Flag.Parallel == true {
		// stdout and stderr to stdoutBuf
		var stdoutBuf bytes.Buffer
		session.Stdout = &stdoutBuf
		session.Stderr = &stdoutBuf

		// cmdStatus:
		// Chan can not continuously read buffer in for.
		// reason, the processing end is detected using a variable.
		cmdStatus := true

		// Exec Command(parallel)
		go func() {
			err = session.Run(execCmd)
			cmdStatus = false
		}()

		// var "x" is Readed Byte position from Buffer
		x := 0
		for {
			time.Sleep(100 * time.Millisecond)

			stdoutBufToStr := stdoutBuf.String()

			if len(stdoutBufToStr) == 0 {
				continue
			}

			stdoutBufToByte := []byte(stdoutBufToStr)
			stdoutBufArray := regexp.MustCompile("\r\n|\n\r|\n|\r").Split(string(stdoutBufToByte[x:]), -1)
			x = len(stdoutBufToStr)

			for i, v := range stdoutBufArray {
				if i == len(stdoutBufArray)-1 {
					break
				}
				lineHeader := fmt.Sprintf("%-*s", c.ServerMaxLength, c.Server)
				fmt.Println(outColorStrings(c.Index, lineHeader)+" :: ", v)
			}

			if cmdStatus == true {
				continue
			} else {
				break
			}
		}

		if err != nil {
			fmt.Fprintf(os.Stderr, "%v Error: %v\n", c.Server, err)
			if ee, ok := err.(*ssh.ExitError); ok {
				return ee.ExitStatus()
			}
		}
	} else {
		// stdout and stderr to stdoutBuf
		var stdoutBuf bytes.Buffer
		session.Stdout = &stdoutBuf
		session.Stderr = &stdoutBuf

		// Exec Command(for loop)
		err = session.Run(execCmd)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v Error: %v\n", c.Server, err)
			if ee, ok := err.(*ssh.ExitError); ok {
				return ee.ExitStatus()
			}
		}

		stdoutBufArray := regexp.MustCompile("\r\n|\n\r|\n|\r").Split(stdoutBuf.String(), -1)
		for i, v := range stdoutBufArray {
			if i == len(stdoutBufArray)-1 {
				break
			}
			lineHeader := fmt.Sprintf("%-*s", c.ServerMaxLength, c.Server)
			fmt.Println(outColorStrings(c.Index, lineHeader)+":: ", v)
		}
	}
	return 0
}

// remote ssh server exec command only
func (r *RunInfoCmd) ConSshCmd() int {
	// Stdin only pipes
	if terminal.IsTerminal(syscall.Stdin) == false {
		stdin, _ = ioutil.ReadAll(os.Stdin)
	}

	// get connect server name max length
	conServerNameMax := 0
	for _, conServerName := range r.ServerList {
		if conServerNameMax < len(conServerName) {
			conServerNameMax = len(conServerName)
		}
	}

	finished := make(chan bool)

	// for command exec
	x := 1
	for _, v := range r.ServerList {
		y := x
		c := new(ConInfoCmd)
		conServer := v
		go func() {
			c.Index = y
			c.Count = len(r.ServerList)
			c.Server = conServer
			c.ServerMaxLength = conServerNameMax
			c.Addr = r.ConfList.Server[c.Server].Addr
			c.User = r.ConfList.Server[c.Server].User
			c.Port = "22"
			if r.ConfList.Server[c.Server].Port != "" {
				c.Port = r.ConfList.Server[c.Server].Port
			}
			c.Pass = ""
			if r.ConfList.Server[c.Server].Pass != "" {
				c.Pass = r.ConfList.Server[c.Server].Pass
			}
			c.KeyPath = ""
			if r.ConfList.Server[c.Server].Key != "" {
				c.KeyPath = r.ConfList.Server[c.Server].Key
			}
			c.Cmd = r.ExecCmd
			c.Flag.Parallel = r.Pflag
			c.Flag.PesudoTerm = r.Tflag

			connect, err := c.CreateConnect()
			if err != nil {
				fmt.Fprintf(os.Stderr, "cannot connect %v:%v, %v \n", c.Server, c.Port, err)
				finished <- true
				return
			}
			c.Run(connect)

			finished <- true
		}()
		x++
	}

	for i := 1; i <= len(r.ServerList); i++ {
		<-finished
	}
	return 0
}
