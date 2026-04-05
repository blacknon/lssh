package mview

import (
	"sync"

	"github.com/gdamore/tcell/v2"
)

// Configuration values.
const (
	FlexRow = iota
	FlexColumn
)

// flexItem holds layout options for one item.
type flexItem struct {
	Item       Primitive // The item to be positioned. May be nil for an empty item.
	FixedSize  int       // The item's fixed size which may not be changed, 0 if it has no fixed size.
	Proportion int       // The item's proportion.
	Focus      bool      // Whether or not this item attracts the layout's focus.
}

// Flex is a basic implementation of the Flexbox layout. The contained
// primitives are arranged horizontally or vertically. The way they are
// distributed along that dimension depends on their layout settings, which is
// either a fixed length or a proportional length. See AddItem() for details.
type Flex struct {
	*Box

	// The items to be positioned.
	items []*flexItem

	// FlexRow or FlexColumn.
	direction int

	// If set to true, Flex will use the entire screen as its available space
	// instead its box dimensions.
	fullScreen bool

	sync.RWMutex
}

// NewFlex returns a new flexbox layout container with no primitives and its
// direction set to FlexColumn. To add primitives to this layout, see AddItem().
// To change the direction, see SetDirection().
//
// Note that Flex will have a transparent background by default so that any nil
// flex items will show primitives behind the Flex.
// To disable this transparency:
//
//   flex.SetBackgroundTransparent(false)
func NewFlex() *Flex {
	f := &Flex{
		Box:       NewBox(),
		direction: FlexColumn,
	}
	f.SetBackgroundTransparent(true)
	f.focus = f
	return f
}

// GetDirection returns the direction in which the contained primitives are
// distributed. This can be either FlexColumn (default) or FlexRow.
func (f *Flex) GetDirection() int {
	f.RLock()
	defer f.RUnlock()
	return f.direction
}

// SetDirection sets the direction in which the contained primitives are
// distributed. This can be either FlexColumn (default) or FlexRow.
func (f *Flex) SetDirection(direction int) {
	f.Lock()
	defer f.Unlock()

	f.direction = direction
}

// SetFullScreen sets the flag which, when true, causes the flex layout to use
// the entire screen space instead of whatever size it is currently assigned to.
func (f *Flex) SetFullScreen(fullScreen bool) {
	f.Lock()
	defer f.Unlock()

	f.fullScreen = fullScreen
}

// AddItem adds a new item to the container. The "fixedSize" argument is a width
// or height that may not be changed by the layout algorithm. A value of 0 means
// that its size is flexible and may be changed. The "proportion" argument
// defines the relative size of the item compared to other flexible-size items.
// For example, items with a proportion of 2 will be twice as large as items
// with a proportion of 1. The proportion must be at least 1 if fixedSize == 0
// (ignored otherwise).
//
// If "focus" is set to true, the item will receive focus when the Flex
// primitive receives focus. If multiple items have the "focus" flag set to
// true, the first one will receive focus.
//
// A nil value for the primitive represents empty space.
func (f *Flex) AddItem(item Primitive, fixedSize, proportion int, focus bool) {
	f.Lock()
	defer f.Unlock()

	if item == nil {
		item = NewBox()
		item.SetVisible(false)
	}

	f.items = append(f.items, &flexItem{Item: item, FixedSize: fixedSize, Proportion: proportion, Focus: focus})
}

// AddItemAtIndex adds an item to the flex at a given index.
// For more information see AddItem.
func (f *Flex) AddItemAtIndex(index int, item Primitive, fixedSize, proportion int, focus bool) {
	f.Lock()
	defer f.Unlock()
	newItem := &flexItem{Item: item, FixedSize: fixedSize, Proportion: proportion, Focus: focus}

	if index == 0 {
		f.items = append([]*flexItem{newItem}, f.items...)
	} else {
		f.items = append(f.items[:index], append([]*flexItem{newItem}, f.items[index:]...)...)
	}
}

// RemoveItem removes all items for the given primitive from the container,
// keeping the order of the remaining items intact.
func (f *Flex) RemoveItem(p Primitive) {
	f.Lock()
	defer f.Unlock()

	for index := len(f.items) - 1; index >= 0; index-- {
		if f.items[index].Item == p {
			f.items = append(f.items[:index], f.items[index+1:]...)
		}
	}
}

// ResizeItem sets a new size for the item(s) with the given primitive. If there
// are multiple Flex items with the same primitive, they will all receive the
// same size. For details regarding the size parameters, see AddItem().
func (f *Flex) ResizeItem(p Primitive, fixedSize, proportion int) {
	f.Lock()
	defer f.Unlock()

	for _, item := range f.items {
		if item.Item == p {
			item.FixedSize = fixedSize
			item.Proportion = proportion
		}
	}
}

// Draw draws this primitive onto the screen.
func (f *Flex) Draw(screen tcell.Screen) {
	if !f.GetVisible() {
		return
	}

	f.Box.Draw(screen)

	f.Lock()
	defer f.Unlock()

	// Calculate size and position of the items.

	// Do we use the entire screen?
	if f.fullScreen {
		width, height := screen.Size()
		f.SetRect(0, 0, width, height)
	}

	// How much space can we distribute?
	x, y, width, height := f.GetInnerRect()
	var proportionSum int
	distSize := width
	if f.direction == FlexRow {
		distSize = height
	}
	for _, item := range f.items {
		if item.FixedSize > 0 {
			distSize -= item.FixedSize
		} else {
			proportionSum += item.Proportion
		}
	}

	// Calculate positions and draw items.
	pos := x
	if f.direction == FlexRow {
		pos = y
	}
	for _, item := range f.items {
		size := item.FixedSize
		if size <= 0 {
			if proportionSum > 0 {
				size = distSize * item.Proportion / proportionSum
				distSize -= size
				proportionSum -= item.Proportion
			} else {
				size = 0
			}
		}
		if item.Item != nil {
			if f.direction == FlexColumn {
				item.Item.SetRect(pos, y, size, height)
			} else {
				item.Item.SetRect(x, pos, width, size)
			}
		}
		pos += size

		if item.Item != nil {
			if item.Item.GetFocusable().HasFocus() {
				defer item.Item.Draw(screen)
			} else {
				item.Item.Draw(screen)
			}
		}
	}
}

// Focus is called when this primitive receives focus.
func (f *Flex) Focus(delegate func(p Primitive)) {
	f.Lock()

	for _, item := range f.items {
		if item.Item != nil && item.Focus {
			f.Unlock()
			delegate(item.Item)
			return
		}
	}

	f.Unlock()
}

// HasFocus returns whether or not this primitive has focus.
func (f *Flex) HasFocus() bool {
	f.RLock()
	defer f.RUnlock()

	for _, item := range f.items {
		if item.Item != nil && item.Item.GetFocusable().HasFocus() {
			return true
		}
	}
	return false
}

// MouseHandler returns the mouse handler for this primitive.
func (f *Flex) MouseHandler() func(action MouseAction, event *tcell.EventMouse, setFocus func(p Primitive)) (consumed bool, capture Primitive) {
	return f.WrapMouseHandler(func(action MouseAction, event *tcell.EventMouse, setFocus func(p Primitive)) (consumed bool, capture Primitive) {
		if !f.InRect(event.Position()) {
			return false, nil
		}

		// Pass mouse events along to the first child item that takes it.
		for _, item := range f.items {
			if item.Item == nil {
				continue
			}

			consumed, capture = item.Item.MouseHandler()(action, event, setFocus)
			if consumed {
				return
			}
		}

		return
	})
}
