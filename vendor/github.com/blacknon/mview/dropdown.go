package mview

import (
	"strings"
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
)

// DropDownOption is one option that can be selected in a drop-down primitive.
type DropDownOption struct {
	text      string                                  // The text to be displayed in the drop-down.
	selected  func(index int, option *DropDownOption) // The (optional) callback for when this option was selected.
	reference interface{}                             // An optional reference object.

	sync.RWMutex
}

// NewDropDownOption returns a new option for a dropdown.
func NewDropDownOption(text string) *DropDownOption {
	return &DropDownOption{text: text}
}

// GetText returns the text of this dropdown option.
func (d *DropDownOption) GetText() string {
	d.RLock()
	defer d.RUnlock()

	return d.text
}

// SetText returns the text of this dropdown option.
func (d *DropDownOption) SetText(text string) {
	d.text = text
}

// SetSelectedFunc sets the handler to be called when this option is selected.
func (d *DropDownOption) SetSelectedFunc(handler func(index int, option *DropDownOption)) {
	d.selected = handler
}

// GetReference returns the reference object of this dropdown option.
func (d *DropDownOption) GetReference() interface{} {
	d.RLock()
	defer d.RUnlock()

	return d.reference
}

// SetReference allows you to store a reference of any type in this option.
func (d *DropDownOption) SetReference(reference interface{}) {
	d.reference = reference
}

// DropDown implements a selection widget whose options become visible in a
// drop-down list when activated.
type DropDown struct {
	*Box

	// The options from which the user can choose.
	options []*DropDownOption

	// Strings to be placed before and after each drop-down option.
	optionPrefix, optionSuffix string

	// The index of the currently selected option. Negative if no option is
	// currently selected.
	currentOption int

	// Strings to be placed before and after the current option.
	currentOptionPrefix, currentOptionSuffix string

	// The text to be displayed when no option has yet been selected.
	noSelection string

	// Set to true if the options are visible and selectable.
	open bool

	// The runes typed so far to directly access one of the list items.
	prefix string

	// The list element for the options.
	list *List

	// The text to be displayed before the input area.
	label string

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

	// The color for prefixes.
	prefixTextColor tcell.Color

	// The screen width of the label area. A value of 0 means use the width of
	// the label text.
	labelWidth int

	// The screen width of the input area. A value of 0 means extend as much as
	// possible.
	fieldWidth int

	// An optional function which is called when the user indicated that they
	// are done selecting options. The key which was pressed is provided (tab,
	// shift-tab, or escape).
	done func(tcell.Key)

	// A callback function set by the Form class and called when the user leaves
	// this form item.
	finished func(tcell.Key)

	// A callback function which is called when the user changes the drop-down's
	// selection.
	selected func(index int, option *DropDownOption)

	// Set to true when mouse dragging is in progress.
	dragging bool

	// The chars to show when the option's text gets shortened.
	abbreviationChars string

	// The symbol to draw at the end of the field when closed.
	dropDownSymbol rune

	// The symbol to draw at the end of the field when opened.
	dropDownOpenSymbol rune

	// The symbol used to draw the selected item when opened.
	dropDownSelectedSymbol rune

	// A flag that determines whether the drop down symbol is always drawn.
	alwaysDrawDropDownSymbol bool

	sync.RWMutex
}

// NewDropDown returns a new drop-down.
func NewDropDown() *DropDown {
	list := NewList()
	list.ShowSecondaryText(false)
	list.SetMainTextColor(Styles.SecondaryTextColor)
	list.SetSelectedTextColor(Styles.PrimitiveBackgroundColor)
	list.SetSelectedBackgroundColor(Styles.PrimaryTextColor)
	list.SetHighlightFullLine(true)
	list.SetBackgroundColor(Styles.ContrastBackgroundColor)

	d := &DropDown{
		Box:                         NewBox(),
		currentOption:               -1,
		list:                        list,
		labelColor:                  Styles.SecondaryTextColor,
		fieldBackgroundColor:        Styles.MoreContrastBackgroundColor,
		fieldTextColor:              Styles.PrimaryTextColor,
		prefixTextColor:             Styles.ContrastSecondaryTextColor,
		dropDownSymbol:              Styles.DropDownSymbol,
		dropDownOpenSymbol:          Styles.DropDownOpenSymbol,
		dropDownSelectedSymbol:      Styles.DropDownSelectedSymbol,
		abbreviationChars:           Styles.DropDownAbbreviationChars,
		labelColorFocused:           ColorUnset,
		fieldBackgroundColorFocused: ColorUnset,
		fieldTextColorFocused:       ColorUnset,
	}

	if sym := d.dropDownSelectedSymbol; sym != 0 {
		list.SetIndicators(" "+string(sym)+" ", "", "   ", "")
	}
	d.focus = d

	return d
}

