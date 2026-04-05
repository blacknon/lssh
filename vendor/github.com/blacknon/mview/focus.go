package mview

import "sync"

// Focusable provides a method which determines if a primitive has focus.
// Composed primitives may be focused based on the focused state of their
// contained primitives.
type Focusable interface {
	HasFocus() bool
}

type focusElement struct {
	primitive Primitive
	disabled  bool
}

// FocusManager manages application focus.
type FocusManager struct {
	elements []*focusElement

	focused    int
	wrapAround bool

	setFocus func(p Primitive)

	sync.RWMutex
}

// NewFocusManager returns a new FocusManager object.
func NewFocusManager(setFocus func(p Primitive)) *FocusManager {
	return &FocusManager{setFocus: setFocus}
}

// SetWrapAround sets the flag that determines whether navigation will wrap
// around. That is, navigating forwards on the last field will move the
// selection to the first field (similarly in the other direction). If set to
// false, the focus won't change when navigating forwards on the last element
// or navigating backwards on the first element.
func (f *FocusManager) SetWrapAround(wrapAround bool) {
	f.Lock()
	defer f.Unlock()

	f.wrapAround = wrapAround
}

// Add adds an element to the focus handler.
func (f *FocusManager) Add(p ...Primitive) {
	f.Lock()
	defer f.Unlock()

	for _, primitive := range p {
		f.elements = append(f.elements, &focusElement{primitive: primitive})
	}
}

// AddAt adds an element to the focus handler at the specified index.
func (f *FocusManager) AddAt(index int, p Primitive) {
	f.Lock()
	defer f.Unlock()

	if index < 0 || index > len(f.elements) {
		panic("index out of range")
	}

	element := &focusElement{primitive: p}

	if index == len(f.elements) {
		f.elements = append(f.elements, element)
		return
	}
	f.elements = append(f.elements[:index+1], f.elements[index:]...)
	f.elements[index] = element
}

// Focus focuses the provided element.
func (f *FocusManager) Focus(p Primitive) {
	f.Lock()
	defer f.Unlock()

	for i, element := range f.elements {
		if p == element.primitive && !element.disabled {
			f.focused = i
			break
		}
	}
	f.setFocus(f.elements[f.focused].primitive)
}

// FocusPrevious focuses the previous element.
func (f *FocusManager) FocusPrevious() {
	f.Lock()
	defer f.Unlock()

	f.focused--
	f.updateFocusIndex(true)
	f.setFocus(f.elements[f.focused].primitive)
}

// FocusNext focuses the next element.
func (f *FocusManager) FocusNext() {
	f.Lock()
	defer f.Unlock()

	f.focused++
	f.updateFocusIndex(false)
	f.setFocus(f.elements[f.focused].primitive)
}

// FocusAt focuses the element at the provided index.
func (f *FocusManager) FocusAt(index int) {
	f.Lock()
	defer f.Unlock()

	f.focused = index
	f.setFocus(f.elements[f.focused].primitive)
}

// GetFocusIndex returns the index of the currently focused element.
func (f *FocusManager) GetFocusIndex() int {
	f.Lock()
	defer f.Unlock()

	return f.focused
}

// GetFocusedPrimitive returns the currently focused primitive.
func (f *FocusManager) GetFocusedPrimitive() Primitive {
	f.Lock()
	defer f.Unlock()

	return f.elements[f.focused].primitive
}

func (f *FocusManager) updateFocusIndex(decreasing bool) {
	for i := 0; i < len(f.elements); i++ {
		if f.focused < 0 {
			if f.wrapAround {
				f.focused = len(f.elements) - 1
			} else {
				f.focused = 0
			}
		} else if f.focused >= len(f.elements) {
			if f.wrapAround {
				f.focused = 0
			} else {
				f.focused = len(f.elements) - 1
			}
		}

		item := f.elements[f.focused]
		if !item.disabled {
			break
		}

		if decreasing {
			f.focused--
		} else {
			f.focused++
		}
	}
}

// Transform modifies the current focus.
func (f *FocusManager) Transform(tr Transformation) {
	var decreasing bool
	switch tr {
	case TransformFirstItem:
		f.focused = 0
		decreasing = true
	case TransformLastItem:
		f.focused = len(f.elements) - 1
	case TransformPreviousItem:
		f.focused--
		decreasing = true
	case TransformNextItem:
		f.focused++
	}
	f.updateFocusIndex(decreasing)
}
