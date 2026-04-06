// Copyright (c) 2026 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package mux

import (
	"github.com/blacknon/tvxterm"
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
