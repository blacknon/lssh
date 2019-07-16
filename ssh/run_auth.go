package ssh

import (
	"fmt"
	"strings"

	"github.com/blacknon/go-sshlib"
	"golang.org/x/crypto/ssh"
)

// Create ssh.Signer into r.AuthMap. Passwords is not get this function.
func (r *Run) createAuthMap() {
	r.AuthMap = map[AuthKey][]ssh.Signer{}

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
	// TODO(blacknon): キー入力でのパスフレーズ取得を実装
	authMethod, err := sshlib.CreateAuthMethodPublicKey(key, password)
	if err != nil {
		return
	}

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
