package main

import (
	"errors"
	"testing"

	"github.com/blacknon/lssh/internal/providerapi"
)

func TestOnePasswordAuthMode(t *testing.T) {
	tests := []struct {
		name    string
		config  map[string]interface{}
		want    string
		wantErr bool
	}{
		{name: "default auto", config: map[string]interface{}{}, want: onePasswordAuthModeAuto},
		{name: "service account", config: map[string]interface{}{"auth_mode": "service_account"}, want: onePasswordAuthModeServiceAccount},
		{name: "cli", config: map[string]interface{}{"auth_mode": "cli"}, want: onePasswordAuthModeCLI},
		{name: "invalid", config: map[string]interface{}{"auth_mode": "weird"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := onePasswordAuthMode(tt.config)
			if tt.wantErr {
				if err == nil {
					t.Fatal("onePasswordAuthMode() error = nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("onePasswordAuthMode() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("onePasswordAuthMode() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetSecretCLI(t *testing.T) {
	original := runOnePasswordCLI
	defer func() { runOnePasswordCLI = original }()

	runOnePasswordCLI = func(config map[string]interface{}, args ...string) ([]byte, error) {
		if len(args) != 2 || args[0] != "read" || args[1] != "op://Vault/item/password" {
			t.Fatalf("args = %#v", args)
		}
		return []byte("secret-from-cli\n"), nil
	}

	got, err := getSecret(providerapi.SecretGetParams{
		Config: map[string]interface{}{"auth_mode": "cli"},
		Ref:    "op://Vault/item/password",
	})
	if err != nil {
		t.Fatalf("getSecret() error = %v", err)
	}
	if got != "secret-from-cli" {
		t.Fatalf("getSecret() = %q", got)
	}
}

func TestGetSecretAutoWithoutTokenFallsBackToCLI(t *testing.T) {
	original := runOnePasswordCLI
	defer func() { runOnePasswordCLI = original }()

	runOnePasswordCLI = func(config map[string]interface{}, args ...string) ([]byte, error) {
		return []byte("secret-from-auto\n"), nil
	}

	got, err := getSecret(providerapi.SecretGetParams{
		Config: map[string]interface{}{"auth_mode": "auto"},
		Ref:    "op://Vault/item/password",
	})
	if err != nil {
		t.Fatalf("getSecret() error = %v", err)
	}
	if got != "secret-from-auto" {
		t.Fatalf("getSecret() = %q", got)
	}
}

func TestOnePasswordHealthCheckCLI(t *testing.T) {
	original := runOnePasswordCLI
	defer func() { runOnePasswordCLI = original }()

	runOnePasswordCLI = func(config map[string]interface{}, args ...string) ([]byte, error) {
		if len(args) != 1 || args[0] != "whoami" {
			t.Fatalf("args = %#v", args)
		}
		return []byte("demo@example.com\n"), nil
	}

	got, err := onePasswordHealthCheck(map[string]interface{}{"auth_mode": "cli"})
	if err != nil {
		t.Fatalf("onePasswordHealthCheck() error = %v", err)
	}
	if !got.OK {
		t.Fatalf("onePasswordHealthCheck().OK = false")
	}
}

func TestOnePasswordHealthCheckCLIFailure(t *testing.T) {
	original := runOnePasswordCLI
	defer func() { runOnePasswordCLI = original }()

	runOnePasswordCLI = func(config map[string]interface{}, args ...string) ([]byte, error) {
		return nil, errors.New("not signed in")
	}

	_, err := onePasswordHealthCheck(map[string]interface{}{"auth_mode": "cli"})
	if err == nil {
		t.Fatal("onePasswordHealthCheck() error = nil")
	}
}
