package monitor

import (
	mview "github.com/blacknon/mview"
	"github.com/gdamore/tcell/v2"
)

type topAlignedPrimitive struct {
	*mview.Flex

	content         mview.Primitive
	filler          mview.Primitive
	preferredHeight func() int
}

func newTopAlignedPrimitive(title string, content mview.Primitive, preferredHeight func() int) *topAlignedPrimitive {
	flex := mview.NewFlex()
	flex.SetDirection(mview.FlexRow)
	flex.SetBorder(true)
	flex.SetBorderColor(monitorBorderColor)
	flex.SetTitle(title)
	flex.SetTitleAlign(mview.AlignLeft)
	flex.SetTitleColor(monitorAccentColor)

	filler := createEmptyPrimitive()
	flex.AddItem(content, 0, 1, false)
	flex.AddItem(filler, 0, 1, false)

	return &topAlignedPrimitive{
		Flex:            flex,
		content:         content,
		filler:          filler,
		preferredHeight: preferredHeight,
	}
}

func (p *topAlignedPrimitive) Draw(screen tcell.Screen) {
	if p.content != nil && p.preferredHeight != nil {
		if preferred := p.preferredHeight(); preferred > 0 {
			p.ResizeItem(p.content, preferred, 0)
			p.ResizeItem(p.filler, 0, 1)
		}
	}

	p.Flex.Draw(screen)
}
