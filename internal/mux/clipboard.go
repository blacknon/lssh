//go:build !windows

package mux

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
)

func setClipboardText(screen tcell.Screen, text string) error {
	if screen == nil {
		return fmt.Errorf("clipboard unavailable")
	}
	screen.SetClipboard([]byte(text))
	return nil
}
