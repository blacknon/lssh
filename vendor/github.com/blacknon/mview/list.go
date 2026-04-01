package mview

import (
	"bytes"
	"fmt"
	"strings"
	"sync"

	"github.com/gdamore/tcell/v2"
)

// ListItem represents an item in a List.
type ListItem struct {
	disabled      bool        // Whether or not the list item is selectable.
	mainText      []byte      // The main text of the list item.
	secondaryText []byte      // A secondary text to be shown underneath the main text.
	shortcut      rune        // The key to select the list item directly, 0 if there is no shortcut.
	selected      func()      // The optional function which is called when the item is selected.
	reference     interface{} // An optional reference object.

	sync.RWMutex
}

// NewListItem returns a new item for a list.
func NewListItem(mainText string) *ListItem {
	return &ListItem{
		mainText: []byte(mainText),
	}
}

// SetMainBytes sets the main text of the list item.
func (l *ListItem) SetMainBytes(val []byte) {
	l.Lock()
	defer l.Unlock()

	l.mainText = val
}

// SetMainText sets the main text of the list item.
func (l *ListItem) SetMainText(val string) {
	l.SetMainBytes([]byte(val))
}

// GetMainBytes returns the item's main text.
func (l *ListItem) GetMainBytes() []byte {
	l.RLock()
	defer l.RUnlock()

	return l.mainText
}

// GetMainText returns the item's main text.
func (l *ListItem) GetMainText() string {
	return string(l.GetMainBytes())
}

// SetSecondaryBytes sets a secondary text to be shown underneath the main text.
func (l *ListItem) SetSecondaryBytes(val []byte) {
	l.Lock()
	defer l.Unlock()

	l.secondaryText = val
}

// SetSecondaryText sets a secondary text to be shown underneath the main text.
func (l *ListItem) SetSecondaryText(val string) {
	l.SetSecondaryBytes([]byte(val))
}

// GetSecondaryBytes returns the item's secondary text.
func (l *ListItem) GetSecondaryBytes() []byte {
	l.RLock()
	defer l.RUnlock()

	return l.secondaryText
}

// GetSecondaryText returns the item's secondary text.
func (l *ListItem) GetSecondaryText() string {
	return string(l.GetSecondaryBytes())
}

// SetShortcut sets the key to select the ListItem directly, 0 if there is no shortcut.
func (l *ListItem) SetShortcut(val rune) {
	l.Lock()
	defer l.Unlock()

	l.shortcut = val
}

// GetShortcut returns the ListItem's shortcut.
func (l *ListItem) GetShortcut() rune {
	l.RLock()
	defer l.RUnlock()

	return l.shortcut
}

// SetSelectedFunc sets a function which is called when the ListItem is selected.
func (l *ListItem) SetSelectedFunc(handler func()) {
	l.Lock()
	defer l.Unlock()

	l.selected = handler
}

// SetReference allows you to store a reference of any type in the item
func (l *ListItem) SetReference(val interface{}) {
	l.Lock()
	defer l.Unlock()

	l.reference = val
}

// GetReference returns the item's reference object.
func (l *ListItem) GetReference() interface{} {
	l.RLock()
	defer l.RUnlock()

	return l.reference
}

// List displays rows of items, each of which can be selected.
type List struct {
	*Box
	*ContextMenu

	// The items of the list.
	items []*ListItem

	// The index of the currently selected item.
	currentItem int

	// Whether or not to show the secondary item texts.
	showSecondaryText bool

	// The item main text color.
	mainTextColor tcell.Color

	// The item secondary text color.
	secondaryTextColor tcell.Color

	// The item shortcut text color.
	shortcutColor tcell.Color

	// The text color for selected items.
	selectedTextColor tcell.Color

	// The style attributes for selected items.
	selectedTextAttributes tcell.AttrMask

	// Visibility of the scroll bar.
	scrollBarVisibility ScrollBarVisibility

	// The scroll bar color.
	scrollBarColor tcell.Color

	// The background color for selected items.
	selectedBackgroundColor tcell.Color

	// If true, the selection is only shown when the list has focus.
	selectedFocusOnly bool

	// If true, the selection must remain visible when scrolling.
	selectedAlwaysVisible bool

	// If true, the selection must remain centered when scrolling.
	selectedAlwaysCentered bool

	// If true, the entire row is highlighted when selected.
	highlightFullLine bool

	// Whether or not navigating the list will wrap around.
	wrapAround bool

	// Whether or not hovering over an item will highlight it.
	hover bool

	// The number of list items and columns by which the list is scrolled
	// down/to the right.
	itemOffset, columnOffset int

	// An optional function which is called when the user has navigated to a list
	// item.
	changed func(index int, item *ListItem)

	// An optional function which is called when a list item was selected. This
	// function will be called even if the list item defines its own callback.
	selected func(index int, item *ListItem)

	// An optional function which is called when the user presses the Escape key.
	done func()

	// The height of the list the last time it was drawn.
	height int

	// Prefix and suffix strings drawn for unselected elements.
	unselectedPrefix, unselectedSuffix []byte

	// Prefix and suffix strings drawn for selected elements.
	selectedPrefix, selectedSuffix []byte

	// Maximum prefix and suffix width.
	prefixWidth, suffixWidth int

	sync.RWMutex
}

