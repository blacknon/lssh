package mview

import (
	"math"
	"sync"

	"github.com/gdamore/tcell/v2"
)

// Slider is a progress bar which may be modified via keyboard and mouse.
type Slider struct {
	*ProgressBar

	// The text to be displayed before the slider.
	label []byte

	// The screen width of the label area. A value of 0 means use the width of
	// the label text.
	labelWidth int

	// The label color.
	labelColor tcell.Color

	// The label color when focused.
	labelColorFocused tcell.Color

	// The background color of the input area.
	fieldBackgroundColor tcell.Color

	// The background color of the input area when focused.
	fieldBackgroundColorFocused tcell.Color

	// The text color of the input area.
	fieldTextColor tcell.Color

	// The text color of the input area when focused.
	fieldTextColorFocused tcell.Color

	// The amount to increment by when modified via keyboard.
	increment int

	// Set to true when mouse dragging is in progress.
	dragging bool

	// An optional function which is called when the user changes the value of
	// this slider.
	changed func(value int)

	// An optional function which is called when the user indicated that they
	// are done entering text. The key which was pressed is provided (tab,
	// shift-tab, or escape).
	done func(tcell.Key)

	// A callback function set by the Form class and called when the user leaves
	// this form item.
	finished func(tcell.Key)

	sync.RWMutex
}

// NewSlider returns a new slider.
func NewSlider() *Slider {
	s := &Slider{
		ProgressBar:                 NewProgressBar(),
		increment:                   10,
		labelColor:                  Styles.SecondaryTextColor,
		fieldBackgroundColor:        Styles.MoreContrastBackgroundColor,
		fieldBackgroundColorFocused: Styles.ContrastBackgroundColor,
		fieldTextColor:              Styles.PrimaryTextColor,
		labelColorFocused:           ColorUnset,
		fieldTextColorFocused:       ColorUnset,
	}
	return s
}

// SetLabel sets the text to be displayed before the input area.
func (s *Slider) SetLabel(label string) {
	s.Lock()
	defer s.Unlock()

	s.label = []byte(label)
}

// GetLabel returns the text to be displayed before the input area.
func (s *Slider) GetLabel() string {
	s.RLock()
	defer s.RUnlock()

	return string(s.label)
}

// SetLabelWidth sets the screen width of the label. A value of 0 will cause the
// primitive to use the width of the label string.
func (s *Slider) SetLabelWidth(width int) {
	s.Lock()
	defer s.Unlock()

	s.labelWidth = width
}

// SetLabelColor sets the color of the label.
func (s *Slider) SetLabelColor(color tcell.Color) {
	s.Lock()
	defer s.Unlock()

	s.labelColor = color
}

// SetLabelColorFocused sets the color of the label when focused.
func (s *Slider) SetLabelColorFocused(color tcell.Color) {
	s.Lock()
	defer s.Unlock()

	s.labelColorFocused = color
}

// SetFieldBackgroundColor sets the background color of the input area.
func (s *Slider) SetFieldBackgroundColor(color tcell.Color) {
	s.Lock()
	defer s.Unlock()

	s.fieldBackgroundColor = color
}

// SetFieldBackgroundColorFocused sets the background color of the input area when focused.
func (s *Slider) SetFieldBackgroundColorFocused(color tcell.Color) {
	s.Lock()
	defer s.Unlock()

	s.fieldBackgroundColorFocused = color
}

// SetFieldTextColor sets the text color of the input area.
func (s *Slider) SetFieldTextColor(color tcell.Color) {
	s.Lock()
	defer s.Unlock()

	s.fieldTextColor = color
}

// SetFieldTextColorFocused sets the text color of the input area when focused.
func (s *Slider) SetFieldTextColorFocused(color tcell.Color) {
	s.Lock()
	defer s.Unlock()

	s.fieldTextColorFocused = color
}

// GetFieldHeight returns the height of the field.
func (s *Slider) GetFieldHeight() int {
	return 1
}

// GetFieldWidth returns this primitive's field width.
func (s *Slider) GetFieldWidth() int {
	return 0
}

// SetIncrement sets the amount the slider is incremented by when modified via
// keyboard.
func (s *Slider) SetIncrement(increment int) {
	s.Lock()
	defer s.Unlock()

	s.increment = increment
}

// SetChangedFunc sets a handler which is called when the value of this slider
// was changed by the user. The handler function receives the new value.
func (s *Slider) SetChangedFunc(handler func(value int)) {
	s.Lock()
	defer s.Unlock()

	s.changed = handler
}

// SetDoneFunc sets a handler which is called when the user is done using the
// slider. The callback function is provided with the key that was pressed,
// which is one of the following:
//
//   - KeyEscape: Abort text input.
//   - KeyTab: Move to the next field.
//   - KeyBacktab: Move to the previous field.
func (s *Slider) SetDoneFunc(handler func(key tcell.Key)) {
	s.Lock()
	defer s.Unlock()

	s.done = handler
}

// SetFinishedFunc sets a callback invoked when the user leaves this form item.
func (s *Slider) SetFinishedFunc(handler func(key tcell.Key)) {
	s.Lock()
	defer s.Unlock()

	s.finished = handler
}

