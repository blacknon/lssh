// Copyright (c) 2019 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

// TODO(blacknon): crypto11のバージョンがv0.1.0に依存しているため、v1.0.2に対応するように変更する。

package sshlib

import (
	"crypto"
	"fmt"

	"github.com/ThalesIgnite/crypto11"
	"github.com/miekg/pkcs11"
)

// PKCS11 struct for pkcs11 processing.
type PKCS11 struct {
	// pkcs11 provider path
	Pkcs11Provider string
	Ctx            *pkcs11.Ctx
	Label          string
	SlotID         uint
	KeyID          map[int][]byte
	PIN            string
	SessionHandle  pkcs11.SessionHandle
}

// CreateCtx create and into PKCS11.Ctx.
// This is the first process to be performed when processing with PKCS11.
func (p *PKCS11) CreateCtx() (err error) {
	ctx := pkcs11.New(p.Pkcs11Provider)
	err = ctx.Initialize()
	if err != nil {
		return
	}
	p.Ctx = ctx
	return
}

// GetTokenLabel get pkcs11 token label. and into PKCS11.Label.
// Only one token is supported.
func (p *PKCS11) GetTokenLabel() (err error) {
	slots, err := p.Ctx.GetSlotList(false)
	if err != nil {
		return
	}

	if len(slots) > 1 {
		err = fmt.Errorf("err: %s", "Single token only")
		return
	}

	if len(slots) == 0 {
		err = fmt.Errorf("err: %s", "No token")
		return
	}

	slotID := slots[0]
	tokenInfo, err := p.Ctx.GetTokenInfo(slotID)
	if err != nil {
		return
	}

	p.SlotID = slotID
	p.Label = tokenInfo.Label
	return
}

// RecreateCtx exchange PKCS11.Ctx with PIN accessible ctx.
// Recreate Ctx to access information after PIN entry.
func (p *PKCS11) RecreateCtx(pkcs11Provider string) (err error) {
	p.Ctx.Destroy()
	config := &crypto11.PKCS11Config{
		Path:       pkcs11Provider,
		TokenLabel: p.Label,
		Pin:        p.PIN,
	}

	ctx, err := crypto11.Configure(config)
	if err != nil {
		return
	}

	session, err := ctx.OpenSession(p.SlotID, pkcs11.CKF_SERIAL_SESSION)
	if err != nil {
		return
	}

	p.Ctx = ctx
	p.SessionHandle = session
	return
}

// GetKeyID acquire KeyID via PKCS11 and store it in PKCS11 structure.
func (p *PKCS11) GetKeyID() (err error) {
	findTemplate := []*pkcs11.Attribute{
		pkcs11.NewAttribute(pkcs11.CKA_ID, true), // KeyID
		pkcs11.NewAttribute(pkcs11.CKA_CLASS, pkcs11.CKO_PRIVATE_KEY),
		pkcs11.NewAttribute(pkcs11.CKA_PRIVATE, true),
		pkcs11.NewAttribute(pkcs11.CKA_KEY_TYPE, pkcs11.CKK_RSA),
	}

	p.Ctx.FindObjectsInit(p.SessionHandle, findTemplate)
	obj, _, err := p.Ctx.FindObjects(p.SessionHandle, 1000)
	if err != nil {
		return
	}

	err = p.Ctx.FindObjectsFinal(p.SessionHandle)
	if err != nil {
		return
	}

	p.KeyID = map[int][]byte{}
	for num, objValue := range obj {
		attrs, _ := p.Ctx.GetAttributeValue(p.SessionHandle, objValue, findTemplate)
		p.KeyID[num] = attrs[0].Value
	}

	return
}

// GetCryptoSigner return []crypto.Signer
func (p *PKCS11) GetCryptoSigner() (signers []crypto.Signer, err error) {
	c11Session := &crypto11.PKCS11Session{p.Ctx, p.SessionHandle}
	for _, keyID := range p.KeyID {
		prv, err := crypto11.FindKeyPairOnSession(c11Session, p.SlotID, keyID, nil)
		if err != nil {
			return signers, err
		}

		// append signer
		signers = append(signers, prv.(crypto.Signer))
	}

	return signers, err
}

// GetPin prompt for PIN if P11.Pin is blank
// Only Support UNIX-like OS.
func (p *PKCS11) GetPIN() (err error) {
	if p.PIN == "" {
		p.PIN, err = getPassphrase("PKCS11 PIN:")
	}

	return
}

// TODO(blacknon): ssh-agent用に、PrivateKeyのinterfaceを返す関数の作成