// NewList returns a new form.
func NewList() *List {
	l := &List{
		Box:                     NewBox(),
		showSecondaryText:       true,
		scrollBarVisibility:     ScrollBarAuto,
		mainTextColor:           Styles.PrimaryTextColor,
		secondaryTextColor:      Styles.TertiaryTextColor,
		shortcutColor:           Styles.SecondaryTextColor,
		selectedTextColor:       Styles.PrimitiveBackgroundColor,
		scrollBarColor:          Styles.ScrollBarColor,
		selectedBackgroundColor: Styles.PrimaryTextColor,
	}

	l.ContextMenu = NewContextMenu(l)
	l.focus = l

	return l
}

// SetCurrentItem sets the currently selected item by its index, starting at 0
// for the first item. If a negative index is provided, items are referred to
// from the back (-1 = last item, -2 = second-to-last item, and so on). Out of
// range indices are clamped to the beginning/end.
//
// Calling this function triggers a "changed" event if the selection changes.
func (l *List) SetCurrentItem(index int) {
	l.Lock()

	if index < 0 {
		index = len(l.items) + index
	}
	if index >= len(l.items) {
		index = len(l.items) - 1
	}
	if index < 0 {
		index = 0
	}

	previousItem := l.currentItem
	l.currentItem = index

	l.updateOffset()

	if index != previousItem && index < len(l.items) && l.changed != nil {
		item := l.items[index]
		l.Unlock()
		l.changed(index, item)
	} else {
		l.Unlock()
	}
}

// GetCurrentItem returns the currently selected list item,
// Returns nil if no item is selected.
func (l *List) GetCurrentItem() *ListItem {
	l.RLock()
	defer l.RUnlock()

	if len(l.items) == 0 || l.currentItem >= len(l.items) {
		return nil
	}
	return l.items[l.currentItem]
}

// GetCurrentItemIndex returns the index of the currently selected list item,
// starting at 0 for the first item and its struct.
func (l *List) GetCurrentItemIndex() int {
	l.RLock()
	defer l.RUnlock()
	return l.currentItem
}

// GetItems returns all list items.
func (l *List) GetItems() []*ListItem {
	l.RLock()
	defer l.RUnlock()
	return l.items
}

// RemoveItem removes the item with the given index (starting at 0) from the
// list. If a negative index is provided, items are referred to from the back
// (-1 = last item, -2 = second-to-last item, and so on). Out of range indices
// are clamped to the beginning/end, i.e. unless the list is empty, an item is
// always removed.
//
// The currently selected item is shifted accordingly. If it is the one that is
// removed, a "changed" event is fired.
func (l *List) RemoveItem(index int) {
	l.Lock()

	if len(l.items) == 0 {
		l.Unlock()
		return
	}

	// Adjust index.
	if index < 0 {
		index = len(l.items) + index
	}
	if index >= len(l.items) {
		index = len(l.items) - 1
	}
	if index < 0 {
		index = 0
	}

	// Remove item.
	l.items = append(l.items[:index], l.items[index+1:]...)

	// If there is nothing left, we're done.
	if len(l.items) == 0 {
		l.Unlock()
		return
	}

	// Shift current item.
	previousItem := l.currentItem
	if l.currentItem >= index && l.currentItem > 0 {
		l.currentItem--
	}

	// Fire "changed" event for removed items.
	if previousItem == index && index < len(l.items) && l.changed != nil {
		item := l.items[l.currentItem]
		l.Unlock()
		l.changed(l.currentItem, item)
	} else {
		l.Unlock()
	}
}

// SetOffset sets the number of list items and columns by which the list is
// scrolled down/to the right.
func (l *List) SetOffset(items, columns int) {
	l.Lock()
	defer l.Unlock()

	if items < 0 {
		items = 0
	}
	if columns < 0 {
		columns = 0
	}

	l.itemOffset, l.columnOffset = items, columns
}

// GetOffset returns the number of list items and columns by which the list is
// scrolled down/to the right.
func (l *List) GetOffset() (int, int) {
	l.Lock()
	defer l.Unlock()

	return l.itemOffset, l.columnOffset
}

// SetMainTextColor sets the color of the items' main text.
func (l *List) SetMainTextColor(color tcell.Color) {
	l.Lock()
	defer l.Unlock()

	l.mainTextColor = color
}

// SetSecondaryTextColor sets the color of the items' secondary text.
func (l *List) SetSecondaryTextColor(color tcell.Color) {
	l.Lock()
	defer l.Unlock()

	l.secondaryTextColor = color
}