// SetDropDownSymbolRune sets the rune to be drawn at the end of the dropdown field
// to indicate that this field is a dropdown.
func (d *DropDown) SetDropDownSymbolRune(symbol rune) {
	d.Lock()
	defer d.Unlock()
	d.dropDownSymbol = symbol
}

// SetDropDownOpenSymbolRune sets the rune to be drawn at the end of the
// dropdown field to indicate that the a dropdown is open.
func (d *DropDown) SetDropDownOpenSymbolRune(symbol rune) {
	d.Lock()
	defer d.Unlock()
	d.dropDownOpenSymbol = symbol

	if symbol != 0 {
		d.list.SetIndicators(" "+string(symbol)+" ", "", "   ", "")
	} else {
		d.list.SetIndicators("", "", "", "")
	}

}

// SetDropDownSelectedSymbolRune sets the rune to be drawn at the start of the
// selected list item.
func (d *DropDown) SetDropDownSelectedSymbolRune(symbol rune) {
	d.Lock()
	defer d.Unlock()
	d.dropDownSelectedSymbol = symbol
}

// SetAlwaysDrawDropDownSymbol sets a flad that determines whether the drop
// down symbol is always drawn. The symbol is normally only drawn when focused.
func (d *DropDown) SetAlwaysDrawDropDownSymbol(alwaysDraw bool) {
	d.Lock()
	defer d.Unlock()
	d.alwaysDrawDropDownSymbol = alwaysDraw
}

// SetCurrentOption sets the index of the currently selected option. This may
// be a negative value to indicate that no option is currently selected. Calling
// this function will also trigger the "selected" callback (if there is one).
func (d *DropDown) SetCurrentOption(index int) {
	d.Lock()
	if index >= 0 && index < len(d.options) {
		d.currentOption = index
		d.list.SetCurrentItem(index)
		if d.selected != nil {
			d.Unlock()
			d.selected(index, d.options[index])
			d.Lock()
		}
		if d.options[index].selected != nil {
			d.Unlock()
			d.options[index].selected(index, d.options[index])
			d.Lock()
		}
	} else {
		d.currentOption = -1
		d.list.SetCurrentItem(0) // Set to 0 because -1 means "last item".
		if d.selected != nil {
			d.Unlock()
			d.selected(-1, nil)
			d.Lock()
		}
	}
	d.Unlock()
}

// GetCurrentOption returns the index of the currently selected option as well
// as the option itself. If no option was selected, -1 and nil is returned.
func (d *DropDown) GetCurrentOption() (int, *DropDownOption) {
	d.RLock()
	defer d.RUnlock()

	var option *DropDownOption
	if d.currentOption >= 0 && d.currentOption < len(d.options) {
		option = d.options[d.currentOption]
	}
	return d.currentOption, option
}

// SetTextOptions sets the text to be placed before and after each drop-down
// option (prefix/suffix), the text placed before and after the currently
// selected option (currentPrefix/currentSuffix) as well as the text to be
// displayed when no option is currently selected. Per default, all of these
// strings are empty.
func (d *DropDown) SetTextOptions(prefix, suffix, currentPrefix, currentSuffix, noSelection string) {
	d.Lock()
	defer d.Unlock()

	d.currentOptionPrefix = currentPrefix
	d.currentOptionSuffix = currentSuffix
	d.noSelection = noSelection
	d.optionPrefix = prefix
	d.optionSuffix = suffix
	for index := 0; index < d.list.GetItemCount(); index++ {
		d.list.SetItemText(index, prefix+d.options[index].text+suffix, "")
	}
}

// SetLabel sets the text to be displayed before the input area.
func (d *DropDown) SetLabel(label string) {
	d.Lock()
	defer d.Unlock()

	d.label = label
}

// GetLabel returns the text to be displayed before the input area.
func (d *DropDown) GetLabel() string {
	d.RLock()
	defer d.RUnlock()

	return d.label
}

// SetLabelWidth sets the screen width of the label. A value of 0 will cause the
// primitive to use the width of the label string.
func (d *DropDown) SetLabelWidth(width int) {
	d.Lock()
	defer d.Unlock()

	d.labelWidth = width
}

