package mview

import (
	"sync"

	"github.com/gdamore/tcell/v2"
)

// WindowManager provides an area which windows may be added to.
type WindowManager struct {
	*Box

	windows []*Window

	sync.RWMutex
}

// NewWindowManager returns a new window manager.
func NewWindowManager() *WindowManager {
	return &WindowManager{
		Box: NewBox(),
	}
}

// Add adds a window to the manager.
func (wm *WindowManager) Add(w ...*Window) {
	wm.Lock()
	defer wm.Unlock()

	for _, window := range w {
		window.SetBorder(true)
	}

	wm.windows = append(wm.windows, w...)
}

// Clear removes all windows from the manager.
func (wm *WindowManager) Clear() {
	wm.Lock()
	defer wm.Unlock()

	wm.windows = nil
}

// Focus is called when this primitive receives focus.
func (wm *WindowManager) Focus(delegate func(p Primitive)) {
	wm.Lock()
	defer wm.Unlock()

	if len(wm.windows) == 0 {
		return
	}

	wm.windows[len(wm.windows)-1].Focus(delegate)
}

// HasFocus returns whether or not this primitive has focus.
func (wm *WindowManager) HasFocus() bool {
	wm.RLock()
	defer wm.RUnlock()

	for _, w := range wm.windows {
		if w.HasFocus() {
			return true
		}
	}

	return false
}

// Draw draws this primitive onto the screen.
func (wm *WindowManager) Draw(screen tcell.Screen) {
	if !wm.GetVisible() {
		return
	}

	wm.RLock()
	defer wm.RUnlock()

	wm.Box.Draw(screen)

	x, y, width, height := wm.GetInnerRect()

	var hasFullScreen bool
	for _, w := range wm.windows {
		if !w.fullscreen || !w.GetVisible() {
			continue
		}

		hasFullScreen = true
		w.SetRect(x-1, y, width+2, height+1)

		w.Draw(screen)
	}
	if hasFullScreen {
		return
	}

	for _, w := range wm.windows {
		if !w.GetVisible() {
			continue
		}

		// Reposition out of bounds windows
		margin := 3
		wx, wy, ww, wh := w.GetRect()
		ox, oy := wx, wy
		if wx > x+width-margin {
			wx = x + width - margin
		}
		if wx+ww < x+margin {
			wx = x - ww + margin
		}
		if wy > y+height-margin {
			wy = y + height - margin
		}
		if wy < y {
			wy = y // No top margin
		}
		if wx != ox || wy != oy {
			w.SetRect(wx, wy, ww, wh)
		}

		w.Draw(screen)
	}
}

// MouseHandler returns the mouse handler for this primitive.
func (wm *WindowManager) MouseHandler() func(action MouseAction, event *tcell.EventMouse, setFocus func(p Primitive)) (consumed bool, capture Primitive) {
	return wm.WrapMouseHandler(func(action MouseAction, event *tcell.EventMouse, setFocus func(p Primitive)) (consumed bool, capture Primitive) {
		if !wm.InRect(event.Position()) {
			return false, nil
		}

		if action == MouseMove {
			mouseX, mouseY := event.Position()

			for _, w := range wm.windows {
				if w.dragWX != -1 || w.dragWY != -1 {
					offsetX := w.x - mouseX
					offsetY := w.y - mouseY

					w.x -= offsetX + w.dragWX
					w.y -= offsetY + w.dragWY

					w.updateInnerRect()
					consumed = true
				}

				if w.dragX != 0 {
					if w.dragX == -1 {
						offsetX := w.x - mouseX

						if w.width+offsetX >= Styles.WindowMinWidth {
							w.x -= offsetX
							w.width += offsetX
						}
					} else {
						offsetX := mouseX - (w.x + w.width) + 1

						if w.width+offsetX >= Styles.WindowMinWidth {
							w.width += offsetX
						}
					}

					w.updateInnerRect()
					consumed = true
				}

				if w.dragY != 0 {
					if w.dragY == -1 {
						offsetY := mouseY - (w.y + w.height) + 1

						if w.height+offsetY >= Styles.WindowMinHeight {
							w.height += offsetY
						}
					} else {
						offsetY := w.y - mouseY

						if w.height+offsetY >= Styles.WindowMinHeight {
							w.y -= offsetY
							w.height += offsetY
						}
					}

					w.updateInnerRect()
					consumed = true
				}
			}
		} else if action == MouseLeftUp {
			for _, w := range wm.windows {
				w.dragX, w.dragY = 0, 0
				w.dragWX, w.dragWY = -1, -1
			}
		}

		// Focus window on mousedown
		var (
			focusWindow      *Window
			focusWindowIndex int
		)
		for i := len(wm.windows) - 1; i >= 0; i-- {
			if wm.windows[i].InRect(event.Position()) {
				focusWindow = wm.windows[i]
				focusWindowIndex = i
				break
			}
		}
		if focusWindow != nil {
			if action == MouseLeftDown || action == MouseMiddleDown || action == MouseRightDown {
				for _, w := range wm.windows {
					if w != focusWindow {
						w.Blur()
					}
				}

				wm.windows = append(append(wm.windows[:focusWindowIndex], wm.windows[focusWindowIndex+1:]...), focusWindow)
			}

			return focusWindow.MouseHandler()(action, event, setFocus)
		}

		return consumed, nil
	})
}
