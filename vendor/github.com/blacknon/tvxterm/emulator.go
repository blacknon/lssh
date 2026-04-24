package tvxterm

import (
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
)

// Emulator is a deliberately small VT-like emulator.
//
// It is responsible for converting terminal bytes into a cell grid plus a small
// amount of terminal state. Applications embedding View usually do not need to
// use Emulator directly, but it remains exported for testing and advanced use.
type Emulator struct {
	mu sync.RWMutex

	cols int
	rows int

	cursorX      int
	cursorY      int
	savedX       int
	savedY       int
	scrollTop    int
	scrollBottom int

	cells [][]Cell
	style StyleState

	utf8buf []byte
	state   parseState
	csibuf  []byte
	oscbuf  []byte
	escbuf  byte

	wrapPending    bool
	autoWrap       bool
	cursorVis      bool
	cursorBlink    bool
	appCursor      bool
	appKeypad      bool
	columnMode132  bool
	originMode     bool
	reverseVideo   bool
	bracketedPaste bool
	focusReporting bool
	mouseX10       bool
	mouseVT200     bool
	mouseButtonEvt bool
	mouseAnyEvt    bool
	mouseSGR       bool
	responses      [][]byte
	windowTitle    string
	g0Charset      charsetSet
	g1Charset      charsetSet
	useG1Charset   bool

	scrollback      [][]Cell
	maxScrollback   int
	usingAlt        bool
	savedPrimary    *screenState
	hasScrollRegion bool
}

type screenState struct {
	cursorX         int
	cursorY         int
	savedX          int
	savedY          int
	scrollTop       int
	scrollBottom    int
	hasScrollRegion bool
	originMode      bool
	g0Charset       charsetSet
	g1Charset       charsetSet
	useG1Charset    bool
	cells           [][]Cell
	scrollback      [][]Cell
	style           StyleState
	wrapPending     bool
}

// Cell is a single rendered terminal cell.
type Cell struct {
	Ch       rune
	Comb     []rune
	Style    tcell.Style
	Width    int
	Occupied bool
}

// StyleState is the emulator's logical text style before it is converted into
// a tcell.Style for drawing.
type StyleState struct {
	FG        tcell.Color
	BG        tcell.Color
	Bold      bool
	Underline bool
	Reverse   bool
}

type parseState int

type charsetSet uint8

const (
	stateGround parseState = iota
	stateEsc
	stateEscIntermediate
	stateCSI
	stateOSC
	stateOSCMayTerminate
	stateControlString
	stateControlStringMayTerminate
)

const (
	charsetASCII charsetSet = iota
	charsetDECSpecial
)

// NewEmulator creates an emulator with the given initial size.
func NewEmulator(cols, rows int) *Emulator {
	e := &Emulator{cursorVis: true, autoWrap: true, maxScrollback: 1000}
	e.Resize(cols, rows)
	e.scrollTop = 0
	e.scrollBottom = max(0, e.rows-1)
	e.resetStyle()
	return e
}

