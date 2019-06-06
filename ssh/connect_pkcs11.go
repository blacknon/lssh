package ssh

import (
	"crypto"
	"fmt"
	"syscall"

	"github.com/ThalesIgnite/crypto11"
	"github.com/miekg/pkcs11"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"
)

func (c *Connect) getSshSignerFromPkcs11(server string) (signers []ssh.Signer, err error) {
	conf := c.Conf.Server[server]
	pkcs11Provider := conf.PKCS11Provider
	pkcs11PIN := conf.PKCS11PIN

	p := new(P11)
	p.PIN = pkcs11PIN

	// create pkcs11 ctx
	err = p.CreateCtx(pkcs11Provider)
	if err != nil {
		return
	}

	// get token label
	err = p.GetTokenLabel()
	if err != nil {
		return
	}

	// get PIN code
	err = p.GetPIN()
	if err != nil {
		return
	}

	// recreate ctx (pkcs11=>crypto11)
	err = p.RecreateCtx(pkcs11Provider)
	if err != nil {
		return
	}

	// get KeyID
	err = p.GetKeyID()
	if err != nil {
		return
	}

	// get crypto signers
	cryptoSigners, err := p.GetCryptoSigner()
	if err != nil {
		return
	}

	// @TODO: ↑までを別のfunctionにして、CryptoSignerを別に分ける
	//        (CertAuth時に鍵ファイルをPKCS11で扱えるようにするため。)

	for _, cryptoSigner := range cryptoSigners {
		signer, _ := ssh.NewSignerFromSigner(cryptoSigner)
		signers = append(signers, signer)
	}

	return
}

type P11 struct {
	Ctx           *pkcs11.Ctx
	Label         string
	SlotID        uint
	KeyID         map[int][]byte
	PIN           string
	SessionHandle pkcs11.SessionHandle
	Signers       []ssh.Signer
}

// pkcs11Provider ... PATH
func (p *P11) CreateCtx(pkcs11Provider string) (err error) {
	ctx := pkcs11.New(pkcs11Provider)
	err = ctx.Initialize()
	if err != nil {
		return
	}
	p.Ctx = ctx
	return
}

func (p *P11) GetTokenLabel() (err error) {
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

func (p *P11) GetPIN() (err error) {
	if p.PIN == "" {
		fmt.Printf("PIN: ")
		pin, err := terminal.ReadPassword(int(syscall.Stdin))
		if err != nil {
			return err
		}

		if len(pin) == 0 {
			err = fmt.Errorf("err: %s", "PIN empty")
			return err
		}

		p.PIN = string(pin)
		fmt.Println()
	}

	return
}

func (p *P11) RecreateCtx(pkcs11Provider string) (err error) {
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

func (p *P11) GetKeyID() (err error) {
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

func (p *P11) GetCryptoSigner() (signers []crypto.Signer, err error) {
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
