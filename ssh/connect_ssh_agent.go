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

// @TODO: v0.5.3
//     ssh-agentの処理について、既存のagentがない場合は自動生成するように修正する
//     ※ Agentを作成しておき、それを使ってExtendAgentを作るようにすればいい
//     　 とりあえず、Agentがない場合は適当な名前のファイルで作成するようにしてやればいいか…
//     　 (~/.lssh_sshagent_$(md5 unixtime).sock)とかで
func (c *Connect) CreateSshAgent() (err error) {
	// Get SSH_AUTH-SOCK
	sock, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to SSH Agent %v", err)
		return
	}

	// declare sshAgent
	sshAgent := agent.NewClient(sock)

	// user path
	usr, _ := user.Current()

	conf := c.Conf.Server[c.Server]
	sshKeys := conf.SSHAgentKeyPath

	for _, keyPathData := range sshKeys {
		// parse ssh key strings
		//    * keyPathArray[0] ... KeyPath
		//    * keyPathArray[1] ... KeyPassPhase
		keyPathArray := strings.SplitN(keyPathData, "::", 2)

		// key path to fullpath
		keyPath := strings.Replace(keyPathArray[0], "~", usr.HomeDir, 1)

		// read key file
		keyData, err := ioutil.ReadFile(keyPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed read key file: %v\n", err)
			continue
		}

		// parse key data
		var key interface{}
		if len(keyPathArray) > 1 {
			key, err = ssh.ParseRawPrivateKeyWithPassphrase(keyData, []byte(keyPathArray[1]))
		} else {
			key, err = ssh.ParseRawPrivateKey(keyData)
		}

		if err != nil {
			fmt.Fprintf(os.Stderr, "failed parse key file: %v, %v\n", keyPath, err)
			continue
		}

		// add key to sshAgent
		err = sshAgent.Add(agent.AddedKey{
			PrivateKey:       key,
			ConfirmBeforeUse: true,
			LifetimeSecs:     30,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed add key to sshAgent: %v, %v\n", keyPath, err)
			continue
		}
	}

	c.sshAgent = sshAgent

	return
}
