package lspipe

import (
	"reflect"
	"testing"
)

func TestFilterNonDaemonArgs(t *testing.T) {
	args := []string{"--name", "prod", "--replace", "-F", "conf", "--raw", "hostname"}
	got := filterNonDaemonArgs(args)
	want := []string{"-F", "conf"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("filterNonDaemonArgs() = %#v, want %#v", got, want)
	}
}

func TestTernaryStatus(t *testing.T) {
	if got := ternaryStatus(false); got != "alive" {
		t.Fatalf("ternaryStatus(false) = %q", got)
	}
	if got := ternaryStatus(true); got != "stale" {
		t.Fatalf("ternaryStatus(true) = %q", got)
	}
}
