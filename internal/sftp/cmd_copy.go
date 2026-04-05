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
	"sync"
	"time"

	"github.com/blacknon/lssh/internal/common"
	"github.com/blacknon/lssh/internal/output"
	"github.com/urfave/cli"
	"github.com/vbauerster/mpb"
)

// copy - lsftp build-in command: copy
//
//	copy @source_host:/path... @target_host:/path
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

		r.ProgressWG = new(sync.WaitGroup)
		r.Progress = mpb.New(mpb.WithWaitGroup(r.ProgressWG))

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

		newWorkerOutput := func(base *output.Output, server string) *output.Output {
			if base == nil {
				return nil
			}

			o := &output.Output{
				Templete:      base.Templete,
				ServerList:    append([]string(nil), base.ServerList...),
				Conf:          base.Conf,
				Progress:      r.Progress,
				ProgressWG:    r.ProgressWG,
				EnableHeader:  base.EnableHeader,
				DisableHeader: base.DisableHeader,
				AutoColor:     base.AutoColor,
			}
			o.Create(server)

			return o
		}

		for _, target := range targets {
			serverName := ""
			for name, client := range r.Client {
				if client.Connect == target.Connect {
					serverName = name
					break
				}
			}
			target.Output = newWorkerOutput(target.Output, serverName)
		}

		targetIsDir := len(sources) > 1
		exit := make(chan bool, len(targets))
		for _, target := range targets {
			targetClient := target
			go func() {
				for _, src := range sources {
					if err := r.copyRemoteToRemote(src, targetClient, targetPath, targetIsDir); err != nil {
						fmt.Fprintf(os.Stderr, "Error: %s\n", err)
					}
				}
				exit <- true
			}()
		}

		for i := 0; i < len(targets); i++ {
			<-exit
		}
		close(exit)

		r.Progress.Wait()
		time.Sleep(300 * time.Millisecond)

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

	resolvedDstPath, err := resolveRemoteCopyPath(dst, dstPath)
	if err != nil {
		return fmt.Errorf("target %s: %w", dstPath, err)
	}

	targetIsDir, err = r.isRemoteCopyTargetDir(dst, dstPath, resolvedDstPath, targetIsDir)
	if err != nil {
		return fmt.Errorf("target %s: %w", resolvedDstPath, err)
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

		targetPath := resolvedDstPath
		if targetIsDir || info.IsDir() {
			targetPath = filepath.Join(resolvedDstPath, filepath.Base(sourcePath))
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

func resolveRemoteCopyPath(client *TargetConnectMap, path string) (string, error) {
	switch {
	case path == "~":
		return client.Connect.Getwd()
	case strings.HasPrefix(path, "~/"):
		home, err := client.Connect.Getwd()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, path[2:]), nil
	case !filepath.IsAbs(path):
		return filepath.Join(client.Pwd, path), nil
	default:
		return path, nil
	}
}

func (r *RunSftp) isRemoteCopyTargetDir(dst *TargetConnectMap, rawPath, resolvedPath string, defaultIsDir bool) (bool, error) {
	if defaultIsDir || strings.HasSuffix(rawPath, "/") {
		return true, nil
	}

	stat, err := dst.Connect.Stat(resolvedPath)
	if err == nil {
		return stat.IsDir(), nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}

	// Missing path and SSH_FX_NO_SUCH_FILE are both fine here: in that case
	// the caller is creating a new destination file or directory.
	if strings.Contains(strings.ToLower(err.Error()), "no such file") {
		return false, nil
	}

	return false, err
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

	var size int64
	if stat, err := srcClient.Connect.Stat(sourcePath); err == nil {
		size = stat.Size()
	}

	rd := io.TeeReader(srcFile, dstFile)
	r.ProgressWG.Add(1)
	dst.Output.ProgressPrinter(size, rd, sourcePath)

	if r.Permission {
		if err := dst.Connect.Chmod(targetPath, mode); err != nil {
			return err
		}
	}

	return nil
}
