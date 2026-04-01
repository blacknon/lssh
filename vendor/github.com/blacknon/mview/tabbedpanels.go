package mview

import (
	"bytes"
	"fmt"
	"sync"

	"github.com/gdamore/tcell/v2"
)

// TabbedPanels is a tabbed container for other primitives. The tab switcher
// may be positioned vertically or horizontally, before or after the content.
type TabbedPanels struct {
	*Flex
	Switcher *TextView
	panels   *Panels

	tabLabels  map[string]string
	currentTab string

	dividerStart string
	dividerMid   string
	dividerEnd   string

	switcherVertical     bool
	switcherAfterContent bool
	switcherHeight       int

	width, lastWidth int

	setFocus func(Primitive)

	sync.RWMutex
}

// NewTabbedPanels returns a new TabbedPanels object.
func NewTabbedPanels() *TabbedPanels {
	t := &TabbedPanels{
		Flex:       NewFlex(),
		Switcher:   NewTextView(),
		panels:     NewPanels(),
		dividerMid: string(BoxDrawingsDoubleVertical),
		dividerEnd: string(BoxDrawingsLightVertical),
		tabLabels:  make(map[string]string),
	}

	s := t.Switcher
	s.SetDynamicColors(true)
	s.SetHighlightForegroundColor(Styles.InverseTextColor)
	s.SetHighlightBackgroundColor(Styles.PrimaryTextColor)
	s.SetRegions(true)
	s.SetScrollable(true)
	s.SetWrap(true)
	s.SetWordWrap(true)
	s.SetHighlightedFunc(func(added, removed, remaining []string) {
		if len(added) == 0 {
			return
		}

		s.ScrollToHighlight()
		t.SetCurrentTab(added[0])
		if t.setFocus != nil {
			t.setFocus(t.panels)
		}
	})

	t.rebuild()

	return t
}

// SetChangedFunc sets a handler which is called whenever a tab is added,
// selected, reordered or removed.
func (t *TabbedPanels) SetChangedFunc(handler func()) {
	t.panels.SetChangedFunc(handler)
}

// AddTab adds a new tab. Tab names should consist only of letters, numbers
// and spaces.
func (t *TabbedPanels) AddTab(name, label string, item Primitive) {
	t.Lock()
	t.tabLabels[name] = label
	t.Unlock()

	t.panels.AddPanel(name, item, true, false)

	t.updateAll()
}

// RemoveTab removes a tab.
func (t *TabbedPanels) RemoveTab(name string) {
	t.panels.RemovePanel(name)

	t.updateAll()
}

// HasTab returns true if a tab with the given name exists in this object.
func (t *TabbedPanels) HasTab(name string) bool {
	t.RLock()
	defer t.RUnlock()

	for _, panel := range t.panels.panels {
		if panel.Name == name {
			return true
		}
	}
	return false
}

// SetCurrentTab sets the currently visible tab.
func (t *TabbedPanels) SetCurrentTab(name string) {
	t.Lock()

	if t.currentTab == name {
		t.Unlock()
		return
	}

	t.currentTab = name

	t.updateAll()

	t.Unlock()

	h := t.Switcher.GetHighlights()
	var found bool
	for _, hl := range h {
		if hl == name {
			found = true
			break
		}
	}
	if !found {
		t.Switcher.Highlight(t.currentTab)
	}
	t.Switcher.ScrollToHighlight()
}

// GetCurrentTab returns the currently visible tab.
func (t *TabbedPanels) GetCurrentTab() string {
	t.RLock()
	defer t.RUnlock()
	return t.currentTab
}

// SetTabLabel sets the label of a tab.
func (t *TabbedPanels) SetTabLabel(name, label string) {
	t.Lock()
	defer t.Unlock()

	if t.tabLabels[name] == label {
		return
	}

	t.tabLabels[name] = label
	t.updateTabLabels()
}

// SetTabTextColor sets the color of the tab text.
func (t *TabbedPanels) SetTabTextColor(color tcell.Color) {
	t.Switcher.SetTextColor(color)
}

// SetTabTextColorFocused sets the color of the tab text when the tab is in focus.
func (t *TabbedPanels) SetTabTextColorFocused(color tcell.Color) {
	t.Switcher.SetHighlightForegroundColor(color)
}

// SetTabBackgroundColor sets the background color of the tab.
func (t *TabbedPanels) SetTabBackgroundColor(color tcell.Color) {
	t.Switcher.SetBackgroundColor(color)
}

// SetTabBackgroundColorFocused sets the background color of the tab when the
// tab is in focus.
func (t *TabbedPanels) SetTabBackgroundColorFocused(color tcell.Color) {
	t.Switcher.SetHighlightBackgroundColor(color)
}

// SetTabSwitcherDivider sets the tab switcher divider text. Color tags are supported.
func (t *TabbedPanels) SetTabSwitcherDivider(start, mid, end string) {
	t.Lock()
	defer t.Unlock()
	t.dividerStart, t.dividerMid, t.dividerEnd = start, mid, end
}

// SetTabSwitcherHeight sets the tab switcher height. This setting only applies
// when rendering horizontally. A value of 0 (the default) indicates the height
// should automatically adjust to fit all of the tab labels.
func (t *TabbedPanels) SetTabSwitcherHeight(height int) {
	t.Lock()
	defer t.Unlock()

	t.switcherHeight = height
	t.rebuild()
}

// SetTabSwitcherVertical sets the orientation of the tab switcher.
func (t *TabbedPanels) SetTabSwitcherVertical(vertical bool) {
	t.Lock()
	defer t.Unlock()

	if t.switcherVertical == vertical {
		return
	}

	t.switcherVertical = vertical
	t.rebuild()
}

