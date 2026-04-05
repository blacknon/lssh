package mview

import (
	"bytes"
	"sort"
	"sync"

	"github.com/gdamore/tcell/v2"
	colorful "github.com/lucasb-eyer/go-colorful"
)

// TableCell represents one cell inside a Table. You can instantiate this type
// directly but all colors (background and text) will be set to their default
// which is black.
type TableCell struct {
	// The reference object.
	Reference interface{}

	// The text to be displayed in the table cell.
	Text []byte

	// The alignment of the cell text. One of AlignLeft (default), AlignCenter,
	// or AlignRight.
	Align int

	// The maximum width of the cell in screen space. This is used to give a
	// column a maximum width. Any cell text whose screen width exceeds this width
	// is cut off. Set to 0 if there is no maximum width.
	MaxWidth int

	// If the total table width is less than the available width, this value is
	// used to add extra width to a column. See SetExpansion() for details.
	Expansion int

	// The color of the cell text.
	Color tcell.Color

	// The background color of the cell.
	BackgroundColor tcell.Color

	// The style attributes of the cell.
	Attributes tcell.AttrMask

	// If set to true, this cell cannot be selected.
	NotSelectable bool

	// The position and width of the cell the last time table was drawn.
	x, y, width int

	sync.RWMutex

	// `isSelected` is a temporary boolean value used to preserve the selection state of values during sorting.
	isSelected bool

	//
	isHeader bool

	//
	SortedColor tcell.Color

	//
	SortedBackgroundColor tcell.Color
}

// NewTableCell returns a new table cell with sensible defaults. That is, left
// aligned text with the primary text color (see Styles) and a transparent
// background (using the background of the Table).
func NewTableCell(text string) *TableCell {
	return &TableCell{
		Text:            []byte(text),
		Align:           AlignLeft,
		Color:           Styles.PrimaryTextColor,
		BackgroundColor: tcell.ColorDefault,
	}
}

// SetBytes sets the cell's text.
func (c *TableCell) SetBytes(text []byte) {
	c.Lock()
	defer c.Unlock()

	c.Text = text
}

// SetText sets the cell's text.
func (c *TableCell) SetText(text string) {
	c.SetBytes([]byte(text))
}

// GetBytes returns the cell's text.
func (c *TableCell) GetBytes() []byte {
	c.RLock()
	defer c.RUnlock()

	return c.Text
}

// GetText returns the cell's text.
func (c *TableCell) GetText() string {
	return string(c.GetBytes())
}

// SetAlign sets the cell's text alignment, one of AlignLeft, AlignCenter, or
// AlignRight.
func (c *TableCell) SetAlign(align int) {
	c.Lock()
	defer c.Unlock()

	c.Align = align
}

// SetMaxWidth sets maximum width of the cell in screen space. This is used to
// give a column a maximum width. Any cell text whose screen width exceeds this
// width is cut off. Set to 0 if there is no maximum width.
func (c *TableCell) SetMaxWidth(maxWidth int) {
	c.Lock()
	defer c.Unlock()

	c.MaxWidth = maxWidth
}

// SetExpansion sets the value by which the column of this cell expands if the
// available width for the table is more than the table width (prior to applying
// this expansion value). This is a proportional value. The amount of unused
// horizontal space is divided into widths to be added to each column. How much
// extra width a column receives depends on the expansion value: A value of 0
// (the default) will not cause the column to increase in width. Other values
// are proportional, e.g. a value of 2 will cause a column to grow by twice
// the amount of a column with a value of 1.
//
// Since this value affects an entire column, the maximum over all visible cells
// in that column is used.
//
// This function panics if a negative value is provided.
func (c *TableCell) SetExpansion(expansion int) {
	c.Lock()
	defer c.Unlock()

	if expansion < 0 {
		panic("Table cell expansion values may not be negative")
	}
	c.Expansion = expansion
}

// SetTextColor sets the cell's text color.
func (c *TableCell) SetTextColor(color tcell.Color) {
	c.Lock()
	defer c.Unlock()

	c.Color = color
}

// SetBackgroundColor sets the cell's background color. Set to
// tcell.ColorDefault to use the table's background color.
func (c *TableCell) SetBackgroundColor(color tcell.Color) {
	c.Lock()
	defer c.Unlock()

	c.BackgroundColor = color
}

// SetAttributes sets the cell's text attributes. You can combine different
// attributes using bitmask operations:
//
//	cell.SetAttributes(tcell.AttrUnderline | tcell.AttrBold)
func (c *TableCell) SetAttributes(attr tcell.AttrMask) {
	c.Lock()
	defer c.Unlock()

	c.Attributes = attr
}

// SetStyle sets the cell's style (foreground color, background color, and
// attributes) all at once.
func (c *TableCell) SetStyle(style tcell.Style) {
	c.Lock()
	defer c.Unlock()

	c.Color, c.BackgroundColor, c.Attributes = style.Decompose()
}

// SetSelectable sets whether or not this cell can be selected by the user.
func (c *TableCell) SetSelectable(selectable bool) {
	c.Lock()
	defer c.Unlock()

	c.NotSelectable = !selectable
}

// SetReference allows you to store a reference of any type in this cell. This
// will allow you to establish a mapping between the cell and your
// actual data.
func (c *TableCell) SetReference(reference interface{}) {
	c.Lock()
	defer c.Unlock()

	c.Reference = reference
}

// GetReference returns this cell's reference object.
func (c *TableCell) GetReference() interface{} {
	c.RLock()
	defer c.RUnlock()

	return c.Reference
}

func (c *TableCell) SetIsHeader(isHeader bool) {
	c.RLock()
	defer c.RUnlock()

	c.isHeader = isHeader
}

// GetLastPosition returns the position of the table cell the last time it was
// drawn on screen. If the cell is not on screen, the return values are
// undefined.
//
// Because the Table class will attempt to keep selected cells on screen, this
// function is most useful in response to a "selected" event (see
// SetSelectedFunc()) or a "selectionChanged" event (see
// SetSelectionChangedFunc()).
func (c *TableCell) GetLastPosition() (x, y, width int) {
	c.RLock()
	defer c.RUnlock()

	return c.x, c.y, c.width
}

