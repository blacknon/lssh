package mview

import (
	"reflect"
	"sync"

	"github.com/gdamore/tcell/v2"
)

// DefaultFormFieldWidth is the default field screen width of form elements
// whose field width is flexible (0). This is used in the Form class for
// horizontal layouts.
var DefaultFormFieldWidth = 10

// FormItemAttributes is a set of attributes to be applied.
type FormItemAttributes struct {
	// The screen width of the label. A value of 0 will cause the primitive to
	// use the width of the label string.
	LabelWidth int

	BackgroundColor             tcell.Color
	LabelColor                  tcell.Color
	LabelColorFocused           tcell.Color
	FieldBackgroundColor        tcell.Color
	FieldBackgroundColorFocused tcell.Color
	FieldTextColor              tcell.Color
	FieldTextColorFocused       tcell.Color

	FinishedFunc func(key tcell.Key)
}

// FormItem is the interface all form items must implement to be able to be
// included in a form.
type FormItem interface {
	Primitive

	// GetLabel returns the item's label text.
	GetLabel() string

	// SetLabelWidth sets the screen width of the label. A value of 0 will cause the
	// primitive to use the width of the label string.
	SetLabelWidth(int)

	// SetLabelColor sets the color of the label.
	SetLabelColor(tcell.Color)

	// SetLabelColor sets the color of the label when focused.
	SetLabelColorFocused(tcell.Color)

	// GetFieldWidth returns the width of the form item's field (the area which
	// is manipulated by the user) in number of screen cells. A value of 0
	// indicates the the field width is flexible and may use as much space as
	// required.
	GetFieldWidth() int

	// GetFieldHeight returns the height of the form item.
	GetFieldHeight() int

	// SetFieldTextColor sets the text color of the input area.
	SetFieldTextColor(tcell.Color)

	// SetFieldTextColorFocused sets the text color of the input area when focused.
	SetFieldTextColorFocused(tcell.Color)

	// SetFieldBackgroundColor sets the background color of the input area.
	SetFieldBackgroundColor(tcell.Color)

	// SetFieldBackgroundColor sets the background color of the input area when focused.
	SetFieldBackgroundColorFocused(tcell.Color)

	// SetBackgroundColor sets the background color of the form item.
	SetBackgroundColor(tcell.Color)

	// SetFinishedFunc sets a callback invoked when the user leaves the form item.
	SetFinishedFunc(func(key tcell.Key))
}

// Form allows you to combine multiple one-line form elements into a vertical
// or horizontal layout. Form elements include types such as InputField or
// CheckBox. These elements can be optionally followed by one or more buttons
// for which you can define form-wide actions (e.g. Save, Clear, Cancel).
type Form struct {
	*Box

	// The items of the form (one row per item).
	items []FormItem

	// The buttons of the form.
	buttons []*Button

	// If set to true, instead of position items and buttons from top to bottom,
	// they are positioned from left to right.
	horizontal bool

	// The alignment of the buttons.
	buttonsAlign int

	// The number of empty rows between items.
	itemPadding int

	// The index of the item or button which has focus. (Items are counted first,
	// buttons are counted last.) This is only used when the form itself receives
	// focus so that the last element that had focus keeps it.
	focusedElement int

	// Whether or not navigating the form will wrap around.
	wrapAround bool

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

	// The background color of the buttons.
	buttonBackgroundColor tcell.Color

	// The background color of the buttons when focused.
	buttonBackgroundColorFocused tcell.Color

	// The color of the button text.
	buttonTextColor tcell.Color

	// The color of the button text when focused.
	buttonTextColorFocused tcell.Color

	// An optional function which is called when the user hits Escape.
	cancel func()

	sync.RWMutex
}

// NewForm returns a new form.
func NewForm() *Form {
	box := NewBox()
	box.SetPadding(1, 1, 1, 1)

	f := &Form{
		Box:                          box,
		itemPadding:                  1,
		labelColor:                   Styles.SecondaryTextColor,
		fieldBackgroundColor:         Styles.MoreContrastBackgroundColor,
		fieldBackgroundColorFocused:  Styles.ContrastBackgroundColor,
		fieldTextColor:               Styles.PrimaryTextColor,
		fieldTextColorFocused:        Styles.PrimaryTextColor,
		buttonBackgroundColor:        Styles.MoreContrastBackgroundColor,
		buttonBackgroundColorFocused: Styles.ContrastBackgroundColor,
		buttonTextColor:              Styles.PrimaryTextColor,
		buttonTextColorFocused:       Styles.PrimaryTextColor,
		labelColorFocused:            ColorUnset,
	}

	f.focus = f
	return f
}

