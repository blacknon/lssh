//go:build darwin || linux

package internal

import (
	"encoding/json"
	"errors"
	"unsafe"
)

/*
#cgo LDFLAGS: -ldl

#include <dlfcn.h>
#include <stdlib.h>
#include <stdint.h>

// Function pointer types matching Rust exports
typedef int32_t (*send_message_t)(
    const uint8_t* msg_ptr,
    size_t msg_len,
    uint8_t** out_buf,
    size_t* out_len,
    size_t* out_cap
);

typedef void (*free_message_t)(
    uint8_t* buf,
    size_t len,
    size_t cap
);

// Trampoline for calling `send_message`, as Go cannot call function pointers directly.
static inline int32_t call_send_message(
    send_message_t fn,
    const uint8_t* msg_ptr,
    size_t msg_len,
    uint8_t** out_buf,
    size_t* out_len,
    size_t* out_cap
) {
    return fn(msg_ptr, msg_len, out_buf, out_len, out_cap);
}

// Trampoline for calling `free_message`, as Go cannot call function pointers directly.
static inline void call_free_message(
    free_message_t fn,
    uint8_t* buf,
    size_t len,
    size_t cap
) {
    fn(buf, len, cap);
}

// dlopen wrapper
static void* open_library(const char* path) {
    return dlopen(path, RTLD_NOW);
}

// dlsym wrapper
static void* load_symbol(void* handle, const char* name) {
    return dlsym(handle, name);
}

// dlclose wrapper
static int close_library(void* handle) {
    return dlclose(handle);
}

*/
import "C"

type SharedLibCore struct {
	accountName  string
	handle       unsafe.Pointer
	sendMessage  C.send_message_t
	freeResponse C.free_message_t
}

var coreLib *SharedLibCore

func loadCore(path string) (*SharedLibCore, error) {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	handle := C.open_library(cPath)
	if handle == nil {
		return nil, errors.New("failed to open library")
	}

	symbol := C.CString("op_sdk_ipc_send_message")
	defer C.free(unsafe.Pointer(symbol))

	fnSend := C.load_symbol(handle, symbol)
	if fnSend == nil {
		C.close_library(handle)
		return nil, errors.New("failed to load send_message")
	}

	symbolFree := C.CString("op_sdk_ipc_free_response")
	defer C.free(unsafe.Pointer(symbolFree))

	fnFree := C.load_symbol(handle, symbolFree)
	if fnFree == nil {
		C.close_library(handle)
		return nil, errors.New("failed to load free_message")
	}

	return &SharedLibCore{
		handle:       handle,
		sendMessage:  (C.send_message_t)(fnSend),
		freeResponse: (C.free_message_t)(fnFree),
	}, nil
}

func (slc *SharedLibCore) callSharedLibrary(input []byte) ([]byte, error) {
	if len(input) == 0 {
		return nil, errors.New("internal: empty input")
	}

	var outBuf *C.uint8_t
	var outLen C.size_t
	var outCap C.size_t

	retCode := C.call_send_message(
		slc.sendMessage,
		(*C.uint8_t)(unsafe.Pointer(&input[0])),
		C.size_t(len(input)),
		&outBuf,
		&outLen,
		&outCap,
	)
	err := errorFromReturnCode(int32(retCode))
	if err != nil {
		return nil, err
	}

	resp := C.GoBytes(unsafe.Pointer(outBuf), C.int(outLen))
	// Call trampoline with the function pointer
	C.call_free_message(slc.freeResponse, outBuf, outLen, outCap)

	var response Response
	err = json.Unmarshal(resp, &response)
	if err != nil {
		return nil, err
	}

	if response.Success {
		return response.Payload, nil
	} else {
		return nil, response
	}
}
