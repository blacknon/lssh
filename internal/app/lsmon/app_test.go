//go:build !windows

package lsmon

import (
	"testing"

	"github.com/urfave/cli"
)

func TestLsmonHasShareConnectFlag(t *testing.T) {
	t.Parallel()

	app := Lsmon()
	for _, flag := range app.Flags {
		boolFlag, ok := flag.(cli.BoolFlag)
		if ok && boolFlag.Name == "share-connect,s" {
			return
		}
	}

	t.Fatal("share-connect flag not found")
}