// SetItemPadding sets the number of empty rows between form items for vertical
// layouts and the number of empty cells between form items for horizontal
// layouts.
func (f *Form) SetItemPadding(padding int) {
	f.Lock()
	defer f.Unlock()

	f.itemPadding = padding
}

// SetHorizontal sets the direction the form elements are laid out. If set to
// true, instead of positioning them from top to bottom (the default), they are
// positioned from left to right, moving into the next row if there is not
// enough space.
func (f *Form) SetHorizontal(horizontal bool) {
	f.Lock()
	defer f.Unlock()

	f.horizontal = horizontal
}

// SetLabelColor sets the color of the labels.
func (f *Form) SetLabelColor(color tcell.Color) {
	f.Lock()
	defer f.Unlock()

	f.labelColor = color
}

// SetLabelColorFocused sets the color of the labels when focused.
func (f *Form) SetLabelColorFocused(color tcell.Color) {
	f.Lock()
	defer f.Unlock()

	f.labelColorFocused = color
}

// SetFieldBackgroundColor sets the background color of the input areas.
func (f *Form) SetFieldBackgroundColor(color tcell.Color) {
	f.Lock()
	defer f.Unlock()

	f.fieldBackgroundColor = color
}

// SetFieldBackgroundColorFocused sets the background color of the input areas when focused.
func (f *Form) SetFieldBackgroundColorFocused(color tcell.Color) {
	f.Lock()
	defer f.Unlock()

	f.fieldBackgroundColorFocused = color
}

// SetFieldTextColor sets the text color of the input areas.
func (f *Form) SetFieldTextColor(color tcell.Color) {
	f.Lock()
	defer f.Unlock()

	f.fieldTextColor = color
}

// SetFieldTextColorFocused sets the text color of the input areas when focused.
func (f *Form) SetFieldTextColorFocused(color tcell.Color) {
	f.Lock()
	defer f.Unlock()

	f.fieldTextColorFocused = color
}

// SetButtonsAlign sets how the buttons align horizontally, one of AlignLeft
// (the default), AlignCenter, and AlignRight. This is only
func (f *Form) SetButtonsAlign(align int) {
	f.Lock()
	defer f.Unlock()

	f.buttonsAlign = align
}

// SetButtonBackgroundColor sets the background color of the buttons.
func (f *Form) SetButtonBackgroundColor(color tcell.Color) {
	f.Lock()
	defer f.Unlock()

	f.buttonBackgroundColor = color
}

// SetButtonBackgroundColorFocused sets the background color of the buttons when focused.
func (f *Form) SetButtonBackgroundColorFocused(color tcell.Color) {
	f.Lock()
	defer f.Unlock()

	f.buttonBackgroundColorFocused = color
}

// SetButtonTextColor sets the color of the button texts.
func (f *Form) SetButtonTextColor(color tcell.Color) {
	f.Lock()
	defer f.Unlock()

	f.buttonTextColor = color
}

// SetButtonTextColorFocused sets the color of the button texts when focused.
func (f *Form) SetButtonTextColorFocused(color tcell.Color) {
	f.Lock()
	defer f.Unlock()

	f.buttonTextColorFocused = color
}

// SetFocus shifts the focus to the form element with the given index, counting
// non-button items first and buttons last. Note that this index is only used
// when the form itself receives focus.
func (f *Form) SetFocus(index int) {
	f.Lock()
	defer f.Unlock()

	if index < 0 {
		f.focusedElement = 0
	} else if index >= len(f.items)+len(f.buttons) {
		f.focusedElement = len(f.items) + len(f.buttons)
	} else {
		f.focusedElement = index
	}
}

