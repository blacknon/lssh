package ssh

import (
	"bytes"
	"fmt"
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
	c.Session.Start(cmd)
	c.Session.Wait()

	fmt.Println(2) // debug

	isExit <- true
	outputExit <- true
	c.Session.Close()

	return
}

func sendOutput(outputChan chan<- []byte, buf *bytes.Buffer, isExit <-chan bool) {
	// @TODO: Bufferが掴んじゃってるから、普通に[]byteでReadをするしかない
	// その方式に切り替える

loop:
	for {
		if rd.Buffered() > 0 {
			fmt.Println(9)
			line, err := rd.ReadBytes('\n')
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
		if rd.Buffered() > 0 {
			line, _ := rd.ReadBytes('\n')
			outputChan <- line
		} else {
			break
		}
	}
	close(outputChan)
}

func (c *shellConn) Kill() (err error) {
	fmt.Println(1) // debug

	time.Sleep(10 * time.Millisecond)
	c.Session.Signal(ssh.SIGINT)
	time.Sleep(10 * time.Millisecond)
	c.Session.Close()
	return
}
