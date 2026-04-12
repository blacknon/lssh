package lspipe

import (
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	conf "github.com/blacknon/lssh/internal/config"
	pipeapp "github.com/blacknon/lssh/internal/lspipe"
	"github.com/urfave/cli"
)

func newTestContext(t *testing.T, args ...string) *cli.Context {
	t.Helper()
	app := cli.NewApp()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.String("name", "", "")
	fs.String("fifo-name", "default", "")
	fs.Bool("replace", false, "")
	fs.Var(new(cli.StringSlice), "create-host", "")
	fs.Var(new(cli.StringSlice), "host", "")
	if err := fs.Parse(args); err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	return cli.NewContext(app, fs, nil)
}

func TestEnsureSessionReusesAliveSession(t *testing.T) {
	origLoad, origMark, origFormat, origSpawn := loadSessionFn, markSessionAliveFn, formatSessionFn, spawnDaemonFn
	t.Cleanup(func() {
		loadSessionFn, markSessionAliveFn, formatSessionFn, spawnDaemonFn = origLoad, origMark, origFormat, origSpawn
	})

	loadSessionFn = func(name string) (pipeapp.Session, error) {
		return pipeapp.Session{Name: name}, nil
	}
	markSessionAliveFn = func(session *pipeapp.Session) {
		session.Stale = false
	}
	formatSessionFn = func(session pipeapp.Session) string { return "reused" }
	spawnDaemonFn = func(c *cli.Context, name string, hosts []string) error {
		return errors.New("should not spawn")
	}

	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	t.Cleanup(func() {
		os.Stdout = origStdout
	})

	err := ensureSession(newTestContext(t), conf.Config{}, "prod")
	_ = w.Close()
	data, _ := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ensureSession() error = %v", err)
	}
	if !strings.Contains(string(data), "reused") {
		t.Fatalf("stdout = %q", string(data))
	}
}

func TestEnsureSessionStaleSessionTriggersRecreation(t *testing.T) {
	origLoad, origMark, origSpawn := loadSessionFn, markSessionAliveFn, spawnDaemonFn
	t.Cleanup(func() {
		loadSessionFn, markSessionAliveFn, spawnDaemonFn = origLoad, origMark, origSpawn
	})

	loadSessionFn = func(name string) (pipeapp.Session, error) {
		return pipeapp.Session{Name: name}, nil
	}
	markSessionAliveFn = func(session *pipeapp.Session) {
		session.Stale = true
	}
	spawned := false
	spawnDaemonFn = func(c *cli.Context, name string, hosts []string) error {
		spawned = true
		return nil
	}

	if err := ensureSession(newTestContext(t, "--create-host", "web01"), conf.Config{
		Server: map[string]conf.ServerConfig{
			"web01": {Addr: "127.0.0.1", User: "demo", Pass: "secret"},
		},
	}, "prod"); err != nil {
		t.Fatalf("ensureSession() error = %v", err)
	}
	if !spawned {
		t.Fatal("spawnDaemon was not called")
	}
}

func TestCloseSessionRemovesFIFOBridgesAndSession(t *testing.T) {
	cacheDir := filepath.Join(t.TempDir(), "cache")
	t.Setenv("XDG_CACHE_HOME", cacheDir)

	session := pipeapp.Session{
		Name:      "prod",
		Hosts:     []string{"web01"},
		CreatedAt: time.Now(),
		LastUsedAt: time.Now(),
	}
	if err := pipeapp.SaveSession(session); err != nil {
		t.Fatalf("SaveSession() error = %v", err)
	}

	baseDir := filepath.Join(cacheDir, "lssh", "lspipe", "fifo", "prod", "ops")
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := pipeapp.SaveFIFORecord(pipeapp.FIFORecord{
		SessionName: "prod",
		Name:        "ops",
		Dir:         baseDir,
		Hosts:       []string{"web01"},
	}); err != nil {
		t.Fatalf("SaveFIFORecord() error = %v", err)
	}

	origPing := pingSessionFn
	t.Cleanup(func() { pingSessionFn = origPing })
	pingSessionFn = func(session pipeapp.Session) bool { return false }

	if err := closeSession("prod"); err != nil {
		t.Fatalf("closeSession() error = %v", err)
	}
	if _, err := pipeapp.LoadSession("prod"); !os.IsNotExist(err) {
		t.Fatalf("session still exists: %v", err)
	}
	if _, err := pipeapp.LoadFIFORecord("prod", "ops"); !os.IsNotExist(err) {
		t.Fatalf("fifo record still exists: %v", err)
	}
	if _, err := os.Stat(baseDir); !os.IsNotExist(err) {
		t.Fatalf("fifo dir still exists: %v", err)
	}
}

func TestEnsureFIFOBridgeReturnsExistingRecord(t *testing.T) {
	origResolve, origLoadRecord := resolveSessionFn, loadFIFORecordFn
	t.Cleanup(func() {
		resolveSessionFn, loadFIFORecordFn = origResolve, origLoadRecord
	})

	resolveSessionFn = func(name string) (pipeapp.Session, error) {
		return pipeapp.Session{Name: name, Hosts: []string{"web01"}}, nil
	}
	loadFIFORecordFn = func(sessionName, fifoName string) (pipeapp.FIFORecord, error) {
		return pipeapp.FIFORecord{SessionName: sessionName, Name: fifoName, Dir: "/tmp/fifo", PID: 12}, nil
	}

	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = origStdout })

	err := ensureFIFOBridge(newTestContext(t), "prod")
	_ = w.Close()
	data, _ := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ensureFIFOBridge() error = %v", err)
	}
	if !strings.Contains(string(data), "/tmp/fifo") {
		t.Fatalf("stdout = %q", string(data))
	}
}
