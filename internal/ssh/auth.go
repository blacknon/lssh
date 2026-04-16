// Copyright (c) 2022 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package ssh

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/blacknon/go-sshlib"
	"github.com/blacknon/lssh/internal/common"
	conf "github.com/blacknon/lssh/internal/config"
	"golang.org/x/crypto/ssh"
)

const SSH_AUTH_SOCK = "SSH_AUTH_SOCK"

// CreateAuthMethodMap Create ssh.AuthMethod, into r.AuthMethodMap.
func (r *Run) CreateAuthMethodMap() {
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
		if config.Pass != "" || config.PassRef != "" {
			pass, err := r.resolveLiteralOrRef(server, "pass", config.Pass, config.PassRef)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
			} else if pass != "" {
				r.registAuthMapPassword(server, pass)
			}
		}

		// Multiple Password
		if len(config.Passes) > 0 {
			for _, pass := range config.Passes {
				r.registAuthMapPassword(server, pass)
			}
		}

		// PublicKey
		if config.Key != "" || config.KeyRef != "" {
			err := r.registAuthMapPublicKey(server, config)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
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
				err := r.registAuthMapPublicKeyFile(server, keyName, keyPass)
				if err != nil {
					fmt.Fprintln(os.Stderr, err)
					continue
				}
			}
		}

		// Public Key Command
		if config.KeyCommand != "" {
			keyCommandPass, err := r.resolveLiteralOrRef(server, "keycmdpass", config.KeyCommandPass, config.KeyCommandPassRef)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
			} else {
				err = r.registAuthMapPublicKeyCommand(server, config.KeyCommand, keyCommandPass)
			}
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		}

		// Certificate
		if config.Cert != "" || config.CertRef != "" {
			err := r.registAuthMapCertificate(server, config)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				continue
			}
		}
		if len(config.Certs) > 0 {
			for _, cert := range config.Certs {
				pair := strings.SplitN(cert, "::", 3)
				if len(pair) < 2 {
					continue
				}
				certName := pair[0]
				keyName := pair[1]
				keyPass := ""
				if len(pair) > 2 {
					keyPass = pair[2]
				}

				keySigner, err := sshlib.CreateSignerPublicKeyPrompt(keyName, keyPass)
				if err != nil {
					fmt.Fprintln(os.Stderr, err)
					continue
				}

				err = r.registAuthMapCertificateSigner(server, certName, keySigner)
				if err != nil {
					fmt.Fprintln(os.Stderr, err)
					continue
				}
			}
		}

		// PKCS11
		if config.PKCS11Use {
			pin, err := r.resolveLiteralOrRef(server, "pkcs11pin", config.PKCS11PIN, config.PKCS11PINRef)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
			} else {
				err = r.registAuthMapPKCS11(server, config.PKCS11Provider, pin)
			}
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		}

		// SSH Agent
		if config.SSHAgentUse {
			r.SetupSshAgent()
			err := r.registAuthMapAgent(server)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		}
	}
}

func (r *Run) SetupSshAgent() {
	// Connect ssh-agent
	r.agent = sshlib.ConnectSshAgent()
}

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

