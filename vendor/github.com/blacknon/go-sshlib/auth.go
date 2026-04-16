// Copyright (c) 2026 Blacknon. All rights reserved.
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
	"os"
	"regexp"
	"strings"
	"sync"
	"unsafe"

	"github.com/ScaleFT/sshkeys"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

type ControlPersistAuth struct {
	// AuthMethods allows reusing auth methods created by sshlib helper functions
	// such as CreateAuthMethodPassword/CreateAuthMethodPublicKey.
	AuthMethods []ssh.AuthMethod `json:"-"`

	// Methods stores serializable auth definitions for detached ControlPersist
	// helpers and proxy routes.
	Methods []ControlPersistAuthMethod
}

type ControlPersistAuthMethod struct {
	Type string

	Password string

	KeyPath   string
	KeyPass   string
	Transient bool

	PKCS11Provider string
	PKCS11PIN      string
}

type authMethodRegistryKey struct {
	typ  uintptr
	data uintptr
}

type controlPersistAuthMethodDefinition struct {
	Type string

	Password string

	KeyPath   string
	KeyPass   string
	Transient bool

	PKCS11Provider string
	PKCS11PIN      string
}

var controlPersistAuthMethodRegistry sync.Map

func (a *ControlPersistAuth) resolved() ([]controlPersistAuthMethodDefinition, error) {
	if a == nil {
		return nil, fmt.Errorf("sshlib: ControlPersistAuth is required for detached ControlPersist helper")
	}

	if len(a.Methods) > 0 {
		resolved := make([]controlPersistAuthMethodDefinition, 0, len(a.Methods))
		for _, method := range a.Methods {
			resolved = append(resolved, controlPersistAuthMethodDefinition{
				Type:           method.Type,
				Password:       method.Password,
				KeyPath:        method.KeyPath,
				KeyPass:        method.KeyPass,
				Transient:      method.Transient,
				PKCS11Provider: method.PKCS11Provider,
				PKCS11PIN:      method.PKCS11PIN,
			})
		}
		if err := validateControlPersistAuthDefinitions(resolved); err != nil {
			return nil, err
		}
		return resolved, nil
	}

	if len(a.AuthMethods) == 0 {
		return nil, fmt.Errorf("sshlib: ControlPersistAuth.AuthMethods is required for detached ControlPersist helper")
	}

	resolved := make([]controlPersistAuthMethodDefinition, 0, len(a.AuthMethods))
	for _, authMethod := range a.AuthMethods {
		persistAuth, ok := lookupControlPersistAuthMethod(authMethod)
		if !ok {
			return nil, fmt.Errorf("sshlib: unsupported authMethod for ControlPersistAuth; use sshlib.CreateAuthMethodPassword/CreateAuthMethodPublicKey")
		}
		resolved = append(resolved, *persistAuth)
	}
	return resolved, nil
}

func createControlPersistAuthMethods(definitions []controlPersistAuthMethodDefinition) ([]ssh.AuthMethod, error) {
	return createControlPersistAuthMethodsWithPrompt(definitions, nil)
}

func createControlPersistAuthMethodsWithPrompt(definitions []controlPersistAuthMethodDefinition, prompt PromptFunc) ([]ssh.AuthMethod, error) {
	if err := validateControlPersistAuthDefinitions(definitions); err != nil {
		return nil, err
	}

	transientKeyPaths := make([]string, 0, len(definitions))
	defer func() {
		cleanupControlPersistTransientFiles(transientKeyPaths)
	}()

	authMethods := make([]ssh.AuthMethod, 0, len(definitions))
	for _, persistAuth := range definitions {
		switch persistAuth.Type {
		case "password":
			authMethods = append(authMethods, CreateAuthMethodPassword(persistAuth.Password))
		case "publickey":
			auth, err := CreateAuthMethodPublicKey(persistAuth.KeyPath, persistAuth.KeyPass)
			if err != nil {
				return nil, err
			}
			if persistAuth.Transient {
				transientKeyPaths = append(transientKeyPaths, persistAuth.KeyPath)
			}
			authMethods = append(authMethods, auth)
		case "pkcs11":
			auth, err := CreateAuthMethodPKCS11WithPrompt(persistAuth.PKCS11Provider, persistAuth.PKCS11PIN, prompt)
			if err != nil {
				return nil, err
			}
			authMethods = append(authMethods, auth...)
		default:
			return nil, fmt.Errorf("sshlib: unsupported ControlPersistAuth type %q", persistAuth.Type)
		}
	}

	return authMethods, nil
}

