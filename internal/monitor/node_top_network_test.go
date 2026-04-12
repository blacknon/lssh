package monitor

import "testing"

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

	rx := formatNetworkBytesCell([]uint64{1, 2, 3, 4}).GetText()
	tx := formatNetworkBytesCell([]uint64{100, 200, 300, 400}).GetText()

	if rx == tx {
		t.Fatalf("expected different cell text for different histories, got %q", rx)
	}
}
