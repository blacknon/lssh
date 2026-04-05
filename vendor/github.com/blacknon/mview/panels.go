package mview

import (
	"sync"

	"github.com/gdamore/tcell/v2"
)

// panel represents a single panel of a Panels object.
type panel struct {
	Name    string    // The panel's name.
	Item    Primitive // The panel's primitive.
	Resize  bool      // Whether or not to resize the panel when it is drawn.
	Visible bool      // Whether or not this panel is visible.
}

// Panels is a container for other primitives often used as the application's
// root primitive. It allows to easily switch the visibility of the contained
// primitives.
type Panels struct {
	*Box

	// The contained panels. (Visible) panels are drawn from back to front.
	panels []*panel

	// We keep a reference to the function which allows us to set the focus to
	// a newly visible panel.
	setFocus func(p Primitive)

	// An optional handler which is called whenever the visibility or the order of
	// panels changes.
	changed func()

	sync.RWMutex
}

// NewPanels returns a new Panels object.
func NewPanels() *Panels {
	p := &Panels{
		Box: NewBox(),
	}
	p.focus = p
	return p
}

// SetChangedFunc sets a handler which is called whenever the visibility or the
// order of any visible panels changes. This can be used to redraw the panels.
func (p *Panels) SetChangedFunc(handler func()) {
	p.Lock()
	defer p.Unlock()

	p.changed = handler
}

// GetPanelCount returns the number of panels currently stored in this object.
func (p *Panels) GetPanelCount() int {
	p.RLock()
	defer p.RUnlock()

	return len(p.panels)
}

// AddPanel adds a new panel with the given name and primitive. If there was
// previously a panel with the same name, it is overwritten. Leaving the name
// empty may cause conflicts in other functions so always specify a non-empty
// name.
//
// Visible panels will be drawn in the order they were added (unless that order
// was changed in one of the other functions). If "resize" is set to true, the
// primitive will be set to the size available to the Panels primitive whenever
// the panels are drawn.
func (p *Panels) AddPanel(name string, item Primitive, resize, visible bool) {
	hasFocus := p.HasFocus()

	p.Lock()
	defer p.Unlock()

	var added bool
	for i, pg := range p.panels {
		if pg.Name == name {
			p.panels[i] = &panel{Item: item, Name: name, Resize: resize, Visible: visible}
			added = true
			break
		}
	}
	if !added {
		p.panels = append(p.panels, &panel{Item: item, Name: name, Resize: resize, Visible: visible})
	}
	if p.changed != nil {
		p.Unlock()
		p.changed()
		p.Lock()
	}
	if hasFocus {
		p.Unlock()
		p.Focus(p.setFocus)
		p.Lock()
	}
}

// RemovePanel removes the panel with the given name. If that panel was the only
// visible panel, visibility is assigned to the last panel.
func (p *Panels) RemovePanel(name string) {
	hasFocus := p.HasFocus()

	p.Lock()
	defer p.Unlock()

	var isVisible bool
	for index, panel := range p.panels {
		if panel.Name == name {
			isVisible = panel.Visible
			p.panels = append(p.panels[:index], p.panels[index+1:]...)
			if panel.Visible && p.changed != nil {
				p.Unlock()
				p.changed()
				p.Lock()
			}
			break
		}
	}
	if isVisible {
		for index, panel := range p.panels {
			if index < len(p.panels)-1 {
				if panel.Visible {
					break // There is a remaining visible panel.
				}
			} else {
				panel.Visible = true // We need at least one visible panel.
			}
		}
	}
	if hasFocus {
		p.Unlock()
		p.Focus(p.setFocus)
		p.Lock()
	}
}

// HasPanel returns true if a panel with the given name exists in this object.
func (p *Panels) HasPanel(name string) bool {
	p.RLock()
	defer p.RUnlock()

	for _, panel := range p.panels {
		if panel.Name == name {
			return true
		}
	}
	return false
}

// ShowPanel sets a panel's visibility to "true" (in addition to any other panels
// which are already visible).
func (p *Panels) ShowPanel(name string) {
	hasFocus := p.HasFocus()

	p.Lock()
	defer p.Unlock()

	for _, panel := range p.panels {
		if panel.Name == name {
			panel.Visible = true
			if p.changed != nil {
				p.Unlock()
				p.changed()
				p.Lock()
			}
			break
		}
	}
	if hasFocus {
		p.Unlock()
		p.Focus(p.setFocus)
		p.Lock()
	}
}

// HidePanel sets a panel's visibility to "false".
func (p *Panels) HidePanel(name string) {
	hasFocus := p.HasFocus()

	p.Lock()
	defer p.Unlock()

	for _, panel := range p.panels {
		if panel.Name == name {
			panel.Visible = false
			if p.changed != nil {
				p.Unlock()
				p.changed()
				p.Lock()
			}
			break
		}
	}
	if hasFocus {
		p.Unlock()
		p.Focus(p.setFocus)
		p.Lock()
	}
}

// SetCurrentPanel sets a panel's visibility to "true" and all other panels'
// visibility to "false".
func (p *Panels) SetCurrentPanel(name string) {
	hasFocus := p.HasFocus()

	p.Lock()
	defer p.Unlock()

	for _, panel := range p.panels {
		if panel.Name == name {
			panel.Visible = true
		} else {
			panel.Visible = false
		}
	}
	if p.changed != nil {
		p.Unlock()
		p.changed()
		p.Lock()
	}
	if hasFocus {
		p.Unlock()
		p.Focus(p.setFocus)
		p.Lock()
	}
}

