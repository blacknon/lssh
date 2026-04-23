// Copyright (c) 2024 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package pshell

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/blacknon/go-sshlib"
	"github.com/blacknon/lssh/internal/output"
	lsync "github.com/blacknon/lssh/internal/sync"
	pkgsftp "github.com/pkg/sftp"
	"github.com/vbauerster/mpb"
	"golang.org/x/crypto/ssh"
)

var (
	pShellHelptext = `{{.Name}} - {{.Usage}}

	{{.HelpName}} {{if .VisibleFlags}}[options]{{end}} {{if .ArgsUsage}}{{.ArgsUsage}}{{end}}
	{{range .VisibleFlags}}	{{.}}
	{{end}}
	`
)

// TODO(blacknon): 以下のBuild-in Commandを追加する (v0.8.0)
//     - %cd <PATH>         ... リモートのディレクトリを変更する(事前のチェックにsftpを使用か？)
// TODO(blacknon): 以下のBuild-in Commandを追加する (v0.8.0)
//     - %lcd <PATH>        ... ローカルのディレクトリを変更する
// TODO(blacknon): 以下のBuild-in Commandを追加する
//     - %save <num> <PATH> ... 指定したnumの履歴をPATHに記録する (v0.8.0)
// TODO(blacknon): 以下のBuild-in Commandを追加する
//     - %set <args..>      ... 指定されたオプションを設定する(Optionsにて管理) (v0.80)
// TODO(blacknon): 任意のBuild-in Commandを追加できるようにする (v0.8.0)
//    - configにて、環境変数に過去のoutの出力をつけて任意のスクリプトを実行できるようにしてやることで、任意のスクリプト実行が可能に出来たら良くないか？というネタ
//    - もしくは、Goのモジュールとして機能追加できるようにするって方法もありかも？？

// checkBuildInCommand return true if cmd is build-in command.
func checkBuildInCommand(cmd string) (isBuildInCmd bool) {
	// check build-in command
	switch cmd {
	case "exit", "quit", "clear": // build-in command
		isBuildInCmd = true

	case
		"%history",
		"%out", "%outlist", "%outexec",
		"%get", "%put", "%sync",
		"%status", "%reconnect",
		"%save",
		"%set": // parsent build-in command.
		isBuildInCmd = true
	}

	return
}

// checkLocalCommand return bool, check is pshell build-in command or
// local machine command(%%command).
func checkLocalCommand(cmd string) (isLocalCmd bool) {
	return strings.HasPrefix(cmd, "+")
}

// check local or build-in command
func checkLocalBuildInCommand(cmd string) (result bool) {
	// check build-in command
	result = checkBuildInCommand(cmd)
	if result {
		return result
	}

	// check local command
	result = checkLocalCommand(cmd)

	return result
}

func normalizeLocalCommand(cmd string) string {
	if strings.HasPrefix(cmd, "++") {
		return strings.TrimPrefix(cmd, "++")
	}

	return strings.TrimPrefix(cmd, "+")
}

// runBuildInCommand is run buildin or local machine command.
func (s *shell) run(pline pipeLine, in *io.PipeReader, out *io.PipeWriter, ch chan<- bool, kill chan bool) (err error) {
	// get 1st element
	command := pline.Args[0]

	// check and exec build-in command
	switch command {
	// exit or quit
	case "exit", "quit":
		s.exit(0, "")
		return

	// clear
	case "clear":
		fmt.Printf("\033[H\033[2J")
		return

	// %history
	case "%history":
		s.buildin_history(out, ch)
		return

	// %outlist
	case "%outlist":
		s.buildin_outlist(out, ch)
		return

	// %out [num]
	case "%out":
		num := s.Count - 1
		if len(pline.Args) > 1 {
			switch pline.Args[1] {
			case "--help", "-h":
				_, _ = io.WriteString(setOutput(out), "%out [num]\n")
				ch <- true
				return
			}

			num, err = strconv.Atoi(pline.Args[1])
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: invalid history number: %s\n", pline.Args[1])
				ch <- true
				return
			}
		}

		s.buildin_out(num, out, ch)
		return

	// %outexec [num]
	case "%outexec":
		s.buildin_outexec(pline, in, out, ch, kill)
		return

	// %get remote local
	case "%get":
		s.buildin_get(pline.Args, out, ch)
		return

	// %put local remote
	case "%put":
		s.buildin_put(pline.Args, out, ch)
		return

	// %sync local:/path... remote:/path
	case "%sync":
		s.buildin_sync(pline.Args, out, ch)
		return
	case "%status":
		s.buildin_status(out, ch)
		return
	case "%reconnect":
		s.buildin_reconnect(pline.Args, out, ch)
		return
	}

	// check and exec local command
	switch {
	case checkLocalCommand(command):
		// exec local machine
		s.executeLocalPipeLine(pline, in, out, ch, kill, os.Environ())
	default:
		// exec remote machine
		s.executeRemotePipeLine(pline, in, out, ch, kill)
	}

	return
}