// Draw draws this primitive onto the screen.
func (s *Slider) Draw(screen tcell.Screen) {
	if !s.GetVisible() {
		return
	}

	s.Box.Draw(screen)
	hasFocus := s.GetFocusable().HasFocus()

	s.Lock()

	// Select colors
	labelColor := s.labelColor
	fieldBackgroundColor := s.fieldBackgroundColor
	fieldTextColor := s.fieldTextColor
	if hasFocus {
		if s.labelColorFocused != ColorUnset {
			labelColor = s.labelColorFocused
		}
		if s.fieldBackgroundColorFocused != ColorUnset {
			fieldBackgroundColor = s.fieldBackgroundColorFocused
		}
		if s.fieldTextColorFocused != ColorUnset {
			fieldTextColor = s.fieldTextColorFocused
		}
	}

	// Prepare.
	x, y, width, height := s.GetInnerRect()
	rightLimit := x + width
	if height < 1 || rightLimit <= x {
		s.Unlock()
		return
	}

	// Draw label.
	if len(s.label) > 0 {
		if s.vertical {
			height--

			// TODO draw label on bottom
		} else {
			if s.labelWidth > 0 {
				labelWidth := s.labelWidth
				if labelWidth > rightLimit-x {
					labelWidth = rightLimit - x
				}
				Print(screen, []byte(s.label), x, y, labelWidth, AlignLeft, labelColor)
				x += labelWidth + 1
				width -= labelWidth + 1
			} else {
				_, drawnWidth := Print(screen, []byte(s.label), x, y, rightLimit-x, AlignLeft, labelColor)
				x += drawnWidth + 1
				width -= drawnWidth + 1
			}
		}
	}

	// Draw slider.
	s.Unlock()
	s.ProgressBar.SetRect(x, y, width, height)
	s.ProgressBar.SetEmptyColor(fieldBackgroundColor)
	s.ProgressBar.SetFilledColor(fieldTextColor)
	s.ProgressBar.Draw(screen)
}

// InputHandler returns the handler for this primitive.
func (s *Slider) InputHandler() func(event *tcell.EventKey, setFocus func(p Primitive)) {
	return s.WrapInputHandler(func(event *tcell.EventKey, setFocus func(p Primitive)) {
		if HitShortcut(event, Keys.Cancel, Keys.MovePreviousField, Keys.MoveNextField) {
			if s.done != nil {
				s.done(event.Key())
			}
			if s.finished != nil {
				s.finished(event.Key())
			}
			return
		}

		previous := s.progress

		if HitShortcut(event, Keys.MoveFirst, Keys.MoveFirst2) {
			s.SetProgress(0)
		} else if HitShortcut(event, Keys.MoveLast, Keys.MoveLast2) {
			s.SetProgress(s.max)
		} else if HitShortcut(event, Keys.MoveUp, Keys.MoveUp2, Keys.MoveRight, Keys.MoveRight2, Keys.MovePreviousField) {
			s.AddProgress(s.increment)
		} else if HitShortcut(event, Keys.MoveDown, Keys.MoveDown2, Keys.MoveLeft, Keys.MoveLeft2, Keys.MoveNextField) {
			s.AddProgress(s.increment * -1)
		}

		if s.progress != previous && s.changed != nil {
			s.changed(s.progress)
		}
	})
}

// MouseHandler returns the mouse handler for this primitive.
func (s *Slider) MouseHandler() func(action MouseAction, event *tcell.EventMouse, setFocus func(p Primitive)) (consumed bool, capture Primitive) {
	return s.WrapMouseHandler(func(action MouseAction, event *tcell.EventMouse, setFocus func(p Primitive)) (consumed bool, capture Primitive) {
		x, y := event.Position()
		if !s.InRect(x, y) {
			s.dragging = false
			return false, nil
		}

		// Process mouse event.
		if action == MouseLeftClick {
			setFocus(s)
			consumed = true
		}

		handleMouse := func() {
			if !s.ProgressBar.InRect(x, y) {
				s.dragging = false
				return
			}

			bx, by, bw, bh := s.GetInnerRect()
			var clickPos, clickRange int
			if s.ProgressBar.vertical {
				clickPos = (bh - 1) - (y - by)
				clickRange = bh - 1
			} else {
				clickPos = x - bx
				clickRange = bw - 1
			}
			setValue := int(math.Floor(float64(s.max) * (float64(clickPos) / float64(clickRange))))
			if setValue != s.progress {
				s.SetProgress(setValue)
				if s.changed != nil {
					s.changed(s.progress)
				}
			}
		}

		// Handle dragging. Clicks are implicitly handled by this logic.
		switch action {
		case MouseLeftDown:
			setFocus(s)
			consumed = true
			capture = s
			s.dragging = true

			handleMouse()
		case MouseMove:
			if s.dragging {
				consumed = true
				capture = s

				handleMouse()
			}
		case MouseLeftUp:
			if s.dragging {
				consumed = true
				s.dragging = false

				handleMouse()
			}
		}

		return
	})
}
