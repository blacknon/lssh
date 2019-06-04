package ssh

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"time"

	"golang.org/x/crypto/ssh"
)

// Convert []*Connect to []*shellConn, and Connect ssh
func (s *shell) CreateConn(conns []*Connect) {
	isExit := make(chan bool)
	connectChan := make(chan *Connect)

	// create ssh connect
	for _, c := range conns {
		conn := c

		// Connect ssh (goroutine)
		go func() {
			// Connect ssh
			err := conn.CreateClient()

			// send exit channel
			isExit <- true

			// check error
			if err != nil {
				fmt.Fprintf(os.Stderr, "Cannot connect session %v, %v\n", conn.Server, err)
				return
			}

			// send ssh client
			connectChan <- conn
		}()
	}

	for i := 0; i < len(conns); i++ {
		<-isExit

		select {
		case c := <-connectChan:
			// create shellConn
			sc := new(shellConn)
			sc.ExecHistory = map[int]*ExecHistory{}
			sc.Connect = c
			sc.ServerList = s.ServerList
			sc.OutputPrompt = s.OPROMPT

			// append shellConn
			s.Connects = append(s.Connects, sc)
		case <-time.After(10 * time.Millisecond):
			continue
		}
	}
}

// @TODO: from run_shell.go send signal
// func (s *shell) sendSignal() {}

type shellConn struct {
	*Connect
	Session      *ssh.Session
	ServerList   []string
	OutputPrompt string
	Count        int
	ExecHistory  map[int]*ExecHistory
}

type ExecHistory struct {
	Cmd        string
	OutputData *bytes.Buffer
	StdoutData *bytes.Buffer
	StderrData *bytes.Buffer
}

// @TODO: 変数名その他いろいろと見直しをする！！
//        ローカルのコマンドとパイプでつなげるような処理を実装する予定なので、Stdin、Stdout、Stderrの扱いを分離して扱いやすくする
func (c *shellConn) SshShellCmdRun(cmd string, isExit chan<- bool) (err error) {
	// Request tty
	c.Session, err = c.setIsTerm(c.Session)
	if err != nil {
		fmt.Println(err)
	}

	// ExecHistory
	execHist := &ExecHistory{
		Cmd:        cmd,
		OutputData: new(bytes.Buffer),
		StdoutData: new(bytes.Buffer),
		StderrData: new(bytes.Buffer),
	}

	// set output
	outputData := new(bytes.Buffer)
	c.Session.Stdout = io.MultiWriter(outputData, execHist.OutputData, execHist.StdoutData)
	c.Session.Stderr = io.MultiWriter(outputData, execHist.OutputData, execHist.StderrData)

	// Create Output
	o := &Output{
		Templete:   c.OutputPrompt,
		Count:      c.Count,
		ServerList: c.ServerList,
		Conf:       c.Conf.Server[c.Server],
		AutoColor:  true,
	}
	o.Create(c.Server)

	// craete output data channel
	outputChan := make(chan []byte)
	outputExit := make(chan bool)
	sendExit := make(chan bool)

	// start output
	go sendOutput(outputChan, outputData, outputExit, sendExit)
	go printOutput(o, outputChan)

	// @TODO: test(keepalive)
	go func() {
		for {
			_, _ = c.Session.SendRequest("keepalive@golang.org", true, nil)
			time.Sleep(15 * time.Second)
		}
	}()

	// run command
	c.Session.Start(cmd)

	c.Session.Wait()
	time.Sleep(10 * time.Millisecond)

	// send output exit
	outputExit <- true

	// wait output exit finish
	<-sendExit

	// session close
	c.Session.Close()

	// exit run command
	isExit <- true

	// append ExecHistory
	c.ExecHistory[c.Count] = execHist

	return
}

func sendOutput(outputChan chan<- []byte, buf *bytes.Buffer, isExit <-chan bool, sendExit chan<- bool) {
	beforeLen := 0
loop:
	for {
		length := buf.Len()
		if length != beforeLen {
			for {
				line, err := buf.ReadBytes('\n')
				if err == io.EOF {
					if len(line) > 0 {
						outputChan <- line
					}
					break
				}
				outputChan <- line
			}
			beforeLen = length
		} else {
			select {
			case <-isExit:
				break loop
			case <-time.After(10 * time.Millisecond):
				continue loop
			}
		}
	}

	for {
		if buf.Len() != beforeLen {
			for {
				line, err := buf.ReadBytes('\n')
				if err == io.EOF {
					if len(line) > 0 {
						outputChan <- line
					}
					break
				}
				outputChan <- line
			}
		} else {
			break
		}
	}
	close(outputChan)
	sendExit <- true
}

func (c *shellConn) Kill() (err error) {
	time.Sleep(10 * time.Millisecond)
	c.Session.Signal(ssh.SIGINT)

	// Session Close
	err = c.Session.Close()

	return
}
