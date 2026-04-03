// Copyright (c) 2022 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package sftp

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/blacknon/lssh/internal/common"
	"github.com/urfave/cli"
)

// copy - lsftp build-in command: copy
//
//	copy @source_host:/path... @target_host:/path
//
// TODO(blacknon): 転送時の進捗状況を表示するプログレスバーの表示はさせること
func (r *RunSftp) copy(args []string) {
	app := cli.NewApp()

	app.CustomAppHelpTemplate = helptext
	app.Name = "copy"
	app.Usage = "lsftp build-in command: copy"
	app.ArgsUsage = "@source_host:/path... @target_host:/path"
	app.HideHelp = true
	app.HideVersion = true
	app.EnableBashCompletion = true

	app.Action = func(c *cli.Context) error {
		if len(c.Args()) < 2 {
			fmt.Println("Requires over two arguments")
			fmt.Println("copy @source_host:/path... @target_host:/path")
			return nil
		}

		argsSize := len(c.Args()) - 1
		sourceArgs := c.Args()[:argsSize]
		targetArg := c.Args()[argsSize]

		sources := []remoteCopySource{}
		for _, srcArg := range sourceArgs {
			srcList, err := r.parseRemoteCopySource(srcArg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %s\n", err)
				return nil
			}
			sources = append(sources, srcList...)
		}
		if len(sources) == 0 {
			fmt.Fprintln(os.Stderr, "Error: source host not found.")
			return nil
		}

		targets, targetPath, err := r.parseRemoteCopyTarget(targetArg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			return nil
		}
		if len(targets) == 0 {
			fmt.Fprintln(os.Stderr, "Error: target host not found.")
			return nil
		}

		targetIsDir := len(sources) > 1
		for _, src := range sources {
			for _, target := range targets {
				if err := r.copyRemoteToRemote(src, target, targetPath, targetIsDir); err != nil {
					fmt.Fprintf(os.Stderr, "Error: %s\n", err)
				}
			}
		}

		return nil
	}

	args = common.ParseArgs(app.Flags, args)
	app.Run(args)
}

type remoteCopySource struct {
	Host string
	Path string
}

func (r *RunSftp) parseRemoteCopySource(value string) ([]remoteCopySource, error) {
	hosts, path := common.ParseHostPath(strings.TrimPrefix(value, "@"))
	if len(hosts) == 0 || path == "" {
		return nil, fmt.Errorf("source must be '@host:/path' format")
	}

	result := []remoteCopySource{}
	for _, host := range hosts {
		host = strings.TrimPrefix(strings.TrimSpace(host), "@")
		if _, ok := r.Client[host]; !ok {
			return nil, fmt.Errorf("host %s not found", host)
		}
		result = append(result, remoteCopySource{Host: host, Path: path})
	}

	return result, nil
}

func (r *RunSftp) parseRemoteCopyTarget(value string) ([]*TargetConnectMap, string, error) {
	hosts, path := common.ParseHostPath(strings.TrimPrefix(value, "@"))
	if len(hosts) == 0 || path == "" {
		return nil, "", fmt.Errorf("target must be '@host:/path' format")
	}

	targets := []*TargetConnectMap{}
	for _, host := range hosts {
		host = strings.TrimPrefix(strings.TrimSpace(host), "@")
		client, ok := r.Client[host]
		if !ok {
			return nil, "", fmt.Errorf("host %s not found", host)
		}

		target := &TargetConnectMap{}
		target.SftpConnect = *client
		target.Path = []string{path}
		targets = append(targets, target)
	}

	return targets, path, nil
}

func (r *RunSftp) copyRemoteToRemote(src remoteCopySource, dst *TargetConnectMap, dstPath string, targetIsDir bool) error {
	srcClient, ok := r.Client[src.Host]
	if !ok {
		return fmt.Errorf("host %s not found", src.Host)
	}

	sourceTarget := &TargetConnectMap{}
	sourceTarget.SftpConnect = *srcClient
	sourceTarget.Path = []string{src.Path}

	sourcePaths, err := ExpandRemotePath(sourceTarget, src.Path)
	if err != nil {
		return fmt.Errorf("%s:%s: %w", src.Host, src.Path, err)
	}
	if len(sourcePaths) == 0 {
		return fmt.Errorf("%s:%s: file not found", src.Host, src.Path)
	}

	if len(sourcePaths) > 1 {
		targetIsDir = true
	}

	for _, sourcePath := range sourcePaths {
		info, err := srcClient.Connect.Stat(sourcePath)
		if err != nil {
			return fmt.Errorf("%s:%s: %w", src.Host, sourcePath, err)
		}

		targetPath := dstPath
		if targetIsDir || info.IsDir() {
			targetPath = filepath.Join(dstPath, filepath.Base(sourcePath))
		}

		if info.IsDir() {
			if err := r.copyRemoteDirToRemote(srcClient, dst, sourcePath, targetPath); err != nil {
				return err
			}
			continue
		}

		if err := r.copyRemoteFileToRemote(srcClient, dst, sourcePath, targetPath, info.Mode()); err != nil {
			return err
		}
	}

	return nil
}

func (r *RunSftp) copyRemoteDirToRemote(srcClient *SftpConnect, dst *TargetConnectMap, sourcePath, targetPath string) error {
	walker := srcClient.Connect.Walk(sourcePath)
	baseDir := filepath.Dir(sourcePath)

	for walker.Step() {
		if err := walker.Err(); err != nil {
			return err
		}

		relPath, err := filepath.Rel(baseDir, walker.Path())
		if err != nil {
			return err
		}
		dstCurrentPath := filepath.Join(targetPath, strings.TrimPrefix(relPath, filepath.Base(sourcePath)))
		if relPath == filepath.Base(sourcePath) {
			dstCurrentPath = targetPath
		}

		if walker.Stat().IsDir() {
			if err := dst.Connect.MkdirAll(dstCurrentPath); err != nil {
				return err
			}
			if r.Permission {
				if err := dst.Connect.Chmod(dstCurrentPath, walker.Stat().Mode()); err != nil {
					return err
				}
			}
			continue
		}

		if err := r.copyRemoteFileToRemote(srcClient, dst, walker.Path(), dstCurrentPath, walker.Stat().Mode()); err != nil {
			return err
		}
	}

	return nil
}

func (r *RunSftp) copyRemoteFileToRemote(srcClient *SftpConnect, dst *TargetConnectMap, sourcePath, targetPath string, mode os.FileMode) error {
	srcFile, err := srcClient.Connect.Open(sourcePath)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	if err := dst.Connect.MkdirAll(filepath.Dir(targetPath)); err != nil {
		return err
	}

	dstFile, err := dst.Connect.OpenFile(targetPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	if r.Permission {
		if err := dst.Connect.Chmod(targetPath, mode); err != nil {
			return err
		}
	}

	return nil
}
