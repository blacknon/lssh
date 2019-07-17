package ssh

import (
	"fmt"
	"strings"

	"github.com/blacknon/go-sshlib"
	"github.com/blacknon/lssh/common"
	"golang.org/x/crypto/ssh"
)

const SSH_AUTH_SOCK = "SSH_AUTH_SOCK"

// createAuthMethodMap Create ssh.AuthMethod, into r.AuthMethodMap.
func (r *Run) createAuthMethodMap() {
	srvs := r.ServerList
	for _, server := range r.ServerList {
		proxySrvs, _ := getProxyRoute(server, r.Conf)

		for _, proxySrv := range proxySrvs {
			if proxySrv.Type == "ssh" {
				srvs = append(srvs, proxySrv.Name)
			}
		}
	}

	srvs = common.GetUniqueSlice(srvs)

	// Init r.AuthMethodMap
	r.authMethodMap = map[AuthKey][]ssh.AuthMethod{}
	r.serverAuthMethodMap = map[string][]ssh.AuthMethod{}

	for _, server := range srvs {
		// get server config
		config := r.Conf.Server[server]

		// Password
		if config.Pass != "" {
			r.registAuthMapPassword(server, config.Pass)
		}

		// Multiple Password
		if len(config.Passes) > 0 {
			for _, pass := range config.Passes {
				r.registAuthMapPassword(server, pass)
			}
		}

		// PublicKey
		if config.Key != "" {
			err := r.registAuthMapPublicKey(server, config.Key, config.KeyPass)
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
				err := r.registAuthMapPublicKey(server, keyName, keyPass)
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

			err = r.registAuthMapCertificate(server, config.Cert, keySigner)
			if err != nil {
				fmt.Println(err)
				continue
			}
		}

		// ssh-agent
		// if config.AgentAuth {}

		// PKCS11
		if config.PKCS11Use {
			err := r.registAuthMapPKCS11(server, config.PKCS11Provider, config.PKCS11PIN)
			if err != nil {
				fmt.Println(err)
				continue
			}
		}
	}
}

//
func (r *Run) SetupSshAgent() {
	// Connect ssh-agent
	r.agent = sshlib.ConnectSshAgent()
}

//
func (r *Run) registAuthMapPassword(server, password string) {
	authKey := AuthKey{AUTHKEY_PASSWORD, password}
	if _, ok := r.authMethodMap[authKey]; !ok {
		authMethod := sshlib.CreateAuthMethodPassword(password)

		// Regist AuthMethod to authMethodMap
		r.authMethodMap[authKey] = append(r.authMethodMap[authKey], authMethod)
	}

	// Regist AuthMethod to serverAuthMethodMap from authMethodMap
	r.serverAuthMethodMap[server] = append(r.serverAuthMethodMap[server], r.authMethodMap[authKey]...)
}

//
func (r *Run) registAuthMapPublicKey(server, key, password string) (err error) {
	authKey := AuthKey{AUTHKEY_KEY, key}
	if _, ok := r.authMethodMap[authKey]; !ok {
		// Create signer with key input
		signer, err := sshlib.CreateSignerPublicKeyPrompt(key, password)
		if err != nil {
			return err
		}

		// Create AuthMethod
		authMethod := ssh.PublicKeys(signer)

		// Regist AuthMethod to authMethodMap
		r.authMethodMap[authKey] = append(r.authMethodMap[authKey], authMethod)
	}

	// Regist AuthMethod to serverAuthMethodMap from authMethodMap
	r.serverAuthMethodMap[server] = append(r.serverAuthMethodMap[server], r.authMethodMap[authKey]...)

	return
}

//
func (r *Run) registAuthMapCertificate(server, cert string, signer ssh.Signer) (err error) {
	authKey := AuthKey{AUTHKEY_CERT, cert}
	if _, ok := r.authMethodMap[authKey]; !ok {
		authMethod, err := sshlib.CreateAuthMethodCertificate(cert, signer)
		if err != nil {
			return err
		}

		// Regist AuthMethod to authMethodMap
		r.authMethodMap[authKey] = append(r.authMethodMap[authKey], authMethod)
	}

	// Regist AuthMethod to serverAuthMethodMap from authMethodMap
	r.serverAuthMethodMap[server] = append(r.serverAuthMethodMap[server], r.authMethodMap[authKey]...)

	return
}

//
func (r *Run) registAuthMapAgent(server string) (err error) {
	authKey := AuthKey{AUTHKEY_AGENT, SSH_AUTH_SOCK}
	if _, ok := r.authMethodMap[authKey]; !ok {
		signers, err := sshlib.CreateSignerAgent(r.agent)
		if err != nil {
			return err
		}

		for _, signer := range signers {
			authMethod := ssh.PublicKeys(signer)
			r.authMethodMap[authKey] = append(r.authMethodMap[authKey], authMethod)
		}
	}

	// Regist AuthMethod to serverAuthMethodMap from authMethodMap
	r.serverAuthMethodMap[server] = append(r.serverAuthMethodMap[server], r.authMethodMap[authKey]...)

	return
}

//
func (r *Run) registAuthMapPKCS11(server, provider, pin string) (err error) {
	authKey := AuthKey{AUTHKEY_PKCS11, provider}
	if _, ok := r.authMethodMap[authKey]; !ok {
		// Create Signer with key input
		signers, err := sshlib.CreateSignerPKCS11Prompt(provider, pin)
		if err != nil {
			return err
		}

		for _, signer := range signers {
			// Create AuthMethod
			authMethod := ssh.PublicKeys(signer)

			// Regist AuthMethod to AuthMethodMap
			r.authMethodMap[authKey] = append(r.authMethodMap[authKey], authMethod)
		}
	}

	// Regist AuthMethod to serverAuthMethodMap from authMethodMap
	r.serverAuthMethodMap[server] = append(r.serverAuthMethodMap[server], r.authMethodMap[authKey]...)

	return
}
