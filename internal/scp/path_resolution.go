package scp

import (
	"os"
	"path"
	"path/filepath"
	"strings"
)

func shouldTreatLocalDestinationAsDir(destination string, forceDir bool) bool {
	if forceDir {
		return true
	}

	if strings.HasSuffix(destination, string(filepath.Separator)) {
		return true
	}

	info, err := os.Stat(destination)
	return err == nil && info.IsDir()
}

func shouldTreatRemoteDestinationAsDir(destination string, targetExistsAsDir, sourceIsDir, multipleSources bool) bool {
	return targetExistsAsDir || sourceIsDir || multipleSources || strings.HasSuffix(destination, "/")
}

func resolveRemoteDestinationPath(destination, relpath string, treatAsDir bool) string {
	cleanDestination := path.Clean(filepath.ToSlash(destination))
	relpath = filepath.ToSlash(relpath)

	if !treatAsDir || relpath == "" || relpath == "." {
		return cleanDestination
	}

	return path.Join(cleanDestination, relpath)
}

func resolveLocalDestinationPath(destination, relpath string, treatAsDir bool) string {
	if !treatAsDir || relpath == "" || relpath == "." {
		return filepath.Clean(destination)
	}

	return filepath.Join(destination, relpath)
}
