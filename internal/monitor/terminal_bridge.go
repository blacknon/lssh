package monitor

import (
	mview "github.com/blacknon/mview"
	"github.com/blacknon/tvxterm"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// tvxtermPrimitive adapts a tvxterm/tview primitive to mview.
type tvxtermPrimitive struct {
	*mview.Box

	term *tvxterm.View
}

func newTVXTermPrimitive(term *tvxterm.View) *tvxtermPrimitive {
	return &tvxtermPrimitive{
		Box:  mview.NewBox(),
		term: term,
	}
}

func (p *tvxtermPrimitive) Draw(screen tcell.Screen) {
	if !p.GetVisible() {
		return
	}

	x, y, width, height := p.GetRect()
	p.term.SetRect(x, y, width, height)
	p.term.Draw(screen)
}

func (p *tvxtermPrimitive) InputHandler() func(event *tcell.EventKey, setFocus func(mview.Primitive)) {
	handler := p.term.InputHandler()
	return p.WrapInputHandler(func(event *tcell.EventKey, setFocus func(mview.Primitive)) {
		if handler == nil {
			return
		}

		handler(event, func(_ tview.Primitive) {
			setFocus(p)
		})
	})
}

func (p *tvxtermPrimitive) Focus(delegate func(mview.Primitive)) {
	p.Box.Focus(delegate)
	p.term.Focus(func(tview.Primitive) {})
}

func (p *tvxtermPrimitive) Blur() {
	p.Box.Blur()
	p.term.Blur()
}

func (p *tvxtermPrimitive) MouseHandler() func(action mview.MouseAction, event *tcell.EventMouse, setFocus func(mview.Primitive)) (consumed bool, capture mview.Primitive) {
	handler := p.term.MouseHandler()
	return p.WrapMouseHandler(func(action mview.MouseAction, event *tcell.EventMouse, setFocus func(mview.Primitive)) (consumed bool, capture mview.Primitive) {
		if handler == nil {
			return false, nil
		}

		consumed, _ = handler(tview.MouseAction(action), event, func(_ tview.Primitive) {
			setFocus(p)
		})
		if consumed {
			return true, p
		}
		return false, nil
	})
}

func (p *tvxtermPrimitive) Close() error {
	if p.term == nil {
		return nil
	}
	return p.term.Close()
}