// localCmd_set is set pshll option.
// TODO(blacknon): Optionsの値などについて、あとから変更できるようにする。
// func (s *shell) buildin_set(args []string, out *io.PipeWriter, ch chan<- bool) {
// }

// localCmd_save is save HistoryResult results as a file local.
//     %save num PATH(独自の環境変数を利用して個別のファイルに保存できるようにする)
// TODO(blacknon): Optionsの値などについて、あとから変更できるようにする。
// func (s *shell) buildin_save(args []string, out *io.PipeWriter, ch chan<- bool) {
// }

func expandLocalPath(path string) []string {
	if path == "" {
		return nil
	}

	if path == "~" || strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			if path == "~" {
				path = home
			} else {
				path = filepath.Join(home, path[2:])
			}
		}
	}

	paths, err := filepath.Glob(path)
	if err != nil || len(paths) == 0 {
		return []string{path}
	}

	return paths
}

func expandRemotePath(client *pkgsftp.Client, path string) ([]string, error) {
	if path == "" {
		return nil, nil
	}

	dir, err := client.Getwd()
	if err != nil {
		return nil, err
	}

	switch {
	case path == "~":
		path = dir
	case strings.HasPrefix(path, "~/"):
		path = filepath.Join(dir, path[2:])
	case !filepath.IsAbs(path):
		path = filepath.Join(dir, path)
	}

	paths, err := client.Glob(path)
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		paths = append(paths, path)
	}

	return paths, nil
}

func (s *shell) openSFTPClient(conn *sConnect) (*pkgsftp.Client, func(), error) {
	if conn != nil && conn.Connector {
		return nil, func() {}, fmt.Errorf("sftp unavailable for connector-backed host %s", conn.Name)
	}
	if conn == nil || conn.Connect == nil {
		return nil, func() {}, fmt.Errorf("sftp unavailable")
	}
	if !conn.Connected {
		return nil, func() {}, fmt.Errorf("host %s is disconnected; use %%reconnect", conn.Name)
	}

	if conn.Connect.Client != nil {
		client, err := pkgsftp.NewClient(conn.Connect.Client)
		if err != nil {
			return nil, func() {}, err
		}

		return client, func() {
			client.Close()
		}, nil
	}

	if s.Run == nil {
		return nil, func() {}, fmt.Errorf("sftp unavailable")
	}

	direct, err := s.Run.CreateSshConnectDirect(conn.Name)
	if err != nil {
		return nil, func() {}, err
	}
	if direct == nil || direct.Client == nil {
		if direct != nil {
			_ = direct.Close()
		}
		return nil, func() {}, fmt.Errorf("ssh client is not available for sftp")
	}

	client, err := pkgsftp.NewClient(direct.Client)
	if err != nil {
		_ = direct.Close()
		return nil, func() {}, err
	}

	return client, func() {
		client.Close()
		_ = direct.Close()
	}, nil
}

func copyWithProgress(dst io.Writer, src io.Reader, size int64, progressOutput *output.Output, path string) error {
	if progressOutput == nil || progressOutput.Progress == nil || progressOutput.ProgressWG == nil {
		_, err := io.Copy(dst, src)
		return err
	}

	pr, pw := io.Pipe()
	progressOutput.ProgressWG.Add(1)
	go progressOutput.ProgressPrinter(size, pr, path)

	_, err := io.Copy(io.MultiWriter(dst, pw), src)
	closeErr := pw.Close()
	if err != nil {
		return err
	}

	return closeErr
}

func copyRemoteFile(client *pkgsftp.Client, remotePath, localPath string, progressOutput *output.Output) error {
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

	info, err := client.Stat(remotePath)
	if err != nil {
		return err
	}

	return copyWithProgress(dst, src, info.Size(), progressOutput, remotePath)
}

func copyRemotePath(client *pkgsftp.Client, remotePath, localBase string, forceDir bool, progressOutput *output.Output) error {
	info, err := client.Stat(remotePath)
	if err != nil {
		return err
	}

	if !info.IsDir() && !forceDir {
		return copyRemoteFile(client, remotePath, localBase, progressOutput)
	}

	base := filepath.Dir(remotePath)
	walker := client.Walk(remotePath)
	for walker.Step() {
		if err := walker.Err(); err != nil {
			return err
		}

		current := walker.Path()
		stat := walker.Stat()
		rel, err := filepath.Rel(base, current)
		if err != nil {
			return err
		}

		target := filepath.Join(localBase, rel)
		if stat.IsDir() {
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
			continue
		}

		if err := copyRemoteFile(client, current, target, progressOutput); err != nil {
			return err
		}
	}

	return nil
}

func resolveRemotePutPath(client *pkgsftp.Client, path string) ([]string, error) {
	if path == "" {
		return nil, nil
	}

	if strings.ContainsAny(path, "*?[") {
		return expandRemotePath(client, path)
	}

	dir, err := client.Getwd()
	if err != nil {
		return nil, err
	}

	switch {
	case path == "~":
		path = dir
	case strings.HasPrefix(path, "~/"):
		path = filepath.Join(dir, path[2:])
	case !filepath.IsAbs(path):
		path = filepath.Join(dir, path)
	}

	return []string{path}, nil
}