func cleanupControlPersistTransientFiles(paths []string) {
	if len(paths) == 0 {
		return
	}

	seen := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		if path == "" {
			continue
		}

		absPath := getAbsPath(path)
		if _, ok := seen[absPath]; ok {
			continue
		}
		seen[absPath] = struct{}{}

		_ = os.Remove(absPath)
	}
}

func validateControlPersistAuthDefinitions(definitions []controlPersistAuthMethodDefinition) error {
	if len(definitions) == 0 {
		return fmt.Errorf("sshlib: ControlPersistAuth.AuthMethods is required for detached ControlPersist helper")
	}

	for _, persistAuth := range definitions {
		switch persistAuth.Type {
		case "password":
			if persistAuth.Password == "" {
				return fmt.Errorf("sshlib: password auth requires Password")
			}
		case "publickey":
			if persistAuth.KeyPath == "" {
				return fmt.Errorf("sshlib: publickey auth requires KeyPath")
			}
		case "pkcs11":
			if persistAuth.PKCS11Provider == "" {
				return fmt.Errorf("sshlib: pkcs11 auth requires PKCS11Provider")
			}
		default:
			return fmt.Errorf("sshlib: unsupported ControlPersistAuth type %q", persistAuth.Type)
		}
	}

	return nil
}

func registerControlPersistAuthMethod(auth ssh.AuthMethod, persistAuth controlPersistAuthMethodDefinition) {
	key, ok := controlPersistAuthMethodKey(auth)
	if !ok {
		return
	}

	controlPersistAuthMethodRegistry.Store(key, persistAuth)
}

func lookupControlPersistAuthMethod(auth ssh.AuthMethod) (*controlPersistAuthMethodDefinition, bool) {
	key, ok := controlPersistAuthMethodKey(auth)
	if !ok {
		return nil, false
	}

	value, ok := controlPersistAuthMethodRegistry.Load(key)
	if !ok {
		return nil, false
	}

	persistAuth, ok := value.(controlPersistAuthMethodDefinition)
	if !ok {
		return nil, false
	}

	return &persistAuth, true
}

func controlPersistAuthMethodKey(auth ssh.AuthMethod) (authMethodRegistryKey, bool) {
	if auth == nil {
		return authMethodRegistryKey{}, false
	}

	representation := *(*[2]uintptr)(unsafe.Pointer(&auth))
	if representation[1] == 0 {
		return authMethodRegistryKey{}, false
	}

	return authMethodRegistryKey{
		typ:  representation[0],
		data: representation[1],
	}, true
}

// CreateAuthMethodPassword returns ssh.AuthMethod generated from password.
func CreateAuthMethodPassword(password string) (auth ssh.AuthMethod) {
	auth = ssh.Password(password)
	registerControlPersistAuthMethod(auth, controlPersistAuthMethodDefinition{
		Type:     "password",
		Password: password,
	})
	return
}

// CreateAuthMethodPublicKey returns ssh.AuthMethod generated from PublicKey.
// If you have not specified a passphrase, please specify a empty character("").
func CreateAuthMethodPublicKey(key, password string) (auth ssh.AuthMethod, err error) {
	return createAuthMethodPublicKey(key, password, false)
}

// CreateAuthMethodPublicKeyTransient returns ssh.AuthMethod generated from PublicKey.
// The serialized ControlPersist definition is marked as transient, so detached
// helpers remove the key file after rebuilding the signer in memory.
func CreateAuthMethodPublicKeyTransient(key, password string) (auth ssh.AuthMethod, err error) {
	return createAuthMethodPublicKey(key, password, true)
}

func createAuthMethodPublicKey(key, password string, transient bool) (auth ssh.AuthMethod, err error) {
	signer, err := CreateSignerPublicKey(key, password)
	if err != nil {
		return
	}

	auth = ssh.PublicKeys(signer)
	registerControlPersistAuthMethod(auth, controlPersistAuthMethodDefinition{
		Type:      "publickey",
		KeyPath:   key,
		KeyPass:   password,
		Transient: transient,
	})
	return
}

// CreateSignerPublicKey returns []ssh.Signer generated from public key.
// If you have not specified a passphrase, please specify a empty character("").
func CreateSignerPublicKey(key, password string) (signer ssh.Signer, err error) {
	// get absolute path
	key = getAbsPath(key)

	// Read PrivateKey file
	keyData, err := os.ReadFile(key)
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
	keyData, err := os.ReadFile(key)
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
	certData, err := os.ReadFile(cert)
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