// Resize changes the emulator's visible cell dimensions.
func (e *Emulator) Resize(cols, rows int) {
	if cols <= 0 {
		cols = 1
	}
	if rows <= 0 {
		rows = 1
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	e.cols = cols
	e.rows = rows
	e.cells = e.resizeCells(e.cells, cols, rows)
	e.scrollback = e.resizeHistory(e.scrollback, cols)
	if !e.hasScrollRegion {
		e.scrollTop = 0
		e.scrollBottom = rows - 1
	} else if e.scrollBottom < e.scrollTop || e.scrollBottom >= rows {
		e.scrollTop = 0
		e.scrollBottom = rows - 1
		e.hasScrollRegion = false
	}
	if e.savedPrimary != nil {
		e.savedPrimary.cells = e.resizeCells(e.savedPrimary.cells, cols, rows)
		e.savedPrimary.scrollback = e.resizeHistory(e.savedPrimary.scrollback, cols)
		if !e.savedPrimary.hasScrollRegion {
			e.savedPrimary.scrollTop = 0
			e.savedPrimary.scrollBottom = rows - 1
		} else if e.savedPrimary.scrollBottom < e.savedPrimary.scrollTop || e.savedPrimary.scrollBottom >= rows {
			e.savedPrimary.scrollTop = 0
			e.savedPrimary.scrollBottom = rows - 1
			e.savedPrimary.hasScrollRegion = false
		}
		if e.savedPrimary.cursorX >= cols {
			e.savedPrimary.cursorX = cols - 1
		}
		if e.savedPrimary.cursorY >= rows {
			e.savedPrimary.cursorY = rows - 1
		}
		if e.savedPrimary.savedX >= cols {
			e.savedPrimary.savedX = cols - 1
		}
		if e.savedPrimary.savedY >= rows {
			e.savedPrimary.savedY = rows - 1
		}
	}
	if e.cursorX >= cols {
		e.cursorX = cols - 1
	}
	if e.cursorY >= rows {
		e.cursorY = rows - 1
	}
}

// Snapshot returns a copy of the current visible screen state.
func (e *Emulator) Snapshot() Snapshot {
	e.mu.RLock()
	defer e.mu.RUnlock()

	rows := make([][]Cell, e.rows)
	for y := 0; y < e.rows; y++ {
		rows[y] = make([]Cell, e.cols)
		copy(rows[y], e.cells[y])
	}

	return Snapshot{
		Cols:           e.cols,
		Rows:           e.rows,
		CursorX:        e.cursorX,
		CursorY:        e.cursorY,
		CursorVis:      e.cursorVis,
		CursorBlink:    e.cursorBlink,
		AppCursor:      e.appCursor,
		AppKeypad:      e.appKeypad,
		ColumnMode132:  e.columnMode132,
		AutoWrap:       e.autoWrap,
		UsingAlt:       e.usingAlt,
		ReverseVideo:   e.reverseVideo,
		BracketedPaste: e.bracketedPaste,
		FocusReporting: e.focusReporting,
		MouseX10:       e.mouseX10,
		MouseVT200:     e.mouseVT200,
		MouseButtonEvt: e.mouseButtonEvt,
		MouseAnyEvt:    e.mouseAnyEvt,
		MouseSGR:       e.mouseSGR,
		ScrollTop:      e.scrollTop,
		ScrollBottom:   e.scrollBottom,
		ScrollbackRows: len(e.scrollback),
		WindowTitle:    e.windowTitle,
		Cells:          rows,
	}
}

// SnapshotAt returns a view of the screen including local scrollback.
//
// An offset of 0 returns the live screen. Positive offsets walk backward into
// retained scrollback rows.
func (e *Emulator) SnapshotAt(offset int) Snapshot {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if offset < 0 {
		offset = 0
	}
	if offset > len(e.scrollback) {
		offset = len(e.scrollback)
	}

	allRows := make([][]Cell, 0, len(e.scrollback)+len(e.cells))
	for _, row := range e.scrollback {
		copied := make([]Cell, len(row))
		copy(copied, row)
		allRows = append(allRows, copied)
	}
	for _, row := range e.cells {
		copied := make([]Cell, len(row))
		copy(copied, row)
		allRows = append(allRows, copied)
	}

	start := max(0, len(allRows)-e.rows-offset)
	end := min(len(allRows), start+e.rows)
	rows := make([][]Cell, e.rows)
	for y := 0; y < e.rows; y++ {
		rows[y] = make([]Cell, e.cols)
		for x := 0; x < e.cols; x++ {
			rows[y][x] = e.blankCell()
		}
	}
	for y := start; y < end; y++ {
		copy(rows[y-start], allRows[y])
	}

	cursorVis := e.cursorVis && offset == 0
	cursorY := e.cursorY
	if offset > 0 {
		cursorY = max(0, min(e.rows-1, e.cursorY-offset))
	}

	return Snapshot{
		Cols:           e.cols,
		Rows:           e.rows,
		CursorX:        e.cursorX,
		CursorY:        cursorY,
		CursorVis:      cursorVis,
		CursorBlink:    e.cursorBlink,
		AppCursor:      e.appCursor,
		AppKeypad:      e.appKeypad,
		ColumnMode132:  e.columnMode132,
		AutoWrap:       e.autoWrap,
		UsingAlt:       e.usingAlt,
		ReverseVideo:   e.reverseVideo,
		BracketedPaste: e.bracketedPaste,
		FocusReporting: e.focusReporting,
		MouseX10:       e.mouseX10,
		MouseVT200:     e.mouseVT200,
		MouseButtonEvt: e.mouseButtonEvt,
		MouseAnyEvt:    e.mouseAnyEvt,
		MouseSGR:       e.mouseSGR,
		ScrollTop:      e.scrollTop,
		ScrollBottom:   e.scrollBottom,
		ScrollbackRows: len(e.scrollback),
		WindowTitle:    e.windowTitle,
		Cells:          rows,
	}
}

// Snapshot is an immutable copy of the emulator's visible state.
type Snapshot struct {
	Cols           int
	Rows           int
	CursorX        int
	CursorY        int
	CursorVis      bool
	CursorBlink    bool
	AppCursor      bool
	AppKeypad      bool
	ColumnMode132  bool
	AutoWrap       bool
	UsingAlt       bool
	ReverseVideo   bool
	BracketedPaste bool
	FocusReporting bool
	MouseX10       bool
	MouseVT200     bool
	MouseButtonEvt bool
	MouseAnyEvt    bool
	MouseSGR       bool
	ScrollTop      int
	ScrollBottom   int
	ScrollbackRows int
	WindowTitle    string
	Cells          [][]Cell
}

// Write feeds terminal output bytes into the emulator.
func (e *Emulator) Write(p []byte) (int, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, b := range p {
		e.feedByte(b)
	}
	return len(p), nil
}

// DrainResponses returns terminal responses generated by the emulator, such as
// DSR or DA replies, and clears the pending response queue.
func (e *Emulator) DrainResponses() [][]byte {
	e.mu.Lock()
	defer e.mu.Unlock()

	out := e.responses
	e.responses = nil
	return out
}

func (e *Emulator) feedByte(b byte) {
	switch e.state {
	case stateGround:
		e.handleGroundByte(b)
	case stateEsc:
		e.handleEscByte(b)
	case stateEscIntermediate:
		e.handleEscIntermediateByte(b)
	case stateCSI:
		e.handleCSIByte(b)
	case stateOSC:
		e.handleOSCByte(b)
	case stateOSCMayTerminate:
		e.handleOSCMayTerminateByte(b)
	case stateControlString:
		e.handleControlStringByte(b)
	case stateControlStringMayTerminate:
		e.handleControlStringMayTerminateByte(b)
	}
}

func (e *Emulator) handleGroundByte(b byte) {
	switch b {
	case 0x1b:
		e.wrapPending = false
		e.state = stateEsc
		return
	case '\r':
		e.cursorX = 0
		e.wrapPending = false
		return
	case '\n':
		e.cursorX = 0
		e.wrapPending = false
		e.lineFeed()
		return
	case '\b':
		e.wrapPending = false
		if e.cursorX > 0 {
			e.cursorX--
		}
		return
	case '\t':
		e.wrapPending = false
		next := ((e.cursorX / 8) + 1) * 8
		for e.cursorX < next {
			e.putRune(' ')
		}
		return
	case 0x0e:
		e.useG1Charset = true
		return
	case 0x0f:
		e.useG1Charset = false
		return
	}

	if b < 0x20 {
		return
	}

	e.utf8buf = append(e.utf8buf, b)
	if !utf8.FullRune(e.utf8buf) {
		return
	}
	r, size := utf8.DecodeRune(e.utf8buf)
	if r == utf8.RuneError && size == 1 {
		e.utf8buf = e.utf8buf[:0]
		return
	}
	e.utf8buf = e.utf8buf[:0]
	e.putRune(r)
}

func (e *Emulator) handleEscByte(b byte) {
	e.wrapPending = false
	switch b {
	case '[':
		e.state = stateCSI
		e.csibuf = e.csibuf[:0]
	case ']':
		e.oscbuf = e.oscbuf[:0]
		e.state = stateOSC
	case 'P', '_', '^', 'X':
		e.state = stateControlString
	case '(', ')', '*', '+', '-', '.', '/':
		e.escbuf = b
		e.state = stateEscIntermediate
	case '=':
		e.appKeypad = true
		e.state = stateGround
	case '>':
		e.appKeypad = false
		e.state = stateGround
	case '7':
		e.saveCursor()
		e.state = stateGround
	case '8':
		e.restoreCursor()
		e.state = stateGround
	case 'D':
		e.index()
		e.state = stateGround
	case 'E':
		e.cursorX = 0
		e.index()
		e.state = stateGround
	case 'M':
		e.reverseIndex()
		e.state = stateGround
	case 'c':
		e.reset()
		e.state = stateGround
	default:
		e.state = stateGround
	}
}

func (e *Emulator) handleEscIntermediateByte(b byte) {
	switch e.escbuf {
	case '(':
		e.g0Charset = parseCharsetSet(b)
	case ')':
		e.g1Charset = parseCharsetSet(b)
	}
	e.state = stateGround
	e.escbuf = 0
}

func (e *Emulator) handleCSIByte(b byte) {
	if b >= 0x20 && b <= 0x3f {
		e.csibuf = append(e.csibuf, b)
		return
	}
	if b < 0x40 || b > 0x7e {
		e.state = stateGround
		e.csibuf = e.csibuf[:0]
		e.wrapPending = false
		return
	}

	rawParams := string(e.csibuf)
	params := parseCSIParams(rawParams)
	e.csibuf = e.csibuf[:0]
	e.state = stateGround
	e.wrapPending = false

	switch b {
	case 'A':
		e.cursorY = clamp(e.cursorY-defaultOne(params, 0), e.originTop(), e.originBottom())
	case 'B':
		e.cursorY = clamp(e.cursorY+defaultOne(params, 0), e.originTop(), e.originBottom())
	case 'C':
		e.cursorX = min(e.cols-1, e.cursorX+defaultOne(params, 0))
	case 'D':
		e.cursorX = max(0, e.cursorX-defaultOne(params, 0))
	case 'E':
		e.cursorY = clamp(e.cursorY+defaultOne(params, 0), e.originTop(), e.originBottom())
		e.cursorX = 0
	case 'F':
		e.cursorY = clamp(e.cursorY-defaultOne(params, 0), e.originTop(), e.originBottom())
		e.cursorX = 0
	case 'G':
		e.cursorX = clamp(defaultOne(params, 0)-1, 0, e.cols-1)
	case 'd':
		e.cursorY = e.clampRowParam(defaultOne(params, 0))
	case 'H', 'f':
		row := 1
		col := 1
		if len(params) >= 1 && params[0] > 0 {
			row = params[0]
		}
		if len(params) >= 2 && params[1] > 0 {
			col = params[1]
		}
		e.cursorY = e.clampRowParam(row)
		e.cursorX = clamp(col-1, 0, e.cols-1)
	case 'J':
		mode := defaultZero(params, 0)
		e.eraseDisplay(mode)
	case 'K':
		mode := defaultZero(params, 0)
		e.eraseLine(mode)
	case '@':
		e.insertChars(defaultOne(params, 0))
	case 'P':
		e.deleteChars(defaultOne(params, 0))
	case 'X':
		e.eraseChars(defaultOne(params, 0))
	case 'L':
		e.insertLines(defaultOne(params, 0))
	case 'M':
		e.deleteLines(defaultOne(params, 0))
	case 'S':
		e.scrollUp(defaultOne(params, 0))
	case 'T':
		e.scrollDown(defaultOne(params, 0))
	case 'm':
		e.applySGR(params)
	case 's':
		e.saveCursor()
	case 'u':
		e.restoreCursor()
	case 'r':
		e.setScrollRegion(params)
	case 'h', 'l':
		e.applyMode(rawParams, params, b == 'h')
	case 'n':
		e.handleDeviceStatusReport(params)
	case 'c':
		e.handleDeviceAttributes(rawParams, params)
	}
}

func (e *Emulator) handleOSCByte(b byte) {
	switch b {
	case 0x07:
		e.finishOSC()
		e.state = stateGround
	case 0x1b:
		e.state = stateOSCMayTerminate
	default:
		e.oscbuf = append(e.oscbuf, b)
	}
}

func (e *Emulator) handleOSCMayTerminateByte(b byte) {
	if b == '\\' {
		e.finishOSC()
		e.state = stateGround
		return
	}
	e.oscbuf = append(e.oscbuf, 0x1b)
	e.state = stateOSC
	e.handleOSCByte(b)
}

func (e *Emulator) finishOSC() {
	e.handleOSC(string(e.oscbuf))
	e.oscbuf = e.oscbuf[:0]
}

func (e *Emulator) handleControlStringByte(b byte) {
	switch b {
	case 0x1b:
		e.state = stateControlStringMayTerminate
	case 0x18, 0x1a:
		e.state = stateGround
	}
}

func (e *Emulator) handleControlStringMayTerminateByte(b byte) {
	if b == '\\' {
		e.state = stateGround
		return
	}
	e.state = stateControlString
	e.handleControlStringByte(b)
}

func (e *Emulator) handleOSC(payload string) {
	if payload == "" {
		return
	}
	parts := strings.SplitN(payload, ";", 2)
	if len(parts) != 2 {
		return
	}
	switch parts[0] {
	case "0", "2":
		e.windowTitle = parts[1]
	}
}

func (e *Emulator) putRune(r rune) {
	r = e.translateRune(r)
	if e.wrapPending {
		e.cursorX = 0
		e.lineFeed()
		e.wrapPending = false
	}

	if e.cursorY < 0 || e.cursorY >= e.rows {
		return
	}
	if e.cursorX < 0 || e.cursorX >= e.cols {
		e.lineFeed()
	}
	if e.cursorX < 0 || e.cursorX >= e.cols {
		return
	}

	width := runewidth.RuneWidth(r)
	if width <= 0 {
		e.appendCombiningRune(r)
		return
	}
	if width > e.cols {
		width = e.cols
	}
	if e.cursorX+width > e.cols {
		e.cursorX = 0
		e.lineFeed()
	}
	if e.cursorY < 0 || e.cursorY >= e.rows {
		return
	}

	e.cells[e.cursorY][e.cursorX] = Cell{
		Ch:       r,
		Style:    e.makeStyle(),
		Width:    width,
		Occupied: true,
	}
	for i := 1; i < width; i++ {
		e.cells[e.cursorY][e.cursorX+i] = Cell{
			Style:    e.makeStyle(),
			Width:    0,
			Occupied: true,
		}
	}
	if e.cursorX+width == e.cols {
		if !e.autoWrap {
			e.cursorX = e.cols - 1
			return
		}
		e.wrapPending = true
		return
	}
	e.cursorX += width
}

func (e *Emulator) translateRune(r rune) rune {
	if r > 0x7f {
		return r
	}
	active := e.g0Charset
	if e.useG1Charset {
		active = e.g1Charset
	}
	if active != charsetDECSpecial {
		return r
	}
	if mapped, ok := decSpecialGraphics[r]; ok {
		return mapped
	}
	return r
}

func (e *Emulator) appendCombiningRune(r rune) {
	if e.cursorX > 0 {
		cell := &e.cells[e.cursorY][e.cursorX-1]
		if cell.Occupied && cell.Width > 0 {
			cell.Comb = append(cell.Comb, r)
		}
	}
}

func (e *Emulator) lineFeed() {
	bottom := e.activeScrollBottom()
	if e.cursorY == bottom {
		e.scrollUpInRegion(1, e.activeScrollTop(), bottom)
	} else {
		e.cursorY++
	}
}

func (e *Emulator) index() {
	bottom := e.activeScrollBottom()
	if e.cursorY == bottom {
		e.scrollUpInRegion(1, e.activeScrollTop(), bottom)
		return
	}
	if e.cursorY < e.rows-1 {
		e.cursorY++
	}
}

func (e *Emulator) reverseIndex() {
	top := e.activeScrollTop()
	if e.cursorY == top {
		e.scrollDownInRegion(1, top, e.activeScrollBottom())
		return
	}
	if e.cursorY > 0 {
		e.cursorY--
	}
}

func (e *Emulator) scrollUp(lines int) {
	e.scrollUpInRegion(lines, e.activeScrollTop(), e.activeScrollBottom())
}

func (e *Emulator) scrollDown(lines int) {
	e.scrollDownInRegion(lines, e.activeScrollTop(), e.activeScrollBottom())
}

func (e *Emulator) eraseDisplay(mode int) {
	switch mode {
	case 0:
		for y := e.cursorY; y < e.rows; y++ {
			start := 0
			if y == e.cursorY {
				start = e.cursorX
			}
			for x := start; x < e.cols; x++ {
				e.cells[y][x] = e.blankCell()
			}
		}
	case 1:
		for y := 0; y <= e.cursorY; y++ {
			end := e.cols
			if y == e.cursorY {
				end = e.cursorX + 1
			}
			for x := 0; x < end; x++ {
				e.cells[y][x] = e.blankCell()
			}
		}
	case 2:
		for y := 0; y < e.rows; y++ {
			for x := 0; x < e.cols; x++ {
				e.cells[y][x] = e.blankCell()
			}
		}
		e.cursorX = 0
		e.cursorY = 0
	}
}

func (e *Emulator) eraseLine(mode int) {
	switch mode {
	case 0:
		for x := e.cursorX; x < e.cols; x++ {
			e.cells[e.cursorY][x] = e.blankCell()
		}
	case 1:
		for x := 0; x <= e.cursorX; x++ {
			e.cells[e.cursorY][x] = e.blankCell()
		}
	case 2:
		for x := 0; x < e.cols; x++ {
			e.cells[e.cursorY][x] = e.blankCell()
		}
	}
}

func (e *Emulator) insertChars(n int) {
	if n <= 0 || e.cursorY < 0 || e.cursorY >= e.rows {
		return
	}
	n = min(n, e.cols-e.cursorX)
	row := e.cells[e.cursorY]
	copy(row[e.cursorX+n:], row[e.cursorX:e.cols-n])
	for x := e.cursorX; x < e.cursorX+n; x++ {
		row[x] = e.blankCell()
	}
}

func (e *Emulator) deleteChars(n int) {
	if n <= 0 || e.cursorY < 0 || e.cursorY >= e.rows {
		return
	}
	n = min(n, e.cols-e.cursorX)
	row := e.cells[e.cursorY]
	copy(row[e.cursorX:], row[e.cursorX+n:])
	for x := e.cols - n; x < e.cols; x++ {
		row[x] = e.blankCell()
	}
}

func (e *Emulator) eraseChars(n int) {
	if n <= 0 || e.cursorY < 0 || e.cursorY >= e.rows {
		return
	}
	end := min(e.cols, e.cursorX+n)
	for x := e.cursorX; x < end; x++ {
		e.cells[e.cursorY][x] = e.blankCell()
	}
}

func (e *Emulator) insertLines(n int) {
	if n <= 0 || e.cursorY < 0 || e.cursorY >= e.rows {
		return
	}
	top := e.activeScrollTop()
	bottom := e.activeScrollBottom()
	if e.cursorY < top || e.cursorY > bottom {
		return
	}
	n = min(n, bottom-e.cursorY+1)
	copy(e.cells[e.cursorY+n:bottom+1], e.cells[e.cursorY:bottom+1-n])
	for y := e.cursorY; y < e.cursorY+n; y++ {
		e.cells[y] = e.blankRow()
	}
}

func (e *Emulator) deleteLines(n int) {
	if n <= 0 || e.cursorY < 0 || e.cursorY >= e.rows {
		return
	}
	top := e.activeScrollTop()
	bottom := e.activeScrollBottom()
	if e.cursorY < top || e.cursorY > bottom {
		return
	}
	n = min(n, bottom-e.cursorY+1)
	copy(e.cells[e.cursorY:bottom+1-n], e.cells[e.cursorY+n:bottom+1])
	for y := bottom - n + 1; y <= bottom; y++ {
		e.cells[y] = e.blankRow()
	}
}

func (e *Emulator) applySGR(params []int) {
	if len(params) == 0 {
		params = []int{0}
	}
	for i := 0; i < len(params); i++ {
		p := params[i]
		switch {
		case p == 0:
			e.resetStyle()
		case p == 1:
			e.style.Bold = true
		case p == 4:
			e.style.Underline = true
		case p == 7:
			e.style.Reverse = true
		case p == 22:
			e.style.Bold = false
		case p == 24:
			e.style.Underline = false
		case p == 27:
			e.style.Reverse = false
		case p == 39:
			e.style.FG = tcell.ColorDefault
		case p == 49:
			e.style.BG = tcell.ColorDefault
		case p == 38 || p == 48:
			next, color, ok := parseExtendedColor(params, i+1)
			if !ok {
				continue
			}
			if p == 38 {
				e.style.FG = color
			} else {
				e.style.BG = color
			}
			i = next - 1
		case 30 <= p && p <= 37:
			e.style.FG = ansiColor(p - 30)
		case 90 <= p && p <= 97:
			e.style.FG = ansiBrightColor(p - 90)
		case 40 <= p && p <= 47:
			e.style.BG = ansiColor(p - 40)
		case 100 <= p && p <= 107:
			e.style.BG = ansiBrightColor(p - 100)
		}
	}
}

func parseExtendedColor(params []int, start int) (next int, color tcell.Color, ok bool) {
	if start >= len(params) {
		return start, tcell.ColorDefault, false
	}

	switch params[start] {
	case 5:
		if start+1 >= len(params) {
			return start + 1, tcell.ColorDefault, false
		}
		return start + 2, tcell.PaletteColor(clamp(params[start+1], 0, 255)), true
	case 2:
		if start+3 >= len(params) {
			return start + 1, tcell.ColorDefault, false
		}
		r := int32(clamp(params[start+1], 0, 255))
		g := int32(clamp(params[start+2], 0, 255))
		b := int32(clamp(params[start+3], 0, 255))
		return start + 4, tcell.NewRGBColor(r, g, b), true
	default:
		return start + 1, tcell.ColorDefault, false
	}
}

func parseCharsetSet(final byte) charsetSet {
	switch final {
	case '0':
		return charsetDECSpecial
	default:
		return charsetASCII
	}
}

var decSpecialGraphics = map[rune]rune{
	'j': '┘',
	'k': '┐',
	'l': '┌',
	'm': '└',
	'n': '┼',
	'q': '─',
	't': '├',
	'u': '┤',
	'v': '┴',
	'w': '┬',
	'x': '│',
}

func (e *Emulator) applyMode(raw string, params []int, set bool) {
	if strings.HasPrefix(raw, "?") {
		for _, p := range params {
			switch p {
			case 1:
				e.appCursor = set
			case 3:
				e.columnMode132 = set
				e.clearForColumnMode()
			case 5:
				e.reverseVideo = set
			case 6:
				e.originMode = set
				e.homeCursor()
			case 7:
				e.autoWrap = set
			case 25:
				e.cursorVis = set
			case 12:
				e.cursorBlink = set
			case 66:
				e.appKeypad = set
			case 9:
				e.mouseX10 = set
			case 1000:
				e.mouseVT200 = set
			case 1002:
				e.mouseButtonEvt = set
			case 1003:
				e.mouseAnyEvt = set
			case 1004:
				e.focusReporting = set
			case 1006:
				e.mouseSGR = set
			case 2004:
				e.bracketedPaste = set
			case 1048:
				if set {
					e.saveCursor()
				} else {
					e.restoreCursor()
				}
			case 47, 1047, 1049:
				if set {
					e.enterAlternateScreen()
				} else {
					e.exitAlternateScreen()
				}
			}
		}
	}
}

func (e *Emulator) handleDeviceStatusReport(params []int) {
	switch defaultZero(params, 0) {
	case 6:
		row := e.cursorY + 1
		if e.originMode {
			row = (e.cursorY - e.originTop()) + 1
		}
		resp := "\x1b[" + strconv.Itoa(row) + ";" + strconv.Itoa(e.cursorX+1) + "R"
		e.responses = append(e.responses, []byte(resp))
	}
}

func (e *Emulator) handleDeviceAttributes(raw string, params []int) {
	if strings.HasPrefix(raw, ">") {
		e.responses = append(e.responses, []byte("\x1b[>0;10;1c"))
		return
	}
	if len(params) == 0 || defaultZero(params, 0) == 0 {
		e.responses = append(e.responses, []byte("\x1b[?1;2c"))
	}
}

func (e *Emulator) saveCursor() {
	e.savedX = e.cursorX
	e.savedY = e.cursorY
}

func (e *Emulator) restoreCursor() {
	e.cursorX = clamp(e.savedX, 0, e.cols-1)
	e.cursorY = clamp(e.savedY, 0, e.rows-1)
}

func (e *Emulator) makeStyle() tcell.Style {
	fg := e.style.FG
	bg := e.style.BG
	if e.style.Reverse {
		fg, bg = bg, fg
	}
	style := tcell.StyleDefault.Foreground(fg).Background(bg)
	if e.style.Bold {
		style = style.Bold(true)
	}
	if e.style.Underline {
		style = style.Underline(true)
	}
	return style
}

func (e *Emulator) reset() {
	e.resetStyle()
	e.cells = e.resizeCells(nil, e.cols, e.rows)
	e.cursorX = 0
	e.cursorY = 0
	e.savedX = 0
	e.savedY = 0
	e.utf8buf = e.utf8buf[:0]
	e.csibuf = e.csibuf[:0]
	e.oscbuf = e.oscbuf[:0]
	e.escbuf = 0
	e.wrapPending = false
	e.autoWrap = true
	e.cursorVis = true
	e.cursorBlink = false
	e.appCursor = false
	e.appKeypad = false
	e.columnMode132 = false
	e.originMode = false
	e.reverseVideo = false
	e.bracketedPaste = false
	e.focusReporting = false
	e.mouseX10 = false
	e.mouseVT200 = false
	e.mouseButtonEvt = false
	e.mouseAnyEvt = false
	e.mouseSGR = false
	e.windowTitle = ""
	e.g0Charset = charsetASCII
	e.g1Charset = charsetASCII
	e.useG1Charset = false
	e.scrollTop = 0
	e.scrollBottom = max(0, e.rows-1)
	e.hasScrollRegion = false
	e.scrollback = nil
	e.responses = nil
	e.usingAlt = false
	e.savedPrimary = nil
	e.state = stateGround
}

func (e *Emulator) blankCell() Cell {
	return Cell{
		Ch:    ' ',
		Style: e.makeStyle(),
		Width: 1,
	}
}

func (e *Emulator) blankRow() []Cell {
	row := make([]Cell, e.cols)
	for x := 0; x < e.cols; x++ {
		row[x] = e.blankCell()
	}
	return row
}

func (e *Emulator) resizeCells(old [][]Cell, cols, rows int) [][]Cell {
	newCells := make([][]Cell, rows)
	for y := 0; y < rows; y++ {
		newCells[y] = make([]Cell, cols)
		for x := 0; x < cols; x++ {
			newCells[y][x] = e.blankCell()
		}
	}

	for y := 0; y < min(rows, len(old)); y++ {
		for x := 0; x < min(cols, len(old[y])); x++ {
			newCells[y][x] = old[y][x]
		}
	}

	return newCells
}

func (e *Emulator) resizeHistory(old [][]Cell, cols int) [][]Cell {
	out := make([][]Cell, len(old))
	for y := range old {
		row := make([]Cell, cols)
		for x := 0; x < cols; x++ {
			row[x] = e.blankCell()
		}
		for x := 0; x < min(cols, len(old[y])); x++ {
			row[x] = old[y][x]
		}
		out[y] = row
	}
	return out
}

func (e *Emulator) cloneCells(src [][]Cell) [][]Cell {
	out := make([][]Cell, len(src))
	for y := range src {
		out[y] = make([]Cell, len(src[y]))
		copy(out[y], src[y])
	}
	return out
}

func (e *Emulator) captureScreenState() screenState {
	return screenState{
		cursorX:         e.cursorX,
		cursorY:         e.cursorY,
		savedX:          e.savedX,
		savedY:          e.savedY,
		scrollTop:       e.scrollTop,
		scrollBottom:    e.scrollBottom,
		hasScrollRegion: e.hasScrollRegion,
		originMode:      e.originMode,
		g0Charset:       e.g0Charset,
		g1Charset:       e.g1Charset,
		useG1Charset:    e.useG1Charset,
		cells:           e.cloneCells(e.cells),
		scrollback:      e.cloneCells(e.scrollback),
		style:           e.style,
		wrapPending:     e.wrapPending,
	}
}

func (e *Emulator) restoreScreenState(s screenState) {
	e.cursorX = clamp(s.cursorX, 0, e.cols-1)
	e.cursorY = clamp(s.cursorY, 0, e.rows-1)
	e.savedX = clamp(s.savedX, 0, e.cols-1)
	e.savedY = clamp(s.savedY, 0, e.rows-1)
	e.scrollTop = clamp(s.scrollTop, 0, max(0, e.rows-1))
	e.scrollBottom = clamp(s.scrollBottom, e.scrollTop, max(0, e.rows-1))
	e.hasScrollRegion = s.hasScrollRegion
	e.originMode = s.originMode
	e.g0Charset = s.g0Charset
	e.g1Charset = s.g1Charset
	e.useG1Charset = s.useG1Charset
	e.cells = e.resizeCells(s.cells, e.cols, e.rows)
	e.scrollback = e.resizeHistory(s.scrollback, e.cols)
	e.style = s.style
	e.wrapPending = s.wrapPending
}

func (e *Emulator) enterAlternateScreen() {
	if e.usingAlt {
		return
	}
	saved := e.captureScreenState()
	e.savedPrimary = &saved
	e.cells = e.resizeCells(nil, e.cols, e.rows)
	e.cursorX = 0
	e.cursorY = 0
	e.savedX = 0
	e.savedY = 0
	e.scrollTop = 0
	e.scrollBottom = max(0, e.rows-1)
	e.hasScrollRegion = false
	e.originMode = false
	e.scrollback = nil
	e.wrapPending = false
	e.usingAlt = true
}

func (e *Emulator) exitAlternateScreen() {
	if !e.usingAlt {
		return
	}
	if e.savedPrimary != nil {
		e.restoreScreenState(*e.savedPrimary)
	}
	e.savedPrimary = nil
	e.usingAlt = false
}

func (e *Emulator) activeScrollTop() int {
	if !e.hasScrollRegion {
		return 0
	}
	return clamp(e.scrollTop, 0, max(0, e.rows-1))
}

func (e *Emulator) activeScrollBottom() int {
	if !e.hasScrollRegion {
		return max(0, e.rows-1)
	}
	return clamp(e.scrollBottom, e.activeScrollTop(), max(0, e.rows-1))
}

func (e *Emulator) scrollUpInRegion(lines, top, bottom int) {
	if lines <= 0 || top < 0 || bottom >= e.rows || top > bottom {
		return
	}
	height := bottom - top + 1
	lines = min(lines, height)
	if !e.hasScrollRegion && !e.usingAlt {
		for y := 0; y < lines; y++ {
			e.pushScrollbackRow(e.cells[top+y])
		}
	}
	copy(e.cells[top:bottom+1-lines], e.cells[top+lines:bottom+1])
	for y := bottom - lines + 1; y <= bottom; y++ {
		e.cells[y] = e.blankRow()
	}
}

func (e *Emulator) scrollDownInRegion(lines, top, bottom int) {
	if lines <= 0 || top < 0 || bottom >= e.rows || top > bottom {
		return
	}
	height := bottom - top + 1
	lines = min(lines, height)
	copy(e.cells[top+lines:bottom+1], e.cells[top:bottom+1-lines])
	for y := top; y < top+lines; y++ {
		e.cells[y] = e.blankRow()
	}
}

func (e *Emulator) setScrollRegion(params []int) {
	top := 1
	bottom := e.rows
	if len(params) >= 1 && params[0] > 0 {
		top = params[0]
	}
	if len(params) >= 2 && params[1] > 0 {
		bottom = params[1]
	}
	if len(params) == 0 || (top == 1 && bottom == e.rows) {
		e.scrollTop = 0
		e.scrollBottom = max(0, e.rows-1)
		e.hasScrollRegion = false
	} else {
		top--
		bottom--
		if top < 0 {
			top = 0
		}
		if bottom >= e.rows {
			bottom = e.rows - 1
		}
		if top >= bottom {
			return
		}
		e.scrollTop = top
		e.scrollBottom = bottom
		e.hasScrollRegion = true
	}
	e.homeCursor()
}

func (e *Emulator) homeCursor() {
	e.cursorX = 0
	e.cursorY = e.originTop()
}

func (e *Emulator) clearForColumnMode() {
	e.cells = e.resizeCells(nil, e.cols, e.rows)
	e.scrollback = nil
	e.wrapPending = false
	e.hasScrollRegion = false
	e.scrollTop = 0
	e.scrollBottom = max(0, e.rows-1)
	e.homeCursor()
}

func (e *Emulator) originTop() int {
	if e.originMode {
		return e.activeScrollTop()
	}
	return 0
}

func (e *Emulator) originBottom() int {
	if e.originMode {
		return e.activeScrollBottom()
	}
	return max(0, e.rows-1)
}

func (e *Emulator) clampRowParam(row int) int {
	return clamp(e.originTop()+row-1, e.originTop(), e.originBottom())
}

func (e *Emulator) pushScrollbackRow(row []Cell) {
	copied := make([]Cell, len(row))
	copy(copied, row)
	e.scrollback = append(e.scrollback, copied)
	if e.maxScrollback > 0 && len(e.scrollback) > e.maxScrollback {
		extra := len(e.scrollback) - e.maxScrollback
		e.scrollback = e.scrollback[extra:]
	}
}

func (e *Emulator) resetStyle() {
	e.style = StyleState{FG: tcell.ColorDefault, BG: tcell.ColorDefault}
}

func parseCSIParams(raw string) []int {
	raw = strings.TrimLeft(raw, "<=>?")
	if raw == "" {
		return nil
	}
	raw = strings.ReplaceAll(raw, ":", ";")
	parts := strings.Split(raw, ";")
	out := make([]int, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			out = append(out, 0)
			continue
		}
		n, err := strconv.Atoi(part)
		if err != nil {
			out = append(out, 0)
			continue
		}
		out = append(out, n)
	}
	return out
}

func defaultOne(params []int, idx int) int {
	if idx >= len(params) || params[idx] == 0 {
		return 1
	}
	return params[idx]
}

func defaultZero(params []int, idx int) int {
	if idx >= len(params) {
		return 0
	}
	return params[idx]
}

func ansiColor(i int) tcell.Color {
	colors := []tcell.Color{
		tcell.ColorBlack,
		tcell.ColorMaroon,
		tcell.ColorGreen,
		tcell.ColorOlive,
		tcell.ColorNavy,
		tcell.ColorPurple,
		tcell.ColorTeal,
		tcell.ColorSilver,
	}
	return colors[clamp(i, 0, len(colors)-1)]
}

func ansiBrightColor(i int) tcell.Color {
	colors := []tcell.Color{
		tcell.ColorGray,
		tcell.ColorRed,
		tcell.ColorLime,
		tcell.ColorYellow,
		tcell.ColorBlue,
		tcell.ColorFuchsia,
		tcell.ColorAqua,
		tcell.ColorWhite,
	}
	return colors[clamp(i, 0, len(colors)-1)]
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
