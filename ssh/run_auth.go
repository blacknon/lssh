package ssh

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"strings"
	"syscall"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"
)

// @brief:
//     Create ssh.Signer into r.AuthMap.
//     Passwords is not get this function.
func (r *Run) createAuthMap() {
	for _, server := range r.ServerList {
		// get server config
		config := r.Conf.Server[server]

		// Public key auth (single)
		if config.Key != "" {
			r.registAuthMapPublicKey(server, config.Key, config.Pass)
		}

		// Public keys auth (array)
		if len(config.Keys) > 0 {
			for _, key := range config.Keys {
				// @TODO: バグあるので、対応が必要！！！
				// パスワードがない場合、配列の数をオーバーしちゃうので分岐する！
				keyPair := strings.SplitN(key, "::", 2)
				r.registAuthMapPublicKey(server, keyPair[0], keyPair[1])
			}
		}

		// Certificate auth
		if config.Cert != "" {
			r.registAuthMapCertificate(server, config.Cert, config.CertKey, config.CertKeyPass)
		}

		// ssh-agent
		// if config.AgentAuth {

		// }

		// pkcs11
		// if config.PKCS11Use {

		// }
	}
}

func (r *Run) registAuthMapPublicKey(server, key, pass string) {
	authKey := AuthKey{AUTHKEY_KEY, key}

	if _, ok := r.AuthMap[authKey]; !ok {
		signer, err := createSshSignerPublicKey(key, pass)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s's create public key ssh.Signer err: %s\n", server, err)
		}
		r.AuthMap[authKey] = []ssh.Signer{signer}
	}
}

func (r *Run) registAuthMapCertificate(server, cert, key, pass string) {
	authKey := AuthKey{AUTHKEY_CERT, cert}

	if _, ok := r.AuthMap[authKey]; !ok {
		signer, err := createSshSignerCertificate(cert, key, pass)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s's create certificate ssh.Signer err: %s\n", server, err)
		}
		r.AuthMap[authKey] = []ssh.Signer{signer}
	}
}

// @brief:
//     create ssh.Signer from Publickey
func createSshSignerPublicKey(key, pass string) (signer ssh.Signer, err error) {
	// repeat count
	rep := 3

	usr, _ := user.Current()
	key = strings.Replace(key, "~", usr.HomeDir, 1)

	// Read PrivateKey file
	keyData, err := ioutil.ReadFile(key)
	if err != nil {
		return signer, err
	}

	if pass != "" {
		signer, err = ssh.ParsePrivateKeyWithPassphrase(keyData, []byte(pass))
	} else {
		signer, err = ssh.ParsePrivateKey(keyData)
		if err == errors.New("ssh: cannot decode encrypted private keys") {
			msg := key + "'s passphase:"

			for i := 0; i < rep; i++ {
				pass, _ = getPassPhase(msg)
				signer, err = ssh.ParsePrivateKeyWithPassphrase(keyData, []byte(pass))

				if err != errors.New("x509: decryption password incorrect") {
					break
				}
			}
		}
	}

	return
}

// @brief:
//     create ssh.Signer from Certificate
func createSshSignerCertificate(cert, key, pass string) (signer ssh.Signer, err error) {
	usr, _ := user.Current()
	cert = strings.Replace(cert, "~", usr.HomeDir, 1)
	key = strings.Replace(key, "~", usr.HomeDir, 1)

	// Read Cert file
	certData, err := ioutil.ReadFile(cert)
	if err != nil {
		return signer, err
	}

	// Create PublicKey from Cert
	pubkey, _, _, _, err := ssh.ParseAuthorizedKey(certData)
	if err != nil {
		return signer, err
	}

	// Create Certificate Struct
	certificate, ok := pubkey.(*ssh.Certificate)
	if !ok {
		err = fmt.Errorf("%s\n", "Error: Not create certificate struct data")
		return signer, err
	}

	// create key signer
	var keySigner ssh.Signer
	keySigner, err = createSshSignerPublicKey(key, pass)
	if err != nil {
		return signer, err
	}

	// Create Certificate Signer
	signer, err = ssh.NewCertSigner(certificate, keySigner)
	if err != nil {
		return signer, err
	}

	return
}

func getPassPhase(msg string) (input string, err error) {
	fmt.Printf(msg)
	result, err := terminal.ReadPassword(int(syscall.Stdin))

	if len(result) == 0 {
		err = fmt.Errorf("err: input is empty")
		return
	}

	return
}
