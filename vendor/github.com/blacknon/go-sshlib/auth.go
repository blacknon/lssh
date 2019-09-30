// Copyright (c) 2019 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package sshlib

import (
	"fmt"
	"io/ioutil"
	"regexp"
	"strings"

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

func CreateSignerPublicKeyData(keyData []byte, password string) (signer ssh.Signer, err error) {
	if password != "" { // password is not empty
		signer, err = ssh.ParsePrivateKeyWithPassphrase(keyData, []byte(password))
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

// CreateAuthMethodPKCS11 return []ssh.AuthMethod generated from pkcs11 token.
// PIN is required to generate a AuthMethod from a PKCS 11 token.
//
// WORNING: Does not work if multiple tokens are stuck at the same time.
func CreateAuthMethodPKCS11(provider, pin string) (auth []ssh.AuthMethod, err error) {
	signers, err := CreateSignerPKCS11(provider, pin)
	if err != nil {
		return
	}

	for _, signer := range signers {
		auth = append(auth, ssh.PublicKeys(signer))
	}
	return
}

// CreateSignerCertificate returns []ssh.Signer generated from PKCS11 token.
// PIN is required to generate a Signer from a PKCS 11 token.
//
// WORNING: Does not work if multiple tokens are stuck at the same time.
func CreateSignerPKCS11(provider, pin string) (signers []ssh.Signer, err error) {
	// get absolute path
	provider = getAbsPath(provider)

	// Create PKCS11 struct
	p11 := new(PKCS11)
	p11.Pkcs11Provider = provider
	p11.PIN = pin

	// Create pkcs11 ctx
	err = p11.CreateCtx()
	if err != nil {
		return
	}

	// Get token label
	err = p11.GetTokenLabel()
	if err != nil {
		return
	}

	// Recreate ctx (pkcs11=>crypto11)
	err = p11.RecreateCtx(p11.Pkcs11Provider)
	if err != nil {
		return
	}

	// Get KeyID
	err = p11.GetKeyID()
	if err != nil {
		return
	}

	// Get crypto.Signer
	cryptoSigners, err := p11.GetCryptoSigner()
	if err != nil {
		return
	}

	// Exchange crypto.signer to ssh.Signer
	for _, cryptoSigner := range cryptoSigners {
		signer, _ := ssh.NewSignerFromSigner(cryptoSigner)
		signers = append(signers, signer)
	}

	return
}

// CreateSignerPKCS11Prompt rapper CreateSignerPKCS11.
// Output a PIN input prompt if the PIN is not entered or incorrect.
//
// Only Support UNIX-like OS.
func CreateSignerPKCS11Prompt(provider, pin string) (signers []ssh.Signer, err error) {
	// get absolute path
	provider = getAbsPath(provider)

	// Create PKCS11 struct
	p11 := new(PKCS11)
	p11.Pkcs11Provider = provider
	p11.PIN = pin

	// Create pkcs11 ctx
	err = p11.CreateCtx()
	if err != nil {
		return
	}

	// Get token label
	err = p11.GetTokenLabel()
	if err != nil {
		return
	}

	// get PIN code
	err = p11.GetPIN()
	if err != nil {
		return
	}

	// Recreate ctx (pkcs11=>crypto11)
	err = p11.RecreateCtx(p11.Pkcs11Provider)
	if err != nil {
		return
	}

	// Get KeyID
	err = p11.GetKeyID()
	if err != nil {
		return
	}

	// Get crypto.Signer
	cryptoSigners, err := p11.GetCryptoSigner()
	if err != nil {
		return
	}

	// Exchange crypto.signer to ssh.Signer
	for _, cryptoSigner := range cryptoSigners {
		signer, _ := ssh.NewSignerFromSigner(cryptoSigner)
		signers = append(signers, signer)
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