// SetShortcutColor sets the color of the items' shortcut.
func (l *List) SetShortcutColor(color tcell.Color) {
	l.Lock()
	defer l.Unlock()

	l.shortcutColor = color
}

// SetSelectedTextColor sets the text color of selected items.
func (l *List) SetSelectedTextColor(color tcell.Color) {
	l.Lock()
	defer l.Unlock()

	l.selectedTextColor = color
}

// SetSelectedTextAttributes sets the style attributes of selected items.
func (l *List) SetSelectedTextAttributes(attr tcell.AttrMask) {
	l.Lock()
	defer l.Unlock()

	l.selectedTextAttributes = attr
}

// SetSelectedBackgroundColor sets the background color of selected items.
func (l *List) SetSelectedBackgroundColor(color tcell.Color) {
	l.Lock()
	defer l.Unlock()

	l.selectedBackgroundColor = color
}

// SetSelectedFocusOnly sets a flag which determines when the currently selected
// list item is highlighted. If set to true, selected items are only highlighted
// when the list has focus. If set to false, they are always highlighted.
func (l *List) SetSelectedFocusOnly(focusOnly bool) {
	l.Lock()
	defer l.Unlock()

	l.selectedFocusOnly = focusOnly
}

// SetSelectedAlwaysVisible sets a flag which determines whether the currently
// selected list item must remain visible when scrolling.
func (l *List) SetSelectedAlwaysVisible(alwaysVisible bool) {
	l.Lock()
	defer l.Unlock()

	l.selectedAlwaysVisible = alwaysVisible
}

// SetSelectedAlwaysCentered sets a flag which determines whether the currently
// selected list item must remain centered when scrolling.
func (l *List) SetSelectedAlwaysCentered(alwaysCentered bool) {
	l.Lock()
	defer l.Unlock()

	l.selectedAlwaysCentered = alwaysCentered
}

// SetHighlightFullLine sets a flag which determines whether the colored
// background of selected items spans the entire width of the view. If set to
// true, the highlight spans the entire view. If set to false, only the text of
// the selected item from beginning to end is highlighted.
func (l *List) SetHighlightFullLine(highlight bool) {
	l.Lock()
	defer l.Unlock()

	l.highlightFullLine = highlight
}

// ShowSecondaryText determines whether or not to show secondary item texts.
func (l *List) ShowSecondaryText(show bool) {
	l.Lock()
	defer l.Unlock()

	l.showSecondaryText = show
	return
}

// SetScrollBarVisibility specifies the display of the scroll bar.
func (l *List) SetScrollBarVisibility(visibility ScrollBarVisibility) {
	l.Lock()
	defer l.Unlock()

	l.scrollBarVisibility = visibility
}

// SetScrollBarColor sets the color of the scroll bar.
func (l *List) SetScrollBarColor(color tcell.Color) {
	l.Lock()
	defer l.Unlock()

	l.scrollBarColor = color
}

// SetHover sets the flag that determines whether hovering over an item will
// highlight it (without triggering callbacks set with SetSelectedFunc).
func (l *List) SetHover(hover bool) {
	l.Lock()
	defer l.Unlock()

	l.hover = hover
}

// SetWrapAround sets the flag that determines whether navigating the list will
// wrap around. That is, navigating downwards on the last item will move the
// selection to the first item (similarly in the other direction). If set to
// false, the selection won't change when navigating downwards on the last item
// or navigating upwards on the first item.
func (l *List) SetWrapAround(wrapAround bool) {
	l.Lock()
	defer l.Unlock()

	l.wrapAround = wrapAround
}

// SetChangedFunc sets the function which is called when the user navigates to
// a list item. The function receives the item's index in the list of items
// (starting with 0) and the list item.
//
// This function is also called when the first item is added or when
// SetCurrentItem() is called.
func (l *List) SetChangedFunc(handler func(index int, item *ListItem)) {
	l.Lock()
	defer l.Unlock()

	l.changed = handler
}

// SetSelectedFunc sets the function which is called when the user selects a
// list item by pressing Enter on the current selection. The function receives
// the item's index in the list of items (starting with 0) and its struct.
func (l *List) SetSelectedFunc(handler func(int, *ListItem)) {
	l.Lock()
	defer l.Unlock()

	l.selected = handler
}

// SetDoneFunc sets a function which is called when the user presses the Escape
// key.
func (l *List) SetDoneFunc(handler func()) {
	l.Lock()
	defer l.Unlock()

	l.done = handler
}

// AddItem calls InsertItem() with an index of -1.
func (l *List) AddItem(item *ListItem) {
	l.InsertItem(-1, item)
}

