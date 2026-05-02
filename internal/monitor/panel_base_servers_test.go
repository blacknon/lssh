package monitor

import (
	"strings"
	"testing"
)

func TestSyncTableSelectionToSelectedNode(t *testing.T) {
	t.Parallel()

	m := &Monitor{
		Nodes: []*Node{
			NewNode("web02"),
			NewNode("web01"),
		},
		selectedNode: "web01",
	}

	table := m.createBaseGridTable()
	m.table = table

	table.Select(1, 0)
	m.selectedNode = "web01"
	m.syncTableSelectionToSelectedNode()

	row, _ := table.GetSelection()
	if row <= 0 {
		t.Fatalf("selection row = %d, want data row", row)
	}

	got := strings.TrimSpace(table.GetCell(row, 0).GetText())
	if got != "web01" {
		t.Fatalf("selected server = %q, want %q", got, "web01")
	}
}
