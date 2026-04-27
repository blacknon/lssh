//go:build windows

package internal

import (
	"encoding/json"
	"errors"
	"unsafe"

	"golang.org/x/sys/windows"
)

type SharedLibCore struct {
	accountName string
	dll         *windows.DLL
	procSend    *windows.Proc
	procFree    *windows.Proc
}

var coreLib *SharedLibCore

func loadCore(path string) (*SharedLibCore, error) {
	dll, err := windows.LoadDLL(path) // absolute path avoids search path surprises
	if err != nil {
		return nil, err
	}
	send, err := dll.FindProc("op_sdk_ipc_send_message")
	if err != nil {
		dll.Release()
		return nil, errors.New("failed to load send_message")
	}
	free, err := dll.FindProc("op_sdk_ipc_free_response")
	if err != nil {
		dll.Release()
		return nil, errors.New("failed to load free_message")
	}
	return &SharedLibCore{
		dll:      dll,
		procSend: send,
		procFree: free,
	}, nil
}

func (slc *SharedLibCore) callSharedLibrary(input []byte) ([]byte, error) {
	if len(input) == 0 {
		return nil, errors.New("internal: empty input")
	}

	// Signature we’re calling (from Rust exports):
	// int32_t op_sdk_ipc_send_message(const uint8_t* msg_ptr, size_t msg_len,
	//                                 uint8_t** out_buf, size_t* out_len, size_t* out_cap);
	var outBuf *byte
	var outLen uintptr
	var outCap uintptr

	r1, _, callErr := slc.procSend.Call(
		uintptr(unsafe.Pointer(&input[0])),
		uintptr(len(input)),
		uintptr(unsafe.Pointer(&outBuf)),
		uintptr(unsafe.Pointer(&outLen)),
		uintptr(unsafe.Pointer(&outCap)),
	)
	// syscall layer error
	if callErr != nil && callErr != windows.ERROR_SUCCESS {
		return nil, callErr
	}
	// library-level return code
	err := errorFromReturnCode(int32(r1))
	if err != nil {
		return nil, err
	}

	// Copy response out of the DLL buffer, then free via exported function
	resp := unsafe.Slice(outBuf, outLen)
	out := make([]byte, outLen)
	copy(out, resp)

	// void op_sdk_ipc_free_response(uint8_t* buf, size_t len, size_t cap);
	_, _, _ = slc.procFree.Call(
		uintptr(unsafe.Pointer(outBuf)),
		outLen,
		outCap,
	)

	// Match Unix: decode envelope and return payload or error
	var response Response
	if err = json.Unmarshal(out, &response); err != nil {
		return nil, err
	}
	if response.Success {
		return response.Payload, nil
	}
	return nil, response
}
