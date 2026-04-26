package version

import (
	"strings"
	"testing"
)

func TestAppVersionIncludesSuiteVersionAndClassification(t *testing.T) {
	cases := map[string]string{
		"lssh":   "lssh-suite 0.10.0 (stable/core)",
		"lsdiff": "lssh-suite 0.10.0 (beta/sysadmin)",
		"lsshfs": "lssh-suite 0.10.0 (beta/transfer)",
		"lspipe": "lssh-suite 0.10.0 (alpha/sysadmin)",
	}

	for command, want := range cases {
		if got := AppVersion(command); got != want {
			t.Fatalf("AppVersion(%q) = %q, want %q", command, got, want)
		}
	}
}

func TestForCommandUnknownDefaultsToAlphaUnknown(t *testing.T) {
	got := ForCommand("unknown")
	if got.Maturity != Alpha || got.Domain != Unknown {
		t.Fatalf("ForCommand(unknown) = %#v", got)
	}
	if !strings.Contains(got.String(), "(alpha/unknown)") {
		t.Fatalf("String() = %q", got.String())
	}
}
