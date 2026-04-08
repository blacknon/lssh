package tvxterm

import (
	"fmt"
	"io"
	"os"
	"sync"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// View is a tview primitive that renders a terminal stream inside a boxed widget.
// Design intent:
//   - frontend: tview primitive
//   - state: tiny VT-like emulator
//   - backend: PTY/SSH/anything implementing Backend
//
// This mirrors the separation used by gowid's terminal widget: input/output are
// not tied to a subprocess launcher, and drawing happens from an intermediate
// terminal state rather than directly from raw bytes.
type View struct {
	*tview.Box

	app *tview.Application

	mu            sync.RWMutex
	backend       Backend
	emu           *Emulator
	debug         *os.File
	onBackendExit func(*View, error)
	onTitle       func(*View, string)
	focused       bool
	closed        bool
	scrollOffset  int
	lastTitle     string
}

// New creates a terminal view bound to the given tview application.
//
// The application reference is used for redraw scheduling and optional
// focus/paste integration. Passing nil is allowed for tests or headless use.
func New(app *tview.Application) *View {
	var debug *os.File
	if path := os.Getenv("TVIEW_TERM_DEBUG_IO"); path != "" {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err == nil {
			debug = f
		}
	}
	return &View{
		Box:   tview.NewBox(),
		app:   app,
		emu:   NewEmulator(80, 24),
		debug: debug,
	}
}

// Attach connects a backend to the view and starts reading from it.
//
// Callers typically create one backend per view and attach it once.
func (v *View) Attach(backend Backend) {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.backend = backend
	go v.readLoop(backend)
}

// SetBackendExitHandler installs a callback invoked when the attached backend
// read loop exits with an error or EOF.
func (v *View) SetBackendExitHandler(fn func(*View, error)) *View {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.onBackendExit = fn
	return v
}

// SetTitleHandler installs a callback for OSC title updates observed from the
// terminal stream.
func (v *View) SetTitleHandler(fn func(*View, string)) *View {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.onTitle = fn
	return v
}

// Close closes the current backend and releases debug-log resources.
func (v *View) Close() error {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.closed = true
	if v.debug != nil {
		_ = v.debug.Close()
		v.debug = nil
	}
	if v.backend != nil {
		return v.backend.Close()
	}
	return nil
}

func (v *View) Draw(screen tcell.Screen) {
	v.Box.DrawForSubclass(screen, v)

	x, y, width, height := v.GetInnerRect()
	if width <= 0 || height <= 0 {
		return
	}

	v.emu.Resize(width, height)
	v.resizeBackend(width, height)
	ss := v.emu.SnapshotAt(v.scrollOffset)

	for row := 0; row < ss.Rows; row++ {
		for col := 0; col < ss.Cols; col++ {
			cell := ss.Cells[row][col]
			if cell.Occupied && cell.Width == 0 {
				continue
			}
			screen.SetContent(x+col, y+row, cell.Ch, cell.Comb, drawStyle(cell.Style, ss.ReverseVideo))
		}
	}

	if v.focused && ss.CursorVis {
		screen.ShowCursor(x+ss.CursorX, y+ss.CursorY)
	}
}

func (v *View) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return v.WrapInputHandler(func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
		v.mu.RLock()
		backend := v.backend
		v.mu.RUnlock()
		if backend == nil {
			return
		}

		ss := v.emu.Snapshot()
		seq, ok := keyToBytes(event, ss.AppCursor || ss.AppKeypad)
		if !ok {
			return
		}
		v.sendInput(backend, seq)
	})
}

func (v *View) PasteHandler() func(text string, setFocus func(p tview.Primitive)) {
	return v.WrapPasteHandler(func(text string, setFocus func(p tview.Primitive)) {
		v.mu.RLock()
		backend := v.backend
		v.mu.RUnlock()
		if backend == nil || text == "" {
			return
		}

		ss := v.emu.Snapshot()
		payload := []byte(text)
		if ss.BracketedPaste {
			payload = append([]byte("\x1b[200~"), payload...)
			payload = append(payload, []byte("\x1b[201~")...)
		}
		v.sendInput(backend, payload)
	})
}

// SendKey writes a single key event to the attached backend using the same
// encoding rules as interactive input handling.
func (v *View) SendKey(event *tcell.EventKey) bool {
	if event == nil {
		return false
	}

	v.mu.RLock()
	backend := v.backend
	v.mu.RUnlock()
	if backend == nil {
		return false
	}

	ss := v.emu.Snapshot()
	seq, ok := keyToBytes(event, ss.AppCursor || ss.AppKeypad)
	if !ok {
		return false
	}
	v.sendInput(backend, seq)
	return true
}