// SetTabSwitcherAfterContent sets whether the tab switcher is positioned after content.
func (t *TabbedPanels) SetTabSwitcherAfterContent(after bool) {
	t.Lock()
	defer t.Unlock()

	if t.switcherAfterContent == after {
		return
	}

	t.switcherAfterContent = after
	t.rebuild()
}

func (t *TabbedPanels) rebuild() {
	f := t.Flex
	if t.switcherVertical {
		f.SetDirection(FlexColumn)
	} else {
		f.SetDirection(FlexRow)
	}
	f.RemoveItem(t.panels)
	f.RemoveItem(t.Switcher)
	if t.switcherAfterContent {
		f.AddItem(t.panels, 0, 1, true)
		f.AddItem(t.Switcher, 1, 1, false)
	} else {
		f.AddItem(t.Switcher, 1, 1, false)
		f.AddItem(t.panels, 0, 1, true)
	}

	t.updateTabLabels()

	t.Switcher.SetMaxLines(t.switcherHeight)
}

func (t *TabbedPanels) updateTabLabels() {
	if len(t.panels.panels) == 0 {
		t.Switcher.SetText("")
		t.Flex.ResizeItem(t.Switcher, 0, 1)
		return
	}

	maxWidth := 0
	for _, panel := range t.panels.panels {
		label := t.tabLabels[panel.Name]
		if len(label) > maxWidth {
			maxWidth = len(label)
		}
	}

	var b bytes.Buffer
	if !t.switcherVertical {
		b.WriteString(t.dividerStart)
	}
	l := len(t.panels.panels)
	spacer := []byte(" ")
	for i, panel := range t.panels.panels {
		if i > 0 && t.switcherVertical {
			b.WriteRune('\n')
		}

		if t.switcherVertical && t.switcherAfterContent {
			b.WriteString(t.dividerMid)
			b.WriteRune(' ')
		}

		label := t.tabLabels[panel.Name]
		if !t.switcherVertical {
			label = " " + label
		}

		if t.switcherVertical {
			spacer = bytes.Repeat([]byte(" "), maxWidth-len(label)+1)
		}

		b.WriteString(fmt.Sprintf(`["%s"]%s%s[""]`, panel.Name, label, spacer))

		if i == l-1 && !t.switcherVertical {
			b.WriteString(t.dividerEnd)
		} else if !t.switcherAfterContent {
			b.WriteString(t.dividerMid)
		}
	}
	t.Switcher.SetText(b.String())

	var reqLines int
	if t.switcherVertical {
		reqLines = maxWidth + 2
	} else {
		if t.switcherHeight > 0 {
			reqLines = t.switcherHeight
		} else {
			reqLines = len(WordWrap(t.Switcher.GetText(true), t.width))
			if reqLines < 1 {
				reqLines = 1
			}
		}
	}
	t.Flex.ResizeItem(t.Switcher, reqLines, 1)
}

func (t *TabbedPanels) updateVisibleTabs() {
	allPanels := t.panels.panels

	var newTab string

	var foundCurrent bool
	for _, panel := range allPanels {
		if panel.Name == t.currentTab {
			newTab = panel.Name
			foundCurrent = true
			break
		}
	}
	if !foundCurrent {
		for _, panel := range allPanels {
			if panel.Name != "" {
				newTab = panel.Name
				break
			}
		}
	}

	if t.currentTab != newTab {
		t.SetCurrentTab(newTab)
		return
	}

	for _, panel := range allPanels {
		if panel.Name == t.currentTab {
			t.panels.ShowPanel(panel.Name)
		} else {
			t.panels.HidePanel(panel.Name)
		}
	}
}

func (t *TabbedPanels) updateAll() {
	t.updateTabLabels()
	t.updateVisibleTabs()
}

// Draw draws this primitive onto the screen.
func (t *TabbedPanels) Draw(screen tcell.Screen) {
	if !t.GetVisible() {
		return
	}

	t.Box.Draw(screen)

	_, _, t.width, _ = t.GetInnerRect()
	if t.width != t.lastWidth {
		t.updateTabLabels()
	}
	t.lastWidth = t.width

	t.Flex.Draw(screen)
}

// InputHandler returns the handler for this primitive.
func (t *TabbedPanels) InputHandler() func(event *tcell.EventKey, setFocus func(p Primitive)) {
	return t.WrapInputHandler(func(event *tcell.EventKey, setFocus func(p Primitive)) {
		if t.setFocus == nil {
			t.setFocus = setFocus
		}
		t.Flex.InputHandler()(event, setFocus)
	})
}

// MouseHandler returns the mouse handler for this primitive.
func (t *TabbedPanels) MouseHandler() func(action MouseAction, event *tcell.EventMouse, setFocus func(p Primitive)) (consumed bool, capture Primitive) {
	return t.WrapMouseHandler(func(action MouseAction, event *tcell.EventMouse, setFocus func(p Primitive)) (consumed bool, capture Primitive) {
		if t.setFocus == nil {
			t.setFocus = setFocus
		}

		x, y := event.Position()
		if !t.InRect(x, y) {
			return false, nil
		}

		if t.Switcher.InRect(x, y) {
			if t.setFocus != nil {
				defer t.setFocus(t.panels)
			}
			defer t.Switcher.MouseHandler()(action, event, setFocus)
			return true, nil
		}

		return t.Flex.MouseHandler()(action, event, setFocus)
	})
}
