//go:build cgo || windows

package internal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"runtime"
)

type Request struct {
	Kind        string `json:"kind"`
	AccountName string `json:"account_name"`
	Payload     []byte `json:"payload"`
}

type Response struct {
	Success bool   `json:"success"`
	Payload []byte `json:"payload"`
}

func (r Response) Error() string { return string(r.Payload) }

// find1PasswordLibPath returns the path to the 1Password shared library
// (libop_sdk_ipc_client.dylib/.so/.dll) depending on OS.
func find1PasswordLibPath() (string, error) {
	var locations []string

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	switch runtime.GOOS {
	case "darwin":
		locations = []string{
			"/Applications/1Password.app/Contents/Frameworks/libop_sdk_ipc_client.dylib",
			path.Join(home, "Applications/1Password.app/Contents/Frameworks/libop_sdk_ipc_client.dylib"),
		}

	case "linux":
		locations = []string{
			"/usr/bin/1password/libop_sdk_ipc_client.so",
			"/opt/1Password/libop_sdk_ipc_client.so",
			"/snap/bin/1password/libop_sdk_ipc_client.so",
		}

	case "windows":
		locations = []string{
			path.Join(home, `AppData\Local\1Password\op_sdk_ipc_client.dll`),
			`C:\Program Files\1Password\app\8\op_sdk_ipc_client.dll`,
			`C:\Program Files (x86)\1Password\app\8\op_sdk_ipc_client.dll`,
			path.Join(home, `AppData\Local\1Password\app\8\op_sdk_ipc_client.dll`),
		}

	default:
		return "", fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
	for _, libPath := range locations {
		if _, err := os.Stat(libPath); err == nil {
			return libPath, nil
		}
	}

	return "", fmt.Errorf("1Password desktop application not found")
}

func GetSharedLibCore(accountName string) (*CoreWrapper, error) {
	if coreLib == nil {
		libPath, err := find1PasswordLibPath()
		if err != nil {
			return nil, err
		}
		coreLib, err = loadCore(libPath)
		if err != nil {
			return nil, err
		}
		coreLib.accountName = accountName
	}

	coreWrapper := CoreWrapper{InnerCore: coreLib}

	return &coreWrapper, nil
}

// InitClient creates a client instance in the current core module and returns its unique ID.
func (slc *SharedLibCore) InitClient(ctx context.Context, config []byte) ([]byte, error) {
	const kind = "init_client"
	request := Request{
		Kind:        kind,
		AccountName: slc.accountName,
		Payload:     config,
	}

	requestMarshaled, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	res, err := slc.callSharedLibrary(requestMarshaled)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// Invoke performs an SDK operation.
func (slc *SharedLibCore) Invoke(ctx context.Context, invokeConfig []byte) ([]byte, error) {
	const kind = "invoke"
	request := Request{
		Kind:        kind,
		AccountName: slc.accountName,
		Payload:     invokeConfig,
	}

	requestMarshaled, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	res, err := slc.callSharedLibrary(requestMarshaled)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// ReleaseClient releases memory in the core associated with the given client ID.
func (slc *SharedLibCore) ReleaseClient(clientID []byte) {
	const kind = "release_client"
	request := Request{
		Kind:        kind,
		AccountName: slc.accountName,
		Payload:     clientID,
	}

	requestMarshaled, err := json.Marshal(request)
	if err != nil {
		log.Println("failed to marshal release_client request")
		return
	}

	_, err = slc.callSharedLibrary(requestMarshaled)
	if err != nil {
		log.Println("failed to release client")
	}
}

const (
	errChannelClosed     = "desktop app connection channel is closed. Make sure Settings > Developer > Integrate with other apps is enabled, or contact 1Password support"
	errConnectionDropped = "connection was unexpectedly dropped by the desktop app. Make sure the desktop app is running and Settings > Developer > Integrate with other apps is enabled, or contact 1Password support"
	errInternalFmt       = "an internal error occurred. Please contact 1Password support and mention the return code: %d"
)

func errorFromReturnCode(retCode int32) error {
	if retCode == 0 {
		return nil
	}

	if runtime.GOOS == "darwin" {
		switch retCode {
		case -3:
			return errors.New(errChannelClosed)
		case -7:
			return errors.New(errConnectionDropped)
		default:
			return fmt.Errorf(errInternalFmt, retCode)
		}
	}

	switch retCode {
	case -2:
		return errors.New(errChannelClosed)
	case -5:
		return errors.New(errConnectionDropped)
	default:
		return fmt.Errorf(errInternalFmt, retCode)
	}
}