// Table visualizes two-dimensional data consisting of rows and columns. Each
// Table cell is defined via SetCell() by the TableCell type. They can be added
// dynamically to the table and changed any time.
//
// Each row of the table must have the same number of columns when it is drawn
// or navigated. This isn't strictly enforced, however you may encounter issues
// when navigating a table with rows of varied column sizes.
//
// The most compact display of a table is without borders. Each row will then
// occupy one row on screen and columns are separated by the rune defined via
// SetSeparator() (a space character by default).
//
// When borders are turned on (via SetBorders()), each table cell is surrounded
// by lines. Therefore one table row will require two rows on screen.
//
// Columns will use as much horizontal space as they need. You can constrain
// their size with the MaxWidth parameter of the TableCell type.
//
// # Fixed Columns
//
// You can define fixed rows and rolumns via SetFixed(). They will always stay
// in their place, even when the table is scrolled. Fixed rows are always the
// top rows. Fixed columns are always the leftmost columns.
//
// # Selections
//
// You can call SetSelectable() to set columns and/or rows to "selectable". If
// the flag is set only for columns, entire columns can be selected by the user.
// If it is set only for rows, entire rows can be selected. If both flags are
// set, individual cells can be selected. The "selected" handler set via
// SetSelectedFunc() is invoked when the user presses Enter on a selection.
//
// # Navigation
//
// If the table extends beyond the available space, it can be navigated with
// key bindings similar to Vim:
//
//   - h, left arrow: Move left by one column.
//   - l, right arrow: Move right by one column.
//   - j, down arrow: Move down by one row.
//   - k, up arrow: Move up by one row.
//   - g, home: Move to the top.
//   - G, end: Move to the bottom.
//   - Ctrl-F, page down: Move down by one page.
//   - Ctrl-B, page up: Move up by one page.
//
// When there is no selection, this affects the entire table (except for fixed
// rows and columns). When there is a selection, the user moves the selection.
// The class will attempt to keep the selection from moving out of the screen.
//
// Use SetInputCapture() to override or modify keyboard input.
type Table struct {
	*Box

	// Whether or not this table has borders around each cell.
	borders bool

	// The color of the borders or the separator.
	bordersColor tcell.Color

	// If there are no borders, the column separator.
	separator rune

	// The cells of the table. Rows first, then columns.
	cells [][]*TableCell

	// header
	header []string

	// The rightmost column in the data set.
	lastColumn int

	// If true, when calculating the widths of the columns, all rows are evaluated
	// instead of only the visible ones.
	evaluateAllRows bool

	// The number of fixed rows / columns.
	fixedRows, fixedColumns int

	// Whether or not rows or columns can be selected. If both are set to true,
	// cells can be selected.
	rowsSelectable, columnsSelectable bool

	// The currently selected row and column.
	selectedRow, selectedColumn int

	// The number of rows/columns by which the table is scrolled down/to the
	// right.
	rowOffset, columnOffset int

	// If set to true, the table's last row will always be visible.
	trackEnd bool

	// The sort function of the table. Defaults to a case-sensitive comparison.
	sortFunc func(column int, i, j []byte) bool

	// Whether or not the table should be sorted when a fixed row is clicked.
	sortClicked bool

	// The last direction the table was sorted by when clicked.
	sortClickedDescending bool

	// The last column the table was sorted by when clicked.
	sortClickedColumn int

	// The number of visible rows the last time the table was drawn.
	visibleRows int

	// The indices of the visible columns as of the last time the table was drawn.
	visibleColumnIndices []int

	// The net widths of the visible columns as of the last time the table was
	// drawn.
	visibleColumnWidths []int

	// Visibility of the scroll bar.
	scrollBarVisibility ScrollBarVisibility

	// The scroll bar color.
	scrollBarColor tcell.Color

	// The style of the selected rows. If this value is StyleDefault, selected rows
	// are simply inverted.
	selectedStyle tcell.Style

	// An optional function which gets called when the user presses Enter on a
	// selected cell. If entire rows selected, the column value is undefined.
	// Likewise for entire columns.
	selected func(row, column int)

	// An optional function which gets called when the user changes the selection.
	// If entire rows selected, the column value is undefined.
	// Likewise for entire columns.
	selectionChanged func(row, column int)

	// An optional function which gets called when the user presses Escape, Tab,
	// or Backtab. Also when the user presses Enter if nothing is selectable.
	done func(key tcell.Key)

	sync.RWMutex
}

// NewTable returns a new table.
func NewTable() *Table {
	return &Table{
		Box:                 NewBox(),
		scrollBarVisibility: ScrollBarAuto,
		scrollBarColor:      Styles.ScrollBarColor,
		bordersColor:        Styles.GraphicsColor,
		separator:           ' ',
		sortClicked:         true,
		lastColumn:          -1,
	}
}

// Clear removes all table data.
func (t *Table) Clear() {
	t.Lock()
	defer t.Unlock()

	t.cells = nil
	t.lastColumn = -1
}

// SetBorders sets whether or not each cell in the table is surrounded by a
// border.
func (t *Table) SetBorders(show bool) {
	t.Lock()
	defer t.Unlock()

	t.borders = show
}

// SetBordersColor sets the color of the cell borders.
func (t *Table) SetBordersColor(color tcell.Color) {
	t.Lock()
	defer t.Unlock()

	t.bordersColor = color
}

// SetScrollBarVisibility specifies the display of the scroll bar.
func (t *Table) SetScrollBarVisibility(visibility ScrollBarVisibility) {
	t.Lock()
	defer t.Unlock()

	t.scrollBarVisibility = visibility
}

// SetScrollBarColor sets the color of the scroll bar.
func (t *Table) SetScrollBarColor(color tcell.Color) {
	t.Lock()
	defer t.Unlock()

	t.scrollBarColor = color
}

