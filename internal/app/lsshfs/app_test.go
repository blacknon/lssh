package lsshfs

import (
	"reflect"
	"testing"

	conf "github.com/blacknon/lssh/internal/config"
)

func TestInsertHostFlagBeforePositionalArgs(t *testing.T) {
	args := []string{"/tmp/hoge2", "/tmp/mnt"}
	got := insertHostFlag(args, "web01")
	want := []string{"-H", "web01", "/tmp/hoge2", "/tmp/mnt"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("insertHostFlag() = %#v, want %#v", got, want)
	}
}

func TestInsertHostFlagAfterExistingOptions(t *testing.T) {
	args := []string{"--rw", "-F", "/tmp/lssh.toml", "/tmp/hoge2", "/tmp/mnt"}
	got := insertHostFlag(args, "web01")
	want := []string{"--rw", "-F", "/tmp/lssh.toml", "-H", "web01", "/tmp/hoge2", "/tmp/mnt"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("insertHostFlag() = %#v, want %#v", got, want)
	}
}

func TestLsshfsMountOptions(t *testing.T) {
	cfg := conf.Config{
		Lsshfs: conf.LsshfsConfig{
			MountOptions: []string{"nobrowse"},
			Darwin: conf.LsshfsPlatformConfig{
				MountOptions: []string{"nolocks,locallocks"},
			},
		},
	}

	got := lsshfsMountOptions(cfg, "darwin", []string{"noowners"})
	want := []string{"nobrowse", "nolocks", "locallocks", "noowners"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("lsshfsMountOptions() = %#v, want %#v", got, want)
	}
}