func (r *Run) registAuthMapPublicKey(server string, cfg conf.ServerConfig) (err error) {
	if cfg.KeyRef != "" {
		return r.registAuthMapPublicKeyRef(server, cfg)
	}

	key, cleanup, err := r.resolveSecretFile(server, "key", cfg.Key, cfg.KeyRef)
	if err != nil {
		return err
	}
	if cleanup != nil {
		defer cleanup()
	}

	password, err := r.resolveLiteralOrRef(server, "keypass", cfg.KeyPass, cfg.KeyPassRef)
	if err != nil {
		return err
	}

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

func (r *Run) registAuthMapPublicKeyRef(server string, cfg conf.ServerConfig) error {
	keyData, err := r.resolveLiteralOrRef(server, "key", cfg.Key, cfg.KeyRef)
	if err != nil {
		return err
	}
	if keyData == "" {
		return nil
	}

	password, err := r.resolveLiteralOrRef(server, "keypass", cfg.KeyPass, cfg.KeyPassRef)
	if err != nil {
		return err
	}

	authKey := AuthKey{AUTHKEY_KEY, "ref:" + cfg.KeyRef}
	if _, ok := r.authMethodMap[authKey]; !ok {
		signer, err := createSignerFromKeyDataWithPrompt(server, []byte(keyData), password)
		if err != nil {
			return err
		}
		r.authMethodMap[authKey] = append(r.authMethodMap[authKey], ssh.PublicKeys(signer))
	}

	r.serverAuthMethodMap[server] = append(r.serverAuthMethodMap[server], r.authMethodMap[authKey]...)
	return nil
}

func (r *Run) registAuthMapPublicKeyFile(server, key, password string) (err error) {
	authKey := AuthKey{AUTHKEY_KEY, key}

	if _, ok := r.authMethodMap[authKey]; !ok {
		signer, err := sshlib.CreateSignerPublicKeyPrompt(key, password)
		if err != nil {
			return err
		}

		authMethod := ssh.PublicKeys(signer)
		r.authMethodMap[authKey] = append(r.authMethodMap[authKey], authMethod)
	}

	r.serverAuthMethodMap[server] = append(r.serverAuthMethodMap[server], r.authMethodMap[authKey]...)
	return nil
}

func (r *Run) registAuthMapPublicKeyCommand(server, command, password string) (err error) {
	authKey := AuthKey{AUTHKEY_KEY, command}

	if _, ok := r.authMethodMap[authKey]; !ok {
		// Run key command
		cmd := exec.Command("sh", "-c", command)
		keyData, err := cmd.Output()
		if err != nil {
			return err
		}

		// Create signer
		signer, err := sshlib.CreateSignerPublicKeyData(keyData, password)
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

func (r *Run) registAuthMapCertificate(server string, cfg conf.ServerConfig) (err error) {
	if cfg.CertRef != "" || cfg.CertKeyRef != "" {
		return r.registAuthMapCertificateRef(server, cfg)
	}

	cert, certCleanup, err := r.resolveSecretFile(server, "cert", cfg.Cert, cfg.CertRef)
	if err != nil {
		return err
	}
	if certCleanup != nil {
		defer certCleanup()
	}

	keyPath, keyCleanup, err := r.resolveSecretFile(server, "certkey", cfg.CertKey, cfg.CertKeyRef)
	if err != nil {
		return err
	}
	if keyCleanup != nil {
		defer keyCleanup()
	}

	keyPass, err := r.resolveLiteralOrRef(server, "certkeypass", cfg.CertKeyPass, cfg.CertKeyPassRef)
	if err != nil {
		return err
	}

	signer, err := sshlib.CreateSignerPublicKeyPrompt(keyPath, keyPass)
	if err != nil {
		return err
	}

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

func (r *Run) registAuthMapCertificateRef(server string, cfg conf.ServerConfig) error {
	certData, err := r.resolveLiteralOrRef(server, "cert", cfg.Cert, cfg.CertRef)
	if err != nil {
		return err
	}
	keyData, err := r.resolveLiteralOrRef(server, "certkey", cfg.CertKey, cfg.CertKeyRef)
	if err != nil {
		return err
	}
	if certData == "" || keyData == "" {
		return nil
	}

	password, err := r.resolveLiteralOrRef(server, "certkeypass", cfg.CertKeyPass, cfg.CertKeyPassRef)
	if err != nil {
		return err
	}

	authKey := AuthKey{AUTHKEY_CERT, "ref:" + cfg.CertRef + "::" + cfg.CertKeyRef}
	if _, ok := r.authMethodMap[authKey]; !ok {
		signer, err := createSignerFromKeyDataWithPrompt(server, []byte(keyData), password)
		if err != nil {
			return err
		}
		authMethod, err := createCertificateAuthMethodFromData([]byte(certData), signer)
		if err != nil {
			return err
		}
		r.authMethodMap[authKey] = append(r.authMethodMap[authKey], authMethod)
	}

	r.serverAuthMethodMap[server] = append(r.serverAuthMethodMap[server], r.authMethodMap[authKey]...)
	return nil
}

func createSignerFromKeyDataWithPrompt(server string, keyData []byte, password string) (ssh.Signer, error) {
	if password != "" {
		return sshlib.CreateSignerPublicKeyData(keyData, password)
	}

	signer, err := ssh.ParsePrivateKey(keyData)
	if err == nil {
		return signer, nil
	}

	msg := fmt.Sprintf("%s's passphrase: ", server)
	for i := 0; i < 3; i++ {
		passphrase, promptErr := common.GetPassPhrase(msg)
		if promptErr != nil {
			return nil, promptErr
		}
		signer, err = ssh.ParsePrivateKeyWithPassphrase(keyData, []byte(passphrase))
		if err == nil {
			return signer, nil
		}
		fmt.Println(err.Error())
	}

	return nil, err
}

func createCertificateAuthMethodFromData(certData []byte, signer ssh.Signer) (ssh.AuthMethod, error) {
	pubkey, _, _, _, err := ssh.ParseAuthorizedKey(certData)
	if err != nil {
		return nil, err
	}

	certificate, ok := pubkey.(*ssh.Certificate)
	if !ok {
		return nil, fmt.Errorf("Error: Not create certificate struct data")
	}

	certSigner, err := ssh.NewCertSigner(certificate, signer)
	if err != nil {
		return nil, err
	}

	return ssh.PublicKeys(certSigner), nil
}

func (r *Run) registAuthMapCertificateSigner(server, cert string, signer ssh.Signer) (err error) {
	authKey := AuthKey{AUTHKEY_CERT, cert}

	if _, ok := r.authMethodMap[authKey]; !ok {
		authMethod, err := sshlib.CreateAuthMethodCertificate(cert, signer)
		if err != nil {
			return err
		}
		r.authMethodMap[authKey] = append(r.authMethodMap[authKey], authMethod)
	}

	r.serverAuthMethodMap[server] = append(r.serverAuthMethodMap[server], r.authMethodMap[authKey]...)
	return nil
}

// registAuthMapAgent is Regist ssh-agent signature to r.AuthMethodMap.
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

// registAuthMapPKCS11 is Regist PKCS11 signature to r.AuthMethodMap.
func (r *Run) registAuthMapPKCS11(server, provider, pin string) (err error) {
	authKey := AuthKey{AUTHKEY_PKCS11, provider}
	if _, ok := r.authMethodMap[authKey]; !ok && !r.donedPKCS11 {
		// Create Signer with key input
		// TODO(blacknon): あとでいい感じに記述する(retry対応)
		// signers, err := sshlib.CreateSignerPKCS11Prompt(provider, pin)
		signers, err := sshlib.CreateSignerPKCS11(provider, pin)

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

	// set donedPKCS11
	r.donedPKCS11 = true

	return
}
