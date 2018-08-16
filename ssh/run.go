package ssh

import (
	"bytes"
	"io/ioutil"
	"os"
	"syscall"

	"github.com/blacknon/lssh/conf"
	"golang.org/x/crypto/ssh/terminal"
)

type Run struct {
	ServerList []string
	Conf       conf.Config
	IsTerm     bool
	IsParallel bool
	ExecCmd    []string
	StdinData  []byte
	OutputData *bytes.Buffer
}

func (r *Run) Start() {
	// Get stdin
	if !terminal.IsTerminal(syscall.Stdin) {
		r.StdinData, _ = ioutil.ReadAll(os.Stdin)
	}

	if len(r.ExecCmd) > 0 {
		r.cmd()
	} else {
		r.term()
	}
}
