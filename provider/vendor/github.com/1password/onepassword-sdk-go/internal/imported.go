package internal

import (
	"context"
	"crypto/rand"
	"fmt"
	"time"

	extism "github.com/extism/go-sdk"
	"github.com/tetratelabs/wazero/api"
)

// ImportedFunctions returns all functions 1Password SDK core must import.
func ImportedFunctions() []extism.HostFunction {
	return []extism.HostFunction{randomFillImportedFunc(), currentTimeImportedFunc("op-now"), currentTimeImportedFunc("zxcvbn"), localOffsetImportedFunc()}
}

// randomFillImportedFunc returns an Extism Function for generating random byte sequence of a given length that will be imported into the WASM core.
func randomFillImportedFunc() extism.HostFunction {
	randomFillImported := extism.NewHostFunctionWithStack("random_fill_imported", func(ctx context.Context, p *extism.CurrentPlugin, stack []uint64) {
		randomFill(stack, func(b []byte) (uint64, error) {
			return p.WriteBytes(b)
		})
	}, []api.ValueType{api.ValueTypeI32}, []api.ValueType{api.ValueTypeI64})
	randomFillImported.SetNamespace("op-extism-core")

	return randomFillImported
}

// localOffset returns an Extism Function for retrieving the offset of the local time zone in seconds.
func localOffsetImportedFunc() extism.HostFunction {
	getOffsetFunc := extism.NewHostFunctionWithStack("utc_offset_seconds", func(ctx context.Context, p *extism.CurrentPlugin, stack []uint64) {
		_, offset := time.Now().Zone()
		stack[0] = uint64(offset)
	}, []api.ValueType{}, []api.ValueType{api.ValueTypeI64})
	getOffsetFunc.SetNamespace("op-time")
	return getOffsetFunc
}

// getTimeFunc returns an Extism Function for retrieving the current UNIX time.
func currentTimeImportedFunc(namespace string) extism.HostFunction {
	getTimeFunc := extism.NewHostFunctionWithStack("unix_time_milliseconds_imported", func(ctx context.Context, p *extism.CurrentPlugin, stack []uint64) {
		stack[0] = uint64(time.Now().UnixMilli())
	}, []api.ValueType{}, []api.ValueType{api.ValueTypeI64})
	getTimeFunc.SetNamespace(namespace)

	return getTimeFunc
}

// randomFill writes random bytes to the WASM plugin's memory using crypto.rand and pushes the pointer to that memory on the stack.
// input: stack - contains the length of the byte sequence to generate on position 0. At the end of the function it must contain the pointer to the written bytes.
// input: writeBytesToPluginMemory - writes bytes to the plugin's memory and returns the offset to that memory
func randomFill(stack []uint64, writeBytesToPluginMemory func(b []byte) (uint64, error)) {
	length := api.DecodeU32(stack[0])

	b := make([]byte, length)
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	ptr, err := writeBytesToPluginMemory(b)
	if err != nil {
		panic(fmt.Errorf("failed to write bytes: %v", err))
	}
	stack[0] = ptr
}
