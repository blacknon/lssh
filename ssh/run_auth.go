package ssh

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"regexp"
	"strings"

	"github.com/blacknon/lssh/common"
	"golang.org/x/crypto/ssh"
)

// @brief:
//     Create ssh.Signer into r.AuthMap.
//     Passwords is not get this function.
func (r *Run) createAuthMap() {
	r.AuthMap = map[AuthKey][]ssh.Signer{}

	for _, server := range r.ServerList {
		// get server config
		config := r.Conf.Server[server]

		// Public key auth (single)
		if config.Key != "" {
			r.registAuthMapPublicKey(server, config.Key, config.KeyPass)
		}

		// Public keys auth (array)
		if len(config.Keys) > 0 {
			for _, key := range config.Keys {
				keyPair := strings.SplitN(key, "::", 2)
				if len(keyPair) > 1 {
					r.registAuthMapPublicKey(server, keyPair[0], keyPair[1])
				} else {
					r.registAuthMapPublicKey(server, keyPair[0], "")
				}
			}
		}

		// Certificate auth
		if config.Cert != "" {
			r.registAuthMapCertificate(server, config.Cert, config.CertKey, config.CertKeyPass)
		}

		// PKCS11 Auth
		if config.PKCS11Use {
			r.registAuthMapPKCS11(server)
		}
	}
}

func (r *Run) registAuthMapPublicKey(server, key, pass string) {
	authKey := AuthKey{AUTHKEY_KEY, key}

	if _, ok := r.AuthMap[authKey]; !ok {
		signer, err := createSshSignerPublicKey(key, pass)
		if signer == nil {
			fmt.Fprintf(os.Stderr, "%s's create public key ssh.Signer err: %s\n", server, err)
		}
		r.AuthMap[authKey] = []ssh.Signer{signer}
	}
}

func (r *Run) registAuthMapCertificate(server, cert, key, pass string) {
	authKey := AuthKey{AUTHKEY_CERT, cert}

	if _, ok := r.AuthMap[authKey]; !ok {
		signer, err := createSshSignerCertificate(cert, key, pass)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s's create certificate ssh.Signer err: %s\n", server, err)
		}
		r.AuthMap[authKey] = []ssh.Signer{signer}
	}
}

func (r *Run) registAuthMapPKCS11(server string) {
	conf := r.Conf.Server[server]

	authKey := AuthKey{AUTHKEY_PKCS11, conf.PKCS11Provider}

	if _, ok := r.AuthMap[authKey]; !ok {
		conf := r.Conf.Server[server]

		p := new(P11)
		p.Pkcs11Provider = conf.PKCS11Provider
		p.PIN = conf.PKCS11PIN

		// get crypto signers
		cryptoSigners, err := p.Get()
		if err != nil {
			return
		}

		for _, cryptoSigner := range cryptoSigners {
			signer, _ := ssh.NewSignerFromSigner(cryptoSigner)
			r.AuthMap[authKey] = append(r.AuthMap[authKey], signer)
		}
	}
	return
}

// @brief:
//     create ssh.Signer from Publickey
func createSshSignerPublicKey(key, pass string) (signer ssh.Signer, err error) {
	// repeat count
	rep := 3

	usr, _ := user.Current()
	key = strings.Replace(key, "~", usr.HomeDir, 1)

	// Read PrivateKey file
	keyData, err := ioutil.ReadFile(key)
	if err != nil {
		return signer, err
	}

	if pass != "" {
		signer, err = ssh.ParsePrivateKeyWithPassphrase(keyData, []byte(pass))
	} else {
		rgx := regexp.MustCompile(`cannot decode`)
		signer, err = ssh.ParsePrivateKey(keyData)
		if err != nil {
			if rgx.MatchString(err.Error()) {
				msg := key + "'s passphase:"

				for i := 0; i < rep; i++ {
					pass, _ = common.GetPassPhase(msg)
					pass = strings.TrimRight(pass, "\n")
					sshSigner, err := ssh.ParsePrivateKeyWithPassphrase(keyData, []byte(pass))
					signer = sshSigner
					if err == nil {
						break
					}
					fmt.Println("\n" + err.Error())
				}
			}
		}
	}

	return
}

// @brief:
//     create ssh.Signer from Certificate
// @TODO: 証明書認証でのPKCS11の鍵ファイル指定を追加！
func createSshSignerCertificate(cert, key, pass string) (signer ssh.Signer, err error) {
	usr, _ := user.Current()
	cert = strings.Replace(cert, "~", usr.HomeDir, 1)
	key = strings.Replace(key, "~", usr.HomeDir, 1)

	// Read Cert file
	certData, err := ioutil.ReadFile(cert)
	if err != nil {
		return signer, err
	}

	// Create PublicKey from Cert
	pubkey, _, _, _, err := ssh.ParseAuthorizedKey(certData)
	if err != nil {
		return signer, err
	}

	// Create Certificate Struct
	certificate, ok := pubkey.(*ssh.Certificate)
	if !ok {
		err = fmt.Errorf("%s\n", "Error: Not create certificate struct data")
		return signer, err
	}

	// create key signer
	var keySigner ssh.Signer
	keySigner, err = createSshSignerPublicKey(key, pass)
	if err != nil {
		return signer, err
	}

	// Create Certificate Signer
	signer, err = ssh.NewCertSigner(certificate, keySigner)
	if err != nil {
		return signer, err
	}

	return
}