// AddInputField adds an input field to the form. It has a label, an optional
// initial value, a field width (a value of 0 extends it as far as possible),
// an optional accept function to validate the item's value (set to nil to
// accept any text), and an (optional) callback function which is invoked when
// the input field's text has changed.
func (f *Form) AddInputField(label, value string, fieldWidth int, accept func(textToCheck string, lastChar rune) bool, changed func(text string)) {
	f.Lock()
	defer f.Unlock()

	inputField := NewInputField()
	inputField.SetLabel(label)
	inputField.SetText(value)
	inputField.SetFieldWidth(fieldWidth)
	inputField.SetAcceptanceFunc(accept)
	inputField.SetChangedFunc(changed)

	f.items = append(f.items, inputField)
}

// AddPasswordField adds a password field to the form. This is similar to an
// input field except that the user's input not shown. Instead, a "mask"
// character is displayed. The password field has a label, an optional initial
// value, a field width (a value of 0 extends it as far as possible), and an
// (optional) callback function which is invoked when the input field's text has
// changed.
func (f *Form) AddPasswordField(label, value string, fieldWidth int, mask rune, changed func(text string)) {
	f.Lock()
	defer f.Unlock()

	if mask == 0 {
		mask = '*'
	}

	passwordField := NewInputField()
	passwordField.SetLabel(label)
	passwordField.SetText(value)
	passwordField.SetFieldWidth(fieldWidth)
	passwordField.SetMaskCharacter(mask)
	passwordField.SetChangedFunc(changed)

	f.items = append(f.items, passwordField)
}

// AddDropDownSimple adds a drop-down element to the form. It has a label, options,
// and an (optional) callback function which is invoked when an option was
// selected. The initial option may be a negative value to indicate that no
// option is currently selected.
func (f *Form) AddDropDownSimple(label string, initialOption int, selected func(index int, option *DropDownOption), options ...string) {
	f.Lock()
	defer f.Unlock()

	dd := NewDropDown()
	dd.SetLabel(label)
	dd.SetOptionsSimple(selected, options...)
	dd.SetCurrentOption(initialOption)

	f.items = append(f.items, dd)
}

// AddDropDown adds a drop-down element to the form. It has a label, options,
// and an (optional) callback function which is invoked when an option was
// selected. The initial option may be a negative value to indicate that no
// option is currently selected.
func (f *Form) AddDropDown(label string, initialOption int, selected func(index int, option *DropDownOption), options []*DropDownOption) {
	f.Lock()
	defer f.Unlock()

	dd := NewDropDown()
	dd.SetLabel(label)
	dd.SetOptions(selected, options...)
	dd.SetCurrentOption(initialOption)

	f.items = append(f.items, dd)
}

// AddCheckBox adds a checkbox to the form. It has a label, a message, an
// initial state, and an (optional) callback function which is invoked when the
// state of the checkbox was changed by the user.
func (f *Form) AddCheckBox(label string, message string, checked bool, changed func(checked bool)) {
	f.Lock()
	defer f.Unlock()

	c := NewCheckBox()
	c.SetLabel(label)
	c.SetMessage(message)
	c.SetChecked(checked)
	c.SetChangedFunc(changed)

	f.items = append(f.items, c)
}

// AddSlider adds a slider to the form. It has a label, an initial value, a
// maximum value, an amount to increment by when modified via keyboard, and an
// (optional) callback function which is invoked when the state of the slider
// was changed by the user.
func (f *Form) AddSlider(label string, current, max, increment int, changed func(value int)) {
	f.Lock()
	defer f.Unlock()

	s := NewSlider()
	s.SetLabel(label)
	s.SetMax(max)
	s.SetProgress(current)
	s.SetIncrement(increment)
	s.SetChangedFunc(changed)

	f.items = append(f.items, s)
}

// AddButton adds a new button to the form. The "selected" function is called
// when the user selects this button. It may be nil.
func (f *Form) AddButton(label string, selected func()) {
	f.Lock()
	defer f.Unlock()

	button := NewButton(label)
	button.SetSelectedFunc(selected)
	f.buttons = append(f.buttons, button)
}

// GetButton returns the button at the specified 0-based index. Note that
// buttons have been specially prepared for this form and modifying some of
// their attributes may have unintended side effects.
func (f *Form) GetButton(index int) *Button {
	f.RLock()
	defer f.RUnlock()

	return f.buttons[index]
}

