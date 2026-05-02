package monitor

import (
	mview "github.com/blacknon/mview"
	"github.com/gdamore/tcell/v2"
)

type switchablePrimitive struct {
	*mview.Box
	current mview.Primitive
}

func newSwitchablePrimitive(initial mview.Primitive) *switchablePrimitive {
	return &switchablePrimitive{
		Box:     mview.NewBox(),
		current: initial,
	}
}

func (p *switchablePrimitive) SetPrimitive(next mview.Primitive) {
	p.current = next
}

func (p *switchablePrimitive) Draw(screen tcell.Screen) {
	if !p.GetVisible() {
		return
	}

	if p.current == nil {
		return
	}

	x, y, width, height := p.GetRect()
	p.current.SetRect(x, y, width, height)
	p.current.Draw(screen)
}

func (p *switchablePrimitive) InputHandler() func(event *tcell.EventKey, setFocus func(mview.Primitive)) {
	if p.current == nil {
		return p.WrapInputHandler(func(event *tcell.EventKey, setFocus func(mview.Primitive)) {})
	}

	handler := p.current.InputHandler()
	return p.WrapInputHandler(func(event *tcell.EventKey, setFocus func(mview.Primitive)) {
		if handler != nil {
			handler(event, setFocus)
		}
	})
}

func (p *switchablePrimitive) Focus(delegate func(mview.Primitive)) {
	p.Box.Focus(delegate)
	if p.current != nil {
		p.current.Focus(delegate)
	}
}

func (p *switchablePrimitive) Blur() {
	p.Box.Blur()
	if p.current != nil {
		p.current.Blur()
	}
}

func (p *switchablePrimitive) HasFocus() bool {
	if p.Box.HasFocus() {
		return true
	}
	return p.current != nil && p.current.HasFocus()
}

func (p *switchablePrimitive) MouseHandler() func(action mview.MouseAction, event *tcell.EventMouse, setFocus func(mview.Primitive)) (consumed bool, capture mview.Primitive) {
	if p.current == nil {
		return p.WrapMouseHandler(func(action mview.MouseAction, event *tcell.EventMouse, setFocus func(mview.Primitive)) (bool, mview.Primitive) {
			return false, nil
		})
	}

	handler := p.current.MouseHandler()
	return p.WrapMouseHandler(func(action mview.MouseAction, event *tcell.EventMouse, setFocus func(mview.Primitive)) (consumed bool, capture mview.Primitive) {
		if handler == nil {
			return false, nil
		}
		return handler(action, event, setFocus)
	})
}
