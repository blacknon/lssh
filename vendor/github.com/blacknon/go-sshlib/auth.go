// Copyright (c) 2021 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

// TODO(blacknon):
//     ↓等を読み解いて、Publickey offeringやknown_hostsのチェックを実装する(`v0.2.0`)。
//     既存のライブラリ等はないので、自前でrequestを書く必要があるかも？
//     かなりの手間がかかりそうなので、対応については相応に時間がかかりそう。
//       - https://go.googlesource.com/crypto/+/master/ssh/client_auth.go
//       - https://go.googlesource.com/crypto/+/master/ssh/tcpip.go

package sshlib

import (
	"fmt"
	"io/ioutil"
	"regexp"
	"strings"

	"github.com/ScaleFT/sshkeys"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// CreateAuthMethodPassword returns ssh.AuthMethod generated from password.
func CreateAuthMethodPassword(password string) (auth ssh.AuthMethod) {
	return ssh.Password(password)
}

// CreateAuthMethodPublicKey returns ssh.AuthMethod generated from PublicKey.
// If you have not specified a passphrase, please specify a empty character("").
func CreateAuthMethodPublicKey(key, password string) (auth ssh.AuthMethod, err error) {
	signer, err := CreateSignerPublicKey(key, password)
	if err != nil {
		return
	}

	auth = ssh.PublicKeys(signer)
	return
}

// CreateSignerPublicKey returns []ssh.Signer generated from public key.
// If you have not specified a passphrase, please specify a empty character("").
func CreateSignerPublicKey(key, password string) (signer ssh.Signer, err error) {
	// get absolute path
	key = getAbsPath(key)

	// Read PrivateKey file
	keyData, err := ioutil.ReadFile(key)
	if err != nil {
		return
	}

	signer, err = CreateSignerPublicKeyData(keyData, password)

	return
}

// CreateSignerPublicKeyData return ssh.Signer from private key and password
func CreateSignerPublicKeyData(keyData []byte, password string) (signer ssh.Signer, err error) {
	if password != "" { // password is not empty
		// Parse key data
		data, err := sshkeys.ParseEncryptedRawPrivateKey(keyData, []byte(password))
		if err != nil {
			return signer, err
		}

		// Create ssh.Signer
		signer, err = ssh.NewSignerFromKey(data)
	} else { // password is empty
		signer, err = ssh.ParsePrivateKey(keyData)
	}

	return
}

// CreateSignerPublicKeyPrompt rapper CreateSignerPKCS11.
// Output a passphrase input prompt if the passphrase is not entered or incorrect.
//
// Only Support UNIX-like OS.
func CreateSignerPublicKeyPrompt(key, password string) (signer ssh.Signer, err error) {
	// repeat count
	rep := 3

	// get absolute path
	key = getAbsPath(key)

	// Read PrivateKey file
	keyData, err := ioutil.ReadFile(key)
	if err != nil {
		return
	}

	if password != "" { // password is not empty
		signer, err = ssh.ParsePrivateKeyWithPassphrase(keyData, []byte(password))
	} else { // password is empty
		signer, err = ssh.ParsePrivateKey(keyData)

		rgx := regexp.MustCompile(`cannot decode`)
		if err != nil {
			if rgx.MatchString(err.Error()) {
				msg := key + "'s passphrase:"

				for i := 0; i < rep; i++ {
					password, _ = getPassphrase(msg)
					password = strings.TrimRight(password, "\n")
					signer, err = ssh.ParsePrivateKeyWithPassphrase(keyData, []byte(password))
					if err == nil {
						return
					}
					fmt.Println("\n" + err.Error())
				}
			}
		}
	}

	return
}

// CreateAuthMethodCertificate returns ssh.AuthMethod generated from Certificate.
// To generate an AuthMethod from a certificate, you will need the certificate's private key Signer.
// Signer should be generated from CreateSignerPublicKey() or CreateSignerPKCS11().
func CreateAuthMethodCertificate(cert string, keySigner ssh.Signer) (auth ssh.AuthMethod, err error) {
	signer, err := CreateSignerCertificate(cert, keySigner)
	if err != nil {
		return
	}

	auth = ssh.PublicKeys(signer)
	return
}

// CreateSignerCertificate returns ssh.Signer generated from Certificate.
// To generate an AuthMethod from a certificate, you will need the certificate's private key Signer.
// Signer should be generated from CreateSignerPublicKey() or CreateSignerPKCS11().
func CreateSignerCertificate(cert string, keySigner ssh.Signer) (certSigner ssh.Signer, err error) {
	// get absolute path
	cert = getAbsPath(cert)

	// Read Cert file
	certData, err := ioutil.ReadFile(cert)
	if err != nil {
		return
	}

	// Create PublicKey from Cert
	pubkey, _, _, _, err := ssh.ParseAuthorizedKey(certData)
	if err != nil {
		return
	}

	// Create Certificate Struct
	certificate, ok := pubkey.(*ssh.Certificate)
	if !ok {
		err = fmt.Errorf("%s\n", "Error: Not create certificate struct data")
		return
	}

	// Create Certificate Signer
	certSigner, err = ssh.NewCertSigner(certificate, keySigner)
	if err != nil {
		return
	}

	return
}

// CreateSignerAgent return []ssh.Signer from ssh-agent.
// In sshAgent, put agent.Agent or agent.ExtendedAgent.
func CreateSignerAgent(sshAgent interface{}) (signers []ssh.Signer, err error) {
	switch ag := sshAgent.(type) {
	case agent.Agent:
		signers, err = ag.Signers()
	case agent.ExtendedAgent:
		signers, err = ag.Signers()
	}

	return
}