// SetLabelColor sets the color of the label.
func (d *DropDown) SetLabelColor(color tcell.Color) {
	d.Lock()
	defer d.Unlock()

	d.labelColor = color
}

// SetLabelColorFocused sets the color of the label when focused.
func (d *DropDown) SetLabelColorFocused(color tcell.Color) {
	d.Lock()
	defer d.Unlock()

	d.labelColorFocused = color
}

// SetFieldBackgroundColor sets the background color of the options area.
func (d *DropDown) SetFieldBackgroundColor(color tcell.Color) {
	d.Lock()
	defer d.Unlock()

	d.fieldBackgroundColor = color
}

// SetFieldBackgroundColorFocused sets the background color of the options area when focused.
func (d *DropDown) SetFieldBackgroundColorFocused(color tcell.Color) {
	d.Lock()
	defer d.Unlock()

	d.fieldBackgroundColorFocused = color
}

// SetFieldTextColor sets the text color of the options area.
func (d *DropDown) SetFieldTextColor(color tcell.Color) {
	d.Lock()
	defer d.Unlock()

	d.fieldTextColor = color
}

// SetFieldTextColorFocused sets the text color of the options area when focused.
func (d *DropDown) SetFieldTextColorFocused(color tcell.Color) {
	d.Lock()
	defer d.Unlock()

	d.fieldTextColorFocused = color
}

// SetDropDownTextColor sets text color of the drop-down list.
func (d *DropDown) SetDropDownTextColor(color tcell.Color) {
	d.Lock()
	defer d.Unlock()

	d.list.SetMainTextColor(color)
}

// SetDropDownBackgroundColor sets the background color of the drop-down list.
func (d *DropDown) SetDropDownBackgroundColor(color tcell.Color) {
	d.Lock()
	defer d.Unlock()

	d.list.SetBackgroundColor(color)
}

// SetDropDownSelectedTextColor sets the text color of the selected option in
// the drop-down list.
func (d *DropDown) SetDropDownSelectedTextColor(color tcell.Color) {
	d.Lock()
	defer d.Unlock()

	d.list.SetSelectedTextColor(color)
}

// SetDropDownSelectedBackgroundColor sets the background color of the selected
// option in the drop-down list.
func (d *DropDown) SetDropDownSelectedBackgroundColor(color tcell.Color) {
	d.Lock()
	defer d.Unlock()

	d.list.SetSelectedBackgroundColor(color)
}

// SetPrefixTextColor sets the color of the prefix string. The prefix string is
// shown when the user starts typing text, which directly selects the first
// option that starts with the typed string.
func (d *DropDown) SetPrefixTextColor(color tcell.Color) {
	d.Lock()
	defer d.Unlock()

	d.prefixTextColor = color
}

// SetFieldWidth sets the screen width of the options area. A value of 0 means
// extend to as long as the longest option text.
func (d *DropDown) SetFieldWidth(width int) {
	d.Lock()
	defer d.Unlock()

	d.fieldWidth = width
}

// GetFieldHeight returns the height of the field.
func (d *DropDown) GetFieldHeight() int {
	return 1
}

// GetFieldWidth returns this primitive's field screen width.
func (d *DropDown) GetFieldWidth() int {
	d.RLock()
	defer d.RUnlock()
	return d.getFieldWidth()
}

func (d *DropDown) getFieldWidth() int {
	if d.fieldWidth > 0 {
		return d.fieldWidth
	}
	fieldWidth := 0
	for _, option := range d.options {
		width := TaggedStringWidth(option.text)
		if width > fieldWidth {
			fieldWidth = width
		}
	}
	fieldWidth += len(d.optionPrefix) + len(d.optionSuffix)
	fieldWidth += len(d.currentOptionPrefix) + len(d.currentOptionSuffix)
	// space <text> space + dropDownSymbol + space
	fieldWidth += 4
	return fieldWidth
}

// AddOptionsSimple adds new selectable options to this drop-down.
func (d *DropDown) AddOptionsSimple(options ...string) {
	optionsToAdd := make([]*DropDownOption, len(options))
	for i, option := range options {
		optionsToAdd[i] = NewDropDownOption(option)
	}
	d.AddOptions(optionsToAdd...)
}

// AddOptions adds new selectable options to this drop-down.
func (d *DropDown) AddOptions(options ...*DropDownOption) {
	d.Lock()
	defer d.Unlock()
	d.addOptions(options...)
}

