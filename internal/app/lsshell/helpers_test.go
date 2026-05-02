package lsshell

import (
	"flag"
	"reflect"
	"strings"
	"testing"

	"github.com/blacknon/lssh/internal/common"
	conf "github.com/blacknon/lssh/internal/config"
	"github.com/urfave/cli"
)

func newTestContext(t *testing.T, args ...string) *cli.Context {
	t.Helper()

	app := cli.NewApp()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.Var(new(cli.StringSlice), "R", "")
	fs.String("r", "", "")
	fs.String("m", "", "")
	fs.Bool("term", false, "")
	if err := fs.Parse(args); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	return cli.NewContext(app, fs, nil)
}

func TestSelectServers(t *testing.T) {
	data := conf.Config{}

	t.Run("explicit hosts", func(t *testing.T) {
		got, err := selectServers([]string{"web01"}, []string{"web01", "db01"}, data, true, nil)
		if err != nil {
			t.Fatalf("selectServers() error = %v", err)
		}
		if !reflect.DeepEqual(got, []string{"web01"}) {
			t.Fatalf("selectServers() = %#v", got)
		}
	})

	t.Run("reject missing host", func(t *testing.T) {
		_, err := selectServers([]string{"missing"}, []string{"web01"}, data, true, nil)
		if err == nil || !strings.Contains(err.Error(), "Input Server not found") {
			t.Fatalf("selectServers() error = %v", err)
		}
	})

	t.Run("prompt selection", func(t *testing.T) {
		got, err := selectServers(nil, []string{"web01"}, data, true, func(_ []string, _ conf.Config, _ bool) ([]string, error) {
			return []string{"web01"}, nil
		})
		if err != nil {
			t.Fatalf("selectServers() error = %v", err)
		}
		if !reflect.DeepEqual(got, []string{"web01"}) {
			t.Fatalf("selectServers() = %#v", got)
		}
	})
}

func TestBuildRun(t *testing.T) {
	ctx := newTestContext(
		t,
		"--term",
		"-R", "9090",
		"-R", "10022:localhost:22",
		"-r", "8082",
		"-m", "3049:relative/nfs",
	)

	run, err := buildRun(ctx, conf.Config{}, []string{"web01"}, nil)
	if err != nil {
		t.Fatalf("buildRun() error = %v", err)
	}

	if !run.IsTerm {
		t.Fatal("IsTerm = false")
	}
	if run.ReverseDynamicPortForward != "9090" {
		t.Fatalf("ReverseDynamicPortForward = %q", run.ReverseDynamicPortForward)
	}
	if len(run.PortForward) != 1 {
		t.Fatalf("len(PortForward) = %d, want 1", len(run.PortForward))
	}
	if run.HTTPReverseDynamicPortForward != "8082" {
		t.Fatalf("HTTPReverseDynamicPortForward = %q", run.HTTPReverseDynamicPortForward)
	}
	if run.NFSReverseDynamicForwardPath != common.GetFullPath("relative/nfs") {
		t.Fatalf("NFSReverseDynamicForwardPath = %q", run.NFSReverseDynamicForwardPath)
	}
	if !reflect.DeepEqual(run.ServerList, []string{"web01"}) {
		t.Fatalf("ServerList = %#v", run.ServerList)
	}
}
