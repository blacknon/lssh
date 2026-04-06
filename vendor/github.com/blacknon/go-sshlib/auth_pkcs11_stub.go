// Copyright (c) 2026 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.
//go:build !cgo
// +build !cgo

package sshlib

import (
	"fmt"

	"golang.org/x/crypto/ssh"
)

func CreateAuthMethodPKCS11(provider, pin string) ([]ssh.AuthMethod, error) {
	return CreateAuthMethodPKCS11WithPrompt(provider, pin, nil)
}

func CreateAuthMethodPKCS11WithPrompt(provider, pin string, prompt PromptFunc) ([]ssh.AuthMethod, error) {
	return nil, fmt.Errorf("sshlib: PKCS#11 authentication requires cgo")
}

func CreateSignerPKCS11(provider, pin string) ([]ssh.Signer, error) {
	return CreateSignerPKCS11WithPrompt(provider, pin, nil)
}

func CreateSignerPKCS11WithPrompt(provider, pin string, prompt PromptFunc) ([]ssh.Signer, error) {
	return nil, fmt.Errorf("sshlib: PKCS#11 signer requires cgo")
}
