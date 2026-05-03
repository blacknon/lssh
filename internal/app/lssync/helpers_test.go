package lssync

import (
	"flag"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/blacknon/lssh/internal/app/apputil"
	conf "github.com/blacknon/lssh/internal/config"
	lsync "github.com/blacknon/lssh/internal/sync"
	"github.com/urfave/cli"
)

func newTestContext(t *testing.T, args ...string) *cli.Context {
	t.Helper()

	app := cli.NewApp()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.Bool("daemon", false, "")
	fs.Duration("daemon-interval", 5*time.Second, "")
	fs.Bool("bidirectional", false, "")
	fs.Int("parallel", 1, "")
	fs.Bool("permission", false, "")
	fs.Bool("dry-run", false, "")
	fs.Bool("delete", false, "")
	if err := fs.Parse(args); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	return cli.NewContext(app, fs, nil)
}

func TestValidateCopyTypes(t *testing.T) {
	tests := []struct {
		name    string
		remote  bool
		local   bool
		to      bool
		hosts   int
		wantErr string
	}{
		{name: "mixed from", remote: true, local: true, wantErr: "Can not set LOCAL and REMOTE"},
		{name: "local to local", remote: false, local: true, to: false, wantErr: "It does not correspond LOCAL to LOCAL"},
		{name: "remote remote with hosts", remote: true, local: false, to: true, hosts: 1, wantErr: "does not correspond to host option"},
		{name: "valid", remote: true, local: false, to: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := apputil.ValidateTransferCopyTypes(tt.remote, tt.local, tt.to, tt.hosts)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("validateCopyTypes() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("validateCopyTypes() error = %v", err)
			}
		})
	}
}

func TestParseSyncSpecs(t *testing.T) {
	sourceSpecs, targetSpec, isFromRemote, isFromLocal, explicitHosts, err := parseSyncSpecs(
		[]string{"remote:@web01:/var/log", "local:/tmp/a"},
		"remote:@db01:/backup",
		[]string{"web01", "db01"},
	)
	if err != nil {
		t.Fatalf("parseSyncSpecs() error = %v", err)
	}
	if !isFromRemote || !isFromLocal {
		t.Fatalf("flags = remote:%t local:%t", isFromRemote, isFromLocal)
	}
	if !targetSpec.IsRemote || !reflect.DeepEqual(targetSpec.Hosts, []string{"db01"}) {
		t.Fatalf("targetSpec = %#v", targetSpec)
	}
	if !reflect.DeepEqual(explicitHosts, []string{"web01"}) {
		t.Fatalf("explicitHosts = %#v", explicitHosts)
	}
	if len(sourceSpecs) != 2 {
		t.Fatalf("len(sourceSpecs) = %d", len(sourceSpecs))
	}
}

func TestSelectSyncServers(t *testing.T) {
	cfg := conf.Config{
		Server: map[string]conf.ServerConfig{
			"web01": {},
			"db01":  {},
		},
	}

	t.Run("explicit hosts", func(t *testing.T) {
		from, to, err := selectSyncServers([]string{"web01"}, []string{"web01", "db01"}, []string{"web01", "db01"}, cfg, nil, lsync.PathSpec{}, false, false, apputil.PromptServerSelection)
		if err != nil {
			t.Fatalf("selectSyncServers() error = %v", err)
		}
		if len(from) != 0 || !reflect.DeepEqual(to, []string{"web01"}) {
			t.Fatalf("from=%#v to=%#v", from, to)
		}
	})

	t.Run("prompt remote to remote", func(t *testing.T) {
		calls := 0
		from, to, err := selectSyncServers(nil, []string{"web01", "db01"}, []string{"web01", "db01"}, cfg, nil, lsync.PathSpec{}, true, true, func(prompt string, _ []string, _ conf.Config, _ bool) ([]string, error) {
			calls++
			if strings.Contains(prompt, "(from)") {
				return []string{"web01"}, nil
			}
			return []string{"db01"}, nil
		})
		if err != nil {
			t.Fatalf("selectSyncServers() error = %v", err)
		}
		if calls != 2 || !reflect.DeepEqual(from, []string{"web01"}) || !reflect.DeepEqual(to, []string{"db01"}) {
			t.Fatalf("calls=%d from=%#v to=%#v", calls, from, to)
		}
	})
}

func TestBuildSync(t *testing.T) {
	ctx := newTestContext(t, "--daemon", "--daemon-interval", "2s", "--bidirectional", "--parallel", "3", "--permission", "--dry-run", "--delete")
	tmp := t.TempDir()

	s, err := buildSync(ctx, conf.Config{}, []lsync.PathSpec{{Path: tmp}}, lsync.PathSpec{IsRemote: true, Path: "/var/tmp"}, nil, []string{"web01"}, true, nil)
	if err != nil {
		t.Fatalf("buildSync() error = %v", err)
	}
	if s.From.IsRemote || !reflect.DeepEqual(s.From.Path, []string{tmp}) {
		t.Fatalf("From = %#v", s.From)
	}
	if !s.To.IsRemote || !reflect.DeepEqual(s.To.Server, []string{"web01"}) {
		t.Fatalf("To = %#v", s.To)
	}
	if !s.Daemon || s.DaemonInterval != 2*time.Second || !s.Bidirectional || !s.Permission || !s.DryRun || !s.Delete || s.ParallelNum != 3 {
		t.Fatalf("Sync flags = %#v", s)
	}
}
