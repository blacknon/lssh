package monitor

import (
	"strings"
	"testing"
)

func TestNetworkHistorySeriesUsesProvidedHistory(t *testing.T) {
	t.Parallel()

	got := networkHistorySeries([]uint64{10, 20, 30, 40})
	if len(got) != IOCount {
		t.Fatalf("len(got) = %d, want %d", len(got), IOCount)
	}
	if got[0] != 40 || got[1] != 30 || got[2] != 20 {
		t.Fatalf("got prefix = %v, want [40 30 20 ...]", got[:3])
	}
}

func TestFormatNetworkBytesCellDiffersForDifferentHistories(t *testing.T) {
	t.Parallel()

	rx := formatNetworkBytesCell([]uint64{1, 2, 3, 4}, 0).GetText()
	tx := formatNetworkBytesCell([]uint64{100, 200, 300, 400}, 0).GetText()

	if rx == tx {
		t.Fatalf("expected different cell text for different histories, got %q", rx)
	}
}

func TestNormalizeFloat64SeriesToPercentClampsToRange(t *testing.T) {
	t.Parallel()

	got := normalizeFloat64SeriesToPercent([]float64{0, 25, 50, 150}, 100)
	want := []float64{0, 25, 50, 100}

	if len(got) != len(want) {
		t.Fatalf("len(got) = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestFormatNetworkBytesCellUsesGraphMaxForColorThreshold(t *testing.T) {
	t.Parallel()

	cell := formatNetworkBytesCell([]uint64{1024, 2048, 4096, 41 * 1024}, 1024*1024).GetText()
	if strings.Contains(cell, "#fa1e1e") {
		t.Fatalf("expected non-critical graph color for low usage relative to graph max, got %q", cell)
	}
}

func TestFormatNetworkPacketsCellUsesNormalizedThreshold(t *testing.T) {
	t.Parallel()

	cell := formatNetworkPacketsCell([]uint64{10, 20, 30, 41}, 1000).GetText()
	if strings.Contains(cell, "#fa1e1e") {
		t.Fatalf("expected non-critical graph color for normalized packet history, got %q", cell)
	}
}
