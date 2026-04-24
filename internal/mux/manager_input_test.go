package mux

import "testing"

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
