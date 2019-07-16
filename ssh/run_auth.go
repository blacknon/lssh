package ssh

import (
	"fmt"
	"strings"

	"github.com/blacknon/go-sshlib"
	"golang.org/x/crypto/ssh"
)

// Create ssh.Signer into r.AuthMap. Passwords is not get this function.
func (r *Run) createAuthMap() {
	r.AuthMap = map[AuthKey][]ssh.AuthMethod{}

	for _, server := range r.ServerList {
		// get server config
		config := r.Conf.Server[server]

		// Password
		if config.Pass != "" {
			r.registAuthMapPassword(config.Pass)
		}

		// Multiple Password
		if len(config.Passes) > 0 {
			for _, pass := range config.Passes {
				r.registAuthMapPassword(pass)
			}
		}

		// PublicKey
		if config.Key != "" {
			err := r.registAuthMapPublicKey(config.Key, config.KeyPass)
			if err != nil {
				fmt.Println(err)
				continue
			}
		}

		// Multiple PublicKeys
		if len(config.Keys) > 0 {
			for _, key := range config.Keys {
				//
				pair := strings.SplitN(key, "::", 2)
				keyName := pair[0]
				keyPass := ""

				//
				if len(pair) > 1 {
					keyPass = pair[1]
				}

				//
				err := r.registAuthMapPublicKey(keyName, keyPass)
				if err != nil {
					fmt.Println(err)
					continue
				}
			}
		}

		// Certificate
		if config.Cert != "" {
			keySigner, err := sshlib.CreateSignerPublicKeyPrompt(config.CertKey, config.CertKeyPass)
			if err != nil {
				fmt.Println(err)
				continue
			}

			err = r.registAuthMapCertificate(config.Cert, keySigner)
			if err != nil {
				fmt.Println(err)
				continue
			}
		}

		// PKCS11
		if config.PKCS11Use {
			err = registAuthMapPKCS11(config.PKCS11Provider, config.PKCS11PIN)
			if err != nil {
				fmt.Println(err)
				continue
			}
		}
	}
}

//
func (r *Run) registAuthMapPassword(password string) {
	authMethod := sshlib.CreateAuthMethodPassword(password)

	authKey := AuthKey{AUTHKEY_PASSWORD, password}
	r.AuthMap[authKey] = append(r.AuthMap[authKey], authMethod)
}

//
func (r *Run) registAuthMapPublicKey(key, password string) (err error) {
	// Create signer with key input
	signer, err := sshlib.CreateSignerPublicKeyPrompt(key, password)
	if err != nil {
		return
	}

	// Create AuthMethod
	authMethod := ssh.PublicKeys(signer)

	// Regist AuthMethod to AuthMap
	authKey := AuthKey{AUTHKEY_KEY, key}
	r.AuthMap[authKey] = append(r.AuthMap[authKey], authMethod)
}

//
func (r *Run) registAuthMapCertificate(cert string, signer ssh.Signer) (err error) {
	// TODO(blacknon): キー入力でのパスフレーズ取得を実装
	authMethod, err := sshlib.CreateAuthMethodCertificate(cert, keySigner)
	if err != nil {
		returnn
	}

	authKey := AuthKey{AUTHKEY_CERT, cert}
	r.AuthMap[authKey] = append(r.AuthMap[authKey], authMethod)
}

func (r *Run) registAuthMapPKCS11(provider, pin string) (err error) {
	// Create Signer with key input
	signers, err := sshlib.CreateSignerPKCS11Prompt(provider, pin)
	if err != nil {
		return
	}

	authKey := AuthKey{AUTHKEY_PKCS11, provider}
	for _, signer := range signers {
		// Create AuthMethod
		authMethod := ssh.PublicKeys(signer)

		// Regist AuthMethod to AuthMap
		r.AuthMap[authKey] = append(r.AuthMap[authKey], authMethod)
	}
}
