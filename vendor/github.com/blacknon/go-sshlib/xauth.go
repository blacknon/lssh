// Copyright (c) 2024 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package sshlib

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"unsafe"
)

// xAuthEntry is data struct in xauth file.
type xAuthEntry struct {
	Family  uint16
	Address string
	Number  string
	Name    string
	Data    []byte
}

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

type XAuth struct {
	// path for XAuthority files
	XAuthorityFilePath string

	// environment $DISPLAY.
	// example: /private/tmp/hoge/unix:0
	Display string
}

// getXAuthCookie
func (x *XAuth) GetXAuthCookie(trusted bool) (cookie string, err error) {
	// parse display
	displayFilePath := filepath.Base(x.Display)
	address, number, err := net.SplitHostPort(displayFilePath)
	if err != nil {
		return
	}

	// If trusted is bool or the XAuthority file is not specified,
	// a temporary file is created and a cookie is generated there.
	if !trusted || x.XAuthorityFilePath == "" {

	}

	entries, trustedErr := x.GetXAuthList()
	if trustedErr != nil {
		err = trustedErr
	} else {
		for _, entry := range entries {
			if entry.Address == address && entry.Number == number {
				cookie = strings.ToLower(fmt.Sprintf("%x", entry.Data))
			}
		}
	}

	return cookie, err
}

// getXAuthList
func (x *XAuth) GetXAuthList() (entries []xAuthEntry, err error) {
	xauthorityPath := x.XAuthorityFilePath
	if xauthorityPath == "" {
		err = fmt.Errorf("XAUTHORITY environment variable is not set.")
		return
	}

	// Xauthorityファイルを開きます。
	file, oerr := os.Open(xauthorityPath)
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
