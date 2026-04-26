package lscp

import (
	"testing"

	"github.com/urfave/cli"
)

func TestLscpHasListFlag(t *testing.T) {
	t.Parallel()

	app := Lscp()
	for _, flag := range app.Flags {
		boolFlag, ok := flag.(cli.BoolFlag)
		if ok && boolFlag.Name == "list,l" {
			return
		}
	}

	t.Fatal("list flag not found")
}