func (d *DropDown) addOptions(options ...*DropDownOption) {
	d.options = append(d.options, options...)
	for _, option := range options {
		d.list.AddItem(NewListItem(d.optionPrefix + option.text + d.optionSuffix))
	}
}

// SetOptionsSimple replaces all current options with the ones provided and installs
// one callback function which is called when one of the options is selected.
// It will be called with the option's index and the option itself
// The "selected" parameter may be nil.
func (d *DropDown) SetOptionsSimple(selected func(index int, option *DropDownOption), options ...string) {
	optionsToSet := make([]*DropDownOption, len(options))
	for i, option := range options {
		optionsToSet[i] = NewDropDownOption(option)
	}
	d.SetOptions(selected, optionsToSet...)
}

// SetOptions replaces all current options with the ones provided and installs
// one callback function which is called when one of the options is selected.
// It will be called with the option's index and the option itself.
// The "selected" parameter may be nil.
func (d *DropDown) SetOptions(selected func(index int, option *DropDownOption), options ...*DropDownOption) {
	d.Lock()
	defer d.Unlock()

	d.list.Clear()
	d.options = nil
	d.addOptions(options...)
	d.selected = selected
}

// SetChangedFunc sets a handler which is called when the user changes the
// focused drop-down option. The handler is provided with the selected option's
// index and the option itself. If "no option" was selected, these values are
// -1 and nil.
func (d *DropDown) SetChangedFunc(handler func(index int, option *DropDownOption)) {
	d.list.SetChangedFunc(func(index int, item *ListItem) {
		handler(index, d.options[index])
	})
}

// SetSelectedFunc sets a handler which is called when the user selects a
// drop-down's option. This handler will be called in addition and prior to
// an option's optional individual handler. The handler is provided with the
// selected option's index and the option itself. If "no option" was selected, these values
// are -1 and nil.
func (d *DropDown) SetSelectedFunc(handler func(index int, option *DropDownOption)) {
	d.Lock()
	defer d.Unlock()

	d.selected = handler
}

// SetDoneFunc sets a handler which is called when the user is done selecting
// options. The callback function is provided with the key that was pressed,
// which is one of the following:
//
//   - KeyEscape: Abort selection.
//   - KeyTab: Move to the next field.
//   - KeyBacktab: Move to the previous field.
func (d *DropDown) SetDoneFunc(handler func(key tcell.Key)) {
	d.Lock()
	defer d.Unlock()

	d.done = handler
}

// SetFinishedFunc sets a callback invoked when the user leaves this form item.
func (d *DropDown) SetFinishedFunc(handler func(key tcell.Key)) {
	d.Lock()
	defer d.Unlock()

	d.finished = handler
}