// SendPaste writes pasted text to the attached backend, preserving bracketed
// paste behavior when enabled by the remote terminal.
func (v *View) SendPaste(text string) bool {
	if text == "" {
		return false
	}

	v.mu.RLock()
	backend := v.backend
	v.mu.RUnlock()
	if backend == nil {
		return false
	}

	ss := v.emu.Snapshot()
	payload := []byte(text)
	if ss.BracketedPaste {
		payload = append([]byte("\x1b[200~"), payload...)
		payload = append(payload, []byte("\x1b[201~")...)
	}
	v.sendInput(backend, payload)
	return true
}

func (v *View) Focus(delegate func(p tview.Primitive)) {
	v.focused = true
	v.reportFocus(true)
}

func (v *View) Blur() {
	v.focused = false
	v.reportFocus(false)
}

func (v *View) HasFocus() bool {
	return v.focused
}

func (v *View) MouseHandler() func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(p tview.Primitive)) (consumed bool, capture tview.Primitive) {
	return v.WrapMouseHandler(func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(p tview.Primitive)) (consumed bool, capture tview.Primitive) {
		if event == nil {
			return false, nil
		}
		x, y := event.Position()
		if !v.InRect(x, y) {
			return false, nil
		}
		v.sendMouse(action, event)
		if action == tview.MouseLeftDown {
			setFocus(v)
			return true, nil
		}
		return true, nil
	})
}

func (v *View) readLoop(backend Backend) {
	buf := make([]byte, 4096)
	for {
		n, err := backend.Read(buf)
		if n > 0 {
			v.logBytes("output", buf[:n])
			_, _ = v.emu.Write(buf[:n])
			v.syncTitle()
			for _, resp := range v.emu.DrainResponses() {
				v.logBytes("input", resp)
				_ = writeAll(backend, resp)
			}
			if v.app != nil {
				v.app.QueueUpdateDraw(func() {})
			}
		}
		if err != nil {
			v.mu.RLock()
			closed := v.closed
			handler := v.onBackendExit
			v.mu.RUnlock()
			if !closed && handler != nil {
				handler(v, err)
				return
			}
			if !closed && v.app != nil {
				v.app.QueueUpdate(func() {
					v.mu.Lock()
					v.closed = true
					v.mu.Unlock()
					v.app.Stop()
				})
			}
			return
		}
		v.mu.RLock()
		closed := v.closed
		v.mu.RUnlock()
		if closed {
			return
		}
	}
}

func (v *View) scrollBy(delta int) {
	v.mu.Lock()
	maxOffset := v.emu.Snapshot().ScrollbackRows
	v.scrollOffset = clamp(v.scrollOffset+delta, 0, maxOffset)
	v.mu.Unlock()
	v.requestRedrawAsync()
}

// ScrollbackUp moves the local scrollback view upward by the given number of
// lines. Non-positive values are treated as 1.
func (v *View) ScrollbackUp(lines int) {
	if lines <= 0 {
		lines = 1
	}
	v.scrollBy(lines)
}

// ScrollbackDown moves the local scrollback view downward by the given number
// of lines. Non-positive values are treated as 1.
func (v *View) ScrollbackDown(lines int) {
	if lines <= 0 {
		lines = 1
	}
	v.scrollBy(-lines)
}

// ScrollbackPageUp moves the local scrollback view by one visible page.
func (v *View) ScrollbackPageUp() {
	v.scrollBy(v.emu.Snapshot().Rows)
}

// ScrollbackPageDown moves the local scrollback view down by one visible page.
func (v *View) ScrollbackPageDown() {
	v.scrollBy(-v.emu.Snapshot().Rows)
}

// ScrollbackStatus returns the current local scroll offset and the total number
// of available scrollback rows.
func (v *View) ScrollbackStatus() (offset int, rows int) {
	v.mu.RLock()
	offset = v.scrollOffset
	v.mu.RUnlock()
	rows = v.emu.Snapshot().ScrollbackRows
	return offset, rows
}

func (v *View) scrollToTop() {
	v.mu.Lock()
	v.scrollOffset = v.emu.Snapshot().ScrollbackRows
	v.mu.Unlock()
	v.requestRedrawAsync()
}

func (v *View) scrollToBottom() {
	v.mu.Lock()
	v.scrollOffset = 0
	v.mu.Unlock()
	v.requestRedrawAsync()
}

func (v *View) resetScrollback() {
	v.mu.Lock()
	v.scrollOffset = 0
	v.mu.Unlock()
}

func (v *View) requestRedrawAsync() {
	if v.app == nil {
		return
	}
	go v.app.QueueUpdateDraw(func() {})
}

