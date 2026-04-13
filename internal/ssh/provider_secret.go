package ssh

import (
	"fmt"
	"os"

	"github.com/blacknon/go-sshlib"
	conf "github.com/blacknon/lssh/internal/config"
	"golang.org/x/crypto/ssh"
)

func (r *Run) resolveLiteralOrRef(server, field, literal, ref string) (string, error) {
	if ref == "" {
		return literal, nil
	}
	return r.Conf.ResolveSecretRef(ref, server, field)
}

func (r *Run) createPublicKeyAuthMethod(server string, cfg conf.ServerConfig) (ssh.AuthMethod, error) {
	keyPath, cleanup, err := r.resolveSecretFile(server, "key", cfg.Key, cfg.KeyRef)
	if err != nil {
		return nil, err
	}
	if cleanup != nil {
		defer cleanup()
	}

	keyPass, err := r.resolveLiteralOrRef(server, "keypass", cfg.KeyPass, cfg.KeyPassRef)
	if err != nil {
		return nil, err
	}

	return sshlib.CreateAuthMethodPublicKey(keyPath, keyPass)
}

func (r *Run) createCertificateAuthMethod(server string, cfg conf.ServerConfig) (ssh.AuthMethod, error) {
	certPath, certCleanup, err := r.resolveSecretFile(server, "cert", cfg.Cert, cfg.CertRef)
	if err != nil {
		return nil, err
	}
	if certCleanup != nil {
		defer certCleanup()
	}

	keyPath, keyCleanup, err := r.resolveSecretFile(server, "certkey", cfg.CertKey, cfg.CertKeyRef)
	if err != nil {
		return nil, err
	}
	if keyCleanup != nil {
		defer keyCleanup()
	}

	keyPass, err := r.resolveLiteralOrRef(server, "certkeypass", cfg.CertKeyPass, cfg.CertKeyPassRef)
	if err != nil {
		return nil, err
	}

	signer, err := sshlib.CreateSignerPublicKeyPrompt(keyPath, keyPass)
	if err != nil {
		return nil, err
	}

	return sshlib.CreateAuthMethodCertificate(certPath, signer)
}

func (r *Run) resolveSecretFile(server, field, literal, ref string) (string, func(), error) {
	if ref == "" {
		return literal, nil, nil
	}

	value, err := r.Conf.ResolveSecretRef(ref, server, field)
	if err != nil {
		return "", nil, err
	}

	// Existing file paths can be returned directly by the provider.
	if value != "" && value[0] == '/' {
		return value, nil, nil
	}

	file, err := os.CreateTemp("", "lssh-provider-secret-*")
	if err != nil {
		return "", nil, err
	}
	if _, err := file.WriteString(value); err != nil {
		_ = file.Close()
		_ = os.Remove(file.Name())
		return "", nil, err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(file.Name())
		return "", nil, err
	}
	if err := os.Chmod(file.Name(), 0o600); err != nil {
		_ = os.Remove(file.Name())
		return "", nil, err
	}

	return file.Name(), func() {
		if err := os.Remove(file.Name()); err != nil && !os.IsNotExist(err) {
			fmt.Fprintln(os.Stderr, err)
		}
	}, nil
}
