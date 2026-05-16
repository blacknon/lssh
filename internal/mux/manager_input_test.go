package mux

import (
	"io"
	"strings"
	"testing"

	conf "github.com/blacknon/lssh/internal/config"
	"github.com/blacknon/tvxterm"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func TestUsesLocalScrollbackNilTerm(t *testing.T) {
	if usesLocalScrollback(nil) {
		t.Fatal("usesLocalScrollback(nil) = true, want false")
	}
}

func TestFocusedPaneUsesLocalScrollbackWithoutPage(t *testing.T) {
	m := &Manager{}
	if m.focusedPaneUsesLocalScrollback() {
		t.Fatal("focusedPaneUsesLocalScrollback() = true, want false without focused pane")
	}
}

type pasteStubBackend struct {
	writes [][]byte
}

func (b *pasteStubBackend) Read(p []byte) (int, error) { return 0, io.EOF }
func (b *pasteStubBackend) Write(p []byte) (int, error) {
	b.writes = append(b.writes, append([]byte(nil), p...))
	return len(p), nil
}
func (b *pasteStubBackend) Resize(cols, rows int) error { return nil }
func (b *pasteStubBackend) Close() error                { return nil }

func newPasteTestTerm() (*tvxterm.View, *pasteStubBackend) {
	term := tvxterm.New(nil)
	backend := &pasteStubBackend{}
	term.Attach(backend)
	return term, backend
}

func TestHandlePasteBroadcastsToAllPanes(t *testing.T) {
	focusedTerm, focusedBackend := newPasteTestTerm()
	otherTerm, otherBackend := newPasteTestTerm()

	m := &Manager{
		app:          tview.NewApplication(),
		broadcastAll: true,
		sessionPages: []*page{{
			panes: []*pane{
				{term: focusedTerm},
				{term: otherTerm},
			},
		}},
	}
	m.currentPage = m.sessionPages[0]
	m.currentPage.focus = m.currentPage.panes[0]

	delegated := false
	m.handlePaste("hello", func(text string, setFocus func(p tview.Primitive)) {
		delegated = true
	}, func(p tview.Primitive) {})

	if delegated {
		t.Fatal("handlePaste delegated paste while broadcast was active")
	}
	if len(focusedBackend.writes) != 1 || string(focusedBackend.writes[0]) != "hello" {
		t.Fatalf("focused pane writes = %#v, want hello", focusedBackend.writes)
	}
	if len(otherBackend.writes) != 1 || string(otherBackend.writes[0]) != "hello" {
		t.Fatalf("other pane writes = %#v, want hello", otherBackend.writes)
	}
}

func TestHandlePasteDelegatesWhenBroadcastDisabled(t *testing.T) {
	m := &Manager{app: tview.NewApplication()}
	delegated := false
	m.handlePaste("hello", func(text string, setFocus func(p tview.Primitive)) {
		delegated = true
		if text != "hello" {
			t.Fatalf("delegated text = %q, want hello", text)
		}
	}, func(p tview.Primitive) {})

	if !delegated {
		t.Fatal("handlePaste did not delegate paste when broadcast was disabled")
	}
}

func TestHandlePasteDelegatesWhenOverlayFocused(t *testing.T) {
	term, backend := newPasteTestTerm()
	overlayFocus := tview.NewInputField()

	m := &Manager{
		app:          tview.NewApplication(),
		broadcastAll: true,
		sessionPages: []*page{{
			panes: []*pane{{
				term:        term,
				focusTarget: overlayFocus,
			}},
		}},
	}
	m.currentPage = m.sessionPages[0]
	m.currentPage.focus = m.currentPage.panes[0]
	m.app.SetFocus(overlayFocus)

	delegated := false
	m.handlePaste("hello", func(text string, setFocus func(p tview.Primitive)) {
		delegated = true
	}, func(p tview.Primitive) {})

	if !delegated {
		t.Fatal("handlePaste did not delegate paste while overlay had focus")
	}
	if len(backend.writes) != 0 {
		t.Fatalf("backend writes = %#v, want no broadcast writes", backend.writes)
	}
}

func TestCaptureInputInvalidPrefixCommandCollapsesHelp(t *testing.T) {
	status := tview.NewTextView().SetDynamicColors(true).SetWrap(true)
	root := tview.NewFlex().SetDirection(tview.FlexRow).AddItem(status, 0, 1, false)
	m := &Manager{
		app: tview.NewApplication(),
		conf: conf.Config{
			Mux: conf.MuxConfig{
				Prefix: "Ctrl+A",
			},
		},
		root:   root,
		status: status,
		bindings: map[string]keyBinding{
			"prefix": {key: tcell.KeyCtrlA},
		},
	}

	if got := m.captureInput(tcell.NewEventKey(tcell.KeyCtrlA, 0, tcell.ModCtrl)); got != nil {
		t.Fatalf("prefix key should be consumed, got %#v", got)
	}
	if !m.prefixActive {
		t.Fatal("prefixActive = false after prefix key, want true")
	}

	got := m.captureInput(tcell.NewEventKey(tcell.KeyRune, 'x', tcell.ModNone))
	if got == nil || got.Key() != tcell.KeyRune || got.Rune() != 'x' {
		t.Fatalf("invalid prefix command should pass through, got %#v", got)
	}
	if m.prefixActive {
		t.Fatal("prefixActive = true after invalid prefix command, want false")
	}

	text := status.GetText(true)
	if text != "Select hosts to open panes  Prefix: Ctrl+A" {
		t.Fatalf("status text = %q, want collapsed base help", text)
	}
	if text == "" || containsPrefixExpandedHelp(text) {
		t.Fatalf("status text = %q, want prefix help to be collapsed", text)
	}
}

func containsPrefixExpandedHelp(text string) bool {
	return text != "" && (strings.Contains(text, "new-page") || strings.Contains(text, "new-pane") || strings.Contains(text, "broadcast"))
}