// RemoveButton removes the button at the specified position, starting with 0
// for the button that was added first.
func (f *Form) RemoveButton(index int) {
	f.Lock()
	defer f.Unlock()

	f.buttons = append(f.buttons[:index], f.buttons[index+1:]...)
}

// GetButtonCount returns the number of buttons in this form.
func (f *Form) GetButtonCount() int {
	f.RLock()
	defer f.RUnlock()

	return len(f.buttons)
}

// GetButtonIndex returns the index of the button with the given label, starting
// with 0 for the button that was added first. If no such label was found, -1
// is returned.
func (f *Form) GetButtonIndex(label string) int {
	f.RLock()
	defer f.RUnlock()

	for index, button := range f.buttons {
		if button.GetLabel() == label {
			return index
		}
	}
	return -1
}

// Clear removes all input elements from the form, including the buttons if
// specified.
func (f *Form) Clear(includeButtons bool) {
	f.Lock()
	defer f.Unlock()

	f.items = nil
	if includeButtons {
		f.buttons = nil
	}
	f.focusedElement = 0
}

// ClearButtons removes all buttons from the form.
func (f *Form) ClearButtons() {
	f.Lock()
	defer f.Unlock()

	f.buttons = nil
}

// AddFormItem adds a new item to the form. This can be used to add your own
// objects to the form. Note, however, that the Form class will override some
// of its attributes to make it work in the form context. Specifically, these
// are:
//
//   - The label width
//   - The label color
//   - The background color
//   - The field text color
//   - The field background color
func (f *Form) AddFormItem(item FormItem) {
	f.Lock()
	defer f.Unlock()

	if reflect.ValueOf(item).IsNil() {
		panic("Invalid FormItem")
	}

	f.items = append(f.items, item)
}

// GetFormItemCount returns the number of items in the form (not including the
// buttons).
func (f *Form) GetFormItemCount() int {
	f.RLock()
	defer f.RUnlock()

	return len(f.items)
}

// IndexOfFormItem returns the index of the given FormItem.
func (f *Form) IndexOfFormItem(item FormItem) int {
	f.l.RLock()
	defer f.l.RUnlock()
	for index, formItem := range f.items {
		if item == formItem {
			return index
		}
	}
	return -1
}

// GetFormItem returns the form item at the given position, starting with index
// 0. Elements are referenced in the order they were added. Buttons are not included.
// If index is out of bounds it returns nil.
func (f *Form) GetFormItem(index int) FormItem {
	f.RLock()
	defer f.RUnlock()
	if index > len(f.items)-1 || index < 0 {
		return nil
	}
	return f.items[index]
}

// RemoveFormItem removes the form element at the given position, starting with
// index 0. Elements are referenced in the order they were added. Buttons are
// not included.
func (f *Form) RemoveFormItem(index int) {
	f.Lock()
	defer f.Unlock()

	f.items = append(f.items[:index], f.items[index+1:]...)
}

// GetFormItemByLabel returns the first form element with the given label. If
// no such element is found, nil is returned. Buttons are not searched and will
// therefore not be returned.
func (f *Form) GetFormItemByLabel(label string) FormItem {
	f.RLock()
	defer f.RUnlock()

	for _, item := range f.items {
		if item.GetLabel() == label {
			return item
		}
	}
	return nil
}

// GetFormItemIndex returns the index of the first form element with the given
// label. If no such element is found, -1 is returned. Buttons are not searched
// and will therefore not be returned.
func (f *Form) GetFormItemIndex(label string) int {
	f.RLock()
	defer f.RUnlock()

	for index, item := range f.items {
		if item.GetLabel() == label {
			return index
		}
	}
	return -1
}

// GetFocusedItemIndex returns the indices of the form element or button which
// currently has focus. If they don't, -1 is returned resepectively.
func (f *Form) GetFocusedItemIndex() (formItem, button int) {
	f.RLock()
	defer f.RUnlock()

	index := f.focusIndex()
	if index < 0 {
		return -1, -1
	}
	if index < len(f.items) {
		return index, -1
	}
	return -1, index - len(f.items)
}

