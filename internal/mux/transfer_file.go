// Copyright (c) 2026 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package mux

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	sshcmd "github.com/blacknon/lssh/internal/ssh"
	"github.com/pkg/sftp"
)

type transferSFTPConn struct {
	client  *sftp.Client
	closeFn func() error
}

func (c *transferSFTPConn) Close() error {
	if c == nil {
		return nil
	}

	var firstErr error
	if c.client != nil {
		if err := c.client.Close(); firstErr == nil && err != nil {
			firstErr = err
		}
	}
	if c.closeFn != nil {
		if err := c.closeFn(); firstErr == nil && err != nil {
			firstErr = err
		}
	}
	return firstErr
}

func (w *transferWizard) openTransferSFTPForPane(p *pane) (*transferSFTPConn, error) {
	if w == nil || w.manager == nil || p == nil {
		return nil, fmt.Errorf("sftp unavailable")
	}

	run := &sshcmd.Run{
		ServerList: []string{p.server},
		Conf:       w.manager.conf,
	}
	run.CreateAuthMethodMap()

	connect, err := run.CreateSshConnectDirect(p.server)
	if err != nil {
		return nil, err
	}

	client, err := connect.OpenSFTP()
	if err != nil {
		_ = connect.Close()
		return nil, err
	}

	return &transferSFTPConn{
		client:  client,
		closeFn: connect.Close,
	}, nil
}

func copyRemotePathToLocal(client *sftp.Client, sourcePath, targetPath string) error {
	sourceInfo, err := client.Stat(sourcePath)
	if err != nil {
		return err
	}

	targetPath = resolveLocalTargetPath(targetPath, sourcePath, sourceInfo.IsDir())
	if sourceInfo.IsDir() {
		return copyRemoteDirToLocal(client, sourcePath, targetPath)
	}
	return copyRemoteFileToLocal(client, sourcePath, targetPath)
}

func copyLocalPathToRemote(client *sftp.Client, sourcePath, targetPath string) error {
	sourceInfo, err := os.Lstat(sourcePath)
	if err != nil {
		return err
	}

	targetPath, err = resolveRemoteTargetPath(client, targetPath, filepath.Base(sourcePath), sourceInfo.IsDir())
	if err != nil {
		return err
	}
	if sourceInfo.IsDir() {
		return copyLocalDirToRemote(client, sourcePath, targetPath)
	}
	return copyLocalFileToRemote(client, sourcePath, targetPath)
}

func copyRemotePathToRemote(srcClient, dstClient *sftp.Client, sourcePath, targetPath string) error {
	sourceInfo, err := srcClient.Stat(sourcePath)
	if err != nil {
		return err
	}

	targetPath, err = resolveRemoteTargetPath(dstClient, targetPath, path.Base(sourcePath), sourceInfo.IsDir())
	if err != nil {
		return err
	}
	if sourceInfo.IsDir() {
		return copyRemoteDirToRemote(srcClient, dstClient, sourcePath, targetPath)
	}
	return copyRemoteFileToRemote(srcClient, dstClient, sourcePath, targetPath)
}

func resolveLocalTargetPath(targetPath, sourcePath string, sourceIsDir bool) string {
	if targetPath == "" {
		targetPath = "."
	}
	targetPath = filepath.Clean(targetPath)

	if info, err := os.Stat(targetPath); err == nil && info.IsDir() {
		return filepath.Join(targetPath, filepath.Base(sourcePath))
	}
	if strings.HasSuffix(targetPath, string(os.PathSeparator)) {
		return filepath.Join(targetPath, filepath.Base(sourcePath))
	}
	if sourceIsDir && !strings.HasSuffix(targetPath, filepath.Base(sourcePath)) {
		return targetPath
	}
	return targetPath
}

func resolveRemoteTargetPath(client *sftp.Client, targetPath, baseName string, sourceIsDir bool) (string, error) {
	if targetPath == "" {
		targetPath = "."
	}
	resolved, err := resolveRemotePath(client, targetPath)
	if err != nil {
		return "", err
	}
	if info, err := client.Stat(resolved); err == nil && info.IsDir() {
		return path.Join(resolved, baseName), nil
	}
	if strings.HasSuffix(targetPath, "/") {
		return path.Join(resolved, baseName), nil
	}
	if sourceIsDir && !strings.HasSuffix(resolved, baseName) {
		return resolved, nil
	}
	return resolved, nil
}

func copyRemoteFileToLocal(client *sftp.Client, remotePath, localPath string) error {
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return err
	}

	src, err := client.Open(remotePath)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.OpenFile(localPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	return err
}

func copyRemoteDirToLocal(client *sftp.Client, remotePath, localPath string) error {
	base := remotePath
	walker := client.Walk(remotePath)
	for walker.Step() {
		if err := walker.Err(); err != nil {
			return err
		}
		current := walker.Path()
		rel, err := remoteRel(base, current)
		if err != nil {
			return err
		}
		target := filepath.Join(localPath, filepath.FromSlash(rel))
		if walker.Stat().IsDir() {
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
			continue
		}
		if err := copyRemoteFileToLocal(client, current, target); err != nil {
			return err
		}
	}
	return nil
}

func copyLocalFileToRemote(client *sftp.Client, localPath, remotePath string) error {
	if err := client.MkdirAll(path.Dir(remotePath)); err != nil {
		return err
	}

	src, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := client.OpenFile(remotePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	return err
}

func copyLocalDirToRemote(client *sftp.Client, localPath, remotePath string) error {
	base := localPath
	return filepath.Walk(localPath, func(current string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(base, current)
		if err != nil {
			return err
		}
		target := path.Join(remotePath, filepath.ToSlash(rel))
		if info.IsDir() {
			return client.MkdirAll(target)
		}
		return copyLocalFileToRemote(client, current, target)
	})
}

func copyRemoteFileToRemote(srcClient, dstClient *sftp.Client, sourcePath, targetPath string) error {
	if err := dstClient.MkdirAll(path.Dir(targetPath)); err != nil {
		return err
	}

	src, err := srcClient.Open(sourcePath)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := dstClient.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	return err
}

func copyRemoteDirToRemote(srcClient, dstClient *sftp.Client, sourcePath, targetPath string) error {
	base := sourcePath
	walker := srcClient.Walk(sourcePath)
	for walker.Step() {
		if err := walker.Err(); err != nil {
			return err
		}
		current := walker.Path()
		rel, err := remoteRel(base, current)
		if err != nil {
			return err
		}
		target := path.Join(targetPath, rel)
		if walker.Stat().IsDir() {
			if err := dstClient.MkdirAll(target); err != nil {
				return err
			}
			continue
		}
		if err := copyRemoteFileToRemote(srcClient, dstClient, current, target); err != nil {
			return err
		}
	}
	return nil
}

func remoteRel(base, current string) (string, error) {
	base = path.Clean(base)
	current = path.Clean(current)
	if current == base {
		return ".", nil
	}
	prefix := base
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	if !strings.HasPrefix(current, prefix) {
		return "", fmt.Errorf("path %s is outside %s", current, base)
	}
	return strings.TrimPrefix(current, prefix), nil
}