// InsertItem adds a new item to the list at the specified index. An index of 0
// will insert the item at the beginning, an index of 1 before the second item,
// and so on. An index of GetItemCount() or higher will insert the item at the
// end of the list. Negative indices are also allowed: An index of -1 will
// insert the item at the end of the list, an index of -2 before the last item,
// and so on. An index of -GetItemCount()-1 or lower will insert the item at the
// beginning.
//
// An item has a main text which will be highlighted when selected. It also has
// a secondary text which is shown underneath the main text (if it is set to
// visible) but which may remain empty.
//
// The shortcut is a key binding. If the specified rune is entered, the item
// is selected immediately. Set to 0 for no binding.
//
// The "selected" callback will be invoked when the user selects the item. You
// may provide nil if no such callback is needed or if all events are handled
// through the selected callback set with SetSelectedFunc().
//
// The currently selected item will shift its position accordingly. If the list
// was previously empty, a "changed" event is fired because the new item becomes
// selected.
func (l *List) InsertItem(index int, item *ListItem) {
	l.Lock()

	// Shift index to range.
	if index < 0 {
		index = len(l.items) + index + 1
	}
	if index < 0 {
		index = 0
	} else if index > len(l.items) {
		index = len(l.items)
	}

	// Shift current item.
	if l.currentItem < len(l.items) && l.currentItem >= index {
		l.currentItem++
	}

	// Insert item (make space for the new item, then shift and insert).
	l.items = append(l.items, nil)
	if index < len(l.items)-1 { // -1 because l.items has already grown by one item.
		copy(l.items[index+1:], l.items[index:])
	}
	l.items[index] = item

	// Fire a "change" event for the first item in the list.
	if len(l.items) == 1 && l.changed != nil {
		item := l.items[0]
		l.Unlock()
		l.changed(0, item)
	} else {
		l.Unlock()
	}
}

// GetItem returns the ListItem at the given index.
// Returns nil when index is out of bounds.
func (l *List) GetItem(index int) *ListItem {
	if index > len(l.items)-1 {
		return nil
	}
	return l.items[index]
}

// GetItemCount returns the number of items in the list.
func (l *List) GetItemCount() int {
	l.RLock()
	defer l.RUnlock()

	return len(l.items)
}

// GetItemText returns an item's texts (main and secondary). Panics if the index
// is out of range.
func (l *List) GetItemText(index int) (main, secondary string) {
	l.RLock()
	defer l.RUnlock()
	return string(l.items[index].mainText), string(l.items[index].secondaryText)
}

// SetItemText sets an item's main and secondary text. Panics if the index is
// out of range.
func (l *List) SetItemText(index int, main, secondary string) {
	l.Lock()
	defer l.Unlock()

	item := l.items[index]
	item.mainText = []byte(main)
	item.secondaryText = []byte(secondary)
}

// SetItemEnabled sets whether an item is selectable. Panics if the index is
// out of range.
func (l *List) SetItemEnabled(index int, enabled bool) {
	l.Lock()
	defer l.Unlock()

	item := l.items[index]
	item.disabled = !enabled
}

// SetIndicators is used to set prefix and suffix indicators for selected and unselected items.
func (l *List) SetIndicators(selectedPrefix, selectedSuffix, unselectedPrefix, unselectedSuffix string) {
	l.Lock()
	defer l.Unlock()
	l.selectedPrefix = []byte(selectedPrefix)
	l.selectedSuffix = []byte(selectedSuffix)
	l.unselectedPrefix = []byte(unselectedPrefix)
	l.unselectedSuffix = []byte(unselectedSuffix)
	l.prefixWidth = len(selectedPrefix)
	if len(unselectedPrefix) > l.prefixWidth {
		l.prefixWidth = len(unselectedPrefix)
	}
	l.suffixWidth = len(selectedSuffix)
	if len(unselectedSuffix) > l.suffixWidth {
		l.suffixWidth = len(unselectedSuffix)
	}
}

// FindItems searches the main and secondary texts for the given strings and
// returns a list of item indices in which those strings are found. One of the
// two search strings may be empty, it will then be ignored. Indices are always
// returned in ascending order.
//
// If mustContainBoth is set to true, mainSearch must be contained in the main
// text AND secondarySearch must be contained in the secondary text. If it is
// false, only one of the two search strings must be contained.
//
// Set ignoreCase to true for case-insensitive search.
func (l *List) FindItems(mainSearch, secondarySearch string, mustContainBoth, ignoreCase bool) (indices []int) {
	l.RLock()
	defer l.RUnlock()

	if mainSearch == "" && secondarySearch == "" {
		return
	}

	if ignoreCase {
		mainSearch = strings.ToLower(mainSearch)
		secondarySearch = strings.ToLower(secondarySearch)
	}

	mainSearchBytes := []byte(mainSearch)
	secondarySearchBytes := []byte(secondarySearch)

	for index, item := range l.items {
		mainText := item.mainText
		secondaryText := item.secondaryText
		if ignoreCase {
			mainText = bytes.ToLower(mainText)
			secondaryText = bytes.ToLower(secondaryText)
		}

		// strings.Contains() always returns true for a "" search.
		mainContained := bytes.Contains(mainText, mainSearchBytes)
		secondaryContained := bytes.Contains(secondaryText, secondarySearchBytes)
		if mustContainBoth && mainContained && secondaryContained ||
			!mustContainBoth && (len(mainText) > 0 && mainContained || len(secondaryText) > 0 && secondaryContained) {
			indices = append(indices, index)
		}
	}

	return
}