// SetSelectedStyle sets a specific style for selected cells. If no such style
// is set, per default, selected cells are inverted (i.e. their foreground and
// background colors are swapped).
//
// To reset a previous setting to its default, make the following call:
//
//	table.SetSelectedStyle(tcell.ColorDefault, tcell.ColorDefault, 0)
func (t *Table) SetSelectedStyle(foregroundColor, backgroundColor tcell.Color, attributes tcell.AttrMask) {
	t.Lock()
	defer t.Unlock()

	t.selectedStyle = SetAttributes(tcell.StyleDefault.Foreground(foregroundColor).Background(backgroundColor), attributes)
}

// SetSeparator sets the character used to fill the space between two
// neighboring cells. This is a space character ' ' per default but you may
// want to set it to Borders.Vertical (or any other rune) if the column
// separation should be more visible. If cell borders are activated, this is
// ignored.
//
// Separators have the same color as borders.
func (t *Table) SetSeparator(separator rune) {
	t.Lock()
	defer t.Unlock()

	t.separator = separator
}

// SetFixed sets the number of fixed rows and columns which are always visible
// even when the rest of the cells are scrolled out of view. Rows are always the
// top-most ones. Columns are always the left-most ones.
func (t *Table) SetFixed(rows, columns int) {
	t.Lock()
	defer t.Unlock()

	t.fixedRows, t.fixedColumns = rows, columns
}

// SetSelectable sets the flags which determine what can be selected in a table.
// There are three selection modi:
//
//   - rows = false, columns = false: Nothing can be selected.
//   - rows = true, columns = false: Rows can be selected.
//   - rows = false, columns = true: Columns can be selected.
//   - rows = true, columns = true: Individual cells can be selected.
func (t *Table) SetSelectable(rows, columns bool) {
	t.Lock()
	defer t.Unlock()

	t.rowsSelectable, t.columnsSelectable = rows, columns
}

// GetSelectable returns what can be selected in a table. Refer to
// SetSelectable() for details.
func (t *Table) GetSelectable() (rows, columns bool) {
	t.RLock()
	defer t.RUnlock()

	return t.rowsSelectable, t.columnsSelectable
}

// GetSelection returns the position of the current selection.
// If entire rows are selected, the column index is undefined.
// Likewise for entire columns.
func (t *Table) GetSelection() (row, column int) {
	t.RLock()
	defer t.RUnlock()

	return t.selectedRow, t.selectedColumn
}

// Select sets the selected cell. Depending on the selection settings
// specified via SetSelectable(), this may be an entire row or column, or even
// ignored completely. The "selection changed" event is fired if such a callback
// is available (even if the selection ends up being the same as before and even
// if cells are not selectable).
func (t *Table) Select(row, column int) {
	t.unsetSelected()

	t.Lock()
	defer t.Unlock()

	t.selectedRow, t.selectedColumn = row, column
	if t.selectionChanged != nil {
		t.Unlock()
		t.selectionChanged(row, column)
		t.Lock()
	}
}

// SetOffset sets how many rows and columns should be skipped when drawing the
// table. This is useful for large tables that do not fit on the screen.
// Navigating a selection can change these values.
//
// Fixed rows and columns are never skipped.
func (t *Table) SetOffset(row, column int) {
	t.Lock()
	defer t.Unlock()

	t.rowOffset, t.columnOffset = row, column
	t.trackEnd = false
}

// GetOffset returns the current row and column offset. This indicates how many
// rows and columns the table is scrolled down and to the right.
func (t *Table) GetOffset() (row, column int) {
	t.RLock()
	defer t.RUnlock()

	return t.rowOffset, t.columnOffset
}

// SetEvaluateAllRows sets a flag which determines the rows to be evaluated when
// calculating the widths of the table's columns. When false, only visible rows
// are evaluated. When true, all rows in the table are evaluated.
//
// Set this flag to true to avoid shifting column widths when the table is
// scrolled. (May be slower for large tables.)
func (t *Table) SetEvaluateAllRows(all bool) {
	t.Lock()
	defer t.Unlock()

	t.evaluateAllRows = all
}

// SetSelectedFunc sets a handler which is called whenever the user presses the
// Enter key on a selected cell/row/column. The handler receives the position of
// the selection and its cell contents. If entire rows are selected, the column
// index is undefined. Likewise for entire columns.
func (t *Table) SetSelectedFunc(handler func(row, column int)) {
	t.Lock()
	defer t.Unlock()

	t.selected = handler
}

// SetSelectionChangedFunc sets a handler which is called whenever the current
// selection changes. The handler receives the position of the new selection.
// If entire rows are selected, the column index is undefined. Likewise for
// entire columns.
func (t *Table) SetSelectionChangedFunc(handler func(row, column int)) {
	t.Lock()
	defer t.Unlock()

	t.selectionChanged = handler
}

// SetDoneFunc sets a handler which is called whenever the user presses the
// Escape, Tab, or Backtab key. If nothing is selected, it is also called when
// user presses the Enter key (because pressing Enter on a selection triggers
// the "selected" handler set via SetSelectedFunc()).
func (t *Table) SetDoneFunc(handler func(key tcell.Key)) {
	t.Lock()
	defer t.Unlock()

	t.done = handler
}

// SetCell sets the content of a cell the specified position. It is ok to
// directly instantiate a TableCell object. If the cell has content, at least
// the Text and Color fields should be set.
//
// Note that setting cells in previously unknown rows and columns will
// automatically extend the internal table representation, e.g. starting with
// a row of 100,000 will immediately create 100,000 empty rows.
//
// To avoid unnecessary garbage collection, fill columns from left to right.
func (t *Table) SetCell(row, column int, cell *TableCell) {
	t.Lock()
	defer t.Unlock()

	if row >= len(t.cells) {
		t.cells = append(t.cells, make([][]*TableCell, row-len(t.cells)+1)...)
	}
	rowLen := len(t.cells[row])
	if column >= rowLen {
		t.cells[row] = append(t.cells[row], make([]*TableCell, column-rowLen+1)...)
		for c := rowLen; c < column; c++ {
			t.cells[row][c] = &TableCell{}
		}
	}
	t.cells[row][column] = cell
	if column > t.lastColumn {
		t.lastColumn = column
	}
}

