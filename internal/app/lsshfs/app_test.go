package lsshfs

import (
	"reflect"
	"testing"
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
