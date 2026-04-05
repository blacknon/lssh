//go:build !cgo

package sshlib

import (
	"fmt"

	"golang.org/x/crypto/ssh"
)

func CreateAuthMethodPKCS11(provider, pin string) (auth []ssh.AuthMethod, err error) {
	return CreateAuthMethodPKCS11WithPrompt(provider, pin, nil)
}

func CreateAuthMethodPKCS11WithPrompt(provider, pin string, prompt PromptFunc) (auth []ssh.AuthMethod, err error) {
	return nil, fmt.Errorf("sshlib: pkcs11 authentication requires cgo")
}

func CreateSignerPKCS11(provider, pin string) (signers []ssh.Signer, err error) {
	return CreateSignerPKCS11WithPrompt(provider, pin, nil)
}

func CreateSignerPKCS11WithPrompt(provider, pin string, prompt PromptFunc) (signers []ssh.Signer, err error) {
	return nil, fmt.Errorf("sshlib: pkcs11 authentication requires cgo")
}