// Draw draws this primitive onto the screen.
func (d *DropDown) Draw(screen tcell.Screen) {
	d.Box.Draw(screen)
	hasFocus := d.GetFocusable().HasFocus()

	d.Lock()
	defer d.Unlock()

	// Select colors
	labelColor := d.labelColor
	fieldBackgroundColor := d.fieldBackgroundColor
	fieldTextColor := d.fieldTextColor
	if hasFocus {
		if d.labelColorFocused != ColorUnset {
			labelColor = d.labelColorFocused
		}
		if d.fieldBackgroundColorFocused != ColorUnset {
			fieldBackgroundColor = d.fieldBackgroundColorFocused
		}
		if d.fieldTextColorFocused != ColorUnset {
			fieldTextColor = d.fieldTextColorFocused
		}
	}

	// Prepare.
	x, y, width, height := d.GetInnerRect()
	rightLimit := x + width
	if height < 1 || rightLimit <= x {
		return
	}

	// Draw label.
	if d.labelWidth > 0 {
		labelWidth := d.labelWidth
		if labelWidth > rightLimit-x {
			labelWidth = rightLimit - x
		}
		Print(screen, []byte(d.label), x, y, labelWidth, AlignLeft, labelColor)
		x += labelWidth
	} else {
		_, drawnWidth := Print(screen, []byte(d.label), x, y, rightLimit-x, AlignLeft, labelColor)
		x += drawnWidth
	}

	// What's the longest option text?
	maxWidth := 0
	optionWrapWidth := TaggedStringWidth(d.optionPrefix + d.optionSuffix)
	for _, option := range d.options {
		strWidth := TaggedStringWidth(option.text) + optionWrapWidth
		if strWidth > maxWidth {
			maxWidth = strWidth
		}
	}

	// Draw selection area.
	fieldWidth := d.getFieldWidth()
	if fieldWidth == 0 {
		fieldWidth = maxWidth
		if d.currentOption < 0 {
			noSelectionWidth := TaggedStringWidth(d.noSelection)
			if noSelectionWidth > fieldWidth {
				fieldWidth = noSelectionWidth
			}
		} else if d.currentOption < len(d.options) {
			currentOptionWidth := TaggedStringWidth(d.currentOptionPrefix + d.options[d.currentOption].text + d.currentOptionSuffix)
			if currentOptionWidth > fieldWidth {
				fieldWidth = currentOptionWidth
			}
		}
	}
	if rightLimit-x < fieldWidth {
		fieldWidth = rightLimit - x
	}
	fieldStyle := tcell.StyleDefault.Background(fieldBackgroundColor)
	for index := 0; index < fieldWidth; index++ {
		screen.SetContent(x+index, y, ' ', nil, fieldStyle)
	}

	// Draw selected text.
	if d.open && len(d.prefix) > 0 {
		// Show the prefix.
		currentOptionPrefixWidth := TaggedStringWidth(d.currentOptionPrefix)
		prefixWidth := runewidth.StringWidth(d.prefix)
		listItemText := d.options[d.list.GetCurrentItemIndex()].text
		Print(screen, []byte(d.currentOptionPrefix), x, y, fieldWidth, AlignLeft, fieldTextColor)
		Print(screen, []byte(d.prefix), x+currentOptionPrefixWidth, y, fieldWidth-currentOptionPrefixWidth, AlignLeft, d.prefixTextColor)
		if len(d.prefix) < len(listItemText) {
			Print(screen, []byte(listItemText[len(d.prefix):]+d.currentOptionSuffix), x+prefixWidth+currentOptionPrefixWidth, y, fieldWidth-prefixWidth-currentOptionPrefixWidth, AlignLeft, fieldTextColor)
		}
	} else {
		color := fieldTextColor
		text := d.noSelection
		if d.currentOption >= 0 && d.currentOption < len(d.options) {
			text = d.currentOptionPrefix + d.options[d.currentOption].text + d.currentOptionSuffix
		}
		// Abbreviate text when not fitting
		if fieldWidth > len(d.abbreviationChars)+3 && len(text) > fieldWidth {
			text = text[0:fieldWidth-3-len(d.abbreviationChars)] + d.abbreviationChars
		}

		// Just show the current selection.
		Print(screen, []byte(text), x, y, fieldWidth, AlignLeft, color)
	}

	// Draw drop-down symbol
	if d.alwaysDrawDropDownSymbol || d._hasFocus() {
		symbol := d.dropDownSymbol
		if d.open {
			symbol = d.dropDownOpenSymbol
		}
		screen.SetContent(x+fieldWidth-2, y, symbol, nil, new(tcell.Style).Foreground(fieldTextColor).Background(fieldBackgroundColor))
	}

	// Draw options list.
	if hasFocus && d.open {
		// We prefer to drop-down but if there is no space, maybe drop up?
		lx := x
		ly := y + 1
		lheight := len(d.options)
		_, sheight := screen.Size()
		if ly+lheight >= sheight && ly-2 > lheight-ly {
			ly = y - lheight
			if ly < 0 {
				ly = 0
			}
		}
		if ly+lheight >= sheight {
			lheight = sheight - ly
		}
		lwidth := maxWidth
		if d.list.scrollBarVisibility == ScrollBarAlways || (d.list.scrollBarVisibility == ScrollBarAuto && len(d.options) > lheight) {
			lwidth++ // Add space for scroll bar
		}
		if lwidth < fieldWidth {
			lwidth = fieldWidth
		}
		d.list.SetRect(lx, ly, lwidth, lheight)
		d.list.Draw(screen)
	}
}

// InputHandler returns the handler for this primitive.
func (d *DropDown) InputHandler() func(event *tcell.EventKey, setFocus func(p Primitive)) {
	return d.WrapInputHandler(func(event *tcell.EventKey, setFocus func(p Primitive)) {
		// Process key event.
		switch key := event.Key(); key {
		case tcell.KeyEnter, tcell.KeyRune, tcell.KeyDown:
			d.Lock()
			defer d.Unlock()

			d.prefix = ""

			// If the first key was a letter already, it becomes part of the prefix.
			if r := event.Rune(); key == tcell.KeyRune && r != ' ' {
				d.prefix += string(r)
				d.evalPrefix()
			}

			d.openList(setFocus)
		case tcell.KeyEscape, tcell.KeyTab, tcell.KeyBacktab:
			if d.done != nil {
				d.done(key)
			}
			if d.finished != nil {
				d.finished(key)
			}
		}
	})
}

