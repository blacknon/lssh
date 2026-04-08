package sftp

import (
	"os"
	"path"
	"path/filepath"
	"strings"
)

func shouldTreatLocalGetDestinationAsDir(destination string, forceDir bool) bool {
	if forceDir {
		return true
	}

	if strings.HasSuffix(destination, string(filepath.Separator)) {
		return true
	}

	info, err := os.Stat(destination)
	return err == nil && info.IsDir()
}

func copySourceBase(sourcePath string, sourceIsDir, preserveSourceName bool) string {
	sourcePath = filepath.Clean(sourcePath)

	if sourceIsDir && !preserveSourceName {
		return sourcePath
	}

	return filepath.Dir(sourcePath)
}

func shouldTreatRemotePutDestinationAsDir(destination string, targetExistsAsDir, sourceIsDir, multipleSources bool) bool {
	return targetExistsAsDir || sourceIsDir || multipleSources || strings.HasSuffix(destination, "/")
}

func resolveLocalGetDestinationPath(destination, relpath string, treatAsDir bool) string {
	if !treatAsDir || relpath == "" || relpath == "." {
		return filepath.Clean(destination)
	}

	return filepath.Join(destination, relpath)
}

func resolveRemotePutDestinationPath(destination, relpath string, treatAsDir bool) string {
	cleanDestination := path.Clean(filepath.ToSlash(destination))
	relpath = filepath.ToSlash(relpath)

	if !treatAsDir || relpath == "" || relpath == "." {
		return cleanDestination
	}

	return path.Join(cleanDestination, relpath)
}
