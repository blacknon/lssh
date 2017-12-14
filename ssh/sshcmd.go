package ssh

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"

	"github.com/blacknon/lssh/conf"
	termbox "github.com/nsf/termbox-go"
)

//var (
//	stdin []byte
//)

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

	StdinTempPath string
}

type ConInfoCmdFlag struct {
	PesudoTerm bool
	Parallel   bool
}

func getTmpName() string {
	var n uint64
	binary.Read(rand.Reader, binary.LittleEndian, &n)
	return strconv.FormatUint(n, 36) + ".lssh.tmp"
}

func outColorStrings(num int, inStrings string) string {
	returnStr := ""
	color := num % 5
	switch color {
	// Red
	case 1:
		returnStr = fmt.Sprintf("\x1b[31m%s\x1b[0m", inStrings)
	// Yellow
	case 2:
		returnStr = fmt.Sprintf("\x1b[33m%s\x1b[0m", inStrings)
	// Blue
	case 3:
		returnStr = fmt.Sprintf("\x1b[34m%s\x1b[0m", inStrings)
	// Magenta
	case 4:
		returnStr = fmt.Sprintf("\x1b[35m%s\x1b[0m", inStrings)
	// Cyan
	case 0:
		returnStr = fmt.Sprintf("\x1b[36m%s\x1b[0m", inStrings)
	}
	return returnStr
}

// exec ssh command function
func (c *ConInfoCmd) Run() int {
	// New ssh config
	config := &ssh.ClientConfig{}
	if c.KeyPath != "" {
		// Read PublicKey
		buffer, err := ioutil.ReadFile(c.KeyPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:%s%n", err)
			return 1
		}
		key, err := ssh.ParsePrivateKey(buffer)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:%s%n", err)
			return 1
		}

		// Create ssh client config for KeyAuth
		config = &ssh.ClientConfig{
			User: c.User,
			Auth: []ssh.AuthMethod{
				ssh.PublicKeys(key)},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			Timeout:         60 * time.Second,
		}
	} else {
		// Create ssh client config for PasswordAuth
		config = &ssh.ClientConfig{
			User: c.User,
			Auth: []ssh.AuthMethod{
				ssh.Password(c.Pass)},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			Timeout:         60 * time.Second,
		}
	}

	// New connect
	conn, err := ssh.Dial("tcp", c.Addr+":"+c.Port, config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot connect %v: %v \n", c.Port, err)
		return 1
	}
	defer conn.Close()

	fmt.Println("test")
	// New Session
	session, err := conn.NewSession()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot open new session: %v \n", err)
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

	// stdin tmp file Open.
	stdinTempRead, _ := os.OpenFile(c.StdinTempPath, os.O_RDONLY, 0600)
	session.Stdin = stdinTempRead

	// exec command join
	execCmd := strings.Join(c.Cmd, " ")
	fmt.Println(execCmd)

	if c.Count == 1 {
		session.Stdout = os.Stdout
		session.Stderr = os.Stderr

		err = session.Run(execCmd)

		if err != nil {
			fmt.Fprint(os.Stderr, err)
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

		// commandStatus: Chan can not continuously read buffer in for.
		//                For this reason, the processing end is detected using a variable.
		commandStatus := true

		// Exec Command(parallel)
		go func() {
			err = session.Run(execCmd)
			commandStatus = false
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
				fmt.Println(outColorStrings(c.Index, lineHeader)+":: ", v)

			}
			if commandStatus == true {
				continue
			} else {
				break
			}
		}

		if err != nil {
			fmt.Fprint(os.Stderr, err)
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
			fmt.Fprint(os.Stderr, err)
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

func execCommandOverSsh(connectServerNum int, connectServer string, connectServerHeadLength int, listSum int, confList conf.Config, terminalMode bool, parallelMode bool, stdinTempPath string, execRemoteCmd ...string) int {
	connectPort := "22"
	if confList.Server[connectServer].Port != "" {
		connectPort = confList.Server[connectServer].Port
	}
	connectHostPort := confList.Server[connectServer].Addr + ":" + connectPort
	connectPass := confList.Server[connectServer].Pass
	connectKey := confList.Server[connectServer].Key

	// Set ssh client config
	config := &ssh.ClientConfig{}
	if connectKey != "" {
		// Read PublicKey
		buffer, err := ioutil.ReadFile(connectKey)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:%s%n", err)
			return 1
		}
		key, err := ssh.ParsePrivateKey(buffer)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:%s%n", err)
			return 1
		}

		// Create ssh client config for KeyAuth
		config = &ssh.ClientConfig{
			User: confList.Server[connectServer].User,
			Auth: []ssh.AuthMethod{
				ssh.PublicKeys(key)},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			Timeout:         60 * time.Second,
		}
	} else {
		// Create ssh client config for PasswordAuth
		config = &ssh.ClientConfig{
			User: confList.Server[connectServer].User,
			Auth: []ssh.AuthMethod{
				ssh.Password(connectPass)},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			Timeout:         60 * time.Second,
		}
	}

	// New Connect create
	conn, err := ssh.Dial("tcp", connectHostPort, config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot connect %v: %v \n", connectHostPort, err)
		return 1
	}
	defer conn.Close()

	// New Session
	session, err := conn.NewSession()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot open new session: %v \n", err)
		return 1
	}
	defer session.Close()

	go func() {
		time.Sleep(2419200 * time.Second)
		conn.Close()
	}()

	if terminalMode == true {
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

	// stdin tmp file Open.
	stdinTempRead, _ := os.OpenFile(stdinTempPath, os.O_RDONLY, 0600)
	session.Stdin = stdinTempRead

	// exec command join
	execRemoteCmdString := strings.Join(execRemoteCmd, " ")

	if listSum == 1 {
		session.Stdout = os.Stdout
		session.Stderr = os.Stderr

		err = session.Run(execRemoteCmdString)

		if err != nil {
			fmt.Fprint(os.Stderr, err)
			if ee, ok := err.(*ssh.ExitError); ok {
				return ee.ExitStatus()
			}
		}
		return 0
	} else if parallelMode == true {
		// stdout and stderr to stdoutBuf
		var stdoutBuf bytes.Buffer

		session.Stdout = &stdoutBuf
		session.Stderr = &stdoutBuf

		// commandStatus: Chan can not continuously read buffer in for.
		//                For this reason, the processing end is detected using a variable.
		commandStatus := true

		// Exec Command(parallel)
		go func() {
			err = session.Run(execRemoteCmdString)
			commandStatus = false
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

				lineHeader := fmt.Sprintf("%-*s", connectServerHeadLength, connectServer)
				fmt.Println(outColorStrings(connectServerNum, lineHeader)+":: ", v)

			}
			if commandStatus == true {
				continue
			} else {
				break
			}
		}

		if err != nil {
			fmt.Fprint(os.Stderr, err)
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
		err = session.Run(execRemoteCmdString)
		if err != nil {
			fmt.Fprint(os.Stderr, err)
			if ee, ok := err.(*ssh.ExitError); ok {
				return ee.ExitStatus()
			}
		}

		stdoutBufArray := regexp.MustCompile("\r\n|\n\r|\n|\r").Split(stdoutBuf.String(), -1)

		for i, v := range stdoutBufArray {
			if i == len(stdoutBufArray)-1 {
				break
			}

			lineHeader := fmt.Sprintf("%-*s", connectServerHeadLength, connectServer)
			fmt.Println(outColorStrings(connectServerNum, lineHeader)+":: ", v)

		}

	}

	return 0
}