// SetCellSimple calls SetCell() with the given text, left-aligned, in white.
func (t *Table) SetCellSimple(row, column int, text string) {
	t.SetCell(row, column, NewTableCell(text))
}

// GetCell returns the contents of the cell at the specified position. A valid
// TableCell object is always returned but it will be uninitialized if the cell
// was not previously set. Such an uninitialized object will not automatically
// be inserted. Therefore, repeated calls to this function may return different
// pointers for uninitialized cells.
func (t *Table) GetCell(row, column int) *TableCell {
	t.RLock()
	defer t.RUnlock()

	if row >= len(t.cells) || column >= len(t.cells[row]) {
		return &TableCell{}
	}
	return t.cells[row][column]
}

// RemoveRow removes the row at the given position from the table. If there is
// no such row, this has no effect.
func (t *Table) RemoveRow(row int) {
	t.Lock()
	defer t.Unlock()

	if row < 0 || row >= len(t.cells) {
		return
	}

	t.cells = append(t.cells[:row], t.cells[row+1:]...)
}

// RemoveColumn removes the column at the given position from the table. If
// there is no such column, this has no effect.
func (t *Table) RemoveColumn(column int) {
	t.Lock()
	defer t.Unlock()

	for row := range t.cells {
		if column < 0 || column >= len(t.cells[row]) {
			continue
		}
		t.cells[row] = append(t.cells[row][:column], t.cells[row][column+1:]...)
	}
}

// InsertRow inserts a row before the row with the given index. Cells on the
// given row and below will be shifted to the bottom by one row. If "row" is
// equal or larger than the current number of rows, this function has no effect.
func (t *Table) InsertRow(row int) {
	t.Lock()
	defer t.Unlock()

	if row >= len(t.cells) {
		return
	}
	t.cells = append(t.cells, nil)       // Extend by one.
	copy(t.cells[row+1:], t.cells[row:]) // Shift down.
	t.cells[row] = nil                   // New row is uninitialized.
}

// InsertColumn inserts a column before the column with the given index. Cells
// in the given column and to its right will be shifted to the right by one
// column. Rows that have fewer initialized cells than "column" will remain
// unchanged.
func (t *Table) InsertColumn(column int) {
	t.Lock()
	defer t.Unlock()

	for row := range t.cells {
		if column >= len(t.cells[row]) {
			continue
		}
		t.cells[row] = append(t.cells[row], nil)             // Extend by one.
		copy(t.cells[row][column+1:], t.cells[row][column:]) // Shift to the right.
		t.cells[row][column] = &TableCell{}                  // New element is an uninitialized table cell.
	}
}

// GetRowCount returns the number of rows in the table.
func (t *Table) GetRowCount() int {
	t.RLock()
	defer t.RUnlock()

	return len(t.cells)
}

// GetColumnCount returns the (maximum) number of columns in the table.
func (t *Table) GetColumnCount() int {
	t.RLock()
	defer t.RUnlock()

	if len(t.cells) == 0 {
		return 0
	}
	return t.lastColumn + 1
}

// GetSortClickedColumn returns the sortClickedColumn
func (t *Table) GetSortClickedColumn() int {
	return t.sortClickedColumn
}

// GetSortClickedDescending return the sortClickedDescending
func (t *Table) GetSortClickedDescending() bool {
	return t.sortClickedDescending
}

// cellAt returns the row and column located at the given screen coordinates.
// Each returned value may be negative if there is no row and/or cell. This
// function will also process coordinates outside the table's inner rectangle so
// callers will need to check for bounds themselves.
func (t *Table) cellAt(x, y int) (row, column int) {
	rectX, rectY, _, _ := t.GetInnerRect()

	// Determine row as seen on screen.
	if t.borders {
		row = (y - rectY - 1) / 2
	} else {
		row = y - rectY
	}

	// Respect fixed rows and row offset.
	if row >= 0 {
		if row >= t.fixedRows {
			row += t.rowOffset
		}
		if row >= len(t.cells) {
			row = -1
		}
	}

	// Search for the clicked column.
	column = -1
	if x >= rectX {
		columnX := rectX
		if t.borders {
			columnX++
		}
		for index, width := range t.visibleColumnWidths {
			columnX += width + 1
			if x < columnX {
				column = t.visibleColumnIndices[index]
				break
			}
		}
	}

	return
}

// ScrollToBeginning scrolls the table to the beginning to that the top left
// corner of the table is shown. Note that this position may be corrected if
// there is a selection.
func (t *Table) ScrollToBeginning() {
	t.Lock()
	defer t.Unlock()

	t.trackEnd = false
	t.columnOffset = 0
	t.rowOffset = 0
}

// ScrollToEnd scrolls the table to the beginning to that the bottom left corner
// of the table is shown. Adding more rows to the table will cause it to
// automatically scroll with the new data. Note that this position may be
// corrected if there is a selection.
func (t *Table) ScrollToEnd() {
	t.Lock()
	defer t.Unlock()

	t.trackEnd = true
	t.columnOffset = 0
	t.rowOffset = len(t.cells)
}

// SetSortClicked sets a flag which determines whether the table is sorted when
// a fixed row is clicked. This flag is enabled by default.
func (t *Table) SetSortClicked(sortClicked bool) {
	t.Lock()
	defer t.Unlock()

	t.sortClicked = sortClicked
}

// SetSortFunc sets the sorting function used for the table. When unset, a
// case-sensitive string comparison is used.
func (t *Table) SetSortFunc(sortFunc func(column int, i, j []byte) bool) {
	t.Lock()
	defer t.Unlock()

	t.sortFunc = sortFunc
}