// SetWrapAround sets the flag that determines whether navigating the form will
// wrap around. That is, navigating downwards on the last item will move the
// selection to the first item (similarly in the other direction). If set to
// false, the selection won't change when navigating downwards on the last item
// or navigating upwards on the first item.
func (f *Form) SetWrapAround(wrapAround bool) {
	f.Lock()
	defer f.Unlock()

	f.wrapAround = wrapAround
}

// SetCancelFunc sets a handler which is called when the user hits the Escape
// key.
func (f *Form) SetCancelFunc(callback func()) {
	f.Lock()
	defer f.Unlock()

	f.cancel = callback
}

// GetAttributes returns the current attribute settings of a form.
func (f *Form) GetAttributes() *FormItemAttributes {
	f.Lock()
	defer f.Unlock()

	return f.getAttributes()
}

func (f *Form) getAttributes() *FormItemAttributes {
	attrs := &FormItemAttributes{
		BackgroundColor:      f.backgroundColor,
		LabelColor:           f.labelColor,
		FieldBackgroundColor: f.fieldBackgroundColor,
		FieldTextColor:       f.fieldTextColor,
	}
	if f.labelColorFocused == ColorUnset {
		attrs.LabelColorFocused = f.labelColor
	} else {
		attrs.LabelColorFocused = f.labelColorFocused
	}
	if f.fieldBackgroundColorFocused == ColorUnset {
		attrs.FieldBackgroundColorFocused = f.fieldTextColor
	} else {
		attrs.FieldBackgroundColorFocused = f.fieldBackgroundColorFocused
	}
	if f.fieldTextColorFocused == ColorUnset {
		attrs.FieldTextColorFocused = f.fieldBackgroundColor
	} else {
		attrs.FieldTextColorFocused = f.fieldTextColorFocused
	}
	return attrs
}

