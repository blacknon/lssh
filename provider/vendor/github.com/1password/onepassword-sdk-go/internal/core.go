package internal

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"runtime"
)

const (
	SDKLanguage           = "Go"
	DefaultRequestLibrary = "net/http"
)

//go:embed release/version-build
var SDKSemverVersion string

type Core interface {
	InitClient(ctx context.Context, config []byte) ([]byte, error)
	Invoke(ctx context.Context, invokeConfig []byte) ([]byte, error)
	ReleaseClient(clientID []byte)
}

type CoreWrapper struct {
	InnerCore Core
}

// InitClient creates a client instance in the current core module and returns its unique ID.
func (c *CoreWrapper) InitClient(ctx context.Context, config ClientConfig) (*uint64, error) {
	marshaledConfig, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}

	// first return parameter is a sys.Exit code, which we don't need since the error is fully recoverable
	res, err := c.InnerCore.InitClient(ctx, marshaledConfig)
	if err != nil {
		return nil, err
	}
	var id uint64
	err = json.Unmarshal(res, &id)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

// Invoke calls specified business logic from core
func (c *CoreWrapper) Invoke(ctx context.Context, invokeConfig InvokeConfig) (*string, error) {
	input, err := json.Marshal(invokeConfig)
	if err != nil {
		return nil, err
	}
	if len(input) > messageLimit {
		return nil, fmt.Errorf("message size exceeds the limit of %d bytes, please contact 1Password at support@1password.com or https://developer.1password.com/joinslack if you need help", messageLimit)
	}
	res, err := c.InnerCore.Invoke(ctx, input)
	if err != nil {
		return nil, err
	}

	response := string(res)

	return &response, nil
}

// ReleaseClient releases memory in the core associated with the given client ID.
func (c *CoreWrapper) ReleaseClient(clientID uint64) {
	marshaledClientID, err := json.Marshal(clientID)
	if err != nil {
		log.Println("failed to marshal clientID")
	}
	c.InnerCore.ReleaseClient(marshaledClientID)
}

// ClientConfig contains information required for creating a client.
type ClientConfig struct {
	SAToken               string  `json:"serviceAccountToken"`
	Language              string  `json:"programmingLanguage"`
	SDKVersion            string  `json:"sdkVersion"`
	IntegrationName       string  `json:"integrationName"`
	IntegrationVersion    string  `json:"integrationVersion"`
	RequestLibraryName    string  `json:"requestLibraryName"`
	RequestLibraryVersion string  `json:"requestLibraryVersion"`
	SystemOS              string  `json:"os"`
	SystemOSVersion       string  `json:"osVersion"`
	SystemArch            string  `json:"architecture"`
	AccountName           *string `json:"account_name"`
}

func NewDefaultConfig() ClientConfig {
	// TODO: add logic for determining this for all systems in a different PR.
	const defaultOSVersion = "0.0.0"
	return ClientConfig{
		Language:              SDKLanguage,
		SDKVersion:            SDKSemverVersion,
		RequestLibraryName:    DefaultRequestLibrary,
		RequestLibraryVersion: runtime.Version(),
		SystemOS:              runtime.GOOS,
		SystemArch:            runtime.GOARCH,
		SystemOSVersion:       defaultOSVersion,
	}
}

// InvokeConfig specifies over the FFI on which client the specified method should be invoked on.
type InvokeConfig struct {
	Invocation Invocation `json:"invocation"`
}

// Invocation holds the information required for invoking SDK functionality.
type Invocation struct {
	ClientID   *uint64    `json:"clientId,omitempty"`
	Parameters Parameters `json:"parameters"`
}

type Parameters struct {
	MethodName       string                 `json:"name"`
	SerializedParams map[string]interface{} `json:"parameters"`
}

// InnerClient represents the sdk-core client on which calls will be made.
type InnerClient struct {
	ID     uint64
	Config ClientConfig
	Core   CoreWrapper
}