// Sort sorts the table by the column at the given index. You may set a custom
// sorting function with SetSortFunc.
func (t *Table) Sort(column int, descending bool) {
	t.Lock()
	defer t.Unlock()

	if len(t.cells) == 0 || column < 0 || column >= len(t.cells[0]) {
		return
	}

	if t.sortFunc == nil {
		t.sortFunc = func(column int, i, j []byte) bool {
			return bytes.Compare(i, j) == -1
		}
	}

	sort.SliceStable(t.cells, func(i, j int) bool {
		if i < t.fixedRows {
			return i < j
		} else if j < t.fixedRows {
			return j > i
		}

		ibyte := t.cells[i][column].Text
		jbyte := t.cells[j][column].Text

		if !descending {
			return t.sortFunc(column, ibyte, jbyte)
		}
		return t.sortFunc(column, jbyte, ibyte)
	})

	t.selectedRow, t.selectedColumn = t.findSelected()
	if t.selectionChanged != nil {
		t.Unlock()
		t.selectionChanged(t.selectedRow, t.selectedColumn)
		t.Lock()
	}
}

// Draw draws this primitive onto the screen.
func (t *Table) Draw(screen tcell.Screen) {
	if !t.GetVisible() {
		return
	}

	t.Box.Draw(screen)

	t.Lock()
	defer t.Unlock()

	// What's our available screen space?
	x, y, width, height := t.GetInnerRect()
	if t.borders {
		t.visibleRows = height / 2
	} else {
		t.visibleRows = height
	}

	showVerticalScrollBar := t.scrollBarVisibility == ScrollBarAlways || (t.scrollBarVisibility == ScrollBarAuto && len(t.cells) > t.visibleRows-t.fixedRows)
	if showVerticalScrollBar {
		width-- // Subtract space for scroll bar.
	}

	// Return the cell at the specified position (nil if it doesn't exist).
	getCell := func(row, column int) *TableCell {
		if row < 0 || column < 0 || row >= len(t.cells) || column >= len(t.cells[row]) {
			return nil
		}
		return t.cells[row][column]
	}

	// If this cell is not selectable, find the next one.
	if t.rowsSelectable || t.columnsSelectable {
		if t.selectedColumn < 0 {
			t.selectedColumn = 0
		}
		if t.selectedRow < 0 {
			t.selectedRow = 0
		}
		for t.selectedRow < len(t.cells) {
			cell := getCell(t.selectedRow, t.selectedColumn)
			if cell == nil || !cell.NotSelectable {
				break
			}
			t.selectedColumn++
			if t.selectedColumn > t.lastColumn {
				t.selectedColumn = 0
				t.selectedRow++
			}
		}
	}

	// Clamp row offsets.
	if t.rowsSelectable {
		if t.selectedRow >= t.fixedRows && t.selectedRow < t.fixedRows+t.rowOffset {
			t.rowOffset = t.selectedRow - t.fixedRows
			t.trackEnd = false
		}
		if t.borders {
			if 2*(t.selectedRow+1-t.rowOffset) >= height {
				t.rowOffset = t.selectedRow + 1 - height/2
				t.trackEnd = false
			}
		} else {
			if t.selectedRow+1-t.rowOffset >= height {
				t.rowOffset = t.selectedRow + 1 - height
				t.trackEnd = false
			}
		}
	}
	if t.borders {
		if 2*(len(t.cells)-t.rowOffset) < height {
			t.trackEnd = true
		}
	} else {
		if len(t.cells)-t.rowOffset < height {
			t.trackEnd = true
		}
	}
	if t.trackEnd {
		if t.borders {
			t.rowOffset = len(t.cells) - height/2
		} else {
			t.rowOffset = len(t.cells) - height
		}
	}
	if t.rowOffset < 0 {
		t.rowOffset = 0
	}

	// Clamp column offset. (Only left side here. The right side is more
	// difficult and we'll do it below.)
	if t.columnsSelectable && t.selectedColumn >= t.fixedColumns && t.selectedColumn < t.fixedColumns+t.columnOffset {
		t.columnOffset = t.selectedColumn - t.fixedColumns
	}
	if t.columnOffset < 0 {
		t.columnOffset = 0
	}
	if t.selectedColumn < 0 {
		t.selectedColumn = 0
	}

	// Determine the indices and widths of the columns and rows which fit on the
	// screen.
	var (
		columns, rows, allRows, widths []int
		tableHeight, tableWidth        int
	)
	rowStep := 1
	if t.borders {
		rowStep = 2    // With borders, every table row takes two screen rows.
		tableWidth = 1 // We start at the second character because of the left table border.
	}
	if t.evaluateAllRows {
		allRows = make([]int, len(t.cells))
		for row := range t.cells {
			allRows[row] = row
		}
	}
	indexRow := func(row int) bool { // Determine if this row is visible, store its index.
		if tableHeight >= height {
			return false
		}
		rows = append(rows, row)
		tableHeight += rowStep
		return true
	}
	for row := 0; row < t.fixedRows && row < len(t.cells); row++ { // Do the fixed rows first.
		if !indexRow(row) {
			break
		}
	}
	for row := t.fixedRows + t.rowOffset; row < len(t.cells); row++ { // Then the remaining rows.
		if !indexRow(row) {
			break
		}
	}
	var (
		skipped, lastTableWidth, expansionTotal int
		expansions                              []int
	)
ColumnLoop:
	for column := 0; ; column++ {
		// If we've moved beyond the right border, we stop or skip a column.
		for tableWidth-1 >= width { // -1 because we include one extra column if the separator falls on the right end of the box.
			// We've moved beyond the available space.
			if column < t.fixedColumns {
				break ColumnLoop // We're in the fixed area. We're done.
			}
			if !t.columnsSelectable && skipped >= t.columnOffset {
				break ColumnLoop // There is no selection and we've already reached the offset.
			}
			if t.columnsSelectable && t.selectedColumn-skipped == t.fixedColumns {
				break ColumnLoop // The selected column reached the leftmost point before disappearing.
			}
			if t.columnsSelectable && skipped >= t.columnOffset &&
				(t.selectedColumn < column && lastTableWidth < width-1 && tableWidth < width-1 || t.selectedColumn < column-1) {
				break ColumnLoop // We've skipped as many as requested and the selection is visible.
			}
			if len(columns) <= t.fixedColumns {
				break // Nothing to skip.
			}

			// We need to skip a column.
			skipped++
			lastTableWidth -= widths[t.fixedColumns] + 1
			tableWidth -= widths[t.fixedColumns] + 1
			columns = append(columns[:t.fixedColumns], columns[t.fixedColumns+1:]...)
			widths = append(widths[:t.fixedColumns], widths[t.fixedColumns+1:]...)
			expansions = append(expansions[:t.fixedColumns], expansions[t.fixedColumns+1:]...)
		}

		// What's this column's width (without expansion)?
		maxWidth := -1
		expansion := 0
		evaluationRows := rows
		if t.evaluateAllRows {
			evaluationRows = allRows
		}
		for _, row := range evaluationRows {
			if cell := getCell(row, column); cell != nil {
				_, _, _, _, _, _, cellWidth := decomposeText(cell.Text, true, false)
				if cell.MaxWidth > 0 && cell.MaxWidth < cellWidth {
					cellWidth = cell.MaxWidth
				}
				if cellWidth > maxWidth {
					maxWidth = cellWidth
				}
				if cell.Expansion > expansion {
					expansion = cell.Expansion
				}
			}
		}
		if maxWidth < 0 {
			break // No more cells found in this column.
		}

		// Store new column info at the end.
		columns = append(columns, column)
		widths = append(widths, maxWidth)
		lastTableWidth = tableWidth
		tableWidth += maxWidth + 1
		expansions = append(expansions, expansion)
		expansionTotal += expansion
	}
	t.columnOffset = skipped

	// If we have space left, distribute it.
	if tableWidth < width {
		toDistribute := width - tableWidth
		for index, expansion := range expansions {
			if expansionTotal <= 0 {
				break
			}
			expWidth := toDistribute * expansion / expansionTotal
			widths[index] += expWidth
			toDistribute -= expWidth
			expansionTotal -= expansion
		}
		tableWidth = width - toDistribute
	}

	// Helper function which draws border runes.
	borderStyle := tcell.StyleDefault.Background(t.backgroundColor).Foreground(t.bordersColor)
	drawBorder := func(colX, rowY int, ch rune) {
		screen.SetContent(x+colX, y+rowY, ch, nil, borderStyle)
	}

	// Draw the cells (and borders).
	var columnX int
	if !t.borders {
		columnX--
	}
	for columnIndex, column := range columns {
		columnWidth := widths[columnIndex]
		for rowY, row := range rows {
			if t.borders {
				// Draw borders.
				rowY *= 2
				for pos := 0; pos < columnWidth && columnX+1+pos < width; pos++ {
					drawBorder(columnX+pos+1, rowY, Borders.Horizontal)
				}
				ch := Borders.Cross
				if columnIndex == 0 {
					if rowY == 0 {
						ch = Borders.TopLeft
					} else {
						ch = Borders.LeftT
					}
				} else if rowY == 0 {
					ch = Borders.TopT
				}
				drawBorder(columnX, rowY, ch)
				rowY++
				if rowY >= height {
					break // No space for the text anymore.
				}
				drawBorder(columnX, rowY, Borders.Vertical)
			} else if columnIndex > 0 {
				// Draw separator.
				drawBorder(columnX, rowY, t.separator)
			}

			// Get the cell.
			cell := getCell(row, column)
			if cell == nil {
				continue
			}

			// Draw text.
			finalWidth := columnWidth
			if columnX+1+columnWidth >= width {
				finalWidth = width - columnX - 1
			}
			cell.x, cell.y, cell.width = x+columnX+1, y+rowY, finalWidth

			// NOTE(blacknon): If isHeader is enabled, add arrow if it was a sort column
			text := cell.Text
			if cell.isHeader && columnIndex == t.GetSortClickedColumn() {
				if t.sortClickedDescending {
					text = append(text, []byte("▽")...)
				} else {
					text = append(text, []byte("△")...)
				}
			}
			_, printed := PrintStyle(screen, text, x+columnX+1, y+rowY, finalWidth, cell.Align, SetAttributes(tcell.StyleDefault.Foreground(cell.Color), cell.Attributes))
			if TaggedTextWidth(text)-printed > 0 && printed > 0 {
				_, _, style, _ := screen.GetContent(x+columnX+finalWidth, y+rowY)
				PrintStyle(screen, []byte(string(SemigraphicsHorizontalEllipsis)), x+columnX+finalWidth, y+rowY, 1, AlignLeft, style)
			}
		}

		// Draw bottom border.
		if rowY := 2 * len(rows); t.borders && rowY < height {
			for pos := 0; pos < columnWidth && columnX+1+pos < width; pos++ {
				drawBorder(columnX+pos+1, rowY, Borders.Horizontal)
			}
			ch := Borders.BottomT
			if columnIndex == 0 {
				ch = Borders.BottomLeft
			}
			drawBorder(columnX, rowY, ch)
		}

		columnX += columnWidth + 1
	}

	// Draw right border.
	if t.borders && len(t.cells) > 0 && columnX < width {
		for rowY := range rows {
			rowY *= 2
			if rowY+1 < height {
				drawBorder(columnX, rowY+1, Borders.Vertical)
			}
			ch := Borders.RightT
			if rowY == 0 {
				ch = Borders.TopRight
			}
			drawBorder(columnX, rowY, ch)
		}
		if rowY := 2 * len(rows); rowY < height {
			drawBorder(columnX, rowY, Borders.BottomRight)
		}
	}

	if showVerticalScrollBar {
		// Calculate scroll bar position and dimensions.
		rows := len(t.cells)

		scrollBarItems := rows - t.fixedRows
		scrollBarHeight := t.visibleRows - t.fixedRows

		scrollBarX := x + width
		scrollBarY := y + t.fixedRows
		if scrollBarX > x+tableWidth {
			scrollBarX = x + tableWidth
		}

		padTotalOffset := 1
		if t.borders {
			padTotalOffset = 2

			scrollBarItems *= 2
			scrollBarHeight = (scrollBarHeight * 2) - 1

			scrollBarY += t.fixedRows + 1
		}

		// Draw scroll bar.
		cursor := int(float64(scrollBarItems) * (float64(t.rowOffset) / float64(((rows-t.fixedRows)-t.visibleRows)+padTotalOffset)))
		for printed := 0; printed < scrollBarHeight; printed++ {
			RenderScrollBar(screen, t.scrollBarVisibility, scrollBarX, scrollBarY+printed, scrollBarHeight, scrollBarItems, cursor, printed, t.hasFocus, t.scrollBarColor)
		}
	}

	// TODO Draw horizontal scroll bar

	// Helper function which colors the background of a box.
	// backgroundColor == tcell.ColorDefault => Don't color the background.
	// textColor == tcell.ColorDefault => Don't change the text color.
	// attr == 0 => Don't change attributes.
	// invert == true => Ignore attr, set text to backgroundColor or t.backgroundColor;
	//                   set background to textColor.
	colorBackground := func(fromX, fromY, w, h int, backgroundColor, textColor tcell.Color, attr tcell.AttrMask, invert bool) {
		for by := 0; by < h && fromY+by < y+height; by++ {
			for bx := 0; bx < w && fromX+bx < x+width; bx++ {
				m, c, style, _ := screen.GetContent(fromX+bx, fromY+by)
				fg, bg, a := style.Decompose()
				if invert {
					if fg == textColor || fg == t.bordersColor {
						fg = backgroundColor
					}
					if fg == tcell.ColorDefault {
						fg = t.backgroundColor
					}
					style = style.Background(textColor).Foreground(fg)
				} else {
					if backgroundColor != tcell.ColorDefault {
						bg = backgroundColor
					}
					if textColor != tcell.ColorDefault {
						fg = textColor
					}
					if attr != 0 {
						a = attr
					}
					style = SetAttributes(style.Background(bg).Foreground(fg), a)
				}
				screen.SetContent(fromX+bx, fromY+by, m, c, style)
			}
		}
	}

	// Color the cell backgrounds. To avoid undesirable artefacts, we combine
	// the drawing of a cell by background color, selected cells last.
	type cellInfo struct {
		x, y, w, h  int
		color       tcell.Color
		selected    bool
		isHeader    bool
		columnIndex int
	}
	cellsByBackgroundColor := make(map[tcell.Color][]*cellInfo)
	var backgroundColors []tcell.Color
	for rowY, row := range rows {
		columnX := 0
		rowSelected := t.rowsSelectable && !t.columnsSelectable && row == t.selectedRow
		for columnIndex, column := range columns {
			columnWidth := widths[columnIndex]
			cell := getCell(row, column)
			if cell == nil {
				continue
			}
			bx, by, bw, bh := x+columnX, y+rowY, columnWidth+1, 1
			if t.borders {
				by = y + rowY*2
				bw++
				bh = 3
			}
			columnSelected := t.columnsSelectable && !t.rowsSelectable && column == t.selectedColumn
			cellSelected := !cell.NotSelectable && (columnSelected || rowSelected || t.rowsSelectable && t.columnsSelectable && column == t.selectedColumn && row == t.selectedRow)
			cell.isSelected = cellSelected // add: update isSelected
			entries, ok := cellsByBackgroundColor[cell.BackgroundColor]
			isHeader := cell.isHeader
			cellsByBackgroundColor[cell.BackgroundColor] = append(entries, &cellInfo{
				x:           bx,
				y:           by,
				w:           bw,
				h:           bh,
				color:       cell.Color,
				selected:    cellSelected,
				isHeader:    isHeader,
				columnIndex: columnIndex,
			})
			if !ok {
				backgroundColors = append(backgroundColors, cell.BackgroundColor)
			}
			columnX += columnWidth + 1
		}
	}
	sort.Slice(backgroundColors, func(i int, j int) bool {
		// Draw brightest colors last (i.e. on top).
		r, g, b := backgroundColors[i].RGB()
		c := colorful.Color{R: float64(r) / 255, G: float64(g) / 255, B: float64(b) / 255}
		_, _, li := c.Hcl()
		r, g, b = backgroundColors[j].RGB()
		c = colorful.Color{R: float64(r) / 255, G: float64(g) / 255, B: float64(b) / 255}
		_, _, lj := c.Hcl()
		return li < lj
	})
	selFg, selBg, selAttr := t.selectedStyle.Decompose()
	for _, bgColor := range backgroundColors {
		entries := cellsByBackgroundColor[bgColor]
		for _, cell := range entries {
			// @TODO(blacknon): ここにheaderとselected columnである場合の条件分岐を入れてやる
			if cell.columnIndex == t.sortClickedColumn && cell.isHeader {
				colorBackground(cell.x, cell.y, cell.w, cell.h, selBg, selFg, selAttr, false)
			} else {
				if cell.selected {
					if t.selectedStyle != tcell.StyleDefault {
						defer colorBackground(cell.x, cell.y, cell.w, cell.h, selBg, selFg, selAttr, false)
					} else {
						defer colorBackground(cell.x, cell.y, cell.w, cell.h, bgColor, cell.color, 0, true)
					}
				} else {
					colorBackground(cell.x, cell.y, cell.w, cell.h, bgColor, tcell.ColorDefault, 0, false)
				}
			}
		}
	}

	// Remember column infos.
	t.visibleColumnIndices, t.visibleColumnWidths = columns, widths
}