// Clear removes all items from the list.
func (l *List) Clear() {
	l.Lock()
	defer l.Unlock()

	l.items = nil
	l.currentItem = 0
	l.itemOffset = 0
	l.columnOffset = 0
}

// Focus is called by the application when the primitive receives focus.
func (l *List) Focus(delegate func(p Primitive)) {
	l.Box.Focus(delegate)
	if l.ContextMenu.open {
		delegate(l.ContextMenu.list)
	}
}

// HasFocus returns whether or not this primitive has focus.
func (l *List) HasFocus() bool {
	if l.ContextMenu.open {
		return l.ContextMenu.list.HasFocus()
	}

	l.RLock()
	defer l.RUnlock()
	return l.hasFocus
}

// Transform modifies the current selection.
func (l *List) Transform(tr Transformation) {
	l.Lock()

	previousItem := l.currentItem

	l.transform(tr)

	if l.currentItem != previousItem && l.currentItem < len(l.items) && l.changed != nil {
		item := l.items[l.currentItem]
		l.Unlock()
		l.changed(l.currentItem, item)
	} else {
		l.Unlock()
	}
}

func (l *List) transform(tr Transformation) {
	var decreasing bool

	pageItems := l.height
	if l.showSecondaryText {
		pageItems /= 2
	}
	if pageItems < 1 {
		pageItems = 1
	}

	switch tr {
	case TransformFirstItem:
		l.currentItem = 0
		l.itemOffset = 0
		decreasing = true
	case TransformLastItem:
		l.currentItem = len(l.items) - 1
	case TransformPreviousItem:
		l.currentItem--
		decreasing = true
	case TransformNextItem:
		l.currentItem++
	case TransformPreviousPage:
		l.currentItem -= pageItems
		decreasing = true
	case TransformNextPage:
		l.currentItem += pageItems
		l.itemOffset += pageItems
	}

	for i := 0; i < len(l.items); i++ {
		if l.currentItem < 0 {
			if l.wrapAround {
				l.currentItem = len(l.items) - 1
			} else {
				l.currentItem = 0
				l.itemOffset = 0
			}
		} else if l.currentItem >= len(l.items) {
			if l.wrapAround {
				l.currentItem = 0
				l.itemOffset = 0
			} else {
				l.currentItem = len(l.items) - 1
			}
		}

		item := l.items[l.currentItem]
		if !item.disabled && (item.shortcut > 0 || len(item.mainText) > 0 || len(item.secondaryText) > 0) {
			break
		}

		if decreasing {
			l.currentItem--
		} else {
			l.currentItem++
		}
	}

	l.updateOffset()
}

func (l *List) updateOffset() {
	_, _, _, l.height = l.GetInnerRect()

	h := l.height
	if l.selectedAlwaysCentered {
		h /= 2
	}

	if l.currentItem < l.itemOffset {
		l.itemOffset = l.currentItem
	} else if l.showSecondaryText {
		if 2*(l.currentItem-l.itemOffset) >= h-1 {
			l.itemOffset = (2*l.currentItem + 3 - h) / 2
		}
	} else {
		if l.currentItem-l.itemOffset >= h {
			l.itemOffset = l.currentItem + 1 - h
		}
	}

	if l.showSecondaryText {
		if l.itemOffset > len(l.items)-(l.height/2) {
			l.itemOffset = len(l.items) - l.height/2
		}
	} else {
		if l.itemOffset > len(l.items)-l.height {
			l.itemOffset = len(l.items) - l.height
		}
	}

	if l.itemOffset < 0 {
		l.itemOffset = 0
	}

	// Maximum width of item text
	maxWidth := 0
	for _, option := range l.items {
		strWidth := TaggedTextWidth(option.mainText)
		secondaryWidth := TaggedTextWidth(option.secondaryText)
		if secondaryWidth > strWidth {
			strWidth = secondaryWidth
		}
		if option.shortcut != 0 {
			strWidth += 4
		}

		if strWidth > maxWidth {
			maxWidth = strWidth
		}
	}

	// Additional width for scroll bar
	addWidth := 0
	if l.scrollBarVisibility == ScrollBarAlways ||
		(l.scrollBarVisibility == ScrollBarAuto &&
			((!l.showSecondaryText && len(l.items) > l.innerHeight) ||
				(l.showSecondaryText && len(l.items) > l.innerHeight/2))) {
		addWidth = 1
	}

	if l.columnOffset > (maxWidth-l.innerWidth)+addWidth {
		l.columnOffset = (maxWidth - l.innerWidth) + addWidth
	}
	if l.columnOffset < 0 {
		l.columnOffset = 0
	}
}