// remote ssh server exec command only
func ConSshCmd(serverList []string, confList conf.Config, tFlag bool, pFlag bool, execCmd ...string) int {
	// Create tmp file
	stdinTemp, err := ioutil.TempFile("", getTmpName())
	if err != nil {
		panic(err)
	}
	defer os.Remove(stdinTemp.Name())

	// Stdin only pipes
	if terminal.IsTerminal(syscall.Stdin) == false {
		io.Copy(stdinTemp, os.Stdin)
	}

	// get connect server name max length
	conServerNameMax := 0
	for _, conServerName := range serverList {
		if conServerNameMax < len(conServerName) {
			conServerNameMax = len(conServerName)
		}
	}

	conServerCnt := len(serverList)

	if conServerCnt > 1 {
		finished := make(chan bool)

		// for command exec
		x := 1
		for _, conServer := range serverList {
			y := x
			c := new(ConInfoCmd)
			targetServer := conServer
			go func() {
				c.StdinTempPath = stdinTemp.Name()

				c.Index = y
				c.Count = conServerCnt
				c.Server = targetServer
				c.ServerMaxLength = conServerNameMax
				c.Addr = confList.Server[c.Server].Addr
				c.User = confList.Server[c.Server].User
				c.Port = "22"
				if confList.Server[c.Server].Port != "" {
					c.Port = confList.Server[c.Server].Port
				}
				c.Pass = ""
				if confList.Server[c.Server].Pass != "" {
					c.Pass = confList.Server[c.Server].Pass
				}
				c.KeyPath = ""
				if confList.Server[c.Server].Key != "" {
					c.KeyPath = confList.Server[c.Server].Key
				}
				c.Cmd = execCmd

				c.Run()

				//execCommandOverSsh(y, targetServer, conServerNameMax, conServerCnt, confList, tFlag, pFlag, stdinTemp.Name(), execCmd...)
				finished <- true
			}()
			x++
		}

		for i := 1; i <= conServerCnt; i++ {
			<-finished
		}
	} else {
		for i, conServer := range serverList {
			c := new(ConInfoCmd)
			c.StdinTempPath = stdinTemp.Name()

			c.Index = i
			c.Count = conServerCnt
			c.Server = conServer
			c.ServerMaxLength = conServerNameMax
			c.Addr = confList.Server[c.Server].Addr
			c.User = confList.Server[c.Server].User
			c.Port = "22"
			if confList.Server[c.Server].Port != "" {
				c.Port = confList.Server[c.Server].Port
			}
			c.Pass = ""
			if confList.Server[c.Server].Pass != "" {
				c.Pass = confList.Server[c.Server].Pass
			}
			c.KeyPath = ""
			if confList.Server[c.Server].Key != "" {
				c.KeyPath = confList.Server[c.Server].Key
			}
			c.Cmd = execCmd

			c.Run()
			//execCommandOverSsh(i, conServer, conServerNameMax, conServerCnt, confList, tFlag, pFlag, stdinTemp.Name(), execCmd...)
		}
	}

	return 0
}
