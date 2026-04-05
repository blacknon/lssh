package mview

import (
	"code.rocketnine.space/tslocum/cbind"
	"github.com/gdamore/tcell/v2"
)

// Key defines the keyboard shortcuts of an application.
// Secondary shortcuts apply when not focusing a text input.
type Key struct {
	Cancel []string

	Select  []string
	Select2 []string

	MoveUp     []string
	MoveUp2    []string
	MoveDown   []string
	MoveDown2  []string
	MoveLeft   []string
	MoveLeft2  []string
	MoveRight  []string
	MoveRight2 []string

	MoveFirst  []string
	MoveFirst2 []string
	MoveLast   []string
	MoveLast2  []string

	MovePreviousField []string
	MoveNextField     []string
	MovePreviousPage  []string
	MoveNextPage      []string

	ShowContextMenu []string
}

// Keys defines the keyboard shortcuts of an application.
// Secondary shortcuts apply when not focusing a text input.
var Keys = Key{
	Cancel: []string{"Escape"},

	Select:  []string{"Enter", "Ctrl+J"}, // Ctrl+J = keypad enter
	Select2: []string{"Space"},

	MoveUp:     []string{"Up"},
	MoveUp2:    []string{"k"},
	MoveDown:   []string{"Down"},
	MoveDown2:  []string{"j"},
	MoveLeft:   []string{"Left"},
	MoveLeft2:  []string{"h"},
	MoveRight:  []string{"Right"},
	MoveRight2: []string{"l"},

	MoveFirst:  []string{"Home", "Ctrl+A"},
	MoveFirst2: []string{"g"},
	MoveLast:   []string{"End", "Ctrl+E"},
	MoveLast2:  []string{"G"},

	MovePreviousField: []string{"Backtab"},
	MoveNextField:     []string{"Tab"},
	MovePreviousPage:  []string{"PageUp", "Ctrl+B"},
	MoveNextPage:      []string{"PageDown", "Ctrl+F"},

	ShowContextMenu: []string{"Alt+Enter"},
}

// HitShortcut returns whether the EventKey provided is present in one or more
// sets of keybindings.
func HitShortcut(event *tcell.EventKey, keybindings ...[]string) bool {
	enc, err := cbind.Encode(event.Modifiers(), event.Key(), event.Rune())
	if err != nil {
		return false
	}

	for _, binds := range keybindings {
		for _, key := range binds {
			if key == enc {
				return true
			}
		}
	}

	return false
}
