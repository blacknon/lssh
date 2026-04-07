// Copyright (c) 2026 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package mux

import (
	"math"

	"github.com/blacknon/tvxterm"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type pane struct {
	id      int
	server  string
	term    *tvxterm.View
	session *RemoteSession
	title   string

	primitive   tview.Primitive
	focusTarget tview.Primitive
	transient   bool
	exited      bool
	exitMessage string
	failed      bool
	badgeLabel  string
	badgeColor  tcell.Color
}

type page struct {
	name   string
	panes  []*pane
	focus  *pane
	layout *layoutNode
}

type layoutNode struct {
	pane      *pane
	direction int
	children  []*layoutNode
}

func (n *layoutNode) primitive() tview.Primitive {
	if n == nil {
		return tview.NewBox()
	}
	if n.pane != nil {
		return n.pane.widget()
	}

	flex := tview.NewFlex().SetDirection(n.direction)
	for _, child := range n.children {
		flex.AddItem(child.primitive(), 0, 1, false)
	}
	return flex
}

func (p *pane) widget() tview.Primitive {
	if p == nil {
		return tview.NewBox()
	}
	if p.primitive != nil {
		return p.primitive
	}
	if p.term != nil && p.badgeLabel != "" {
		return newBadgeOverlay(p.term, p.badgeLabel, p.badgeColor)
	}
	return p.term
}

func (p *pane) focusPrimitive() tview.Primitive {
	if p == nil {
		return nil
	}
	if provider, ok := p.primitive.(interface{ focusPrimitive() tview.Primitive }); ok {
		if target := provider.focusPrimitive(); target != nil {
			return target
		}
	}
	if p.focusTarget != nil {
		return p.focusTarget
	}
	if p.primitive != nil {
		return p.primitive
	}
	return p.term
}

func (n *layoutNode) split(target *pane, next *pane, direction int) bool {
	if n == nil {
		return false
	}
	if n.pane == target {
		current := n.pane
		n.pane = nil
		n.direction = direction
		n.children = []*layoutNode{
			{pane: current},
			{pane: next},
		}
		return true
	}
	for _, child := range n.children {
		if child.split(target, next, direction) {
			return true
		}
	}
	return false
}

func (n *layoutNode) remove(target *pane) bool {
	if n == nil {
		return false
	}
	if n.pane == target {
		n.pane = nil
		return true
	}

	for i, child := range n.children {
		if child.remove(target) {
			if child.pane == nil && len(child.children) == 0 {
				n.children = append(n.children[:i], n.children[i+1:]...)
			}
			n.compact()
			return true
		}
	}

	return false
}

func (n *layoutNode) compact() {
	if n == nil {
		return
	}

	switch len(n.children) {
	case 0:
		n.pane = nil
		n.direction = 0
	case 1:
		only := n.children[0]
		n.pane = only.pane
		n.direction = only.direction
		n.children = only.children
	}
}

func buildBalancedLayout(panes []*pane, direction int) *layoutNode {
	switch len(panes) {
	case 0:
		return nil
	case 1:
		return &layoutNode{pane: panes[0]}
	case 2, 3:
		return buildLinearLayout(panes, tview.FlexColumn)
	}

	cols := int(math.Ceil(math.Sqrt(float64(len(panes)))))
	rows := (len(panes) + cols - 1) / cols
	if rows <= 1 {
		return buildLinearLayout(panes, tview.FlexColumn)
	}

	children := make([]*layoutNode, 0, rows)
	offset := 0
	base := len(panes) / rows
	rest := len(panes) % rows
	for i := 0; i < rows; i++ {
		size := base
		if i < rest {
			size++
		}
		children = append(children, buildLinearLayout(panes[offset:offset+size], tview.FlexColumn))
		offset += size
	}
	return &layoutNode{direction: tview.FlexRow, children: children}
}

func buildLinearLayout(panes []*pane, direction int) *layoutNode {
	if len(panes) == 0 {
		return nil
	}
	if len(panes) == 1 {
		return &layoutNode{pane: panes[0]}
	}
	children := make([]*layoutNode, 0, len(panes))
	for _, p := range panes {
		children = append(children, &layoutNode{pane: p})
	}
	return &layoutNode{direction: direction, children: children}
}

type badgeOverlay struct {
	*tview.Box
	child tview.Primitive
	label string
	color tcell.Color
}

type modalOverlay struct {
	*tview.Box
	child         tview.Primitive
	overlay       tview.Primitive
	focusResolver func() tview.Primitive
}

type hitTester interface {
	hitTest(x, y int) bool
}

type centeredPrimitive struct {
	*tview.Box
	child         tview.Primitive
	desiredWidth  int
	desiredHeight int
	innerX        int
	innerY        int
	innerWidth    int
	innerHeight   int
}

func newBadgeOverlay(child tview.Primitive, label string, color tcell.Color) *badgeOverlay {
	return &badgeOverlay{
		Box:   tview.NewBox(),
		child: child,
		label: label,
		color: color,
	}
}

func newModalOverlay(child, overlay tview.Primitive) *modalOverlay {
	return &modalOverlay{
		Box:     tview.NewBox(),
		child:   child,
		overlay: overlay,
	}
}

func (m *modalOverlay) setFocusResolver(resolve func() tview.Primitive) {
	m.focusResolver = resolve
}

func newCenteredPrimitive(child tview.Primitive, width, height int) *centeredPrimitive {
	return &centeredPrimitive{
		Box:           tview.NewBox(),
		child:         child,
		desiredWidth:  width,
		desiredHeight: height,
	}
}

func (b *badgeOverlay) Draw(screen tcell.Screen) {
	if b.child == nil {
		return
	}
	x, y, width, height := b.GetRect()
	b.child.SetRect(x, y, width, height)
	b.child.Draw(screen)

	if b.label == "" || width <= 2 || height <= 2 {
		return
	}

	labelWidth := tview.TaggedStringWidth(b.label)
	if labelWidth <= 0 || labelWidth >= width-1 {
		return
	}

	drawX := x + width - labelWidth - 1
	drawY := y + height - 1
	tview.Print(screen, b.label, drawX, drawY, labelWidth, tview.AlignRight, b.color)
}

func (b *badgeOverlay) SetRect(x, y, width, height int) {
	b.Box.SetRect(x, y, width, height)
	if b.child != nil {
		b.child.SetRect(x, y, width, height)
	}
}

func (b *badgeOverlay) GetRect() (int, int, int, int) {
	return b.Box.GetRect()
}

func (b *badgeOverlay) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	if b.child == nil {
		return nil
	}
	return b.child.InputHandler()
}