func (v *View) syncTitle() {
	ss := v.emu.Snapshot()
	if ss.WindowTitle == "" || ss.WindowTitle == v.lastTitle {
		return
	}

	v.mu.RLock()
	handler := v.onTitle
	v.mu.RUnlock()
	v.lastTitle = ss.WindowTitle
	if handler != nil {
		handler(v, ss.WindowTitle)
	}
}

func (v *View) reportFocus(focused bool) {
	v.mu.RLock()
	backend := v.backend
	v.mu.RUnlock()
	if backend == nil {
		return
	}

	ss := v.emu.Snapshot()
	if !ss.FocusReporting {
		return
	}
	if focused {
		v.sendInput(backend, []byte("\x1b[I"))
		return
	}
	v.sendInput(backend, []byte("\x1b[O"))
}

func (v *View) logBytes(dir string, p []byte) {
	v.mu.RLock()
	debug := v.debug
	v.mu.RUnlock()
	if debug == nil {
		return
	}
	_, _ = fmt.Fprintf(debug, "%s %q\n", dir, p)
}

func (v *View) sendInput(backend Backend, payload []byte) {
	if backend == nil || len(payload) == 0 {
		return
	}
	v.resetScrollback()
	v.logBytes("input", payload)
	_ = writeAll(backend, payload)
}

func drawStyle(style tcell.Style, reverseVideo bool) tcell.Style {
	if !reverseVideo {
		return style
	}
	fg, bg, attr := style.Decompose()
	return tcell.StyleDefault.Foreground(bg).Background(fg).Attributes(attr)
}

func (v *View) sendMouse(action tview.MouseAction, event *tcell.EventMouse) {
	v.mu.RLock()
	backend := v.backend
	v.mu.RUnlock()
	if backend == nil || event == nil {
		return
	}

	x, y, _, _ := v.GetInnerRect()
	ss := v.emu.Snapshot()
	seq, ok := mouseEventToBytes(action, event, ss, x, y)
	if !ok {
		return
	}
	v.sendInput(backend, seq)
}

func mouseEventToBytes(action tview.MouseAction, event *tcell.EventMouse, ss Snapshot, originX, originY int) ([]byte, bool) {
	if !mouseReportingEnabled(ss) {
		return nil, false
	}

	x, y := event.Position()
	col := x - originX + 1
	row := y - originY + 1
	if col < 1 || row < 1 {
		return nil, false
	}

	btn, suffix, ok := mouseReportCode(action, event, ss)
	if !ok {
		return nil, false
	}
	btn += mouseModifierBits(event.Modifiers())

	if ss.MouseSGR {
		return []byte(fmt.Sprintf("\x1b[<%d;%d;%d%c", btn, col, row, suffix)), true
	}
	if col > 223 || row > 223 {
		return nil, false
	}
	return []byte{0x1b, '[', 'M', byte(btn + 32), byte(col + 32), byte(row + 32)}, true
}

func mouseReportingEnabled(ss Snapshot) bool {
	return ss.MouseX10 || ss.MouseVT200 || ss.MouseButtonEvt || ss.MouseAnyEvt
}

func mouseReportCode(action tview.MouseAction, event *tcell.EventMouse, ss Snapshot) (btn int, suffix rune, ok bool) {
	switch action {
	case tview.MouseLeftDown:
		return 0, 'M', true
	case tview.MouseMiddleDown:
		return 1, 'M', true
	case tview.MouseRightDown:
		return 2, 'M', true
	case tview.MouseLeftUp, tview.MouseMiddleUp, tview.MouseRightUp:
		if ss.MouseX10 {
			return 0, 0, false
		}
		return 3, 'm', true
	case tview.MouseScrollUp:
		if ss.MouseX10 {
			return 0, 0, false
		}
		return 64, 'M', true
	case tview.MouseScrollDown:
		if ss.MouseX10 {
			return 0, 0, false
		}
		return 65, 'M', true
	case tview.MouseScrollLeft:
		if ss.MouseX10 {
			return 0, 0, false
		}
		return 66, 'M', true
	case tview.MouseScrollRight:
		if ss.MouseX10 {
			return 0, 0, false
		}
		return 67, 'M', true
	case tview.MouseMove:
		if !ss.MouseAnyEvt && !(ss.MouseButtonEvt && mousePressedButton(event.Buttons()) >= 0) {
			return 0, 0, false
		}
		base := mousePressedButton(event.Buttons())
		if base < 0 {
			base = 3
		}
		return base + 32, 'M', true
	default:
		return 0, 0, false
	}
}

func mousePressedButton(mask tcell.ButtonMask) int {
	switch {
	case mask&tcell.Button1 != 0:
		return 0
	case mask&tcell.Button2 != 0:
		return 1
	case mask&tcell.Button3 != 0:
		return 2
	default:
		return -1
	}
}

