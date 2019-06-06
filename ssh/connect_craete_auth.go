package ssh

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/user"
	"strings"

	"golang.org/x/crypto/ssh"
)

// @brief:
//     Create ssh session auth
// @note:
//     - public key auth
//     - password auth
//     - ssh-agent auth
//     - pkcs11 auth
func (c *Connect) createSshAuth(server string) (auth []ssh.AuthMethod, err error) {
	conf := c.Conf.Server[server]

	// public key (single)
	if conf.Key != "" {
		authMethod, err := createSshAuthPublicKey(conf.Key, conf.KeyPass)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s's create public key ssh.AuthMethod err: %s\n", server, err)
		} else {
			auth = append(auth, authMethod)
		}
	}

	// public key (multiple)
	if len(conf.Keys) > 0 {
		for _, key := range conf.Keys {
			keyPathArray := strings.SplitN(key, "::", 2)
			authMethod, err := createSshAuthPublicKey(keyPathArray[0], keyPathArray[1])
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s's create public keys ssh.AuthMethod err: %s\n", server, err)
			} else {
				auth = append(auth, authMethod)
			}
		}
	}

	// cert
	if conf.Cert != "" {
		authMethod, err := createSshAuthCertificate(conf.Cert, conf.CertKey, conf.CertKey)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s's create certificate ssh.AuthMethod err: %s\n", server, err)
		} else {
			auth = append(auth, authMethod)
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
		var signers []ssh.Signer
		signers, err := c.getSshSignerFromPkcs11(server)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s's create pkcs11 ssh.AuthMethod err: %s\n", server, err)
		} else {
			for _, signer := range signers {
				auth = append(auth, ssh.PublicKeys(signer))
			}
		}
	}

	return auth, err
}

// Craete ssh auth (public key)
func createSshAuthPublicKey(key, pass string) (auth ssh.AuthMethod, err error) {
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

// @brief:
//     Create ssh auth (Certificate)
//     key ... keypath::password
// @TODO: PKCS11の利用もできるよう、引数にSignerを渡すように作り変える
func createSshAuthCertificate(cert, key, pass string) (auth ssh.AuthMethod, err error) {
	usr, _ := user.Current()
	cert = strings.Replace(cert, "~", usr.HomeDir, 1)
	key = strings.Replace(key, "~", usr.HomeDir, 1)

	// Read PrivateKey file
	keyData, err := ioutil.ReadFile(key)
	if err != nil {
		return auth, err
	}

	// Create PrivateKey Signer
	var keySigner ssh.Signer
	if pass != "" {
		keySigner, err = ssh.ParsePrivateKeyWithPassphrase(keyData, []byte(pass))
	} else {
		keySigner, err = ssh.ParsePrivateKey(keyData)
	}

	// check err
	if err != nil {
		return auth, err
	}

	// Read Cert file
	certData, err := ioutil.ReadFile(cert)
	if err != nil {
		return auth, err
	}

	// Create PublicKey from Cert
	pubkey, _, _, _, err := ssh.ParseAuthorizedKey(certData)
	if err != nil {
		return auth, err
	}

	// Create Certificate Struct
	certificate, ok := pubkey.(*ssh.Certificate)
	if !ok {
		err = fmt.Errorf("%s\n", "Error: Not create certificate struct data")
		return auth, err
	}

	// Create Certificate Signer
	signer, err := ssh.NewCertSigner(certificate, keySigner)
	if err != nil {
		return auth, err
	}

	// Create AuthMethod
	auth = ssh.PublicKeys(signer)

	return

}