// Draw draws this primitive onto the screen.
func (f *Form) Draw(screen tcell.Screen) {
	if !f.GetVisible() {
		return
	}

	f.Box.Draw(screen)

	f.Lock()
	defer f.Unlock()

	// Determine the actual item that has focus.
	if index := f.focusIndex(); index >= 0 {
		f.focusedElement = index
	}

	// Determine the dimensions.
	x, y, width, height := f.GetInnerRect()
	topLimit := y
	bottomLimit := y + height
	rightLimit := x + width
	startX := x

	// Find the longest label.
	var maxLabelWidth int
	for _, item := range f.items {
		labelWidth := TaggedStringWidth(item.GetLabel())
		if labelWidth > maxLabelWidth {
			maxLabelWidth = labelWidth
		}
	}
	maxLabelWidth++ // Add one space.

	// Calculate positions of form items.
	positions := make([]struct{ x, y, width, height int }, len(f.items)+len(f.buttons))
	var focusedPosition struct{ x, y, width, height int }
	for index, item := range f.items {
		if !item.GetVisible() {
			continue
		}

		// Calculate the space needed.
		labelWidth := TaggedStringWidth(item.GetLabel())
		var itemWidth int
		if f.horizontal {
			fieldWidth := item.GetFieldWidth()
			if fieldWidth == 0 {
				fieldWidth = DefaultFormFieldWidth
			}
			labelWidth++
			itemWidth = labelWidth + fieldWidth
		} else {
			// We want all fields to align vertically.
			labelWidth = maxLabelWidth
			itemWidth = width
		}

		// Advance to next line if there is no space.
		if f.horizontal && x+labelWidth+1 >= rightLimit {
			x = startX
			y += 2
		}

		// Adjust the item's attributes.
		if x+itemWidth >= rightLimit {
			itemWidth = rightLimit - x
		}

		attributes := f.getAttributes()
		attributes.LabelWidth = labelWidth
		setFormItemAttributes(item, attributes)

		// Save position.
		positions[index].x = x
		positions[index].y = y
		positions[index].width = itemWidth
		positions[index].height = 1
		if item.GetFocusable().HasFocus() {
			focusedPosition = positions[index]
		}

		// Advance to next item.
		if f.horizontal {
			x += itemWidth + f.itemPadding
		} else {
			y += item.GetFieldHeight() + f.itemPadding
		}
	}

	// How wide are the buttons?
	buttonWidths := make([]int, len(f.buttons))
	buttonsWidth := 0
	for index, button := range f.buttons {
		w := TaggedStringWidth(button.GetLabel()) + 4
		buttonWidths[index] = w
		buttonsWidth += w + 1
	}
	buttonsWidth--

	// Where do we place them?
	if !f.horizontal && x+buttonsWidth < rightLimit {
		if f.buttonsAlign == AlignRight {
			x = rightLimit - buttonsWidth
		} else if f.buttonsAlign == AlignCenter {
			x = (x + rightLimit - buttonsWidth) / 2
		}

		// In vertical layouts, buttons always appear after an empty line.
		if f.itemPadding == 0 {
			y++
		}
	}

	// Calculate positions of buttons.
	for index, button := range f.buttons {
		if !button.GetVisible() {
			continue
		}

		space := rightLimit - x
		buttonWidth := buttonWidths[index]
		if f.horizontal {
			if space < buttonWidth-4 {
				x = startX
				y += 2
				space = width
			}
		} else {
			if space < 1 {
				break // No space for this button anymore.
			}
		}
		if buttonWidth > space {
			buttonWidth = space
		}
		button.SetLabelColor(f.buttonTextColor)
		button.SetLabelColorFocused(f.buttonTextColorFocused)
		button.SetBackgroundColorFocused(f.buttonBackgroundColorFocused)
		button.SetBackgroundColor(f.buttonBackgroundColor)

		buttonIndex := index + len(f.items)
		positions[buttonIndex].x = x
		positions[buttonIndex].y = y
		positions[buttonIndex].width = buttonWidth
		positions[buttonIndex].height = 1

		if button.HasFocus() {
			focusedPosition = positions[buttonIndex]
		}

		x += buttonWidth + 1
	}

	// Determine vertical offset based on the position of the focused item.
	var offset int
	if focusedPosition.y+focusedPosition.height > bottomLimit {
		offset = focusedPosition.y + focusedPosition.height - bottomLimit
		if focusedPosition.y-offset < topLimit {
			offset = focusedPosition.y - topLimit
		}
	}

	// Draw items.
	for index, item := range f.items {
		if !item.GetVisible() {
			continue
		}

		// Set position.
		y := positions[index].y - offset
		height := positions[index].height
		item.SetRect(positions[index].x, y, positions[index].width, height)

		// Is this item visible?
		if y+height <= topLimit || y >= bottomLimit {
			continue
		}

		// Draw items with focus last (in case of overlaps).
		if item.GetFocusable().HasFocus() {
			defer item.Draw(screen)
		} else {
			item.Draw(screen)
		}
	}

	// Draw buttons.
	for index, button := range f.buttons {
		if !button.GetVisible() {
			continue
		}

		// Set position.
		buttonIndex := index + len(f.items)
		y := positions[buttonIndex].y - offset
		height := positions[buttonIndex].height
		button.SetRect(positions[buttonIndex].x, y, positions[buttonIndex].width, height)

		// Is this button visible?
		if y+height <= topLimit || y >= bottomLimit {
			continue
		}

		// Draw button.
		button.Draw(screen)
	}
}

func (f *Form) updateFocusedElement(decreasing bool) {
	li := len(f.items)
	l := len(f.items) + len(f.buttons)
	for i := 0; i < l; i++ {
		if f.focusedElement < 0 {
			if f.wrapAround {
				f.focusedElement = l - 1
			} else {
				f.focusedElement = 0
			}
		} else if f.focusedElement >= l {
			if f.wrapAround {
				f.focusedElement = 0
			} else {
				f.focusedElement = l - 1
			}
		}

		if f.focusedElement < li {
			item := f.items[f.focusedElement]
			if item.GetVisible() {
				break
			}
		} else {
			button := f.buttons[f.focusedElement-li]
			if button.GetVisible() {
				break
			}
		}

		if decreasing {
			f.focusedElement--
		} else {
			f.focusedElement++
		}
	}

}

