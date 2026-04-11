package lsdiff

import "testing"

func TestAlignDocumentsMultipleTargets(t *testing.T) {
	comparison := AlignDocuments([]Document{
		{Target: Target{Title: "a"}, Lines: []string{"line1", "line2", "line3"}},
		{Target: Target{Title: "b"}, Lines: []string{"line1", "line2 changed", "line3"}},
		{Target: Target{Title: "c"}, Lines: []string{"line0 inserted", "line1", "line3"}},
	})

	if len(comparison.Rows) < 4 {
		t.Fatalf("expected at least 4 aligned rows, got %d", len(comparison.Rows))
	}

	if !comparison.Rows[0].Changed {
		t.Fatalf("expected inserted row to be marked changed")
	}

	foundChangedMiddle := false
	for _, row := range comparison.Rows {
		if len(row.Cells) == 3 && row.Cells[1].Present && row.Cells[1].Text == "line2 changed" {
			foundChangedMiddle = row.Changed
		}
	}
	if !foundChangedMiddle {
		t.Fatalf("expected changed middle line to be present and marked changed")
	}
}