// evalPrefix selects an item in the drop-down list based on the current prefix.
func (d *DropDown) evalPrefix() {
	if len(d.prefix) > 0 {
		for index, option := range d.options {
			if strings.HasPrefix(strings.ToLower(option.text), d.prefix) {
				d.list.SetCurrentItem(index)
				return
			}
		}

		// Prefix does not match any item. Remove last rune.
		r := []rune(d.prefix)
		d.prefix = string(r[:len(r)-1])
	}
}

// openList hands control over to the embedded List primitive.
func (d *DropDown) openList(setFocus func(Primitive)) {
	d.open = true
	optionBefore := d.currentOption

	d.list.SetSelectedFunc(func(index int, item *ListItem) {
		if d.dragging {
			return // If we're dragging the mouse, we don't want to trigger any events.
		}

		// An option was selected. Close the list again.
		d.currentOption = index
		d.closeList(setFocus)

		// Trigger "selected" event.
		if d.selected != nil {
			d.selected(d.currentOption, d.options[d.currentOption])
		}
		if d.options[d.currentOption].selected != nil {
			d.options[d.currentOption].selected(d.currentOption, d.options[d.currentOption])
		}
	})
	d.list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyRune {
			d.prefix += string(event.Rune())
			d.evalPrefix()
		} else if event.Key() == tcell.KeyBackspace || event.Key() == tcell.KeyBackspace2 {
			if len(d.prefix) > 0 {
				r := []rune(d.prefix)
				d.prefix = string(r[:len(r)-1])
			}
			d.evalPrefix()
		} else if event.Key() == tcell.KeyEscape {
			d.currentOption = optionBefore
			d.list.SetCurrentItem(d.currentOption)
			d.closeList(setFocus)
			if d.selected != nil {
				if d.currentOption > -1 {
					d.selected(d.currentOption, d.options[d.currentOption])
				}
			}
		} else {
			d.prefix = ""
		}

		return event
	})

	setFocus(d.list)
}

// closeList closes the embedded List element by hiding it and removing focus
// from it.
func (d *DropDown) closeList(setFocus func(Primitive)) {
	d.open = false
	if d.list.HasFocus() {
		setFocus(d)
	}
}

// Focus is called by the application when the primitive receives focus.
func (d *DropDown) Focus(delegate func(p Primitive)) {
	d.Box.Focus(delegate)
	if d.open {
		delegate(d.list)
	}
}

// HasFocus returns whether or not this primitive has focus.
func (d *DropDown) HasFocus() bool {
	d.RLock()
	defer d.RUnlock()

	return d._hasFocus()
}

func (d *DropDown) _hasFocus() bool {
	if d.open {
		return d.list.HasFocus()
	}
	return d.hasFocus
}

// MouseHandler returns the mouse handler for this primitive.
func (d *DropDown) MouseHandler() func(action MouseAction, event *tcell.EventMouse, setFocus func(p Primitive)) (consumed bool, capture Primitive) {
	return d.WrapMouseHandler(func(action MouseAction, event *tcell.EventMouse, setFocus func(p Primitive)) (consumed bool, capture Primitive) {
		// Was the mouse event in the drop-down box itself (or on its label)?
		x, y := event.Position()
		_, rectY, _, _ := d.GetInnerRect()
		inRect := y == rectY
		if !d.open && !inRect {
			return d.InRect(x, y), nil // No, and it's not expanded either. Ignore.
		}

		// Handle dragging. Clicks are implicitly handled by this logic.
		switch action {
		case MouseLeftDown:
			consumed = d.open || inRect
			capture = d
			if !d.open {
				d.openList(setFocus)
				d.dragging = true
			} else if consumed, _ := d.list.MouseHandler()(MouseLeftClick, event, setFocus); !consumed {
				d.closeList(setFocus) // Close drop-down if clicked outside of it.
			}
		case MouseMove:
			if d.dragging {
				// We pretend it's a left click so we can see the selection during
				// dragging. Because we don't act upon it, it's not a problem.
				d.list.MouseHandler()(MouseLeftClick, event, setFocus)
				consumed = true
				capture = d
			}
		case MouseLeftUp:
			if d.dragging {
				d.dragging = false
				d.list.MouseHandler()(MouseLeftClick, event, setFocus)
				consumed = true
			}
		}

		return
	})
}
