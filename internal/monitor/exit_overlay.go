package monitor

import (
	mview "github.com/blacknon/mview"
	"github.com/gdamore/tcell/v2"
)

type exitOverlayPrimitive struct {
	*mview.Box

	base      mview.Primitive
	overlay   mview.Primitive
	isVisible func() bool
}

func newExitOverlayPrimitive(base, overlay mview.Primitive, isVisible func() bool) *exitOverlayPrimitive {
	return &exitOverlayPrimitive{
		Box:       mview.NewBox(),
		base:      base,
		overlay:   overlay,
		isVisible: isVisible,
	}
}

func (p *exitOverlayPrimitive) Draw(screen tcell.Screen) {
	if !p.GetVisible() {
		return
	}

	x, y, width, height := p.GetRect()
	if p.base != nil {
		p.base.SetRect(x, y, width, height)
		p.base.Draw(screen)
	}

	if p.overlay != nil && p.visible() {
		ox, oy, ow, oh := p.overlayRect()
		p.overlay.SetRect(ox, oy, ow, oh)
		p.overlay.Draw(screen)
	}
}

func (p *exitOverlayPrimitive) InputHandler() func(event *tcell.EventKey, setFocus func(mview.Primitive)) {
	return p.WrapInputHandler(func(event *tcell.EventKey, setFocus func(mview.Primitive)) {
		target := p.base
		if p.visible() && p.overlay != nil {
			target = p.overlay
		}
		if target == nil {
			return
		}
		if handler := target.InputHandler(); handler != nil {
			handler(event, setFocus)
		}
	})
}

func (p *exitOverlayPrimitive) Focus(delegate func(mview.Primitive)) {
	p.Box.Focus(delegate)

	target := p.base
	if p.visible() && p.overlay != nil {
		target = p.overlay
	}
	if target != nil {
		target.Focus(delegate)
	}
}

func (p *exitOverlayPrimitive) Blur() {
	p.Box.Blur()
	if p.base != nil {
		p.base.Blur()
	}
	if p.overlay != nil {
		p.overlay.Blur()
	}
}

func (p *exitOverlayPrimitive) HasFocus() bool {
	if p.Box.HasFocus() {
		return true
	}
	if p.overlay != nil && p.overlay.HasFocus() {
		return true
	}
	return p.base != nil && p.base.HasFocus()
}

func (p *exitOverlayPrimitive) MouseHandler() func(action mview.MouseAction, event *tcell.EventMouse, setFocus func(mview.Primitive)) (bool, mview.Primitive) {
	return p.WrapMouseHandler(func(action mview.MouseAction, event *tcell.EventMouse, setFocus func(mview.Primitive)) (bool, mview.Primitive) {
		if event == nil {
			return false, nil
		}

		if p.visible() && p.overlay != nil {
			x, y := event.Position()
			ox, oy, ow, oh := p.overlayRect()
			if x >= ox && x < ox+ow && y >= oy && y < oy+oh {
				if handler := p.overlay.MouseHandler(); handler != nil {
					consumed, capture := handler(action, event, setFocus)
					if consumed && capture == nil {
						capture = p.overlay
					}
					return consumed, capture
				}
				return false, p.overlay
			}

			return true, nil
		}

		if p.base == nil {
			return false, nil
		}
		if handler := p.base.MouseHandler(); handler != nil {
			return handler(action, event, setFocus)
		}
		return false, nil
	})
}

func (p *exitOverlayPrimitive) overlayRect() (x, y, width, height int) {
	x, y, width, height = p.GetRect()
	if width <= 0 || height <= 0 {
		return x, y, width, height
	}

	overlayWidth := width / 3
	if overlayWidth < 36 {
		overlayWidth = 36
	}
	if overlayWidth > width-4 {
		overlayWidth = width - 4
	}

	overlayHeight := 7
	if overlayHeight > height-2 {
		overlayHeight = height - 2
	}
	if overlayHeight < 5 {
		overlayHeight = height
	}

	return x + (width-overlayWidth)/2, y + (height-overlayHeight)/2, overlayWidth, overlayHeight
}

func (p *exitOverlayPrimitive) visible() bool {
	return p.isVisible != nil && p.isVisible()
}
