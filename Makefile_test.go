package main

import (
	"os"
	"strings"
	"testing"
)

func TestMakefileIncludesNewCommandsInBuildAndInstall(t *testing.T) {
	data, err := os.ReadFile("Makefile")
	if err != nil {
		t.Fatalf("ReadFile(Makefile) error = %v", err)
	}
	text := string(data)

	for _, needle := range []string{
		"BUILDCMD_LSDIFF=$(GOBUILD) ./cmd/lsdiff",
		"BUILDCMD_LSSHFS=$(GOBUILD) ./cmd/lsshfs",
		"BUILDCMD_LSMUX=$(GOBUILD) ./cmd/lsmux",
		"$(BUILDCMD_LSDIFF)",
		"$(BUILDCMD_LSSHFS)",
		"$(BUILDCMD_LSMUX)",
		"cp lsdiff $(INSTALL_PATH_LSDIFF)",
		"cp lsshfs $(INSTALL_PATH_LSSHFS)",
		"cp lsmux $(INSTALL_PATH_LSMUX)",
		"cp lspipe $(INSTALL_PATH_LSPIPE)",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("Makefile missing %q", needle)
		}
	}
}

func TestMakefileCompletionTargetsExist(t *testing.T) {
	data, err := os.ReadFile("Makefile")
	if err != nil {
		t.Fatalf("ReadFile(Makefile) error = %v", err)
	}
	text := string(data)
	for _, target := range []string{"install-completions:", "install-completions-user:"} {
		if !strings.Contains(text, target) {
			t.Fatalf("Makefile missing target %q", target)
		}
	}
}
