//go:build windows

package lsmuxsession

import "fmt"

type Daemon struct {
	Name       string
	ConfigPath string
	SocketPath string
	Exe        string
	Args       []string
	Env        []string
}

type AttachOptions struct {
	PrefixSpec string
	DetachSpec string
}

func (d *Daemon) Run(ready func()) error {
	return fmt.Errorf("lsmux attach/detach sessions are not supported on Windows yet (ConPTY support is required)")
}

func Attach(session Session, options AttachOptions) error {
	return fmt.Errorf("lsmux attach/detach sessions are not supported on Windows yet (ConPTY support is required)")
}
