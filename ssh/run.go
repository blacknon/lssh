package ssh

import (
	"bytes"

	"github.com/blacknon/lssh/conf"
	"golang.org/x/crypto/ssh"
)

type Run struct {
	ServerList        []string
	Conf              conf.Config
	IsShell           bool
	IsParallel        bool
	IsParallelShell   bool
	IsX11             bool
	PortForwardLocal  string
	PortForwardRemote string
	ExecCmd           []string
	StdinData         []byte        // 使ってる
	OutputData        *bytes.Buffer // use terminal log
	AuthMap           map[AuthKey][]ssh.Signer
}
