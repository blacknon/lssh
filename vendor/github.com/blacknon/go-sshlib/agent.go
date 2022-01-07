// Copyright (c) 2019 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package sshlib

import (
	"net"
	"os"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// AgentInterface Interface for storing agent.Agent or agent.ExtendedAgent.
type AgentInterface interface{}

// ConnectSshAgent
func ConnectSshAgent() (ag AgentInterface) {
	// Get env "SSH_AUTH_SOCK" and connect.
	sockPath := os.Getenv("SSH_AUTH_SOCK")
	sock, err := net.Dial("unix", sockPath)

	if err != nil {
		ag = agent.NewKeyring()
	} else {
		// connect SSH_AUTH_SOCK
		ag = agent.NewClient(sock)
	}

	return
}

// AddKeySshAgent is rapper agent.Add().
// key must be a *rsa.PrivateKey, *dsa.PrivateKey or
// *ecdsa.PrivateKey, which will be inserted into the agent.
//
// Should use `ssh.ParseRawPrivateKey()` or `ssh.ParseRawPrivateKeyWithPassphrase()`.
func (c *Connect) AddKeySshAgent(sshAgent interface{}, key interface{}) {
	addedKey := agent.AddedKey{
		PrivateKey:       key,
		ConfirmBeforeUse: true,
		LifetimeSecs:     3000,
	}

	switch ag := sshAgent.(type) {
	case agent.Agent:
		ag.Add(addedKey)
	case agent.ExtendedAgent:
		ag.Add(addedKey)
	}
}

// ForwardAgent forward ssh-agent in session.
func (c *Connect) ForwardSshAgent(session *ssh.Session) {
	// forward ssh-agent
	switch ag := c.Agent.(type) {
	case agent.Agent:
		agent.ForwardToAgent(c.Client, ag)
	case agent.ExtendedAgent:
		agent.ForwardToAgent(c.Client, ag)
	}

	agent.RequestAgentForwarding(session)
}
