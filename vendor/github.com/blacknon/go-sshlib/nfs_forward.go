// Copyright (c) 2024 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package sshlib

import (
	"fmt"
	"net"
	"strings"

	nfs "github.com/blacknon/go-nfs-sshlib"
	nfshelper "github.com/blacknon/go-nfs-sshlib/helpers"
	osfs "github.com/go-git/go-billy/v5/osfs"
	"github.com/pkg/sftp"
)

func (c *Connect) NFSForward(address, port, basepoint string) (err error) {
	// create listener
	listener, err := net.Listen("tcp", net.JoinHostPort(address, port))
	if err != nil {
		return
	}
	defer listener.Close()

	client, err := sftp.NewClient(c.Client)
	if err != nil {
		return
	}

	// create abs path
	homepoint, err := client.RealPath(".")
	if err != nil {
		return
	}
	basepoint = getRemoteAbsPath(homepoint, basepoint)
	fmt.Println(basepoint)

	sftpfsPlusChange := NewChangeSFTPFS(client, basepoint)

	handler := nfshelper.NewNullAuthHandler(sftpfsPlusChange)
	cacheHelper := nfshelper.NewCachingHandler(handler, 1024)

	// listen
	err = nfs.Serve(listener, cacheHelper)

	return
}

// NFSReverseForward is Start NFS Server and forward port to remote server.
// This port is forawrd GO-NFS Server.
func (c *Connect) NFSReverseForward(address, port, sharepoint string) (err error) {
	// create listener
	listener, err := c.Client.Listen("tcp", net.JoinHostPort(address, port))
	if err != nil {
		return
	}
	defer listener.Close()

	bfs := osfs.New(sharepoint)
	bfsPlusChange := NewChangeOSFS(bfs)

	handler := nfshelper.NewNullAuthHandler(bfsPlusChange)
	cacheHelper := nfshelper.NewCachingHandler(handler, 1024)

	// listen
	err = nfs.Serve(listener, cacheHelper)

	return
}

func getRemoteAbsPath(wdpath, path string) (result string) {
	result = strings.Replace(path, "~", wdpath, 1)
	if !strings.HasPrefix(result, "/") {
		result = wdpath + "/" + result
	}

	return result
}
