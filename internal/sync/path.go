package sync

import "strings"

func hasTrailingSlash(path string, separator string) bool {
	return strings.HasSuffix(path, "/") || (separator != "/" && strings.HasSuffix(path, separator))
}

func copySourceBase(sourcePath string, sourceIsDir, preserveSourceName bool, cleaner func(string) string, dir func(string) string) string {
	sourcePath = cleaner(sourcePath)
	if sourceIsDir && !preserveSourceName {
		return sourcePath
	}

	return dir(sourcePath)
}

func shouldTreatDestinationAsDir(destination string, targetExistsAsDir, sourceIsDir, multipleSources bool, separator string) bool {
	return targetExistsAsDir || sourceIsDir || multipleSources || hasTrailingSlash(destination, separator)
}

func resolveDestinationPath(destination, relpath string, treatAsDir bool, cleaner func(string) string, join func(...string) string) string {
	if !treatAsDir || relpath == "" || relpath == "." {
		return cleaner(destination)
	}

	return join(destination, relpath)
}
