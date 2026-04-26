package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/blacknon/lssh/providerapi"
)

func TestBitwardenAuthMode(t *testing.T) {
	tests := []struct {
		name    string
		config  map[string]interface{}
		want    string
		wantErr bool
	}{
		{name: "default auto", config: map[string]interface{}{}, want: bitwardenAuthModeAuto},
		{name: "sdk", config: map[string]interface{}{"auth_mode": "sdk"}, want: bitwardenAuthModeSDK},
		{name: "cli", config: map[string]interface{}{"auth_mode": "cli"}, want: bitwardenAuthModeCLI},
		{name: "invalid", config: map[string]interface{}{"auth_mode": "weird"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := bitwardenAuthMode(tt.config)
			if tt.wantErr {
				if err == nil {
					t.Fatal("bitwardenAuthMode() error = nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("bitwardenAuthMode() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("bitwardenAuthMode() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBitwardenCLIEnv(t *testing.T) {
	got := bitwardenCLIEnv(map[string]interface{}{
		"appdata_dir": "/tmp/bitwarden-cli",
	})

	if len(got) != 1 || got[0] != "BITWARDENCLI_APPDATA_DIR=/tmp/bitwarden-cli" {
		t.Fatalf("bitwardenCLIEnv() = %#v", got)
	}
}

func TestSplitBitwardenRef(t *testing.T) {
	tests := []struct {
		name        string
		ref         string
		wantLocator string
		wantField   string
	}{
		{name: "plain id", ref: "item-id", wantLocator: "item-id", wantField: "value"},
		{name: "id with field", ref: "item-id/key", wantLocator: "item-id", wantField: "key"},
		{name: "path style with field", ref: "folder/sub/item/key", wantLocator: "folder/sub/item", wantField: "key"},
		{name: "path style custom field", ref: "folder/sub/item/field:ssh_key", wantLocator: "folder/sub/item", wantField: "field:ssh_key"},
		{name: "path style without supported field", ref: "folder/sub/item", wantLocator: "folder/sub/item", wantField: "value"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotLocator, gotField := splitBitwardenRef(tt.ref)
			if gotLocator != tt.wantLocator || gotField != tt.wantField {
				t.Fatalf("splitBitwardenRef(%q) = (%q, %q), want (%q, %q)", tt.ref, gotLocator, gotField, tt.wantLocator, tt.wantField)
			}
		})
	}
}

func TestGetSecretCLI(t *testing.T) {
	original := runBitwardenCLI
	defer func() { runBitwardenCLI = original }()

	runBitwardenCLI = func(config map[string]interface{}, args ...string) ([]byte, error) {
		if len(args) < 3 || args[0] != "get" || args[1] != "password" || args[2] != "item-id" {
			t.Fatalf("args = %#v", args)
		}
		return []byte("secret-from-cli\n"), nil
	}

	got, resultType, err := getSecret(providerapi.SecretGetParams{
		Config: map[string]interface{}{"auth_mode": "cli"},
		Ref:    "item-id/password",
	})
	if err != nil {
		t.Fatalf("getSecret() error = %v", err)
	}
	if got != "secret-from-cli" {
		t.Fatalf("getSecret() = %q", got)
	}
	if resultType != "password" {
		t.Fatalf("resultType = %q", resultType)
	}
}

func TestGetSecretCLIItemField(t *testing.T) {
	original := runBitwardenCLI
	defer func() { runBitwardenCLI = original }()

	runBitwardenCLI = func(config map[string]interface{}, args ...string) ([]byte, error) {
		if len(args) < 3 || args[0] != "get" || args[1] != "item" || args[2] != "item-id" {
			t.Fatalf("args = %#v", args)
		}
		return []byte(`{"name":"GitHub","key":"ignored-top-level-key","notes":"team note","sshKey":{"privateKey":"-----BEGIN OPENSSH PRIVATE KEY-----","publicKey":"ssh-ed25519 AAAA","keyFingerprint":"SHA256:test"},"fields":[{"name":"key","value":"ignored-custom-field"}],"login":{"uris":[{"uri":"https://github.com"}]}}`), nil
	}

	got, resultType, err := getSecret(providerapi.SecretGetParams{
		Config: map[string]interface{}{"auth_mode": "cli"},
		Ref:    "item-id/key",
	})
	if err != nil {
		t.Fatalf("getSecret() error = %v", err)
	}
	if got != "-----BEGIN OPENSSH PRIVATE KEY-----" {
		t.Fatalf("getSecret() = %q", got)
	}
	if resultType != "text" {
		t.Fatalf("resultType = %q", resultType)
	}
}

func TestGetSecretCLISSHKeyPublicKey(t *testing.T) {
	original := runBitwardenCLI
	defer func() { runBitwardenCLI = original }()

	runBitwardenCLI = func(config map[string]interface{}, args ...string) ([]byte, error) {
		return []byte(`{"sshKey":{"privateKey":"-----BEGIN PRIVATE KEY-----","publicKey":"ssh-ed25519 AAAA","keyFingerprint":"SHA256:test"}}`), nil
	}

	got, resultType, err := getSecret(providerapi.SecretGetParams{
		Config: map[string]interface{}{"auth_mode": "cli"},
		Ref:    "item-id/public_key",
	})
	if err != nil {
		t.Fatalf("getSecret() error = %v", err)
	}
	if got != "ssh-ed25519 AAAA" {
		t.Fatalf("getSecret() = %q", got)
	}
	if resultType != "text" {
		t.Fatalf("resultType = %q", resultType)
	}
}

func TestGetSecretCLISSHKeyFingerprint(t *testing.T) {
	original := runBitwardenCLI
	defer func() { runBitwardenCLI = original }()

	runBitwardenCLI = func(config map[string]interface{}, args ...string) ([]byte, error) {
		return []byte(`{"sshKey":{"keyFingerprint":"SHA256:test"}}`), nil
	}

	got, resultType, err := getSecret(providerapi.SecretGetParams{
		Config: map[string]interface{}{"auth_mode": "cli"},
		Ref:    "item-id/fingerprint",
	})
	if err != nil {
		t.Fatalf("getSecret() error = %v", err)
	}
	if got != "SHA256:test" {
		t.Fatalf("getSecret() = %q", got)
	}
	if resultType != "text" {
		t.Fatalf("resultType = %q", resultType)
	}
}

func TestGetSecretCLIKeyFallsBackToNotes(t *testing.T) {
	original := runBitwardenCLI
	defer func() { runBitwardenCLI = original }()

	runBitwardenCLI = func(config map[string]interface{}, args ...string) ([]byte, error) {
		return []byte(`{"name":"GitHub","notes":"-----BEGIN PRIVATE KEY-----"}`), nil
	}

	got, resultType, err := getSecret(providerapi.SecretGetParams{
		Config: map[string]interface{}{"auth_mode": "cli"},
		Ref:    "item-id/key",
	})
	if err != nil {
		t.Fatalf("getSecret() error = %v", err)
	}
	if got != "-----BEGIN PRIVATE KEY-----" {
		t.Fatalf("getSecret() = %q", got)
	}
	if resultType != "text" {
		t.Fatalf("resultType = %q", resultType)
	}
}

func TestGetSecretCLICustomField(t *testing.T) {
	original := runBitwardenCLI
	defer func() { runBitwardenCLI = original }()

	runBitwardenCLI = func(config map[string]interface{}, args ...string) ([]byte, error) {
		return []byte(`{"fields":[{"name":"ssh_key","value":"ssh-private-key-data"}]}`), nil
	}

	got, resultType, err := getSecret(providerapi.SecretGetParams{
		Config: map[string]interface{}{"auth_mode": "cli"},
		Ref:    "item-id/field:ssh_key",
	})
	if err != nil {
		t.Fatalf("getSecret() error = %v", err)
	}
	if got != "ssh-private-key-data" {
		t.Fatalf("getSecret() = %q", got)
	}
	if resultType != "text" {
		t.Fatalf("resultType = %q", resultType)
	}
}

func TestGetSecretAutoWithoutTokenFallsBackToCLI(t *testing.T) {
	original := runBitwardenCLI
	defer func() { runBitwardenCLI = original }()

	runBitwardenCLI = func(config map[string]interface{}, args ...string) ([]byte, error) {
		return []byte("secret-from-auto\n"), nil
	}

	got, resultType, err := getSecret(providerapi.SecretGetParams{
		Config: map[string]interface{}{"auth_mode": "auto"},
		Ref:    "item-id/password",
	})
	if err != nil {
		t.Fatalf("getSecret() error = %v", err)
	}
	if got != "secret-from-auto" {
		t.Fatalf("getSecret() = %q", got)
	}
	if resultType != "password" {
		t.Fatalf("resultType = %q", resultType)
	}
}

func TestGetSecretSDKModeReturnsMigrationError(t *testing.T) {
	_, _, err := getSecret(providerapi.SecretGetParams{
		Config: map[string]interface{}{"auth_mode": "sdk"},
		Ref:    "item-id/password",
	})
	if err == nil {
		t.Fatal("getSecret() error = nil")
	}
	if !strings.Contains(err.Error(), `auth_mode="sdk" is no longer supported`) {
		t.Fatalf("getSecret() error = %v", err)
	}
}

func TestBitwardenHealthCheckCLI(t *testing.T) {
	original := runBitwardenCLI
	defer func() { runBitwardenCLI = original }()

	runBitwardenCLI = func(config map[string]interface{}, args ...string) ([]byte, error) {
		if len(args) < 1 || args[0] != "status" {
			t.Fatalf("args = %#v", args)
		}
		return []byte(`{"status":"unlocked"}`), nil
	}

	got, err := bitwardenHealthCheck(map[string]interface{}{"auth_mode": "cli"})
	if err != nil {
		t.Fatalf("bitwardenHealthCheck() error = %v", err)
	}
	if !got.OK {
		t.Fatalf("bitwardenHealthCheck().OK = false")
	}
}

func TestBitwardenHealthCheckCLIFailure(t *testing.T) {
	original := runBitwardenCLI
	defer func() { runBitwardenCLI = original }()

	runBitwardenCLI = func(config map[string]interface{}, args ...string) ([]byte, error) {
		return nil, errors.New("not logged in")
	}

	_, err := bitwardenHealthCheck(map[string]interface{}{"auth_mode": "cli"})
	if err == nil {
		t.Fatal("bitwardenHealthCheck() error = nil")
	}
}

func TestBitwardenHealthCheckSDKModeReturnsMigrationError(t *testing.T) {
	_, err := bitwardenHealthCheck(map[string]interface{}{"auth_mode": "sdk"})
	if err == nil {
		t.Fatal("bitwardenHealthCheck() error = nil")
	}
	if !strings.Contains(err.Error(), `auth_mode="sdk" is no longer supported`) {
		t.Fatalf("bitwardenHealthCheck() error = %v", err)
	}
}
