package lssh

import (
	"flag"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/blacknon/lssh/internal/common"
	conf "github.com/blacknon/lssh/internal/config"
	"github.com/urfave/cli"
)

func newLsshTestContext(t *testing.T, args ...string) *cli.Context {
	t.Helper()

	app := cli.NewApp()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.Var(new(cli.StringSlice), "L", "")
	fs.Var(new(cli.StringSlice), "R", "")
	fs.String("D", "", "")
	fs.String("d", "", "")
	fs.String("r", "", "")
	fs.String("M", "", "")
	fs.String("m", "", "")
	fs.String("S", "", "")
	fs.String("s", "", "")
	fs.String("tunnel", "", "")
	fs.Bool("not-execute", false, "")
	fs.Bool("parallel", false, "")
	fs.Bool("term", false, "")
	fs.Bool("w", false, "")
	fs.Bool("W", false, "")
	fs.Bool("localrc", false, "")
	fs.Bool("not-localrc", false, "")
	if err := fs.Parse(args); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	return cli.NewContext(app, fs, nil)
}

func TestSelectServers(t *testing.T) {
	data := conf.Config{}

	t.Run("validate explicit hosts", func(t *testing.T) {
		got, err := selectServers([]string{"web01"}, []string{"web01", "db01"}, data, false, false, nil)
		if err != nil {
			t.Fatalf("selectServers() error = %v", err)
		}
		if !reflect.DeepEqual(got, []string{"web01"}) {
			t.Fatalf("selectServers() = %#v", got)
		}
	})

	t.Run("reject missing host", func(t *testing.T) {
		_, err := selectServers([]string{"missing"}, []string{"web01"}, data, false, false, nil)
		if err == nil || !strings.Contains(err.Error(), "Input Server not found") {
			t.Fatalf("selectServers() error = %v", err)
		}
	})

	t.Run("prompt selection and background validation", func(t *testing.T) {
		_, err := selectServers(nil, []string{"web01", "db01"}, data, true, true, func(_ []string, _ conf.Config, _ bool) ([]string, error) {
			return []string{"web01", "db01"}, nil
		})
		if err == nil || !strings.Contains(err.Error(), "-f cannot be used with multiple hosts") {
			t.Fatalf("selectServers() error = %v", err)
		}
	})
}

func TestParseRunForwardSettings(t *testing.T) {
	ctx := newLsshTestContext(
		t,
		"-L", "127.0.0.1:8080:example.com:80",
		"-R", "9090",
		"-R", "10022:localhost:22",
		"-D", "1080",
		"-d", "8081",
		"-r", "8082",
		"-M", "2049:/remote/share",
		"-m", "3049:relative/nfs",
		"-S", "445:/remote/smb",
		"-s", "1445:relative/smb",
	)

	got, err := parseRunForwardSettings(ctx)
	if err != nil {
		t.Fatalf("parseRunForwardSettings() error = %v", err)
	}

	if len(got.PortForward) != 2 {
		t.Fatalf("len(PortForward) = %d, want 2", len(got.PortForward))
	}
	if got.ReverseDynamicPortForward != "9090" {
		t.Fatalf("ReverseDynamicPortForward = %q", got.ReverseDynamicPortForward)
	}
	if got.DynamicPortForward != "1080" || got.HTTPDynamicPortForward != "8081" || got.HTTPReverseDynamicPortForward != "8082" {
		t.Fatalf("dynamic forwards = %#v", got)
	}
	if got.NFSReverseDynamicForwardPath != common.GetFullPath("relative/nfs") {
		t.Fatalf("NFS reverse path = %q", got.NFSReverseDynamicForwardPath)
	}
	if got.SMBReverseDynamicForwardPath != common.GetFullPath("relative/smb") {
		t.Fatalf("SMB reverse path = %q", got.SMBReverseDynamicForwardPath)
	}
	if got.TunnelEnabled || got.TunnelLocal != 0 || got.TunnelRemote != 0 {
		t.Fatalf("unexpected tunnel = enabled:%t local:%d remote:%d", got.TunnelEnabled, got.TunnelLocal, got.TunnelRemote)
	}
}

func TestParseRunForwardSettingsTunnel(t *testing.T) {
	ctx := newLsshTestContext(t, "--tunnel", "1:any")

	got, err := parseRunForwardSettings(ctx)
	if runtime.GOOS == "windows" {
		if err == nil || !strings.Contains(err.Error(), "not supported on Windows") {
			t.Fatalf("parseRunForwardSettings() error = %v", err)
		}
		return
	}
	if err != nil {
		t.Fatalf("parseRunForwardSettings() error = %v", err)
	}
	if !got.TunnelEnabled || got.TunnelLocal != 1 || got.TunnelRemote != -1 {
		t.Fatalf("tunnel = enabled:%t local:%d remote:%d", got.TunnelEnabled, got.TunnelLocal, got.TunnelRemote)
	}
}

func TestParseRunForwardSettingsRejectsInvalidTunnel(t *testing.T) {
	ctx := newLsshTestContext(t, "--tunnel", "bad")

	_, err := parseRunForwardSettings(ctx)
	if runtime.GOOS == "windows" {
		if err == nil || !strings.Contains(err.Error(), "not supported on Windows") {
			t.Fatalf("parseRunForwardSettings() error = %v", err)
		}
		return
	}
	if err == nil || !strings.Contains(err.Error(), "invalid --tunnel format") {
		t.Fatalf("parseRunForwardSettings() error = %v", err)
	}
}

func TestBuildRun(t *testing.T) {
	ctx := newLsshTestContext(t, "--localrc", "--parallel", "--term")

	run, err := buildRun(ctx, conf.Config{}, []string{"web01"}, nil, "sess-1", true, true, true)
	if err != nil {
		t.Fatalf("buildRun() error = %v", err)
	}

	if run.Mode != "shell" {
		t.Fatalf("Mode = %q", run.Mode)
	}
	if !run.IsParallel || !run.IsTerm || !run.IsBashrc || !run.X11 || !run.X11Trusted || !run.ConnectorDetach {
		t.Fatalf("run flags = %#v", run)
	}
	if run.ConnectorAttachSession != "sess-1" {
		t.Fatalf("ConnectorAttachSession = %q", run.ConnectorAttachSession)
	}
	if !reflect.DeepEqual(run.ServerList, []string{"web01"}) {
		t.Fatalf("ServerList = %#v", run.ServerList)
	}
	if run.NFSReverseDynamicForwardPath != "" {
		t.Fatalf("unexpected reverse path = %q", run.NFSReverseDynamicForwardPath)
	}
}