// SendToFront changes the order of the panels such that the panel with the given
// name comes last, causing it to be drawn last with the next update (if
// visible).
func (p *Panels) SendToFront(name string) {
	hasFocus := p.HasFocus()

	p.Lock()
	defer p.Unlock()

	for index, panel := range p.panels {
		if panel.Name == name {
			if index < len(p.panels)-1 {
				p.panels = append(append(p.panels[:index], p.panels[index+1:]...), panel)
			}
			if panel.Visible && p.changed != nil {
				p.Unlock()
				p.changed()
				p.Lock()
			}
			break
		}
	}
	if hasFocus {
		p.Unlock()
		p.Focus(p.setFocus)
		p.Lock()
	}
}

// SendToBack changes the order of the panels such that the panel with the given
// name comes first, causing it to be drawn first with the next update (if
// visible).
func (p *Panels) SendToBack(name string) {
	hasFocus := p.HasFocus()

	p.Lock()
	defer p.Unlock()

	for index, pg := range p.panels {
		if pg.Name == name {
			if index > 0 {
				p.panels = append(append([]*panel{pg}, p.panels[:index]...), p.panels[index+1:]...)
			}
			if pg.Visible && p.changed != nil {
				p.Unlock()
				p.changed()
				p.Lock()
			}
			break
		}
	}
	if hasFocus {
		p.Unlock()
		p.Focus(p.setFocus)
		p.Lock()
	}
}

// GetFrontPanel returns the front-most visible panel. If there are no visible
// panels, ("", nil) is returned.
func (p *Panels) GetFrontPanel() (name string, item Primitive) {
	p.RLock()
	defer p.RUnlock()

	for index := len(p.panels) - 1; index >= 0; index-- {
		if p.panels[index].Visible {
			return p.panels[index].Name, p.panels[index].Item
		}
	}
	return
}

// HasFocus returns whether or not this primitive has focus.
func (p *Panels) HasFocus() bool {
	p.RLock()
	defer p.RUnlock()

	for _, panel := range p.panels {
		if panel.Item.GetFocusable().HasFocus() {
			return true
		}
	}
	return false
}

// Focus is called by the application when the primitive receives focus.
func (p *Panels) Focus(delegate func(p Primitive)) {
	p.Lock()
	defer p.Unlock()

	if delegate == nil {
		return // We cannot delegate so we cannot focus.
	}
	p.setFocus = delegate
	var topItem Primitive
	for _, panel := range p.panels {
		if panel.Visible {
			topItem = panel.Item
		}
	}
	if topItem != nil {
		p.Unlock()
		delegate(topItem)
		p.Lock()
	}
}

// Draw draws this primitive onto the screen.
func (p *Panels) Draw(screen tcell.Screen) {
	if !p.GetVisible() {
		return
	}

	p.Box.Draw(screen)

	p.Lock()
	defer p.Unlock()

	x, y, width, height := p.GetInnerRect()

	for _, panel := range p.panels {
		if !panel.Visible {
			continue
		}
		if panel.Resize {
			panel.Item.SetRect(x, y, width, height)
		}
		panel.Item.Draw(screen)
	}
}

// MouseHandler returns the mouse handler for this primitive.
func (p *Panels) MouseHandler() func(action MouseAction, event *tcell.EventMouse, setFocus func(p Primitive)) (consumed bool, capture Primitive) {
	return p.WrapMouseHandler(func(action MouseAction, event *tcell.EventMouse, setFocus func(p Primitive)) (consumed bool, capture Primitive) {
		if !p.InRect(event.Position()) {
			return false, nil
		}

		// Pass mouse events along to the last visible panel item that takes it.
		for index := len(p.panels) - 1; index >= 0; index-- {
			panel := p.panels[index]
			if panel.Visible {
				consumed, capture = panel.Item.MouseHandler()(action, event, setFocus)
				if consumed {
					return
				}
			}
		}

		return
	})
}

// Support backwards compatibility with Pages.
type page = panel

// Pages is a wrapper around Panels.
//
// Deprecated: This type is provided for backwards compatibility.
// Developers should use Panels instead.
type Pages struct {
	*Panels
}

// NewPages returns a new Panels object.
//
// Deprecated: This function is provided for backwards compatibility.
// Developers should use NewPanels instead.
func NewPages() *Pages {
	return &Pages{NewPanels()}
}

// GetPageCount returns the number of panels currently stored in this object.
func (p *Pages) GetPageCount() int {
	return p.GetPanelCount()
}

// AddPage adds a new panel with the given name and primitive.
func (p *Pages) AddPage(name string, item Primitive, resize, visible bool) {
	p.AddPanel(name, item, resize, visible)
}

// AddAndSwitchToPage calls Add(), then SwitchTo() on that newly added panel.
func (p *Pages) AddAndSwitchToPage(name string, item Primitive, resize bool) {
	p.AddPanel(name, item, resize, true)
	p.SetCurrentPanel(name)
}

// RemovePage removes the panel with the given name.
func (p *Pages) RemovePage(name string) {
	p.RemovePanel(name)
}

// HasPage returns true if a panel with the given name exists in this object.
func (p *Pages) HasPage(name string) bool {
	return p.HasPanel(name)
}

// ShowPage sets a panel's visibility to "true".
func (p *Pages) ShowPage(name string) {
	p.ShowPanel(name)
}

// HidePage sets a panel's visibility to "false".
func (p *Pages) HidePage(name string) {
	p.HidePanel(name)
}

// SwitchToPage sets a panel's visibility to "true" and all other panels'
// visibility to "false".
func (p *Pages) SwitchToPage(name string) {
	p.SetCurrentPanel(name)
}

// GetFrontPage returns the front-most visible panel.
func (p *Pages) GetFrontPage() (name string, item Primitive) {
	return p.GetFrontPanel()
}
