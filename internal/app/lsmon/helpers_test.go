//go:build !windows

package lsmon

import (
	"flag"
	"testing"

	"github.com/urfave/cli"
)

func newTestContext(t *testing.T, args ...string) *cli.Context {
	t.Helper()

	app := cli.NewApp()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.Bool("share-connect", false, "")
	fs.Bool("localrc", false, "")
	fs.Bool("not-localrc", false, "")
	if err := fs.Parse(args); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	return cli.NewContext(app, fs, nil)
}

func TestBuildRun(t *testing.T) {
	ctx := newTestContext(t, "--share-connect", "--localrc")

	run := buildRun(ctx, []string{"web01"}, nil, true)
	if len(run.ServerList) != 1 || run.ServerList[0] != "web01" {
		t.Fatalf("ServerList = %#v", run.ServerList)
	}
	if !run.ShareConnect || !run.IsBashrc || run.IsNotBashrc {
		t.Fatalf("run = %#v", run)
	}
}