// Draw draws this primitive onto the screen.
func (l *List) Draw(screen tcell.Screen) {
	if !l.GetVisible() {
		return
	}

	l.Box.Draw(screen)
	hasFocus := l.GetFocusable().HasFocus()

	l.Lock()
	defer l.Unlock()

	// Determine the dimensions.
	x, y, width, height := l.GetInnerRect()
	leftEdge := x
	fullWidth := width + l.paddingLeft + l.paddingRight + l.prefixWidth + l.suffixWidth
	bottomLimit := y + height

	l.height = height

	screenWidth, _ := screen.Size()
	scrollBarHeight := height
	scrollBarX := x + (width - 1) + l.paddingLeft + l.paddingRight
	if scrollBarX > screenWidth-1 {
		scrollBarX = screenWidth - 1
	}

	// Halve scroll bar height when drawing two lines per list item.
	if l.showSecondaryText {
		scrollBarHeight /= 2
	}

	// Do we show any shortcuts?
	var showShortcuts bool
	for _, item := range l.items {
		if item.shortcut != 0 {
			showShortcuts = true
			x += 4
			width -= 4
			break
		}
	}

	// Adjust offset to keep the current selection in view.
	if l.selectedAlwaysVisible || l.selectedAlwaysCentered {
		l.updateOffset()
	}

	scrollBarCursor := int(float64(len(l.items)) * (float64(l.itemOffset) / float64(len(l.items)-height)))

	// Draw the list items.
	for index, item := range l.items {
		if index < l.itemOffset {
			continue
		}

		if y >= bottomLimit {
			break
		}

		mainText := item.mainText
		secondaryText := item.secondaryText
		if l.columnOffset > 0 {
			if l.columnOffset < len(mainText) {
				mainText = mainText[l.columnOffset:]
			} else {
				mainText = nil
			}
			if l.columnOffset < len(secondaryText) {
				secondaryText = secondaryText[l.columnOffset:]
			} else {
				secondaryText = nil
			}
		}

		if len(item.mainText) == 0 && len(item.secondaryText) == 0 && item.shortcut == 0 { // Divider
			Print(screen, []byte(string(tcell.RuneLTee)), leftEdge-2, y, 1, AlignLeft, l.mainTextColor)
			Print(screen, bytes.Repeat([]byte(string(tcell.RuneHLine)), fullWidth), leftEdge-1, y, fullWidth, AlignLeft, l.mainTextColor)
			Print(screen, []byte(string(tcell.RuneRTee)), leftEdge+fullWidth-1, y, 1, AlignLeft, l.mainTextColor)

			RenderScrollBar(screen, l.scrollBarVisibility, scrollBarX, y, scrollBarHeight, len(l.items), scrollBarCursor, index-l.itemOffset, l.hasFocus, l.scrollBarColor)
			y++
			continue
		}

		if index == l.currentItem {
			if len(l.selectedPrefix) > 0 {
				mainText = append(l.selectedPrefix, mainText...)
			}
			if len(l.selectedSuffix) > 0 {
				mainText = append(mainText, l.selectedSuffix...)
			}

		} else {
			if len(l.unselectedPrefix) > 0 {
				mainText = append(l.unselectedPrefix, mainText...)
			}
			if len(l.unselectedSuffix) > 0 {
				mainText = append(mainText, l.unselectedSuffix...)
			}
		}
		if item.disabled {
			// Shortcuts.
			if showShortcuts && item.shortcut != 0 {
				Print(screen, []byte(fmt.Sprintf("(%c)", item.shortcut)), x-5, y, 4, AlignRight, tcell.ColorDarkSlateGray.TrueColor())
			}

			// Main text.
			Print(screen, mainText, x, y, width, AlignLeft, tcell.ColorGray.TrueColor())

			RenderScrollBar(screen, l.scrollBarVisibility, scrollBarX, y, scrollBarHeight, len(l.items), scrollBarCursor, index-l.itemOffset, l.hasFocus, l.scrollBarColor)
			y++
			continue
		}

		// Shortcuts.
		if showShortcuts && item.shortcut != 0 {
			Print(screen, []byte(fmt.Sprintf("(%c)", item.shortcut)), x-5, y, 4, AlignRight, l.shortcutColor)
		}

		// Main text.
		Print(screen, mainText, x, y, width, AlignLeft, l.mainTextColor)

		// Background color of selected text.
		if index == l.currentItem && (!l.selectedFocusOnly || hasFocus) {
			textWidth := width
			if !l.highlightFullLine {
				if w := TaggedTextWidth(mainText); w < textWidth {
					textWidth = w
				}
			}

			for bx := 0; bx < textWidth; bx++ {
				m, c, style, _ := screen.GetContent(x+bx, y)
				fg, _, _ := style.Decompose()
				if fg == l.mainTextColor {
					fg = l.selectedTextColor
				}
				style = SetAttributes(style.Background(l.selectedBackgroundColor).Foreground(fg), l.selectedTextAttributes)
				screen.SetContent(x+bx, y, m, c, style)
			}
		}

		RenderScrollBar(screen, l.scrollBarVisibility, scrollBarX, y, scrollBarHeight, len(l.items), scrollBarCursor, index-l.itemOffset, l.hasFocus, l.scrollBarColor)

		y++

		if y >= bottomLimit {
			break
		}

		// Secondary text.
		if l.showSecondaryText {
			Print(screen, secondaryText, x, y, width, AlignLeft, l.secondaryTextColor)

			RenderScrollBar(screen, l.scrollBarVisibility, scrollBarX, y, scrollBarHeight, len(l.items), scrollBarCursor, index-l.itemOffset, l.hasFocus, l.scrollBarColor)

			y++
		}
	}

	// Overdraw scroll bar when necessary.
	for y < bottomLimit {
		RenderScrollBar(screen, l.scrollBarVisibility, scrollBarX, y, scrollBarHeight, len(l.items), scrollBarCursor, bottomLimit-y, l.hasFocus, l.scrollBarColor)

		y++
	}

	// Draw context menu.
	if hasFocus && l.ContextMenu.open {
		ctx := l.ContextMenuList()

		x, y, width, height = l.GetInnerRect()

		// What's the longest option text?
		maxWidth := 0
		for _, option := range ctx.items {
			strWidth := TaggedTextWidth(option.mainText)
			if option.shortcut != 0 {
				strWidth += 4
			}
			if strWidth > maxWidth {
				maxWidth = strWidth
			}
		}

		lheight := len(ctx.items)
		lwidth := maxWidth

		// Add space for borders
		lwidth += 2
		lheight += 2

		lwidth += ctx.paddingLeft + ctx.paddingRight
		lheight += ctx.paddingTop + ctx.paddingBottom

		cx, cy := l.ContextMenu.x, l.ContextMenu.y
		if cx < 0 || cy < 0 {
			offsetX := 7
			if showShortcuts {
				offsetX += 4
			}
			offsetY := l.currentItem
			if l.showSecondaryText {
				offsetY *= 2
			}
			x, y, _, _ := l.GetInnerRect()
			cx, cy = x+offsetX, y+offsetY
		}

		_, sheight := screen.Size()
		if cy+lheight >= sheight && cy-2 > lheight-cy {
			for i := (cy + lheight) - sheight; i > 0; i-- {
				cy--
				if cy+lheight < sheight {
					break
				}
			}
			if cy < 0 {
				cy = 0
			}
		}
		if cy+lheight >= sheight {
			lheight = sheight - cy
		}

		if ctx.scrollBarVisibility == ScrollBarAlways || (ctx.scrollBarVisibility == ScrollBarAuto && len(ctx.items) > lheight) {
			lwidth++ // Add space for scroll bar
		}

		ctx.SetRect(cx, cy, lwidth, lheight)
		ctx.Draw(screen)
	}
}

