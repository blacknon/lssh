package sync

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
	syncpkg "sync"
	"time"

	"github.com/blacknon/lssh/internal/output"
)

type ApplyOptions struct {
	Delete      bool
	Permission  bool
	ParallelNum int
	Output      *output.Output
}

func ApplyPlan(ctx context.Context, srcFS FileSystem, dstFS FileSystem, plan *Plan, options ApplyOptions) error {
	if options.ParallelNum < 1 {
		options.ParallelNum = 1
	}

	for _, directory := range sortedDesiredDirectories(plan) {
		if err := ensureDirectory(dstFS, directory, options.Permission); err != nil {
			return err
		}
	}

	copyTargets := []DesiredEntry{}
	for _, file := range sortedDesiredFiles(plan) {
		needsCopy, err := fileNeedsCopy(dstFS, file)
		if err != nil {
			return err
		}
		if needsCopy {
			copyTargets = append(copyTargets, file)
			continue
		}

		if options.Permission {
			_ = dstFS.Chmod(file.DestinationPath, file.Mode)
		}
	}

	if err := copyFiles(ctx, srcFS, dstFS, copyTargets, options); err != nil {
		return err
	}

	if options.Delete {
		if err := deleteExtraPaths(dstFS, plan, options.Output); err != nil {
			return err
		}
	}

	if options.Permission {
		for _, directory := range sortedDesiredDirectories(plan) {
			_ = dstFS.Chmod(directory.DestinationPath, directory.Mode)
		}
	}

	return nil
}

func ensureDirectory(dstFS FileSystem, directory DesiredEntry, permission bool) error {
	info, err := dstFS.Stat(directory.DestinationPath)
	if err == nil && !info.IsDir() {
		if err := removePathRecursive(dstFS, directory.DestinationPath); err != nil {
			return err
		}
	}
	if err != nil && !isNotExistErr(err) {
		return err
	}

	if err := dstFS.MkdirAll(directory.DestinationPath); err != nil {
		return err
	}

	if permission {
		_ = dstFS.Chmod(directory.DestinationPath, directory.Mode)
	}

	return nil
}

func fileNeedsCopy(dstFS FileSystem, file DesiredEntry) (bool, error) {
	info, err := dstFS.Stat(file.DestinationPath)
	switch {
	case err == nil && info.IsDir():
		return true, nil
	case err == nil:
		return info.Size() != file.Size || !sameTimestamp(info.ModTime(), file.ModTime), nil
	case isNotExistErr(err):
		return true, nil
	default:
		return false, err
	}
}

func copyFiles(ctx context.Context, srcFS FileSystem, dstFS FileSystem, files []DesiredEntry, options ApplyOptions) error {
	if len(files) == 0 {
		return nil
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	tasks := make(chan DesiredEntry)
	results := make(chan error, options.ParallelNum)
	var wg syncpkg.WaitGroup

	for i := 0; i < options.ParallelNum; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for file := range tasks {
				select {
				case <-ctx.Done():
					return
				default:
				}

				if err := copySingleFile(srcFS, dstFS, file, options); err != nil {
					select {
					case results <- err:
					default:
					}
					cancel()
					return
				}
			}
		}()
	}

	go func() {
		for _, file := range files {
			select {
			case <-ctx.Done():
				close(tasks)
				wg.Wait()
				close(results)
				return
			case tasks <- file:
			}
		}
		close(tasks)
		wg.Wait()
		close(results)
	}()

	for err := range results {
		if err != nil {
			return err
		}
	}

	return nil
}

func copySingleFile(srcFS FileSystem, dstFS FileSystem, file DesiredEntry, options ApplyOptions) error {
	info, err := dstFS.Stat(file.DestinationPath)
	if err == nil && info.IsDir() {
		if err := removePathRecursive(dstFS, file.DestinationPath); err != nil {
			return err
		}
	} else if err != nil && !isNotExistErr(err) {
		return err
	}

	if err := dstFS.MkdirAll(dstFS.Dir(file.DestinationPath)); err != nil {
		return err
	}

	srcFile, err := srcFS.Open(file.SourcePath)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := dstFS.OpenWriter(file.DestinationPath, 0644)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	transferLabel := formatTransferLabel(file.SourcePath, file.DestinationPath)
	if options.Output != nil {
		ow := options.Output.NewWriter()
		fmt.Fprintf(ow, "copy: %s\n", transferLabel)
		ow.Close()

		reader := io.TeeReader(srcFile, dstFile)
		options.Output.ProgressWG.Add(1)
		options.Output.ProgressPrinter(file.Size, reader, transferLabel)
	} else {
		if _, err := io.Copy(dstFile, srcFile); err != nil {
			return err
		}
	}

	if options.Permission {
		_ = dstFS.Chmod(file.DestinationPath, file.Mode)
	}
	_ = dstFS.Chtimes(file.DestinationPath, file.ModTime, file.ModTime)

	return nil
}

func formatTransferLabel(sourcePath, destinationPath string) string {
	return fmt.Sprintf("%s -> %s", sourcePath, destinationPath)
}

func deleteExtraPaths(dstFS FileSystem, plan *Plan, printer *output.Output) error {
	for _, scope := range plan.DeleteScopes {
		if !scope.IsDir {
			continue
		}

		info, err := dstFS.Stat(scope.Path)
		if err != nil {
			if isNotExistErr(err) {
				continue
			}
			return err
		}
		if !info.IsDir() {
			continue
		}

		existing := []string{}
		if err := dstFS.Walk(scope.Path, func(path string, info fs.FileInfo) error {
			if path == scope.Path {
				return nil
			}
			existing = append(existing, path)
			return nil
		}); err != nil {
			return err
		}

		for _, path := range pathsToDelete(scope, existing, plan.Desired, dstFS.Clean, dstFS.Dir, dstFS.Separator()) {
			if err := removePathRecursive(dstFS, path); err != nil {
				return err
			}
			if printer != nil {
				ow := printer.NewWriter()
				fmt.Fprintf(ow, "delete: %s\n", path)
				ow.Close()
			}
		}
	}

	return nil
}

func removePathRecursive(filesystem FileSystem, path string) error {
	info, err := filesystem.Stat(path)
	if err != nil {
		if isNotExistErr(err) {
			return nil
		}
		return err
	}

	if !info.IsDir() {
		return filesystem.Remove(path)
	}

	children := []string{}
	if err := filesystem.Walk(path, func(path string, info fs.FileInfo) error {
		children = append(children, path)
		return nil
	}); err != nil {
		return err
	}

	for i := len(children) - 1; i >= 0; i-- {
		current := children[i]
		childInfo, err := filesystem.Stat(current)
		if err != nil {
			if isNotExistErr(err) {
				continue
			}
			return err
		}

		if childInfo.IsDir() {
			if err := filesystem.RemoveDir(current); err != nil && !isNotExistErr(err) {
				return err
			}
			continue
		}

		if err := filesystem.Remove(current); err != nil && !isNotExistErr(err) {
			return err
		}
	}

	return nil
}

func sameTimestamp(left, right time.Time) bool {
	return left.Unix() == right.Unix()
}

func isNotExistErr(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, fs.ErrNotExist) || os.IsNotExist(err) {
		return true
	}

	return strings.Contains(strings.ToLower(err.Error()), "no such file")
}
