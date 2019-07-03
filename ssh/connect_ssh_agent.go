package ssh

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/user"
	"strings"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// TODO: sshAgentをinterfaceにしてちゃんとStructに格納する
//     sshAgentをちゃんとinterfaceでStructに格納して、無駄にKeyを格納させるような状態を改善する
//     ※ 0.6.0に上げる際のリファクタ対象
//     // sample
//     switch sshAgent.(type) {
//     case agent.Agent :
//         hogehoge
//     case agent.ExtendedAgent :
//         fugafuga
//     }
func (c *Connect) CreateSshAgent() (err error) {
	conf := c.Conf.Server[c.Server]
	sshKeys := conf.SSHAgentKeyPath

	// TODO(blacknon): Structureの項目をinterface化して分岐を統合する

	// Get SSH_AUTH-SOCK
	sock, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK"))
	if err != nil {
		// declare sshAgent(Agent)
		sshAgent := agent.NewKeyring()
		for _, keyPathData := range sshKeys {
			key, err := parseKeyArray(keyPathData)
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed parse key: %v\n", err)
				continue
			}

			// add key to sshAgent
			err = sshAgent.Add(agent.AddedKey{
				PrivateKey:       key,
				ConfirmBeforeUse: true,
				LifetimeSecs:     3000,
			})
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed add key to sshAgent: %v\n", err)
				continue
			}
		}
		c.sshAgent = sshAgent
		err = nil
	} else {
		// declare sshAgent(ExtendedAgent)
		sshAgent := agent.NewClient(sock)
		for _, keyPathData := range sshKeys {
			key, err := parseKeyArray(keyPathData)
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed parse key: %v\n", err)
				continue
			}

			// add key to sshAgent
			err = sshAgent.Add(agent.AddedKey{
				PrivateKey:       key,
				ConfirmBeforeUse: true,
				LifetimeSecs:     3000,
			})
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed add key to sshAgent: %v\n", err)
				continue
			}
		}
		c.sshExtendedAgent = sshAgent
	}
	return
}

func parseKeyArray(keyPathStr string) (key interface{}, err error) {
	// parse ssh key strings
	//    * keyPathArray[0] ... KeyPath
	//    * keyPathArray[1] ... KeyPassPhase
	keyArray := strings.SplitN(keyPathStr, "::", 2)

	// key path to fullpath
	usr, _ := user.Current()
	keyPath := strings.Replace(keyArray[0], "~", usr.HomeDir, 1)

	// read key file
	keyData, err := ioutil.ReadFile(keyPath)
	if err != nil {
		return
	}

	// parse key data
	if len(keyArray) > 1 {
		key, err = ssh.ParseRawPrivateKeyWithPassphrase(keyData, []byte(keyArray[1]))
	} else {
		key, err = ssh.ParseRawPrivateKey(keyData)
	}

	return
}
