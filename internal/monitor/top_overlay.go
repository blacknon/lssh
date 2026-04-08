package monitor

import (
	mview "github.com/blacknon/mview"
	"github.com/gdamore/tcell/v2"
)

type topOverlayPrimitive struct {
	*mview.Box

	base       mview.Primitive
	overlay    mview.Primitive
	widthRatio int
}

func newTopOverlayPrimitive(base, overlay mview.Primitive, widthRatio int) *topOverlayPrimitive {
	return &topOverlayPrimitive{
		Box:        mview.NewBox(),
		base:       base,
		overlay:    overlay,
		widthRatio: widthRatio,
	}
}

func (p *topOverlayPrimitive) Draw(screen tcell.Screen) {
	if !p.GetVisible() {
		return
	}

	x, y, width, height := p.GetRect()
	if p.base != nil {
		p.base.SetRect(x, y, width, height)
		p.base.Draw(screen)
	}
	if p.overlay != nil {
		ox, oy, ow, oh := p.overlayRect()
		p.overlay.SetRect(ox, oy, ow, oh)
		p.overlay.Draw(screen)
	}
}

func (p *topOverlayPrimitive) InputHandler() func(event *tcell.EventKey, setFocus func(mview.Primitive)) {
	handler := p.overlay.InputHandler()
	return p.WrapInputHandler(func(event *tcell.EventKey, setFocus func(mview.Primitive)) {
		if handler != nil {
			handler(event, setFocus)
		}
	})
}

func (p *topOverlayPrimitive) Focus(delegate func(mview.Primitive)) {
	p.Box.Focus(delegate)
	if p.overlay != nil {
		p.overlay.Focus(delegate)
	}
}

func (p *topOverlayPrimitive) Blur() {
	p.Box.Blur()
	if p.overlay != nil {
		p.overlay.Blur()
	}
}

func (p *topOverlayPrimitive) HasFocus() bool {
	if p.Box.HasFocus() {
		return true
	}
	if p.overlay != nil && p.overlay.HasFocus() {
		return true
	}
	return false
}

func (p *topOverlayPrimitive) MouseHandler() func(action mview.MouseAction, event *tcell.EventMouse, setFocus func(mview.Primitive)) (consumed bool, capture mview.Primitive) {
	handler := p.overlay.MouseHandler()
	return p.WrapMouseHandler(func(action mview.MouseAction, event *tcell.EventMouse, setFocus func(mview.Primitive)) (consumed bool, capture mview.Primitive) {
		if handler == nil || event == nil {
			return false, nil
		}

		x, y := event.Position()
		ox, oy, ow, oh := p.overlayRect()
		if x < ox || x >= ox+ow || y < oy || y >= oy+oh {
			return false, nil
		}

		consumed, capture = handler(action, event, setFocus)
		if consumed && capture == nil {
			capture = p.overlay
		}
		return consumed, capture
	})
}

func (p *topOverlayPrimitive) overlayRect() (x, y, width, height int) {
	x, y, width, height = p.GetRect()
	if width <= 0 || height <= 0 {
		return x, y, width, height
	}

	overlayWidth := width * p.widthRatio / 100
	if overlayWidth < 20 {
		overlayWidth = 20
	}
	if overlayWidth > width {
		overlayWidth = width
	}

	return x + width - overlayWidth, y, overlayWidth, height
}
