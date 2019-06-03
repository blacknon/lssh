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
	stdoutReader, _ := c.Session.StdoutPipe()
	stderrReader, _ := c.Session.StderrPipe()

	//
	mr := io.MultiReader(stdoutReader, stderrReader)
	go io.Copy(outputData, mr)

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
	time.Sleep(10 * time.Millisecond)

	isExit <- true
	outputExit <- true
	c.Session.Close()

	return
}

func sendOutput(outputChan chan<- []byte, buf *bytes.Buffer, isExit <-chan bool) {
	beforeLen := 0
loop:
	for {
		len := buf.Len()
		if len != beforeLen {
			for {
				line, err := buf.ReadBytes('\n')
				outputChan <- line
				if err == io.EOF {
					break
				}
			}
			beforeLen = len
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
				outputChan <- line
				if err == io.EOF {
					break
				}
			}
		} else {
			break
		}
	}
	close(outputChan)
}

func (c *shellConn) Kill() (err error) {
	time.Sleep(10 * time.Millisecond)
	c.Session.Signal(ssh.SIGINT)

	// Session Close
	c.Session.Close()
	// stdout?が掴んでしまってるので、正常にCloseできない。

	// Client Close
	// c.Client.Close()
	// c.Client = nil

	// @TODO:
	//     新しいConnectを作る際にPassやPINが必要な場合にエラーになるので、そこを解消する

	return
}