// InputHandler returns the handler for this primitive.
func (l *List) InputHandler() func(event *tcell.EventKey, setFocus func(p Primitive)) {
	return l.WrapInputHandler(func(event *tcell.EventKey, setFocus func(p Primitive)) {
		l.Lock()

		if HitShortcut(event, Keys.Cancel) {
			if l.ContextMenu.open {
				l.Unlock()

				l.ContextMenu.hide(setFocus)
				return
			}

			if l.done != nil {
				l.Unlock()
				l.done()
			} else {
				l.Unlock()
			}
			return
		} else if HitShortcut(event, Keys.Select, Keys.Select2) {
			if l.currentItem >= 0 && l.currentItem < len(l.items) {
				item := l.items[l.currentItem]
				if !item.disabled {
					if item.selected != nil {
						l.Unlock()
						item.selected()
						l.Lock()
					}
					if l.selected != nil {
						l.Unlock()
						l.selected(l.currentItem, item)
						l.Lock()
					}
				}
			}
		} else if HitShortcut(event, Keys.ShowContextMenu) {
			defer l.ContextMenu.show(l.currentItem, -1, -1, setFocus)
		} else if len(l.items) == 0 {
			l.Unlock()
			return
		}

		if event.Key() == tcell.KeyRune {
			ch := event.Rune()
			if ch != ' ' {
				// It's not a space bar. Is it a shortcut?
				for index, item := range l.items {
					if !item.disabled && item.shortcut == ch {
						// We have a shortcut.
						l.currentItem = index

						item := l.items[l.currentItem]
						if item.selected != nil {
							l.Unlock()
							item.selected()
							l.Lock()
						}
						if l.selected != nil {
							l.Unlock()
							l.selected(l.currentItem, item)
							l.Lock()
						}

						l.Unlock()
						return
					}
				}
			}
		}

		previousItem := l.currentItem

		if HitShortcut(event, Keys.MoveFirst, Keys.MoveFirst2) {
			l.transform(TransformFirstItem)
		} else if HitShortcut(event, Keys.MoveLast, Keys.MoveLast2) {
			l.transform(TransformLastItem)
		} else if HitShortcut(event, Keys.MoveUp, Keys.MoveUp2) {
			l.transform(TransformPreviousItem)
		} else if HitShortcut(event, Keys.MoveDown, Keys.MoveDown2) {
			l.transform(TransformNextItem)
		} else if HitShortcut(event, Keys.MoveLeft, Keys.MoveLeft2) {
			l.columnOffset--
			l.updateOffset()
		} else if HitShortcut(event, Keys.MoveRight, Keys.MoveRight2) {
			l.columnOffset++
			l.updateOffset()
		} else if HitShortcut(event, Keys.MovePreviousPage) {
			l.transform(TransformPreviousPage)
		} else if HitShortcut(event, Keys.MoveNextPage) {
			l.transform(TransformNextPage)
		}

		if l.currentItem != previousItem && l.currentItem < len(l.items) && l.changed != nil {
			item := l.items[l.currentItem]
			l.Unlock()
			l.changed(l.currentItem, item)
		} else {
			l.Unlock()
		}
	})
}

