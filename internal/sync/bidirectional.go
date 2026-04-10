package sync

import (
	"io/fs"
	pathpkg "path"
	"sort"
	"time"
)

type snapshotEntry struct {
	Key      string
	Path     string
	Mode     fs.FileMode
	Size     int64
	Checksum string
	ModTime  time.Time
	IsDir    bool
}

type snapshot struct {
	Root    string
	Entries map[string]snapshotEntry
}

func BuildBidirectionalPlans(leftFS FileSystem, rightFS FileSystem, leftRoot string, rightRoot string) (*Plan, *Plan, error) {
	leftSnapshot, err := buildSnapshot(leftFS, leftRoot)
	if err != nil {
		return nil, nil, err
	}

	rightSnapshot, err := buildSnapshot(rightFS, rightRoot)
	if err != nil {
		return nil, nil, err
	}

	leftToRight := &Plan{Desired: map[string]DesiredEntry{}}
	rightToLeft := &Plan{Desired: map[string]DesiredEntry{}}

	keys := make(map[string]struct{}, len(leftSnapshot.Entries)+len(rightSnapshot.Entries))
	for key := range leftSnapshot.Entries {
		keys[key] = struct{}{}
	}
	for key := range rightSnapshot.Entries {
		keys[key] = struct{}{}
	}

	sortedKeys := make([]string, 0, len(keys))
	for key := range keys {
		sortedKeys = append(sortedKeys, key)
	}
	sort.Strings(sortedKeys)

	for _, key := range sortedKeys {
		leftEntry, hasLeft := leftSnapshot.Entries[key]
		rightEntry, hasRight := rightSnapshot.Entries[key]

		switch {
		case hasLeft && !hasRight:
			addSnapshotEntryPlan(leftToRight, leftEntry, rightSnapshot.Root, rightFS)
		case !hasLeft && hasRight:
			addSnapshotEntryPlan(rightToLeft, rightEntry, leftSnapshot.Root, leftFS)
		case hasLeft && hasRight:
			switch winner := compareSnapshotEntries(leftEntry, rightEntry); winner {
			case -1:
				addSnapshotEntryPlan(rightToLeft, rightEntry, leftSnapshot.Root, leftFS)
			case 1:
				addSnapshotEntryPlan(leftToRight, leftEntry, rightSnapshot.Root, rightFS)
			}
		}
	}

	return leftToRight, rightToLeft, nil
}

func buildSnapshot(filesystem FileSystem, root string) (*snapshot, error) {
	resolvedRoot, err := filesystem.Resolve(root)
	if err != nil {
		return nil, err
	}

	info, err := filesystem.Stat(resolvedRoot)
	if err != nil {
		if isNotExistErr(err) {
			return &snapshot{
				Root:    resolvedRoot,
				Entries: map[string]snapshotEntry{},
			}, nil
		}
		return nil, err
	}

	result := &snapshot{
		Root:    resolvedRoot,
		Entries: map[string]snapshotEntry{},
	}

	if !info.IsDir() {
		checksum, err := fileChecksum(filesystem, resolvedRoot)
		if err != nil {
			return nil, err
		}
		result.Entries["."] = snapshotEntry{
			Key:      ".",
			Path:     resolvedRoot,
			Mode:     info.Mode(),
			Size:     info.Size(),
			Checksum: checksum,
			ModTime:  info.ModTime(),
			IsDir:    false,
		}
		return result, nil
	}

	if err := filesystem.Walk(resolvedRoot, func(current string, currentInfo fs.FileInfo) error {
		relpath, err := relativePath(filesystem, resolvedRoot, current)
		if err != nil {
			return err
		}

		entry := snapshotEntry{
			Key:     toSnapshotKey(relpath),
			Path:    current,
			Mode:    currentInfo.Mode(),
			Size:    currentInfo.Size(),
			ModTime: currentInfo.ModTime(),
			IsDir:   currentInfo.IsDir(),
		}
		if !currentInfo.IsDir() {
			checksum, err := fileChecksum(filesystem, current)
			if err != nil {
				return err
			}
			entry.Checksum = checksum
		}
		result.Entries[toSnapshotKey(relpath)] = entry
		return nil
	}); err != nil {
		return nil, err
	}

	return result, nil
}

func toSnapshotKey(relpath string) string {
	if relpath == "" || relpath == "." {
		return "."
	}
	return pathpkg.Clean(relpath)
}

func compareSnapshotEntries(left, right snapshotEntry) int {
	if left.IsDir != right.IsDir {
		switch {
		case left.ModTime.After(right.ModTime):
			return 1
		case left.ModTime.Before(right.ModTime):
			return -1
		default:
			return 1
		}
	}

	if left.IsDir {
		switch {
		case left.ModTime.After(right.ModTime):
			return 1
		case left.ModTime.Before(right.ModTime):
			return -1
		default:
			return 0
		}
	}

	if left.Size == right.Size && left.Checksum == right.Checksum {
		return 0
	}

	switch {
	case left.ModTime.After(right.ModTime):
		return 1
	case left.ModTime.Before(right.ModTime):
		return -1
	default:
		return 1
	}
}

func addSnapshotEntryPlan(plan *Plan, source snapshotEntry, destinationRoot string, destinationFS FileSystem) {
	destinationPath := destinationRoot
	if source.Key != "." {
		destinationPath = destinationFS.Join(destinationRoot, source.Key)
	}

	plan.Desired[destinationPath] = DesiredEntry{
		SourcePath:      source.Path,
		DestinationPath: destinationPath,
		RelPath:         source.Key,
		Mode:            source.Mode,
		Size:            source.Size,
		ModTime:         source.ModTime,
		IsDir:           source.IsDir,
	}
}
