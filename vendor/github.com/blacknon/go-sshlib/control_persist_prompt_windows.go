//go:build windows

package sshlib

import (
	"os/exec"
)

type controlPersistPromptBridge struct{}

func setupControlPersistPromptIPC(cmd *exec.Cmd) (*controlPersistPromptBridge, func(), error) {
	return nil, func() {}, nil
}

func startControlPersistPromptIPC(bridge *controlPersistPromptBridge) {}

func loadControlPersistPrompt() (PromptFunc, func(), error) {
	return nil, func() {}, nil
}
