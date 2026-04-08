// Copyright (c) 2026 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package mux

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
)

type keyBinding struct {
	key tcell.Key
	mod tcell.ModMask
	ch  rune
}

func parseKeyBinding(spec string) (keyBinding, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return keyBinding{}, fmt.Errorf("empty key binding")
	}

	lower := strings.ToLower(spec)
	switch lower {
	case "enter":
		return keyBinding{key: tcell.KeyEnter}, nil
	case "esc", "escape":
		return keyBinding{key: tcell.KeyEsc}, nil
	case "tab":
		return keyBinding{key: tcell.KeyTab}, nil
	case "backtab":
		return keyBinding{key: tcell.KeyBacktab}, nil
	case "pgup", "pageup":
		return keyBinding{key: tcell.KeyPgUp}, nil
	case "pgdn", "pagedown":
		return keyBinding{key: tcell.KeyPgDn}, nil
	case "left":
		return keyBinding{key: tcell.KeyLeft}, nil
	case "right":
		return keyBinding{key: tcell.KeyRight}, nil
	case "up":
		return keyBinding{key: tcell.KeyUp}, nil
	case "down":
		return keyBinding{key: tcell.KeyDown}, nil
	}

	parts := strings.Split(lower, "+")
	if len(parts) == 2 && parts[0] == "ctrl" && len([]rune(parts[1])) == 1 {
		return keyBinding{key: tcell.KeyCtrlA + tcell.Key(strings.ToUpper(parts[1])[0]-'A'), mod: tcell.ModCtrl}, nil
	}

	runes := []rune(spec)
	if len(runes) == 1 {
		return keyBinding{key: tcell.KeyRune, ch: runes[0]}, nil
	}

	return keyBinding{}, fmt.Errorf("unsupported key binding: %s", spec)
}

func (b keyBinding) match(event *tcell.EventKey) bool {
	if event == nil {
		return false
	}
	if b.key == tcell.KeyRune {
		return event.Key() == tcell.KeyRune && event.Rune() == b.ch
	}
	return event.Key() == b.key
}
