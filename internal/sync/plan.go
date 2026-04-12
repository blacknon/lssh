package sync

import (
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"
)

type DesiredEntry struct {
	SourcePath      string
	DestinationPath string
	RelPath         string
	Mode            fs.FileMode
	Size            int64
	ModTime         time.Time
	IsDir           bool
}

type DeleteScope struct {
	Path  string
	IsDir bool
}

type Plan struct {
	Desired      map[string]DesiredEntry
	DeleteScopes []DeleteScope
}

func BuildPlan(srcFS FileSystem, dstFS FileSystem, sourceArgs []string, destination string) (*Plan, error) {
	type sourceRoot struct {
		Path  string
		IsDir bool
	}

	expandedRoots := []sourceRoot{}
	for _, raw := range sourceArgs {
		paths, err := srcFS.Expand(raw)
		if err != nil {
			return nil, err
		}

		for _, current := range paths {
			info, err := srcFS.Stat(current)
			if err != nil {
				return nil, err
			}

			expandedRoots = append(expandedRoots, sourceRoot{
				Path:  current,
				IsDir: info.IsDir(),
			})
		}
	}

	if len(expandedRoots) == 0 {
		return nil, fmt.Errorf("no source paths to sync")
	}

	resolvedDestination, err := dstFS.Resolve(destination)
	if err != nil {
		return nil, err
	}

	destinationInfo, err := dstFS.Stat(resolvedDestination)
	targetExistsAsDir := err == nil && destinationInfo.IsDir()
	if err != nil && !isNotExistErr(err) {
		return nil, err
	}

	plan := &Plan{
		Desired: map[string]DesiredEntry{},
	}

	multipleSources := len(expandedRoots) > 1
	if multipleSources {
		plan.DeleteScopes = append(plan.DeleteScopes, DeleteScope{
			Path:  resolvedDestination,
			IsDir: true,
		})
	}

	for _, root := range expandedRoots {
		preserveSourceName := shouldPreserveSourceName(root.Path, root.IsDir, destination, resolvedDestination, targetExistsAsDir, multipleSources, dstFS.Separator())
		base := copySourceBase(root.Path, root.IsDir, preserveSourceName, srcFS.Clean, srcFS.Dir)
		treatDestinationAsDir := shouldTreatDestinationAsDir(destination, targetExistsAsDir, root.IsDir, multipleSources, dstFS.Separator())
		rootDestinationPath := ""

		err := srcFS.Walk(root.Path, func(current string, info fs.FileInfo) error {
			relpath, err := relativePath(srcFS, base, current)
			if err != nil {
				return err
			}

			destinationPath := resolveDestinationPath(resolvedDestination, relpath, treatDestinationAsDir, dstFS.Clean, dstFS.Join)
			if current == root.Path {
				rootDestinationPath = destinationPath
			}

			plan.Desired[destinationPath] = DesiredEntry{
				SourcePath:      current,
				DestinationPath: destinationPath,
				RelPath:         relpath,
				Mode:            info.Mode(),
				Size:            info.Size(),
				ModTime:         info.ModTime(),
				IsDir:           info.IsDir(),
			}

			return nil
		})
		if err != nil {
			return nil, err
		}

		if !multipleSources {
			plan.DeleteScopes = append(plan.DeleteScopes, DeleteScope{
				Path:  rootDestinationPath,
				IsDir: root.IsDir,
			})
		}
	}

	return plan, nil
}

func shouldPreserveSourceName(sourcePath string, sourceIsDir bool, rawDestination string, resolvedDestination string, targetExistsAsDir bool, multipleSources bool, separator string) bool {
	if multipleSources {
		return true
	}

	if !sourceIsDir {
		return targetExistsAsDir || hasTrailingSlash(rawDestination, separator)
	}

	// For sync, a single directory source is treated as "sync the contents into
	// the destination root", not "nest the source directory under destination".
	// This keeps repeated syncs stable for existing targets like /tmp/hoge2.
	return false
}

func relativePath(fs FileSystem, base, current string) (string, error) {
	base = fs.Clean(base)
	current = fs.Clean(current)

	if current == base {
		return ".", nil
	}

	prefix := base
	if !strings.HasSuffix(prefix, fs.Separator()) {
		prefix += fs.Separator()
	}

	if !strings.HasPrefix(current, prefix) {
		return "", fmt.Errorf("path %s is outside %s", current, base)
	}

	return strings.TrimPrefix(current, prefix), nil
}

func sortedDesiredDirectories(plan *Plan) []DesiredEntry {
	result := []DesiredEntry{}
	for _, entry := range plan.Desired {
		if entry.IsDir {
			result = append(result, entry)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return pathDepth(result[i].DestinationPath) < pathDepth(result[j].DestinationPath)
	})

	return result
}

func sortedDesiredFiles(plan *Plan) []DesiredEntry {
	result := []DesiredEntry{}
	for _, entry := range plan.Desired {
		if !entry.IsDir {
			result = append(result, entry)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].DestinationPath < result[j].DestinationPath
	})

	return result
}

func pathDepth(path string) int {
	path = strings.Trim(path, "/\\")
	if path == "" {
		return 0
	}

	return len(strings.FieldsFunc(path, func(r rune) bool {
		return r == '/' || r == '\\'
	}))
}

func baseName(path string, separator string) string {
	path = strings.TrimRight(path, separator)
	if path == "" {
		return ""
	}

	idx := strings.LastIndex(path, separator)
	if idx == -1 {
		return path
	}

	return path[idx+len(separator):]
}

func pathsToDelete(scope DeleteScope, existing []string, desired map[string]DesiredEntry, cleaner func(string) string, dir func(string) string, separator string) []string {
	protected := map[string]struct{}{
		cleaner(scope.Path): {},
	}

	for destinationPath := range desired {
		if !hasPathPrefix(destinationPath, scope.Path, cleaner, separator) {
			continue
		}

		current := cleaner(destinationPath)
		for {
			protected[current] = struct{}{}
			if current == cleaner(scope.Path) {
				break
			}
			current = dir(current)
		}
	}

	removals := []string{}
	for _, current := range existing {
		current = cleaner(current)
		if _, ok := protected[current]; ok {
			continue
		}
		removals = append(removals, current)
	}

	sort.Slice(removals, func(i, j int) bool {
		return pathDepth(removals[i]) > pathDepth(removals[j])
	})

	return removals
}

func hasPathPrefix(candidate, prefix string, cleaner func(string) string, separator string) bool {
	candidate = cleaner(candidate)
	prefix = cleaner(prefix)
	if candidate == prefix {
		return true
	}

	if !strings.HasSuffix(prefix, separator) {
		prefix += separator
	}

	return strings.HasPrefix(candidate, prefix)
}