func (f *Form) formItemInputHandler(delegate func(p Primitive)) func(key tcell.Key) {
	return func(key tcell.Key) {
		f.Lock()

		switch key {
		case tcell.KeyTab, tcell.KeyEnter:
			f.focusedElement++
			f.updateFocusedElement(false)
			f.Unlock()
			f.Focus(delegate)
			f.Lock()
		case tcell.KeyBacktab:
			f.focusedElement--
			f.updateFocusedElement(true)
			f.Unlock()
			f.Focus(delegate)
			f.Lock()
		case tcell.KeyEscape:
			if f.cancel != nil {
				f.Unlock()
				f.cancel()
				f.Lock()
			} else {
				f.focusedElement = 0
				f.updateFocusedElement(true)
				f.Unlock()
				f.Focus(delegate)
				f.Lock()
			}
		}

		f.Unlock()
	}
}

// Focus is called by the application when the primitive receives focus.
func (f *Form) Focus(delegate func(p Primitive)) {
	f.Lock()
	if len(f.items)+len(f.buttons) == 0 {
		f.hasFocus = true
		f.Unlock()
		return
	}
	f.hasFocus = false

	// Hand on the focus to one of our child elements.
	if f.focusedElement < 0 || f.focusedElement >= len(f.items)+len(f.buttons) {
		f.focusedElement = 0
	}

	if f.focusedElement < len(f.items) {
		// We're selecting an item.
		item := f.items[f.focusedElement]

		attributes := f.getAttributes()
		attributes.FinishedFunc = f.formItemInputHandler(delegate)

		f.Unlock()

		setFormItemAttributes(item, attributes)
		delegate(item)
	} else {
		// We're selecting a button.
		button := f.buttons[f.focusedElement-len(f.items)]
		button.SetBlurFunc(f.formItemInputHandler(delegate))

		f.Unlock()

		delegate(button)
	}
}

// HasFocus returns whether or not this primitive has focus.
func (f *Form) HasFocus() bool {
	f.Lock()
	defer f.Unlock()

	if f.hasFocus {
		return true
	}
	return f.focusIndex() >= 0
}

// focusIndex returns the index of the currently focused item, counting form
// items first, then buttons. A negative value indicates that no containeed item
// has focus.
func (f *Form) focusIndex() int {
	for index, item := range f.items {
		if item.GetVisible() && item.GetFocusable().HasFocus() {
			return index
		}
	}
	for index, button := range f.buttons {
		if button.GetVisible() && button.focus.HasFocus() {
			return len(f.items) + index
		}
	}
	return -1
}

// MouseHandler returns the mouse handler for this primitive.
func (f *Form) MouseHandler() func(action MouseAction, event *tcell.EventMouse, setFocus func(p Primitive)) (consumed bool, capture Primitive) {
	return f.WrapMouseHandler(func(action MouseAction, event *tcell.EventMouse, setFocus func(p Primitive)) (consumed bool, capture Primitive) {
		if !f.InRect(event.Position()) {
			return false, nil
		}

		// Determine items to pass mouse events to.
		for _, item := range f.items {
			consumed, capture = item.MouseHandler()(action, event, setFocus)
			if consumed {
				return
			}
		}
		for _, button := range f.buttons {
			consumed, capture = button.MouseHandler()(action, event, setFocus)
			if consumed {
				return
			}
		}

		// A mouse click anywhere else will return the focus to the last selected
		// element.
		if action == MouseLeftClick {
			if f.focusedElement < len(f.items) {
				setFocus(f.items[f.focusedElement])
			} else if f.focusedElement < len(f.items)+len(f.buttons) {
				setFocus(f.buttons[f.focusedElement-len(f.items)])
			}
			consumed = true
		}

		return
	})
}

func setFormItemAttributes(item FormItem, attrs *FormItemAttributes) {
	item.SetLabelWidth(attrs.LabelWidth)
	item.SetBackgroundColor(attrs.BackgroundColor)
	item.SetLabelColor(attrs.LabelColor)
	item.SetLabelColorFocused(attrs.LabelColorFocused)
	item.SetFieldTextColor(attrs.FieldTextColor)
	item.SetFieldTextColorFocused(attrs.FieldTextColorFocused)
	item.SetFieldBackgroundColor(attrs.FieldBackgroundColor)
	item.SetFieldBackgroundColorFocused(attrs.FieldBackgroundColorFocused)

	if attrs.FinishedFunc != nil {
		item.SetFinishedFunc(attrs.FinishedFunc)
	}
}