// InputHandler returns the handler for this primitive.
func (t *Table) InputHandler() func(event *tcell.EventKey, setFocus func(p Primitive)) {
	return t.WrapInputHandler(func(event *tcell.EventKey, setFocus func(p Primitive)) {
		t.Lock()
		defer t.Unlock()

		key := event.Key()

		if (!t.rowsSelectable && !t.columnsSelectable && key == tcell.KeyEnter) ||
			key == tcell.KeyEscape ||
			key == tcell.KeyTab ||
			key == tcell.KeyBacktab {
			if t.done != nil {
				t.Unlock()
				t.done(key)
				t.Lock()
			}
			return
		}

		// Movement functions.
		previouslySelectedRow, previouslySelectedColumn := t.selectedRow, t.selectedColumn
		var (
			validSelection = func(row, column int) bool {
				if row < t.fixedRows || row >= len(t.cells) || column < t.fixedColumns || column > t.lastColumn {
					return false
				}
				cell := t.cells[row][column]
				return cell == nil || !cell.NotSelectable
			}

			home = func() {
				if t.rowsSelectable {
					t.selectedRow = 0
					t.selectedColumn = 0
				} else {
					t.trackEnd = false
					t.rowOffset = 0
					t.columnOffset = 0
				}
			}

			end = func() {
				if t.rowsSelectable {
					t.selectedRow = len(t.cells) - 1
					t.selectedColumn = t.lastColumn
				} else {
					t.trackEnd = true
					t.columnOffset = 0
				}
			}

			down = func() {
				if t.rowsSelectable {
					if validSelection(t.selectedRow+1, t.selectedColumn) {
						t.selectedRow++
					}
				} else {
					t.rowOffset++
				}
			}

			up = func() {
				if t.rowsSelectable {
					if validSelection(t.selectedRow-1, t.selectedColumn) {
						t.selectedRow--
					}
				} else {
					t.trackEnd = false
					t.rowOffset--
				}
			}

			left = func() {
				if t.columnsSelectable {
					if validSelection(t.selectedRow, t.selectedColumn-1) {
						t.selectedColumn--
					}
				} else {
					t.columnOffset--
				}
			}

			right = func() {
				if t.columnsSelectable {
					if validSelection(t.selectedRow, t.selectedColumn+1) {
						t.selectedColumn++
					}
				} else {
					t.columnOffset++
				}
			}

			pageDown = func() {
				offsetAmount := t.visibleRows - t.fixedRows
				if offsetAmount < 0 {
					offsetAmount = 0
				}

				if t.rowsSelectable {
					t.selectedRow += offsetAmount
					if t.selectedRow >= len(t.cells) {
						t.selectedRow = len(t.cells) - 1
					}
				} else {
					t.rowOffset += offsetAmount
				}
			}

			pageUp = func() {
				offsetAmount := t.visibleRows - t.fixedRows
				if offsetAmount < 0 {
					offsetAmount = 0
				}

				if t.rowsSelectable {
					t.selectedRow -= offsetAmount
					if t.selectedRow < 0 {
						t.selectedRow = 0
					}
				} else {
					t.trackEnd = false
					t.rowOffset -= offsetAmount
				}
			}
		)

		if HitShortcut(event, Keys.MoveFirst, Keys.MoveFirst2) {
			home()
		} else if HitShortcut(event, Keys.MoveLast, Keys.MoveLast2) {
			end()
		} else if HitShortcut(event, Keys.MoveUp, Keys.MoveUp2) {
			up()
		} else if HitShortcut(event, Keys.MoveDown, Keys.MoveDown2) {
			down()
		} else if HitShortcut(event, Keys.MoveLeft, Keys.MoveLeft2) {
			left()
		} else if HitShortcut(event, Keys.MoveRight, Keys.MoveRight2) {
			right()
		} else if HitShortcut(event, Keys.MovePreviousPage) {
			pageUp()
		} else if HitShortcut(event, Keys.MoveNextPage) {
			pageDown()
		} else if HitShortcut(event, Keys.Select, Keys.Select2) {
			if (t.rowsSelectable || t.columnsSelectable) && t.selected != nil {
				t.Unlock()
				t.selected(t.selectedRow, t.selectedColumn)
				t.Lock()
			}
		}

		// If the selection has changed, notify the handler.
		if t.selectionChanged != nil && ((t.rowsSelectable && previouslySelectedRow != t.selectedRow) || (t.columnsSelectable && previouslySelectedColumn != t.selectedColumn)) {
			t.Unlock()
			t.selectionChanged(t.selectedRow, t.selectedColumn)
			t.Lock()
		}
	})
}

