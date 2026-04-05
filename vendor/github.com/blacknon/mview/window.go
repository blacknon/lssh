package mview

import (
	"sync"

	"github.com/gdamore/tcell/v2"
)

// Window is a draggable, resizable frame around a primitive. Windows must be
// added to a WindowManager.
type Window struct {
	*Box

	primitive Primitive

	fullscreen bool

	normalX, normalY int
	normalW, normalH int

	dragX, dragY   int
	dragWX, dragWY int

	sync.RWMutex
}

// NewWindow returns a new window around the given primitive.
func NewWindow(primitive Primitive) *Window {
	w := &Window{
		Box:       NewBox(),
		primitive: primitive,
		dragWX:    -1,
		dragWY:    -1,
	}
	w.Box.focus = w
	return w
}

// SetFullscreen sets the flag indicating whether or not the the window should
// be drawn fullscreen.
func (w *Window) SetFullscreen(fullscreen bool) {
	w.Lock()
	defer w.Unlock()

	if w.fullscreen == fullscreen {
		return
	}

	w.fullscreen = fullscreen
	if w.fullscreen {
		w.normalX, w.normalY, w.normalW, w.normalH = w.GetRect()
	} else {
		w.SetRect(w.normalX, w.normalY, w.normalW, w.normalH)
	}
}

// Focus is called when this primitive receives focus.
func (w *Window) Focus(delegate func(p Primitive)) {
	w.Lock()
	defer w.Unlock()

	w.Box.Focus(delegate)

	w.primitive.Focus(delegate)
}

// Blur is called when this primitive loses focus.
func (w *Window) Blur() {
	w.Lock()
	defer w.Unlock()

	w.Box.Blur()

	w.primitive.Blur()
}

// HasFocus returns whether or not this primitive has focus.
func (w *Window) HasFocus() bool {
	w.RLock()
	defer w.RUnlock()

	focusable := w.primitive.GetFocusable()
	if focusable != nil {
		return focusable.HasFocus()
	}

	return w.Box.HasFocus()
}

// Draw draws this primitive onto the screen.
func (w *Window) Draw(screen tcell.Screen) {
	if !w.GetVisible() {
		return
	}

	w.RLock()
	defer w.RUnlock()

	w.Box.Draw(screen)

	x, y, width, height := w.GetInnerRect()
	w.primitive.SetRect(x, y, width, height)
	w.primitive.Draw(screen)
}

// InputHandler returns the handler for this primitive.
func (w *Window) InputHandler() func(event *tcell.EventKey, setFocus func(p Primitive)) {
	return w.primitive.InputHandler()
}

// MouseHandler returns the mouse handler for this primitive.
func (w *Window) MouseHandler() func(action MouseAction, event *tcell.EventMouse, setFocus func(p Primitive)) (consumed bool, capture Primitive) {
	return w.WrapMouseHandler(func(action MouseAction, event *tcell.EventMouse, setFocus func(p Primitive)) (consumed bool, capture Primitive) {
		if !w.InRect(event.Position()) {
			return false, nil
		}

		if action == MouseLeftDown || action == MouseMiddleDown || action == MouseRightDown {
			setFocus(w)
		}

		if action == MouseLeftDown {
			x, y, width, height := w.GetRect()
			mouseX, mouseY := event.Position()

			leftEdge := mouseX == x
			rightEdge := mouseX == x+width-1
			bottomEdge := mouseY == y+height-1
			topEdge := mouseY == y

			if mouseY >= y && mouseY <= y+height-1 {
				if leftEdge {
					w.dragX = -1
				} else if rightEdge {
					w.dragX = 1
				}
			}

			if mouseX >= x && mouseX <= x+width-1 {
				if bottomEdge {
					w.dragY = -1
				} else if topEdge {
					if leftEdge || rightEdge {
						w.dragY = 1
					} else {
						w.dragWX = mouseX - x
						w.dragWY = mouseY - y
					}
				}
			}
		}

		_, capture = w.primitive.MouseHandler()(action, event, setFocus)
		return true, capture
	})
}
