package ssh

import (
	"golang.org/x/crypto/ssh"
)

// getSshSignerFromPkcs11 return ssh.Signer[] from pkcs11 token
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

	// TODO(blacknon): ↑までを別のfunctionにして、CryptoSignerを別に分ける(CertAuth時に鍵ファイルをPKCS11で扱えるようにするため。)

	for _, cryptoSigner := range cryptoSigners {
		signer, _ := ssh.NewSignerFromSigner(cryptoSigner)
		signers = append(signers, signer)
	}

	return
}
