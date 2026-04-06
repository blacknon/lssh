// Copyright (c) 2026 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package mux

import (
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
	}

	mid := len(panes) / 2
	nextDirection := tview.FlexColumn
	if direction == tview.FlexColumn {
		nextDirection = tview.FlexRow
	}

	return &layoutNode{
		direction: direction,
		children: []*layoutNode{
			buildBalancedLayout(panes[:mid], nextDirection),
			buildBalancedLayout(panes[mid:], nextDirection),
		},
	}
}

type badgeOverlay struct {
	*tview.Box
	child tview.Primitive
	label string
	color tcell.Color
}

func newBadgeOverlay(child tview.Primitive, label string, color tcell.Color) *badgeOverlay {
	return &badgeOverlay{
		Box:   tview.NewBox(),
		child: child,
		label: label,
		color: color,
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
