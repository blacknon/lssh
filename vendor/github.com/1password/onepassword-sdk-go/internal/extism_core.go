package internal

import (
	"context"
	_ "embed"
	"fmt"
	"sync"

	extism "github.com/extism/go-sdk"
)

//go:embed wasm/core.wasm
var coreWASM []byte

// In empirical tests, we determined that maximum message size that can cross the FFI boundary
// is ~64MB. Past this limit, the Extism FFI will throw an error and the program will crash.
// We set the limit to 50MB to be safe, to be reconsidered upon further testing.
const messageLimit = 50 * 1024 * 1024

const (
	invokeFuncName        = "invoke"
	initClientFuncName    = "init_client"
	releaseClientFuncName = "release_client"
)

var core *ExtismCore

// GetExtismCore initializes the shared core once and returns the already existing one on subsequent calls.
func GetExtismCore() (*CoreWrapper, error) {
	runtimeCtx := context.Background()
	if core == nil {
		p, err := loadWASM(runtimeCtx)
		if err != nil {
			return nil, err
		}
		core = &ExtismCore{plugin: p}
	}

	coreWrapper := CoreWrapper{
		InnerCore: core,
	}

	return &coreWrapper, nil
}

func ReleaseCore() {
	core = nil
}

// ExtismCore implements Core in such a way that all created client instances share the same core resources.
type ExtismCore struct {
	// lock is used to synchronize access to the shared WASM core which is single threaded
	lock sync.Mutex
	// plugin is the Extism plugin which represents the WASM core loaded into memory
	plugin *extism.Plugin
}

// InitClient creates a client instance in the current core module and returns its unique ID.
func (c *ExtismCore) InitClient(ctx context.Context, config []byte) ([]byte, error) {
	// first return parameter is a sys.Exit code, which we don't need since the error is fully recoverable
	res, err := c.callWithCtx(ctx, initClientFuncName, config)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// Invoke calls specified business logic from core
func (c *ExtismCore) Invoke(ctx context.Context, invokeConfig []byte) ([]byte, error) {
	if len(invokeConfig) > messageLimit {
		return nil, fmt.Errorf("message size exceeds the limit of %d bytes, please contact 1Password at support@1password.com or https://developer.1password.com/joinslack if you need help", messageLimit)
	}
	res, err := c.callWithCtx(ctx, invokeFuncName, invokeConfig)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// ReleaseClient releases memory in the core associated with the given client ID.
func (c *ExtismCore) ReleaseClient(clientID []byte) {
	_, err := c.call(releaseClientFuncName, clientID)
	if err != nil {
		c.plugin.Log(extism.LogLevelWarn, "memory couldn't be released")
	}
}

func (c *ExtismCore) callWithCtx(ctx context.Context, functionName string, serializedParameters []byte) ([]byte, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	_, response, err := c.plugin.CallWithContext(ctx, functionName, serializedParameters)
	if err != nil {
		return nil, err
	}
	return response, nil
}

func (c *ExtismCore) call(functionName string, serializedParameters []byte) ([]byte, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	_, response, err := c.plugin.Call(functionName, serializedParameters)
	if err != nil {
		return nil, err
	}
	return response, nil
}

// `loadWASM` returns the WASM core loaded into an `extism.Plugin`.
func loadWASM(ctx context.Context) (*extism.Plugin, error) {
	manifest := extism.Manifest{
		Wasm: []extism.Wasm{
			extism.WasmData{
				Data: coreWASM,
			},
		},
		AllowedHosts: allowed1PHosts(),
	}

	extismConfig := extism.PluginConfig{}
	plugin, err := extism.NewPlugin(ctx, manifest, extismConfig, ImportedFunctions())
	if err != nil {
		return nil, fmt.Errorf("failed to initialize plugin: %v", err)
	}

	return plugin, nil
}

// `allowed1PHosts` returns all hosts accessible through the WASM core.
func allowed1PHosts() []string {
	return []string{
		"*.1password.com",
		"*.1password.ca",
		"*.1password.eu",
		"*.b5staging.com",
		"*.b5dev.com",
		"*.b5dev.ca",
		"*.b5dev.eu",
		"*.b5test.com",
		"*.b5test.ca",
		"*.b5test.eu",
		"*.b5rev.com",
		"*.b5local.com",
	}
}
