package ssh

import (
	"bytes"
	"io"
	"time"

	"golang.org/x/crypto/ssh"
)

type shellConn struct {
	*Connect
	Session      *ssh.Session
	StdoutData   *bytes.Buffer
	StderrData   *bytes.Buffer
	ServerList   []string
	OutputPrompt string
	Count        int
}

// @TODO: 変数名その他いろいろと見直しをする！！
//        ローカルのコマンドとパイプでつなげるような処理を実装する予定なので、Stdin、Stdout等の扱いを分離して扱いやすくする
func (c *shellConn) SshShellCmdRun(cmd string, isExit chan<- bool) (err error) {
	// set output
	// @TODO: Stdout,Stderrについて、別途Bufferに書き込みをするよう定義する
	outputData := new(bytes.Buffer)
	c.Session.Stdout = io.MultiWriter(outputData)
	c.Session.Stderr = io.MultiWriter(outputData)

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

	// run command
	c.Session.Start(cmd)

	c.Session.Wait()
	time.Sleep(10 * time.Millisecond)

	outputExit <- true
	<-sendExit
	c.Session.Close()
	isExit <- true
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
	c.Session.Close()

	return
}
