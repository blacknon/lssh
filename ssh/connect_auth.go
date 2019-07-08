package ssh

import (
	"fmt"
	"net"
	"os"

	"golang.org/x/crypto/ssh"
)

// createSshAuth return the necessary ssh.AuthMethod from AuthMap and ssh-agent.
func (c *Connect) createSshAuth(server string) (auth []ssh.AuthMethod, err error) {
	conf := c.Conf.Server[server]

	// public key (single)
	if conf.Key != "" {
		authKey := AuthKey{AUTHKEY_KEY, conf.Key}
		if _, ok := c.AuthMap[authKey]; ok {
			for _, signer := range c.AuthMap[authKey] {
				if signer != nil {
					authMethod := ssh.PublicKeys(signer)
					auth = append(auth, authMethod)
				}
			}
		}
	}

	// public key (multiple)
	if len(conf.Keys) > 0 {
		for _, key := range conf.Keys {
			authKey := AuthKey{AUTHKEY_KEY, key}
			if _, ok := c.AuthMap[authKey]; ok {
				for _, signer := range c.AuthMap[authKey] {
					if signer != nil {
						authMethod := ssh.PublicKeys(signer)
						auth = append(auth, authMethod)
					}
				}
			}
		}
	}

	// cert
	if conf.Cert != "" {
		authKey := AuthKey{AUTHKEY_CERT, conf.Cert}
		if _, ok := c.AuthMap[authKey]; ok {
			for _, signer := range c.AuthMap[authKey] {
				if signer != nil {
					authMethod := ssh.PublicKeys(signer)
					auth = append(auth, authMethod)
				}
			}
		}
	}

	// ssh password (single)
	if conf.Pass != "" {
		auth = append(auth, ssh.Password(conf.Pass))
	}

	// ssh password (multiple)
	if len(conf.Passes) > 0 {
		for _, pass := range conf.Passes {
			auth = append(auth, ssh.Password(pass))
		}
	}

	// ssh agent
	if conf.AgentAuth {
		var signers []ssh.Signer
		_, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK"))
		if err != nil {
			signers, err = c.sshAgent.Signers()
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s's create sshAgent ssh.AuthMethod err: %s\n", server, err)
			} else {
				auth = append(auth, ssh.PublicKeys(signers...))
			}
		} else {
			signers, err = c.sshExtendedAgent.Signers()
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s's create sshAgent ssh.AuthMethod err: %s\n", server, err)
			} else {
				auth = append(auth, ssh.PublicKeys(signers...))
			}
		}
	}

	if conf.PKCS11Use {
		// @TODO: confのチェック時にPKCS11のProviderのPATHチェックを行う
		authKey := AuthKey{AUTHKEY_PKCS11, conf.PKCS11Provider}
		if _, ok := c.AuthMap[authKey]; ok {
			for _, signer := range c.AuthMap[authKey] {
				if signer != nil {
					authMethod := ssh.PublicKeys(signer)
					auth = append(auth, authMethod)
				}
			}
		}
	}

	return auth, err
}