func copyLocalFile(client *pkgsftp.Client, localPath, remotePath string, progressOutput *output.Output) error {
	if err := client.MkdirAll(filepath.Dir(remotePath)); err != nil {
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

	info, err := os.Stat(localPath)
	if err != nil {
		return err
	}

	return copyWithProgress(dst, src, info.Size(), progressOutput, localPath)
}

func copyLocalPath(client *pkgsftp.Client, localPath, remoteBase string, forceDir bool, progressOutput *output.Output) error {
	info, err := os.Lstat(localPath)
	if err != nil {
		return err
	}

	if !info.IsDir() && !forceDir {
		return copyLocalFile(client, localPath, remoteBase, progressOutput)
	}

	base := filepath.Dir(localPath)
	return filepath.Walk(localPath, func(current string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		rel, err := filepath.Rel(base, current)
		if err != nil {
			return err
		}

		target := filepath.Join(remoteBase, rel)
		if info.IsDir() {
			return client.MkdirAll(target)
		}

		return copyLocalFile(client, current, target, progressOutput)
	})
}

func parseDryRunFlag(args []string) (bool, []string) {
	filtered := make([]string, 0, len(args))
	dryRun := false

	for _, arg := range args {
		if arg == "--dry-run" {
			dryRun = true
			continue
		}
		filtered = append(filtered, arg)
	}

	return dryRun, filtered
}

func (s *shell) buildin_status(out *io.PipeWriter, ch chan<- bool) {
	stdout := setOutput(out)
	s.checkKeepalive(true)
	for _, conn := range s.Connects {
		if conn == nil {
			continue
		}
		state := "connected"
		if !conn.Connected {
			state = "disconnected"
		}
		if conn.LastError != "" {
			fmt.Fprintf(stdout, "%s\t%s\t%s\n", conn.Name, state, conn.LastError)
		} else {
			fmt.Fprintf(stdout, "%s\t%s\n", conn.Name, state)
		}
	}
	if out != nil {
		out.CloseWithError(io.ErrClosedPipe)
	}
	ch <- true
}

func (s *shell) buildin_reconnect(args []string, out *io.PipeWriter, ch chan<- bool) {
	stdout := setOutput(out)
	targets := args[1:]
	if len(targets) == 0 {
		for _, conn := range s.Connects {
			if conn != nil && !conn.Connected {
				targets = append(targets, conn.Name)
			}
		}
	}

	if len(targets) == 0 {
		fmt.Fprintln(stdout, "No disconnected hosts.")
		if out != nil {
			out.CloseWithError(io.ErrClosedPipe)
		}
		ch <- true
		return
	}

	for _, name := range targets {
		if err := s.reconnect(name); err != nil {
			fmt.Fprintf(stdout, "%s\treconnect failed\t%s\n", name, err)
			continue
		}
		fmt.Fprintf(stdout, "%s\treconnected\n", name)
	}

	if out != nil {
		out.CloseWithError(io.ErrClosedPipe)
	}
	ch <- true
}

func (s *shell) buildin_get(args []string, out *io.PipeWriter, ch chan<- bool) {
	stdout := setOutput(out)
	progressWG := new(sync.WaitGroup)
	progress := mpb.New(mpb.WithWaitGroup(progressWG))
	defer func() {
		progress.Wait()
		switch stdout.(type) {
		case *io.PipeWriter:
			out.CloseWithError(io.ErrClosedPipe)
		}
		ch <- true
	}()

	dryRun, args := parseDryRunFlag(args)

	if len(args) != 3 {
		_, _ = io.WriteString(stdout, "%get [--dry-run] remote local\n")
		return
	}

	remotePath := args[1]
	destinationList := expandLocalPath(args[2])
	if len(destinationList) != 1 {
		fmt.Fprintf(stdout, "Error: invalid local path: %s\n", args[2])
		return
	}
	destination := destinationList[0]

	isMultiServer := len(s.Connects) > 1
	if isMultiServer {
		if stat, err := os.Stat(destination); err == nil && !stat.IsDir() {
			fmt.Fprintf(stdout, "Error: destination must be directory when getting from multiple servers: %s\n", destination)
			return
		}
		if err := os.MkdirAll(destination, 0755); err != nil {
			fmt.Fprintf(stdout, "Error: %s\n", err)
			return
		}
	}

	for _, conn := range s.Connects {
		conn.Output.Progress = progress
		conn.Output.ProgressWG = progressWG
		client, closeClient, err := s.openSFTPClient(conn)
		if err != nil {
			fmt.Fprintf(stdout, "Error: %s: %s\n", conn.Name, err)
			continue
		}

		func() {
			defer closeClient()

			remotePaths, err := expandRemotePath(client, remotePath)
			if err != nil {
				fmt.Fprintf(stdout, "Error: %s: %s\n", conn.Name, err)
				return
			}
			if len(remotePaths) == 0 {
				fmt.Fprintf(stdout, "Error: %s: file not found: %s\n", conn.Name, remotePath)
				return
			}

			targetBase := destination
			if isMultiServer {
				targetBase = filepath.Join(destination, conn.Name)
				if err := os.MkdirAll(targetBase, 0755); err != nil {
					fmt.Fprintf(stdout, "Error: %s: %s\n", conn.Name, err)
					return
				}
			}

			forceDir := len(remotePaths) > 1
			if !forceDir {
				if info, err := client.Stat(remotePaths[0]); err == nil && info.IsDir() {
					forceDir = true
				}
			}

			for _, path := range remotePaths {
				if dryRun {
					target := targetBase
					if forceDir {
						target = filepath.Join(targetBase, filepath.Base(path))
					}
					fmt.Fprintf(stdout, "[DRY-RUN] copy: %s:%s -> local:%s\n", conn.Name, path, target)
					continue
				}
				if err := copyRemotePath(client, path, targetBase, forceDir, conn.Output); err != nil {
					fmt.Fprintf(stdout, "Error: %s: %s\n", conn.Name, err)
					return
				}
			}
		}()
	}
}

func (s *shell) buildin_put(args []string, out *io.PipeWriter, ch chan<- bool) {
	stdout := setOutput(out)
	progressWG := new(sync.WaitGroup)
	progress := mpb.New(mpb.WithWaitGroup(progressWG))
	defer func() {
		progress.Wait()
		switch stdout.(type) {
		case *io.PipeWriter:
			out.CloseWithError(io.ErrClosedPipe)
		}
		ch <- true
	}()

	dryRun, args := parseDryRunFlag(args)

	connects, args, err := s.resolveTargetedConnects(args)
	if err != nil {
		fmt.Fprintf(stdout, "Error: %s\n", err)
		return
	}

	if len(args) < 3 {
		_, _ = io.WriteString(stdout, "%put [--dry-run] local... remote\n")
		return
	}

	sourcePaths := make([]string, 0, len(args)-2)
	for _, source := range args[1 : len(args)-1] {
		sourcePaths = append(sourcePaths, expandLocalPath(source)...)
	}
	if len(sourcePaths) == 0 {
		_, _ = io.WriteString(stdout, "Error: invalid local path\n")
		return
	}

	destination := args[len(args)-1]
	forceDir := len(sourcePaths) > 1

	for _, conn := range connects {
		conn.Output.Progress = progress
		conn.Output.ProgressWG = progressWG
		client, closeClient, err := s.openSFTPClient(conn)
		if err != nil {
			fmt.Fprintf(stdout, "Error: %s: %s\n", conn.Name, err)
			continue
		}

		func() {
			defer closeClient()

			targets, err := resolveRemotePutPath(client, destination)
			if err != nil {
				fmt.Fprintf(stdout, "Error: %s: %s\n", conn.Name, err)
				return
			}
			if len(targets) == 0 {
				fmt.Fprintf(stdout, "Error: %s: invalid remote path: %s\n", conn.Name, destination)
				return
			}

			for _, sourcePath := range sourcePaths {
				sourceInfo, err := os.Lstat(sourcePath)
				if err != nil {
					fmt.Fprintf(stdout, "Error: %s: %s\n", conn.Name, err)
					return
				}

				copyAsDir := forceDir || sourceInfo.IsDir()
				for _, target := range targets {
					if dryRun {
						fmt.Fprintf(stdout, "[DRY-RUN] copy: local:%s -> %s:%s\n", sourcePath, conn.Name, target)
						continue
					}

					if targetInfo, err := client.Lstat(target); err == nil && targetInfo.IsDir() {
						if err := copyLocalPath(client, sourcePath, target, true, conn.Output); err != nil {
							fmt.Fprintf(stdout, "Error: %s: %s\n", conn.Name, err)
							return
						}
						continue
					}

					if err := copyLocalPath(client, sourcePath, target, copyAsDir, conn.Output); err != nil {
						fmt.Fprintf(stdout, "Error: %s: %s\n", conn.Name, err)
						return
					}
				}
			}
		}()
	}
}

func (s *shell) buildin_sync(args []string, out *io.PipeWriter, ch chan<- bool) {
	stdout := setOutput(out)
	progressWG := new(sync.WaitGroup)
	progress := mpb.New(mpb.WithWaitGroup(progressWG))
	defer func() {
		progress.Wait()
		switch stdout.(type) {
		case *io.PipeWriter:
			out.CloseWithError(io.ErrClosedPipe)
		}
		ch <- true
	}()

	connects, args, err := s.resolveTargetedConnects(args)
	if err != nil {
		fmt.Fprintf(stdout, "Error: %s\n", err)
		return
	}

	parsed, err := lsync.ParseCommandArgs(args)
	if err != nil {
		fmt.Fprintf(stdout, "Error: %s\n", err)
		_, _ = io.WriteString(stdout, "%sync [--delete] [--dry-run] [-p] [-P num] (local|remote):source... (local|remote):target\n")
		return
	}

	sourceSpecs := make([]lsync.PathSpec, 0, len(parsed.Sources))
	isSourceRemote := false
	isSourceLocal := false
	for _, raw := range parsed.Sources {
		spec, err := lsync.ParsePathSpec(raw)
		if err != nil {
			fmt.Fprintf(stdout, "Error: %s\n", err)
			return
		}
		if spec.IsRemote {
			isSourceRemote = true
		} else {
			isSourceLocal = true
		}
		sourceSpecs = append(sourceSpecs, spec)
	}

	targetSpec, err := lsync.ParsePathSpec(parsed.Destination)
	if err != nil {
		fmt.Fprintf(stdout, "Error: %s\n", err)
		return
	}

	if isSourceRemote && isSourceLocal {
		fmt.Fprintf(stdout, "Error: can not mix LOCAL and REMOTE in source paths.\n")
		return
	}
	if !isSourceRemote && !targetSpec.IsRemote {
		fmt.Fprintf(stdout, "Error: LOCAL to LOCAL sync is not supported.\n")
		return
	}

	parallelNum := parsed.ParallelNum
	if parallelNum < 1 {
		parallelNum = 1
	}

	switch {
	case !isSourceRemote && targetSpec.IsRemote:
		if err := s.syncLocalToRemote(connects, sourceSpecs, targetSpec, parallelNum, parsed.Delete, parsed.Permission, parsed.DryRun, progress, progressWG); err != nil {
			fmt.Fprintf(stdout, "Error: %s\n", err)
		}
	case isSourceRemote && !targetSpec.IsRemote:
		if err := s.syncRemoteToLocal(connects, sourceSpecs, targetSpec, parallelNum, parsed.Delete, parsed.Permission, parsed.DryRun, progress, progressWG); err != nil {
			fmt.Fprintf(stdout, "Error: %s\n", err)
		}
	case isSourceRemote && targetSpec.IsRemote:
		if err := s.syncRemoteToRemote(connects, sourceSpecs, targetSpec, parallelNum, parsed.Delete, parsed.Permission, parsed.DryRun, progress, progressWG); err != nil {
			fmt.Fprintf(stdout, "Error: %s\n", err)
		}
	}
}

func (s *shell) syncSelectConnects(base []*sConnect, hosts []string) ([]*sConnect, error) {
	if len(hosts) == 0 {
		return base, nil
	}

	selected := make([]*sConnect, 0, len(hosts))
	for _, host := range hosts {
		found := false
		for _, conn := range base {
			if conn != nil && conn.Name == strings.TrimSpace(host) {
				selected = append(selected, conn)
				found = true
			}
		}
		if !found {
			return nil, fmt.Errorf("target server not found: %s", host)
		}
	}

	return selected, nil
}

func (s *shell) syncRemoteSourceGroups(base []*sConnect, specs []lsync.PathSpec) (map[string][]string, map[string]*sConnect, error) {
	pathsByServer := map[string][]string{}
	connByServer := map[string]*sConnect{}

	for _, spec := range specs {
		selected, err := s.syncSelectConnects(base, spec.Hosts)
		if err != nil {
			return nil, nil, err
		}
		for _, conn := range selected {
			pathsByServer[conn.Name] = append(pathsByServer[conn.Name], spec.Path)
			connByServer[conn.Name] = conn
		}
	}

	return pathsByServer, connByServer, nil
}

func (s *shell) syncLocalToRemote(base []*sConnect, sourceSpecs []lsync.PathSpec, targetSpec lsync.PathSpec, parallelNum int, deleteExtra, permission, dryRun bool, progress *mpb.Progress, progressWG *sync.WaitGroup) error {
	localFS, err := lsync.NewLocalFS()
	if err != nil {
		return err
	}

	targets, err := s.syncSelectConnects(base, targetSpec.Hosts)
	if err != nil {
		return err
	}

	sourcePaths := make([]string, 0, len(sourceSpecs))
	for _, spec := range sourceSpecs {
		sourcePaths = append(sourcePaths, spec.Path)
	}

	for _, conn := range targets {
		conn.Output.Progress = progress
		conn.Output.ProgressWG = progressWG
		client, closeClient, err := s.openSFTPClient(conn)
		if err != nil {
			return err
		}

		func() {
			defer closeClient()
			pwd, pwdErr := client.Getwd()
			if pwdErr != nil {
				pwd = "."
			}
			remoteFS := lsync.NewRemoteFS(client, pwd)
			plan, planErr := lsync.BuildPlan(localFS, remoteFS, sourcePaths, targetSpec.Path)
			if planErr != nil {
				err = planErr
				return
			}
			err = lsync.ApplyPlan(context.Background(), localFS, remoteFS, plan, lsync.ApplyOptions{
				Delete:      deleteExtra,
				DryRun:      dryRun,
				Permission:  permission,
				ParallelNum: parallelNum,
				Output:      conn.Output,
				SourceLabel: "local",
				TargetLabel: conn.Name,
			})
		}()
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *shell) syncRemoteToLocal(base []*sConnect, sourceSpecs []lsync.PathSpec, targetSpec lsync.PathSpec, parallelNum int, deleteExtra, permission, dryRun bool, progress *mpb.Progress, progressWG *sync.WaitGroup) error {
	localFS, err := lsync.NewLocalFS()
	if err != nil {
		return err
	}

	pathsByServer, connByServer, err := s.syncRemoteSourceGroups(base, sourceSpecs)
	if err != nil {
		return err
	}

	for server, paths := range pathsByServer {
		conn := connByServer[server]
		conn.Output.Progress = progress
		conn.Output.ProgressWG = progressWG
		client, closeClient, err := s.openSFTPClient(conn)
		if err != nil {
			return err
		}

		func() {
			defer closeClient()
			destination := targetSpec.Path
			if len(pathsByServer) > 1 {
				destination = localFS.Join(destination, server)
			}
			pwd, pwdErr := client.Getwd()
			if pwdErr != nil {
				pwd = "."
			}
			remoteFS := lsync.NewRemoteFS(client, pwd)
			plan, planErr := lsync.BuildPlan(remoteFS, localFS, paths, destination)
			if planErr != nil {
				err = planErr
				return
			}
			err = lsync.ApplyPlan(context.Background(), remoteFS, localFS, plan, lsync.ApplyOptions{
				Delete:      deleteExtra,
				DryRun:      dryRun,
				Permission:  permission,
				ParallelNum: parallelNum,
				Output:      conn.Output,
				SourceLabel: server,
				TargetLabel: "local",
			})
		}()
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *shell) syncRemoteToRemote(base []*sConnect, sourceSpecs []lsync.PathSpec, targetSpec lsync.PathSpec, parallelNum int, deleteExtra, permission, dryRun bool, progress *mpb.Progress, progressWG *sync.WaitGroup) error {
	pathsByServer, connByServer, err := s.syncRemoteSourceGroups(base, sourceSpecs)
	if err != nil {
		return err
	}
	if len(pathsByServer) != 1 {
		return fmt.Errorf("remote to remote sync requires source paths from a single host; use remote:@host:/path")
	}

	targets, err := s.syncSelectConnects(base, targetSpec.Hosts)
	if err != nil {
		return err
	}

	var sourceConn *sConnect
	var sourcePaths []string
	for server, paths := range pathsByServer {
		sourceConn = connByServer[server]
		sourcePaths = paths
		break
	}

	sourceConn.Output.Progress = progress
	sourceConn.Output.ProgressWG = progressWG
	sourceClient, closeSource, err := s.openSFTPClient(sourceConn)
	if err != nil {
		return err
	}
	defer closeSource()

	sourcePwd, pwdErr := sourceClient.Getwd()
	if pwdErr != nil {
		sourcePwd = "."
	}
	sourceFS := lsync.NewRemoteFS(sourceClient, sourcePwd)
	for _, conn := range targets {
		conn.Output.Progress = progress
		conn.Output.ProgressWG = progressWG
		targetClient, closeTarget, err := s.openSFTPClient(conn)
		if err != nil {
			return err
		}

		func() {
			defer closeTarget()
			targetPwd, targetPwdErr := targetClient.Getwd()
			if targetPwdErr != nil {
				targetPwd = "."
			}
			targetFS := lsync.NewRemoteFS(targetClient, targetPwd)
			plan, planErr := lsync.BuildPlan(sourceFS, targetFS, sourcePaths, targetSpec.Path)
			if planErr != nil {
				err = planErr
				return
			}
			err = lsync.ApplyPlan(context.Background(), sourceFS, targetFS, plan, lsync.ApplyOptions{
				Delete:      deleteExtra,
				DryRun:      dryRun,
				Permission:  permission,
				ParallelNum: parallelNum,
				Output:      conn.Output,
				SourceLabel: sourceConn.Name,
				TargetLabel: conn.Name,
			})
		}()
		if err != nil {
			return err
		}
	}

	return nil
}

// localCmd_history is printout history (shell history)
func (s *shell) buildin_history(out *io.PipeWriter, ch chan<- bool) {
	stdout := setOutput(out)

	// read history file
	data, err := s.GetHistoryFromFile()
	if err != nil {
		return
	}

	// print out history
	for _, h := range data {
		fmt.Fprintf(stdout, "%s: %s\n", h.Timestamp, h.Command)
	}

	// close out
	switch stdout.(type) {
	case *io.PipeWriter:
		out.CloseWithError(io.ErrClosedPipe)
	}

	// send exit
	ch <- true
}

// localcmd_outlist is print exec history list.
func (s *shell) buildin_outlist(out *io.PipeWriter, ch chan<- bool) {
	stdout := setOutput(out)

	for i := 0; i < len(s.History); i++ {
		h := s.History[i]
		for _, hh := range h {
			fmt.Fprintf(stdout, "%3d : %s\n", i, hh.Command)
			break
		}
	}

	// close out
	switch stdout.(type) {
	case *io.PipeWriter:
		out.CloseWithError(io.ErrClosedPipe)
	}

	// send exit
	ch <- true
}

// localCmd_out is print exec history at number
// example:
//   - %out
//   - %out <num>
func (s *shell) buildin_out(num int, out *io.PipeWriter, ch chan<- bool) {
	stdout := setOutput(out)
	histories := s.History[num]

	// get key
	keys := []string{}
	for k := range histories {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	i := 0
	for _, k := range keys {
		h := histories[k]

		// if first, print out command
		if i == 0 {
			fmt.Fprintf(os.Stderr, "[History:%s ]\n", h.Command)
		}
		i += 1

		// print out result
		if len(histories) > 1 && stdout == os.Stdout && h.Output != nil {
			// set Output.Count
			bc := h.Output.Count
			h.Output.Count = num
			op := h.Output.GetPrompt()

			// TODO(blacknon): Outputを利用させてOPROMPTを生成
			sc := bufio.NewScanner(strings.NewReader(h.Result))
			for sc.Scan() {
				fmt.Fprintf(stdout, "%s %s\n", op, sc.Text())
			}

			// reset Output.Count
			h.Output.Count = bc
		} else {
			fmt.Fprint(stdout, h.Result)
		}
	}

	// close out
	switch stdout.(type) {
	case *io.PipeWriter:
		out.CloseWithError(io.ErrClosedPipe)
	}

	// send exit
	ch <- true
}

// executePipeLineRemote is exec command in remote machine.
// Didn't know how to send data from Writer to Channel, so switch the function if * io.PipeWriter is Nil.
func (s *shell) executeRemotePipeLine(pline pipeLine, in *io.PipeReader, out *io.PipeWriter, ch chan<- bool, kill chan bool) {
	connects, args, err := s.resolveTargetedConnects(pline.Args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		if out != nil {
			out.CloseWithError(io.ErrClosedPipe)
		}
		ch <- true
		return
	}

	// join command
	command := strings.Join(args, " ")

	// set stdin/stdout
	stdin := setInput(in)
	stdout := setOutput(out)
	if in == nil && out != nil {
		stdin = io.NopCloser(strings.NewReader(""))
	}
	defer func() {
		if in != nil {
			_ = in.Close()
		}
	}()

	// create channels
	exit := make(chan bool)
	exitInput := make(chan bool) // Input finish channel
	exitOutput := make(chan bool)

	// create []io.WriteCloser for multi-stdin fanout
	var writers []io.WriteCloser
	var controlWriters []io.WriteCloser

	// create []ssh.Session (direct connections only)
	var sessions []*ssh.Session

	// runCount tracks total goroutines writing to exit channel
	runCount := 0

	active := make([]*sConnect, 0, len(connects))
	for _, c := range connects {
		if c == nil {
			continue
		}
		if !c.Connected {
			fmt.Fprintf(os.Stderr, "%s is disconnected. Use %%reconnect.\n", c.Name)
			continue
		}
		active = append(active, c)
	}
	connects = active
	if len(connects) == 0 {
		if out != nil {
			out.CloseWithError(io.ErrClosedPipe)
		}
		ch <- true
		return
	}
	if in != nil {
		for _, c := range connects {
			if c != nil && c.Connector {
				fmt.Fprintln(os.Stderr, "connector-backed hosts do not support piped stdin in lsshell yet")
				if out != nil {
					out.CloseWithError(io.ErrClosedPipe)
				}
				ch <- true
				return
			}
		}
	}

	for _, c := range connects {
		if c == nil {
			continue
		}

		// Build output writer for this connection
		var ow io.Writer
		ow = stdout
		if ow == os.Stdout {
			// create Output Writer
			c.Output.Count = s.Count
			w := c.Output.NewWriter()
			defer w.CloseWithError(io.ErrClosedPipe)

			// create pShellHistory Writer
			hw := s.NewHistoryWriter(c.Output.Server, c.Output)
			defer hw.CloseWithError(io.ErrClosedPipe)

			ow = io.MultiWriter(w, hw)
		}

		if c.Connector {
			runCount++
			go func(conn *sConnect, outputWriter io.Writer, commandArgs []string) {
				defer func() {
					exit <- true
					if stdout == os.Stdout {
						exitOutput <- true
					}
				}()

				if len(commandArgs) == 0 {
					_, _ = io.WriteString(outputWriter, "connector execution requires a command\n")
					return
				}
				if _, err := s.Run.RunConnectorCommand(conn.Name, append([]string(nil), commandArgs...), nil, outputWriter, outputWriter); err != nil {
					_, _ = fmt.Fprintf(outputWriter, "%s\n", err)
					return
				}
			}(c, ow, args)
			continue
		}
		if c.Connect == nil {
			continue
		}

		stdinR, stdinW := io.Pipe()
		writers = append(writers, stdinW)

		clone := *c.Connect
		clone.Stdin = stdinR
		clone.Stdout = ow
		clone.Stderr = os.Stderr
		clone.TTY = stdin == os.Stdin && stdout == os.Stdout

		if clone.IsControlClient() {
			controlWriters = append(controlWriters, stdinW)
		} else {
			if c.Connect.Client == nil {
				stdinR.CloseWithError(io.ErrClosedPipe)
				stdinW.CloseWithError(io.ErrClosedPipe)
				continue
			}

			session, err := safeCreateSession(c)
			if err != nil {
				stdinR.CloseWithError(io.ErrClosedPipe)
				stdinW.CloseWithError(io.ErrClosedPipe)
				continue
			}

			clone.Session = session
			sessions = append(sessions, session)
		}

		runCount++
		go func(conn sshlib.Connect, r *io.PipeReader) {
			conn.Command(command)
			r.CloseWithError(io.ErrClosedPipe)
			exit <- true
			if stdout == os.Stdout {
				exitOutput <- true
			}
		}(clone, stdinR)
	}

	// multi input-writer
	go output.PushInput(exitInput, writers, stdin)

	// kill
	go func() {
		select {
		case <-kill:
			for _, w := range controlWriters {
				_, _ = w.Write([]byte{3})
			}
			for _, sess := range sessions {
				sess.Signal(ssh.SIGINT)
				sess.Close()
			}
		}
	}()

	// wait
	s.wait(runCount, exit)

	// wait time (0.050 sec)
	time.Sleep(500 * time.Millisecond)

	// send exit
	ch <- true

	// exit input.
	if stdin == os.Stdin {
		exitInput <- true
	}

	// close out
	switch stdout.(type) {
	case *io.PipeWriter:
		out.CloseWithError(io.ErrClosedPipe)
	}

	// wait time (0.050 sec)
	time.Sleep(500 * time.Millisecond)

	return
}

// executePipeLineLocal is exec command in local machine.
// TODO(blacknon): 利用中のShellでの実行+functionや環境変数、aliasの引き継ぎを行えるように実装
func (s *shell) executeLocalPipeLine(pline pipeLine, in *io.PipeReader, out *io.PipeWriter, ch chan<- bool, kill chan bool, envrionment []string) (err error) {
	// set stdin/stdout
	stdin := setInput(in)
	stdout := setOutput(out)
	useTerminalIO := in == nil && out == nil && stdin == os.Stdin && stdout == os.Stdout
	if in == nil && out != nil {
		stdin = io.NopCloser(strings.NewReader(""))
	}
	defer func() {
		if in != nil {
			_ = in.Close()
		}
	}()

	// set HistoryResult
	var stdoutw io.Writer
	stdoutw = stdout
	if stdout == os.Stdout && !useTerminalIO {
		pw := s.NewHistoryWriter("localhost", nil)
		defer pw.CloseWithError(io.ErrClosedPipe)
		stdoutw = io.MultiWriter(pw, stdout)
	} else {
		stdoutw = stdout
	}

	// delete local command prefix (`+` or `++`)
	pline.Args[0] = normalizeLocalCommand(pline.Args[0])

	// join command
	command := strings.Join(pline.Args, " ")
	command, cleanup, err := s.expandLocalProcessSubstitutions(command)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		ch <- true
		return err
	}
	defer cleanup()

	// execute command
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("powershell.exe", "-c", command)
	} else {
		cmd = exec.Command("sh", "-c", command)
	}

	// set stdin, stdout, stderr
	cmd.Stdin = stdin
	if useTerminalIO || s.Options.LocalCommandNotRecordResult {
		cmd.Stdout = stdout
	} else { // default
		cmd.Stdout = stdoutw
	}
	cmd.Stderr = os.Stderr

	// set envrionment
	cmd.Env = envrionment

	// run command
	err = cmd.Start()

	// get signal and kill
	p := cmd.Process
	go func() {
		select {
		case <-kill:
			p.Kill()
		}
	}()

	// wait command
	cmd.Wait()

	// close out, or write pShellHistory
	switch stdout.(type) {
	case *io.PipeWriter:
		out.CloseWithError(io.ErrClosedPipe)
	}

	// send exit
	ch <- true

	return
}

// s.wait
func (s *shell) wait(num int, ch <-chan bool) {
	for i := 0; i < num; i++ {
		<-ch
	}
}

// setInput
func setInput(in io.ReadCloser) (stdin io.ReadCloser) {
	if reflect.ValueOf(in).IsNil() {
		stdin = os.Stdin
	} else {
		stdin = in
	}

	return
}

// setOutput
func setOutput(out io.WriteCloser) (stdout io.WriteCloser) {
	if reflect.ValueOf(out).IsNil() {
		stdout = os.Stdout
	} else {
		stdout = out
	}

	return
}
