// Copyright (c) 2021 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.
//go:build cgo
// +build cgo

package sshlib

import (
	"github.com/miekg/pkcs11"
	"golang.org/x/crypto/ssh"
)

// CreateAuthMethodPKCS11 return []ssh.AuthMethod generated from pkcs11 token.
// PIN is required to generate a AuthMethod from a PKCS 11 token.
// Not available if cgo is disabled.
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

// CreateSignerPKCS11 returns []ssh.Signer generated from PKCS11 token.
// PIN is required to generate a Signer from a PKCS 11 token.
// Not available if cgo is disabled.
//
// WORNING: Does not work if multiple tokens are stuck at the same time.
func CreateSignerPKCS11(provider, pin string) (signers []ssh.Signer, err error) {
	// get absolute path
	provider = getAbsPath(provider)

	ctx := pkcs11.New(provider)
	err = ctx.Initialize()
	if err != nil {
		ctx.Destroy()
		ctx.Finalize()
		return
	}

	slots, err := ctx.GetSlotList(true)
	if err != nil {
		ctx.Destroy()
		ctx.Finalize()
		return
	}

	c11array := []*C11{}
	for _, slot := range slots {
		tokenInfo, err := ctx.GetTokenInfo(slot)
		if err != nil {
			continue
		}

		c := &C11{
			Label: tokenInfo.Label,
			PIN:   pin,
		}

		c11array = append(c11array, c)
	}

	// for loop
	for _, c11 := range c11array {
		err := c11.CreateCtx(ctx)
		if err != nil {
			// TODO: errorをなにかしらの形で返す
			continue

		}

		sigs, err := c11.GetSigner()
		if err != nil {
			// TODO: errorをなにかしらの形で返す
			continue
		}

		for _, sig := range sigs {
			signer, _ := ssh.NewSignerFromSigner(sig)
			signers = append(signers, signer)
		}
	}

	return
}
