//go:build !windows

package lsmon

import "testing"

func TestGetAbsPath(t *testing.T) {
	got := getAbsPath(".")
	if got == "" {
		t.Fatal("getAbsPath returned empty path")
	}
}
