package lsdiff

import (
	"testing"

	"github.com/urfave/cli"
)

func TestLsdiffHasCommonSelectionFlags(t *testing.T) {
	t.Parallel()

	app := Lsdiff()

	hasHost := false
	hasList := false
	for _, flag := range app.Flags {
		switch typed := flag.(type) {
		case cli.StringSliceFlag:
			if typed.Name == "host,H" {
				hasHost = true
			}
		case cli.BoolFlag:
			if typed.Name == "list,l" {
				hasList = true
			}
		}
	}

	if !hasHost {
		t.Fatal("host flag not found")
	}
	if !hasList {
		t.Fatal("list flag not found")
	}
}
