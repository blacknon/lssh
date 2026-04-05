package mview

import (
	"sync"

	"github.com/gdamore/tcell/v2"
)

// Button is labeled box that triggers an action when selected.
type Button struct {
	*Box

	// The text to be displayed before the input area.
	label []byte

	// The label color.
	labelColor tcell.Color

	// The label color when the button is in focus.
	labelColorFocused tcell.Color

	// The background color when the button is in focus.
	backgroundColorFocused tcell.Color

	// An optional function which is called when the button was selected.
	selected func()

	// An optional function which is called when the user leaves the button. A
	// key is provided indicating which key was pressed to leave (tab or backtab).
	blur func(tcell.Key)

	// An optional rune which is drawn after the label when the button is focused.
	cursorRune rune

	sync.RWMutex
}

// NewButton returns a new input field.
func NewButton(label string) *Button {
	box := NewBox()
	box.SetRect(0, 0, TaggedStringWidth(label)+4, 1)
	box.SetBackgroundColor(Styles.MoreContrastBackgroundColor)
	return &Button{
		Box:                    box,
		label:                  []byte(label),
		labelColor:             Styles.PrimaryTextColor,
		labelColorFocused:      Styles.PrimaryTextColor,
		cursorRune:             Styles.ButtonCursorRune,
		backgroundColorFocused: Styles.ContrastBackgroundColor,
	}
}

// SetLabel sets the button text.
func (b *Button) SetLabel(label string) {
	b.Lock()
	defer b.Unlock()

	b.label = []byte(label)
}

// GetLabel returns the button text.
func (b *Button) GetLabel() string {
	b.RLock()
	defer b.RUnlock()

	return string(b.label)
}

// SetLabelColor sets the color of the button text.
func (b *Button) SetLabelColor(color tcell.Color) {
	b.Lock()
	defer b.Unlock()

	b.labelColor = color
}

// SetLabelColorFocused sets the color of the button text when the button is
// in focus.
func (b *Button) SetLabelColorFocused(color tcell.Color) {
	b.Lock()
	defer b.Unlock()

	b.labelColorFocused = color
}

// SetCursorRune sets the rune to show within the button when it is focused.
func (b *Button) SetCursorRune(rune rune) {
	b.Lock()
	defer b.Unlock()

	b.cursorRune = rune
}

// SetBackgroundColorFocused sets the background color of the button text when
// the button is in focus.
func (b *Button) SetBackgroundColorFocused(color tcell.Color) {
	b.Lock()
	defer b.Unlock()

	b.backgroundColorFocused = color
}

// SetSelectedFunc sets a handler which is called when the button was selected.
func (b *Button) SetSelectedFunc(handler func()) {
	b.Lock()
	defer b.Unlock()

	b.selected = handler
}

// SetBlurFunc sets a handler which is called when the user leaves the button.
// The callback function is provided with the key that was pressed, which is one
// of the following:
//
//   - KeyEscape: Leaving the button with no specific direction.
//   - KeyTab: Move to the next field.
//   - KeyBacktab: Move to the previous field.
func (b *Button) SetBlurFunc(handler func(key tcell.Key)) {
	b.Lock()
	defer b.Unlock()

	b.blur = handler
}

// Draw draws this primitive onto the screen.
func (b *Button) Draw(screen tcell.Screen) {
	if !b.GetVisible() {
		return
	}

	b.Lock()
	defer b.Unlock()

	// Draw the box.
	borderColor := b.borderColor
	backgroundColor := b.backgroundColor
	if b.focus.HasFocus() {
		b.backgroundColor = b.backgroundColorFocused
		b.borderColor = b.labelColorFocused
		defer func() {
			b.borderColor = borderColor
		}()
	}
	b.Unlock()
	b.Box.Draw(screen)
	b.Lock()
	b.backgroundColor = backgroundColor

	// Draw label.
	x, y, width, height := b.GetInnerRect()
	if width > 0 && height > 0 {
		y = y + height/2
		labelColor := b.labelColor
		if b.focus.HasFocus() {
			labelColor = b.labelColorFocused
		}
		_, pw := Print(screen, b.label, x, y, width, AlignCenter, labelColor)

		// Draw cursor.
		if b.focus.HasFocus() && b.cursorRune != 0 {
			cursorX := x + int(float64(width)/2+float64(pw)/2)
			if cursorX > x+width-1 {
				cursorX = x + width - 1
			} else if cursorX < x+width {
				cursorX++
			}
			Print(screen, []byte(string(b.cursorRune)), cursorX, y, width, AlignLeft, labelColor)
		}
	}
}

// InputHandler returns the handler for this primitive.
func (b *Button) InputHandler() func(event *tcell.EventKey, setFocus func(p Primitive)) {
	return b.WrapInputHandler(func(event *tcell.EventKey, setFocus func(p Primitive)) {
		// Process key event.
		if HitShortcut(event, Keys.Select, Keys.Select2) {
			if b.selected != nil {
				b.selected()
			}
		} else if HitShortcut(event, Keys.Cancel, Keys.MovePreviousField, Keys.MoveNextField) {
			if b.blur != nil {
				b.blur(event.Key())
			}
		}
	})
}

// MouseHandler returns the mouse handler for this primitive.
func (b *Button) MouseHandler() func(action MouseAction, event *tcell.EventMouse, setFocus func(p Primitive)) (consumed bool, capture Primitive) {
	return b.WrapMouseHandler(func(action MouseAction, event *tcell.EventMouse, setFocus func(p Primitive)) (consumed bool, capture Primitive) {
		if !b.InRect(event.Position()) {
			return false, nil
		}

		// Process mouse event.
		if action == MouseLeftClick {
			setFocus(b)
			if b.selected != nil {
				b.selected()
			}
			consumed = true
		}

		return
	})
}
