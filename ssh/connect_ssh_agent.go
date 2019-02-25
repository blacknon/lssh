package ssh

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"strings"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

func (c *Connect) CreateSshAgentKeyring() (keyring agent.Agent) {
	// declare keyring
	keyring = agent.NewKeyring()

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

		// add key to keyring
		err = keyring.Add(agent.AddedKey{
			PrivateKey:       key,
			ConfirmBeforeUse: true,
			LifetimeSecs:     36000,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed add key to keyring: %v, %v\n", keyPath, err)
			continue
		}
	}

	return
}
