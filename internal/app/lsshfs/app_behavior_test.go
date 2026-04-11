package lsshfs

import (
	"errors"
	"os"
	"os/exec"
	"reflect"
	"testing"

	mountfs "github.com/blacknon/lssh/internal/lsshfs"
)

func TestNotifyParentReadyOnlyInDaemonMode(t *testing.T) {
	orig := os.Getenv("_LSSHFS_DAEMON")
	t.Cleanup(func() { _ = os.Setenv("_LSSHFS_DAEMON", orig) })

	if err := os.Setenv("_LSSHFS_DAEMON", "0"); err != nil {
		t.Fatalf("Setenv: %v", err)
	}
	notifyParentReady()
}

func TestUnmountRecordedMountFallsBackToLaterCommand(t *testing.T) {
	record := mountfs.MountRecord{
		Host:       "web01",
		RemotePath: "/srv/data",
		MountPoint: "/tmp/demo",
		Backend:    "fuse",
		PID:        0,
		ReadWrite:  true,
	}
	origLoad, origRemove, origUnmount, origNormalize, origExec := loadMountRecordFn, removeMountRecordFn, unmountCommandsFn, normalizeMountPtFn, execCommandFn
	t.Cleanup(func() {
		loadMountRecordFn, removeMountRecordFn, unmountCommandsFn, normalizeMountPtFn, execCommandFn = origLoad, origRemove, origUnmount, origNormalize, origExec
	})

	loadMountRecordFn = func(mountpoint string) (mountfs.MountRecord, error) { return record, nil }
	normalizeMountPtFn = func(goos, mountpoint string) (string, error) { return mountpoint, nil }
	commands := []mountfs.CommandSpec{
		{Name: "first", Args: []string{"x"}},
		{Name: "second", Args: []string{"y"}},
	}
	unmountCommandsFn = func(goos, mountpoint string) ([]mountfs.CommandSpec, error) { return commands, nil }
	var tried []string
	execCommandFn = func(name string, args ...string) *exec.Cmd {
		tried = append(tried, name)
		if name == "first" {
			return exec.Command("sh", "-c", "exit 1")
		}
		return exec.Command("sh", "-c", "exit 0")
	}

	if err := unmountRecordedMount(record.MountPoint); err != nil {
		t.Fatalf("unmountRecordedMount() error = %v", err)
	}
	if !reflect.DeepEqual(tried, []string{"first", "second"}) {
		t.Fatalf("commands tried = %#v", tried)
	}
}

func TestUnmountRecordedMountIgnoresMissingRecord(t *testing.T) {
	origLoad, origUnmount, origNormalize, origRemove := loadMountRecordFn, unmountCommandsFn, normalizeMountPtFn, removeMountRecordFn
	t.Cleanup(func() {
		loadMountRecordFn, unmountCommandsFn, normalizeMountPtFn, removeMountRecordFn = origLoad, origUnmount, origNormalize, origRemove
	})

	normalizeMountPtFn = func(goos, mountpoint string) (string, error) { return mountpoint, nil }
	loadMountRecordFn = func(mountpoint string) (mountfs.MountRecord, error) { return mountfs.MountRecord{}, errors.New("missing") }
	unmountCommandsFn = func(goos, mountpoint string) ([]mountfs.CommandSpec, error) {
		return []mountfs.CommandSpec{}, nil
	}
	removeMountRecordFn = func(mountpoint string) error { return nil }

	if err := unmountRecordedMount("/tmp/demo"); err != nil {
		t.Fatalf("unmountRecordedMount() error = %v", err)
	}
}
