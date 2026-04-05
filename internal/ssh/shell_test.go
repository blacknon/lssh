package ssh

import (
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
