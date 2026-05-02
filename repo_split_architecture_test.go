package main

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

func TestNonProviderPackagesDoNotImportProviderImplementations(t *testing.T) {
	files := repositoryGoFiles(t)

	for _, path := range files {
		if strings.Contains(path, string(filepath.Separator)+"vendor"+string(filepath.Separator)) {
			continue
		}

		pkgPath := importPathFromFile(path)
		if strings.HasPrefix(pkgPath, "github.com/blacknon/lssh/provider/") {
			continue
		}

		imports := parseImports(t, path)
		for _, imported := range imports {
			if strings.HasPrefix(imported, "github.com/blacknon/lssh/provider/") {
				t.Errorf("%s imports provider implementation package %s", pkgPath, imported)
			}
		}
	}
}

func TestProviderPackagesOnlyUseApprovedInternalPackages(t *testing.T) {
	files := repositoryGoFiles(t)
	allowed := map[string]struct{}{
		"github.com/blacknon/lssh/internal/connectorruntime": {},
		"github.com/blacknon/lssh/internal/termenv":          {},
	}

	for _, path := range files {
		if strings.Contains(path, string(filepath.Separator)+"vendor"+string(filepath.Separator)) {
			continue
		}

		pkgPath := importPathFromFile(path)
		if !strings.HasPrefix(pkgPath, "github.com/blacknon/lssh/provider/") {
			continue
		}

		imports := parseImports(t, path)
		for _, imported := range imports {
			if !strings.HasPrefix(imported, "github.com/blacknon/lssh/internal/") {
				continue
			}
			if _, ok := allowed[imported]; ok {
				continue
			}
			t.Errorf("%s imports non-approved internal package %s", pkgPath, imported)
		}
	}
}

func repositoryGoFiles(t *testing.T) []string {
	t.Helper()

	var collected []string
	walkErr := filepath.WalkDir(".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			name := entry.Name()
			if name == "vendor" || name == ".git" || name == ".cache" || name == ".gocache" || name == ".gomodcache" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".go") {
			collected = append(collected, path)
		}
		return nil
	})
	if walkErr != nil {
		t.Fatalf("walk repository: %v", walkErr)
	}
	return collected
}

func parseImports(t *testing.T, path string) []string {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	imports := make([]string, 0, len(file.Imports))
	for _, spec := range file.Imports {
		imported := strings.Trim(spec.Path.Value, "\"")
		imports = append(imports, imported)
	}
	return imports
}

func importPathFromFile(path string) string {
	cleaned := filepath.ToSlash(strings.TrimPrefix(path, "./"))
	cleaned = strings.TrimPrefix(cleaned, "/")
	if strings.HasSuffix(cleaned, "_test.go") {
		return "github.com/blacknon/lssh/" + filepath.ToSlash(filepath.Dir(cleaned))
	}
	return "github.com/blacknon/lssh/" + filepath.ToSlash(filepath.Dir(cleaned))
}
