package ssh

import (
	"bytes"
	"io"
	"time"

	"golang.org/x/crypto/ssh"
)

// @TODO:
//     Dataについては、Stdout/Stderrで分ける必要があるか検討する
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

	// start output
	go sendOutput(outputChan, outputData, outputExit)
	go printOutput(o, outputChan)

	// run command
	c.Session.Run(cmd)

	isExit <- true
	outputExit <- true

	return
}

func sendOutput(outputChan chan<- []byte, buf *bytes.Buffer, isExit <-chan bool) {
loop:
	for {
		if buf.Len() > 0 {
			line, err := buf.ReadBytes('\n')
			outputChan <- line
			if err == io.EOF {
				continue loop
			}
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
		if buf.Len() > 0 {
			line, _ := buf.ReadBytes('\n')
			outputChan <- line
		} else {
			break
		}
	}
	close(outputChan)
}

func (c *shellConn) Kill(isExit chan<- bool) (err error) {
	time.Sleep(10 * time.Millisecond)
	c.Session.Signal(ssh.SIGINT)
	err = c.Session.Close()
	return
}
