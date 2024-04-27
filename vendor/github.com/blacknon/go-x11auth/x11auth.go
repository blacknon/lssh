// Copyright (c) 2024 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package x11auth

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unsafe"
)

// readChunk function is reads the size of a chunk and returns a byte slice of that size.
func readChunk(r io.Reader) ([]byte, error) {
	// The chunk is encoded with a length of 2 bytes followed by the actual value.
	size := make([]byte, 2)
	_, err := io.ReadFull(r, size)
	if err != nil {
		return nil, err
	}

	// Read the chunk size and return a byte slice of that size.
	chunkSize := binary.BigEndian.Uint16(size)
	chunk := make([]byte, chunkSize)
	_, err = io.ReadFull(r, chunk)
	if err != nil {
		return nil, err
	}

	return chunk, nil
}

// xAuthEntry is data struct in xauth file.
type xAuthEntry struct {
	Family  uint16
	Address string
	Number  string
	Name    string
	Data    []byte
}

type XAuth struct {
	// environment $DISPLAY.
	// example: /private/tmp/hoge/unix:0
	Display string
}

// generateXAuth function generates a temporary cookie on the specified temporary file like `ssh -X`.
// we need to connect to xserver, so we run the `xauth` command.
func (x *XAuth) generateXAuth(tempFilePath string) (err error) {
	// get xauth full path
	xauthPath, err := exec.LookPath("xauth")
	if err != nil {
		return
	}

	// exec os command...
	cmd := exec.Command(
		xauthPath, "-f", tempFilePath, "generate", x.Display,
	)
	err = cmd.Run()

	return
}

// getXAuthCookie
func (x *XAuth) GetXAuthCookie(xauthorityPath string, trusted bool) (cookie string, err error) {
	// parse display
	displayFilePath := filepath.Base(x.Display)
	address, number, err := net.SplitHostPort(displayFilePath)
	if err != nil {
		return
	}

	// If trusted is bool or the XAuthority file is not specified,
	// a temporary file is created and a cookie is generated there.
	if !trusted || xauthorityPath == "" {
		// create temp file.
		tempFile, tempErr := os.CreateTemp("", ".xauthority_")
		if tempErr != nil {
			err = tempErr
			return
		}

		// generate xauth cookie
		tempErr = x.generateXAuth(tempFile.Name())
		if tempErr != nil {
			err = tempErr
			return
		}

		// update x.XAuthorityFilePath
		xauthorityPath = tempFile.Name()

		defer os.Remove(tempFile.Name())
	}

	entries, trustedErr := x.GetXAuthList(xauthorityPath)
	if trustedErr != nil {
		err = trustedErr
	} else {
		for _, entry := range entries {
			if entry.Address == address && entry.Number == number {
				cookie = strings.ToLower(fmt.Sprintf("%x", entry.Data))
			}
		}
	}

	return
}

// getXAuthList
func (x *XAuth) GetXAuthList(xAuthorityFilePath string) (entries []xAuthEntry, err error) {
	// Xauthorityファイルを開きます。
	file, oerr := os.Open(xAuthorityFilePath)
	if oerr != nil {
		err = fmt.Errorf("Failed to open Xauthority file: %w", oerr)
		return
	}
	defer file.Close()

	// ファイルからエントリを読み取ります。
	for {
		// エントリのファミリーを読み取ります。
		var family uint16
		err := binary.Read(file, binary.BigEndian, &family)
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}

		// エントリの各フィールドを読み取ります。
		address, err := readChunk(file)
		if err != nil {
			return nil, err
		}
		number, err := readChunk(file)
		if err != nil {
			return nil, err
		}
		name, err := readChunk(file)
		if err != nil {
			return nil, err
		}
		data, err := readChunk(file)
		if err != nil {
			return nil, err
		}

		// 読み取ったデータをXauthorityエントリに追加します。
		entry := xAuthEntry{
			Family:  family,
			Address: *(*string)(unsafe.Pointer(&address)),
			Number:  *(*string)(unsafe.Pointer(&number)),
			Name:    *(*string)(unsafe.Pointer(&name)),
			Data:    data,
		}
		entries = append(entries, entry)
	}

	return entries, nil
}