func (b *badgeOverlay) PasteHandler() func(text string, setFocus func(p tview.Primitive)) {
	if b.child == nil {
		return nil
	}
	return b.child.PasteHandler()
}

func (b *badgeOverlay) MouseHandler() func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(p tview.Primitive)) (bool, tview.Primitive) {
	if b.child == nil {
		return nil
	}
	return b.child.MouseHandler()
}

func (b *badgeOverlay) Focus(delegate func(p tview.Primitive)) {
	if b.child != nil {
		b.child.Focus(delegate)
	}
}

func (b *badgeOverlay) Blur() {
	if b.child != nil {
		b.child.Blur()
	}
}

func (b *badgeOverlay) HasFocus() bool {
	if b.child == nil {
		return false
	}
	return b.child.HasFocus()
}

func (m *modalOverlay) Draw(screen tcell.Screen) {
	x, y, width, height := m.GetRect()
	if m.child != nil {
		m.child.SetRect(x, y, width, height)
		m.child.Draw(screen)
	}
	if m.overlay != nil {
		m.overlay.SetRect(x, y, width, height)
		m.overlay.Draw(screen)
	}
}

func (m *modalOverlay) SetRect(x, y, width, height int) {
	m.Box.SetRect(x, y, width, height)
	if m.child != nil {
		m.child.SetRect(x, y, width, height)
	}
	if m.overlay != nil {
		m.overlay.SetRect(x, y, width, height)
	}
}

func (m *modalOverlay) GetRect() (int, int, int, int) {
	return m.Box.GetRect()
}

func (m *modalOverlay) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	if m.overlay == nil {
		return nil
	}
	return m.overlay.InputHandler()
}

func (m *modalOverlay) PasteHandler() func(text string, setFocus func(p tview.Primitive)) {
	if m.overlay == nil {
		return nil
	}
	return m.overlay.PasteHandler()
}

