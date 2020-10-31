// Copyright (c) 2020 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package sshlib

import (
	"github.com/ThalesIgnite/crypto11"
)

// C11 struct for Crypto11 processing.
type C11 struct {
	Label string
	PIN   string
	Ctx   *crypto11.Context
}

// getPIN is set token's PIN Code to c.PIN
func (c *C11) getPIN() (err error) {
	if c.PIN == "" {
		c.PIN, err = getPassphrase(c.Label + "'s PIN:")
	}

	return
}

// CreateCtx is create crypto11.Context
func (c *C11) CreateCtx(provider string) (err error) {
	// Get PIN Code
	err = c.getPIN()
	if err != nil {
		// clear PIN code
		c.PIN = ""
		return
	}

	// Create crypto11 Configure
	config := &crypto11.Config{
		Path:       provider,
		TokenLabel: c.Label,
		Pin:        c.PIN,
	}

	// Create crypto11 Ctx
	c.Ctx, err = crypto11.Configure(config)

	return
}

// GetSigner return []crypto11.Signer
func (c *C11) GetSigner() (signer []crypto11.Signer, err error) {
	return c.Ctx.FindAllKeyPairs()
}