// MouseHandler returns the mouse handler for this primitive.
func (t *Table) MouseHandler() func(action MouseAction, event *tcell.EventMouse, setFocus func(p Primitive)) (consumed bool, capture Primitive) {
	return t.WrapMouseHandler(func(action MouseAction, event *tcell.EventMouse, setFocus func(p Primitive)) (consumed bool, capture Primitive) {
		x, y := event.Position()
		if !t.InRect(x, y) {
			return false, nil
		}

		switch action {
		case MouseLeftClick:
			_, tableY, _, _ := t.GetInnerRect()
			mul := 1
			maxY := tableY
			if t.borders {
				mul = 2
				maxY = tableY + 1
			}

			if t.sortClicked && t.fixedRows > 0 && (y >= tableY && y < maxY+(t.fixedRows*mul)) {
				_, column := t.cellAt(x, y)
				if t.sortClickedColumn != column {
					t.sortClickedColumn = column
					t.sortClickedDescending = false
				} else {
					t.sortClickedDescending = !t.sortClickedDescending
				}
				t.Sort(column, t.sortClickedDescending)

				if t.columnsSelectable {
					t.selectedColumn = column
				}
			} else if t.rowsSelectable || t.columnsSelectable {
				t.Select(t.cellAt(x, y))
			}

			consumed = true
			setFocus(t)
		case MouseScrollUp:
			t.trackEnd = false
			t.rowOffset--
			consumed = true
		case MouseScrollDown:
			t.rowOffset++
			consumed = true
		}

		return
	})
}

func (t *Table) findSelected() (row, column int) {
	for rowIndex, row := range t.cells {
		for colIndex, cell := range row {
			if cell != nil && cell.isSelected {
				return rowIndex, colIndex
			}
		}
	}
	return 0, 0
}

func (t *Table) unsetSelected() {
	for _, row := range t.cells {
		for _, cell := range row {
			if cell != nil && cell.isSelected {
				t.Lock()
				cell.isSelected = false
				t.Unlock()
			}
		}
	}
}
