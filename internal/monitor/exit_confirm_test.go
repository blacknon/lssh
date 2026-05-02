package monitor

import (
	"testing"

	mview "github.com/blacknon/mview"
	"github.com/gdamore/tcell/v2"
)

func TestHandleGlobalMonitorKeyShowsAndHidesExitConfirm(t *testing.T) {
	t.Parallel()

	m := &Monitor{
		View: mview.NewApplication(),
	}
	m.table = mview.NewTable()
	m.exitModal = m.createExitConfirmModal()
	root := newExitOverlayPrimitive(m.table, m.exitModal, m.isExitConfirmVisible)

	m.View.SetRoot(root, true)
	m.View.SetFocus(m.table)

	if !m.handleGlobalMonitorKey(tcell.NewEventKey(tcell.KeyCtrlC, 0, tcell.ModCtrl)) {
		t.Fatalf("ctrl+c should be handled")
	}

	if !m.isExitConfirmVisible() {
		t.Fatalf("exit confirm should be visible")
	}

	if !m.exitModal.HasFocus() {
		t.Fatalf("exit modal should have focus")
	}

	if !m.handleGlobalMonitorKey(tcell.NewEventKey(tcell.KeyRune, 'n', tcell.ModNone)) {
		t.Fatalf("n should be handled while exit confirm is visible")
	}

	if m.isExitConfirmVisible() {
		t.Fatalf("exit confirm should be hidden")
	}

	if focus := m.View.GetFocus(); focus != m.table {
		t.Fatalf("focus = %T, want table", focus)
	}
}
