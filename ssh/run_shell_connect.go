package ssh

import (
	"bytes"
	"time"

	"golang.org/x/crypto/ssh"
)

// @TODO:
//     Dataについては、Stdout/Stderrで分ける必要があるか検討する
type shellConn struct {
	*Connect
	Session    *ssh.Session
	StdoutData *bytes.Buffer
	StderrData *bytes.Buffer
}

// @TODO: 変数名その他いろいろと見直しをする！！
//        ローカルのコマンドとパイプでつなげるような処理を実装する予定なので、Stdin、Stdout等の扱いを分離して扱いやすくする
func (c *shellConn) SshShellCmdRun(cmd string, isExit chan<- bool) (err error) {
	c.Session.Stdout = c.StdoutData
	c.Session.Stderr = c.StderrData

	c.Session.Run(cmd)

	isExit <- true
	return
}

func (c *shellConn) Kill(isExit chan<- bool) (err error) {
	time.Sleep(10 * time.Millisecond)
	c.Session.Signal(ssh.SIGINT)
	err = c.Session.Close()
	return
}
