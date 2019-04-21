package ssh

import (
	"io/ioutil"
	"os/user"
	"strings"

	"golang.org/x/crypto/ssh"
)

// @TODO: v0.5.3
//     ssh authを複数指定できるようにする(conf.goについても修正が必要？)

// @brief:
//     Create ssh session auth
// @note:
//     - public key auth
//     - password auth
//     - ssh-agent auth
func (c *Connect) createSshAuth(server string) (auth []ssh.AuthMethod, err error) {
	conf := c.Conf.Server[server]

	// public key (single)
	if conf.Key != "" {
		authMethod, err := createSshAuthPublicKey(conf.Key, conf.KeyPass)
		if err != nil {
			return auth, err
		}
		auth = append(auth, authMethod)
	}

	// public key (multiple)
	if len(conf.Keys) > 0 {
		for _, key := range conf.Keys {
			keyPathArray := strings.SplitN(keyPathData, "::", 2)
			keyPath := strings.Replace(keyPathArray[0], "~", usr.HomeDir, 1)
			keyPassphase := keyPathArray[1]

			authMethod, err := createSshAuthPublicKey(keyPath, keyPassphase)
			if err != nil {
				return auth, err
			}

			auth = append(auth, authMethod)
		}
	}

	// ssh password (single)
	if conf.Pass != "" {
		auth = append(auth, ssh.Password(conf.Pass))
	}

	// ssh password (multiple)
	if len(conf.Passes) > 0 {
		for _, pass = range conf.Passes {
			auth = append(auth, ssh.Password(pass))
		}
	}

	// ssh agent
	if conf.AgentAuth {
		signers, err := c.sshAgent.Signers()
		if err != nil {
			return auth, err
		}

		auth = append(auth, ssh.PublicKeys(signers...))
	}

	return auth, err
}

// Craete ssh auth (public key)
func createSshAuthPublicKey(key string, pass string) (auth ssh.AuthMethod, err error) {
	usr, _ := user.Current()
	key = strings.Replace(key, "~", usr.HomeDir, 1)

	// Read PrivateKey file
	keyData, err := ioutil.ReadFile(key)
	if err != nil {
		return auth, err
	}

	// Read signer from PrivateKey
	var signer ssh.Signer
	if pass != "" {
		signer, err = ssh.ParsePrivateKeyWithPassphrase(keyData, []byte(pass))
	} else {
		signer, err = ssh.ParsePrivateKey(keyData)
	}

	// check err
	if err != nil {
		return auth, err
	}

	auth = ssh.PublicKeys(signer)
	return auth, err
}