func (m *modalOverlay) MouseHandler() func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(p tview.Primitive)) (bool, tview.Primitive) {
	return m.WrapMouseHandler(func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(p tview.Primitive)) (bool, tview.Primitive) {
		if event == nil {
			return false, nil
		}
		x, y := event.Position()
		if !m.InRect(x, y) {
			return false, nil
		}
		target := m.focusPrimitive()
		if action == tview.MouseLeftDown && target != nil {
			setFocus(target)
		}
		if m.overlay != nil {
			if handler := m.overlay.MouseHandler(); handler != nil {
				consumed, capture := handler(action, event, setFocus)
				if consumed {
					return true, capture
				}
			}
			if target != nil {
				return true, target
			}
			return true, m.overlay
		}
		return true, m
	})
}

func (m *modalOverlay) hitTest(x, y int) bool {
	if tester, ok := m.overlay.(hitTester); ok {
		return tester.hitTest(x, y)
	}
	return m.InRect(x, y)
}

func (m *modalOverlay) Focus(delegate func(p tview.Primitive)) {
	if target := m.focusPrimitive(); target != nil {
		delegate(target)
		return
	}
	if m.overlay != nil {
		m.overlay.Focus(delegate)
	}
}

func (m *modalOverlay) Blur() {
	if m.overlay != nil {
		m.overlay.Blur()
	}
}

func (m *modalOverlay) HasFocus() bool {
	if target := m.focusPrimitive(); target != nil {
		return target.HasFocus()
	}
	if m.overlay == nil {
		return false
	}
	return m.overlay.HasFocus()
}

func (m *modalOverlay) focusPrimitive() tview.Primitive {
	if m.focusResolver != nil {
		return m.focusResolver()
	}
	return m.overlay
}

func (c *centeredPrimitive) Draw(screen tcell.Screen) {
	x, y, width, height := c.GetRect()
	c.updateInnerRect(x, y, width, height)
	if c.child != nil {
		c.child.SetRect(c.innerX, c.innerY, c.innerWidth, c.innerHeight)
		c.child.Draw(screen)
	}
}

func (c *centeredPrimitive) SetRect(x, y, width, height int) {
	c.Box.SetRect(x, y, width, height)
	c.updateInnerRect(x, y, width, height)
	if c.child != nil {
		c.child.SetRect(c.innerX, c.innerY, c.innerWidth, c.innerHeight)
	}
}

func (c *centeredPrimitive) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	if c.child == nil {
		return nil
	}
	return c.child.InputHandler()
}

func (c *centeredPrimitive) PasteHandler() func(text string, setFocus func(p tview.Primitive)) {
	if c.child == nil {
		return nil
	}
	return c.child.PasteHandler()
}

func (c *centeredPrimitive) MouseHandler() func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(p tview.Primitive)) (bool, tview.Primitive) {
	if c.child == nil {
		return nil
	}
	return c.child.MouseHandler()
}

func (c *centeredPrimitive) Focus(delegate func(p tview.Primitive)) {
	if c.child != nil {
		c.child.Focus(delegate)
	}
}

func (c *centeredPrimitive) Blur() {
	if c.child != nil {
		c.child.Blur()
	}
}

func (c *centeredPrimitive) HasFocus() bool {
	if c.child == nil {
		return false
	}
	return c.child.HasFocus()
}

func (c *centeredPrimitive) hitTest(x, y int) bool {
	return x >= c.innerX && x < c.innerX+c.innerWidth && y >= c.innerY && y < c.innerY+c.innerHeight
}

func (c *centeredPrimitive) updateInnerRect(x, y, width, height int) {
	innerWidth := c.desiredWidth
	innerHeight := c.desiredHeight
	if innerWidth > width {
		innerWidth = width
	}
	if innerHeight > height {
		innerHeight = height
	}
	if innerWidth < 1 {
		innerWidth = 1
	}
	if innerHeight < 1 {
		innerHeight = 1
	}
	c.innerWidth = innerWidth
	c.innerHeight = innerHeight
	c.innerX = x + (width-innerWidth)/2
	c.innerY = y + (height-innerHeight)/2
}