// indexAtY returns the index of the list item found at the given Y position
// or a negative value if there is no such list item.
func (l *List) indexAtY(y int) int {
	_, rectY, _, height := l.GetInnerRect()
	if y < rectY || y >= rectY+height {
		return -1
	}

	index := y - rectY
	if l.showSecondaryText {
		index /= 2
	}
	index += l.itemOffset

	if index >= len(l.items) {
		return -1
	}
	return index
}

// indexAtPoint returns the index of the list item found at the given position
// or a negative value if there is no such list item.
func (l *List) indexAtPoint(x, y int) int {
	rectX, rectY, width, height := l.GetInnerRect()
	if x < rectX || x >= rectX+width || y < rectY || y >= rectY+height {
		return -1
	}

	index := y - rectY
	if l.showSecondaryText {
		index /= 2
	}
	index += l.itemOffset

	if index >= len(l.items) {
		return -1
	}
	return index
}

// MouseHandler returns the mouse handler for this primitive.
func (l *List) MouseHandler() func(action MouseAction, event *tcell.EventMouse, setFocus func(p Primitive)) (consumed bool, capture Primitive) {
	return l.WrapMouseHandler(func(action MouseAction, event *tcell.EventMouse, setFocus func(p Primitive)) (consumed bool, capture Primitive) {
		l.Lock()

		// Pass events to context menu.
		if l.ContextMenuVisible() && l.ContextMenuList().InRect(event.Position()) {
			defer l.ContextMenuList().MouseHandler()(action, event, setFocus)
			consumed = true
			l.Unlock()
			return
		}

		if !l.InRect(event.Position()) {
			l.Unlock()
			return false, nil
		}

		// Process mouse event.
		switch action {
		case MouseLeftClick:
			if l.ContextMenuVisible() {
				defer l.ContextMenu.hide(setFocus)
				consumed = true
				l.Unlock()
				return
			}

			l.Unlock()
			setFocus(l)
			l.Lock()

			index := l.indexAtPoint(event.Position())
			if index != -1 {
				item := l.items[index]
				if !item.disabled {
					l.currentItem = index
					if item.selected != nil {
						l.Unlock()
						item.selected()
						l.Lock()
					}
					if l.selected != nil {
						l.Unlock()
						l.selected(index, item)
						l.Lock()
					}
					if index != l.currentItem && l.changed != nil {
						l.Unlock()
						l.changed(index, item)
						l.Lock()
					}
				}
			}
			consumed = true
		case MouseMiddleClick:
			if l.ContextMenu.open {
				defer l.ContextMenu.hide(setFocus)
				consumed = true
				l.Unlock()
				return
			}
		case MouseRightDown:
			if len(l.ContextMenuList().items) == 0 {
				l.Unlock()
				return
			}

			x, y := event.Position()

			index := l.indexAtPoint(event.Position())
			if index != -1 {
				item := l.items[index]
				if !item.disabled {
					l.currentItem = index
					if index != l.currentItem && l.changed != nil {
						l.Unlock()
						l.changed(index, item)
						l.Lock()
					}
				}
			}

			defer l.ContextMenu.show(l.currentItem, x, y, setFocus)
			l.ContextMenu.drag = true
			consumed = true
		case MouseMove:
			if l.hover {
				_, y := event.Position()
				index := l.indexAtY(y)
				if index >= 0 {
					item := l.items[index]
					if !item.disabled {
						l.currentItem = index
					}
				}

				consumed = true
			}
		case MouseScrollUp:
			if l.itemOffset > 0 {
				l.itemOffset--
			}
			consumed = true
		case MouseScrollDown:
			lines := len(l.items) - l.itemOffset
			if l.showSecondaryText {
				lines *= 2
			}
			if _, _, _, height := l.GetInnerRect(); lines > height {
				l.itemOffset++
			}
			consumed = true
		}

		l.Unlock()
		return
	})
}