func mouseModifierBits(mod tcell.ModMask) int {
	var bits int
	if mod&tcell.ModShift != 0 {
		bits |= 4
	}
	if mod&tcell.ModAlt != 0 {
		bits |= 8
	}
	if mod&tcell.ModCtrl != 0 {
		bits |= 16
	}
	return bits
}

func (v *View) resizeBackend(cols, rows int) {
	v.mu.RLock()
	backend := v.backend
	v.mu.RUnlock()
	if backend != nil {
		_ = backend.Resize(cols, rows)
	}
}

func keyToBytes(ev *tcell.EventKey, appCursor bool) ([]byte, bool) {
	if seq, ok := modifiedKeyToBytes(ev); ok {
		return seq, true
	}

	switch ev.Key() {
	case tcell.KeyRune:
		return []byte(string(ev.Rune())), true
	case tcell.KeyEnter:
		return []byte("\r"), true
	case tcell.KeyTab:
		return []byte("\t"), true
	case tcell.KeyBackspace:
		return []byte{0x08}, true
	case tcell.KeyBackspace2:
		return []byte{0x7f}, true
	case tcell.KeyEsc:
		return []byte{0x1b}, true
	case tcell.KeyHome:
		if appCursor {
			return []byte("\x1bOH"), true
		}
		return []byte("\x1b[H"), true
	case tcell.KeyEnd:
		if appCursor {
			return []byte("\x1bOF"), true
		}
		return []byte("\x1b[F"), true
	case tcell.KeyDelete:
		return []byte("\x1b[3~"), true
	case tcell.KeyInsert:
		return []byte("\x1b[2~"), true
	case tcell.KeyPgUp:
		return []byte("\x1b[5~"), true
	case tcell.KeyPgDn:
		return []byte("\x1b[6~"), true
	case tcell.KeyBacktab:
		return []byte("\x1b[Z"), true
	case tcell.KeyUp:
		if appCursor {
			return []byte("\x1bOA"), true
		}
		return []byte("\x1b[A"), true
	case tcell.KeyDown:
		if appCursor {
			return []byte("\x1bOB"), true
		}
		return []byte("\x1b[B"), true
	case tcell.KeyRight:
		if appCursor {
			return []byte("\x1bOC"), true
		}
		return []byte("\x1b[C"), true
	case tcell.KeyLeft:
		if appCursor {
			return []byte("\x1bOD"), true
		}
		return []byte("\x1b[D"), true
	case tcell.KeyCtrlC:
		return []byte{0x03}, true
	case tcell.KeyCtrlD:
		return []byte{0x04}, true
	case tcell.KeyCtrlL:
		return []byte{0x0c}, true
	default:
		return nil, false
	}
}

func modifiedKeyToBytes(ev *tcell.EventKey) ([]byte, bool) {
	mod := ev.Modifiers()

	if ev.Key() >= tcell.KeyCtrlA && ev.Key() <= tcell.KeyCtrlZ {
		return []byte{byte(ev.Key() - tcell.KeyCtrlA + 1)}, true
	}

	switch ev.Key() {
	case tcell.KeyCtrlSpace:
		return []byte{0x00}, true
	}

	if mod&tcell.ModCtrl != 0 {
		switch ev.Key() {
		case tcell.KeyLeft:
			return []byte("\x1b[1;5D"), true
		case tcell.KeyRight:
			return []byte("\x1b[1;5C"), true
		case tcell.KeyDelete:
			return []byte("\x1b[3;5~"), true
		case tcell.KeyBackspace, tcell.KeyBackspace2:
			return []byte{0x17}, true
		}
	}

	if mod&tcell.ModAlt != 0 {
		switch ev.Key() {
		case tcell.KeyBackspace:
			return []byte{0x1b, 0x08}, true
		case tcell.KeyBackspace2:
			return []byte{0x1b, 0x7f}, true
		case tcell.KeyLeft:
			return []byte{0x1b, 'b'}, true
		case tcell.KeyRight:
			return []byte{0x1b, 'f'}, true
		case tcell.KeyDelete:
			return []byte{0x1b, 'd'}, true
		case tcell.KeyUp:
			return []byte("\x1b[1;3A"), true
		case tcell.KeyDown:
			return []byte("\x1b[1;3B"), true
		case tcell.KeyRune:
			buf := make([]byte, 1+utf8.RuneLen(ev.Rune()))
			buf[0] = 0x1b
			n := utf8.EncodeRune(buf[1:], ev.Rune())
			return buf[:1+n], true
		}
	}

	return nil, false
}

func writeAll(w io.Writer, p []byte) error {
	for len(p) > 0 {
		n, err := w.Write(p)
		if n > 0 {
			p = p[n:]
		}
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
	}
	return nil
}
