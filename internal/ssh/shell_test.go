package ssh

import (
	"strings"
	"testing"

	"github.com/blacknon/lssh/internal/common"
)

func TestLocalrcArchiveMode(t *testing.T) {
	t.Parallel()

	if got := localrcArchiveMode(false); got != common.ARCHIVE_NONE {
		t.Fatalf("localrcArchiveMode(false) = %d, want %d", got, common.ARCHIVE_NONE)
	}

	if got := localrcArchiveMode(true); got != common.ARCHIVE_GZIP {
		t.Fatalf("localrcArchiveMode(true) = %d, want %d", got, common.ARCHIVE_GZIP)
	}
}

func TestBuildInteractiveLocalRCShellCommandUsesInteractiveBash(t *testing.T) {
	t.Parallel()

	cmd := BuildInteractiveLocalRCShellCommand([]string{}, "", false, "")
	if !strings.Contains(cmd, "export TERM=") {
		t.Fatalf("BuildInteractiveLocalRCShellCommand() = %q, want TERM export", cmd)
	}
	if !strings.Contains(cmd, "exec bash -lc") {
		t.Fatalf("BuildInteractiveLocalRCShellCommand() = %q, want bash -lc wrapper", cmd)
	}
	if !strings.Contains(cmd, "exec bash --noprofile --rcfile <(") {
		t.Fatalf("BuildInteractiveLocalRCShellCommand() = %q, want interactive bash invocation", cmd)
	}
	if strings.Contains(cmd, "mktemp") {
		t.Fatalf("BuildInteractiveLocalRCShellCommand() = %q, should not create temporary rc files", cmd)
	}
	if !strings.Contains(cmd, "base64 -d") || !strings.Contains(cmd, "printf %s") {
		t.Fatalf("BuildInteractiveLocalRCShellCommand() = %q, want base64-backed marker print command", cmd)
	}
}
