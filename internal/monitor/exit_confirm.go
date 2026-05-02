package monitor

import (
	mview "github.com/blacknon/mview"
)

func (m *Monitor) createExitConfirmModal() *mview.Modal {
	modal := mview.NewModal()
	modal.SetTitle(" Confirm Exit ")
	modal.SetTitleColor(monitorAccentColor)
	modal.SetBorder(true)
	modal.SetBorderColor(monitorBorderColor)
	modal.SetBorderColorFocused(monitorAccentColor)
	modal.SetBackgroundColor(monitorBaseColor)
	modal.SetText("Exit lsmon?")
	modal.SetTextColor(monitorTextColor)
	modal.SetTextAlign(mview.AlignCenter)
	modal.AddButtons([]string{"Yes", "No"})
	modal.SetButtonBackgroundColor(monitorHeaderColor)
	modal.SetButtonTextColor(monitorBaseColor)
	modal.SetButtonsAlign(mview.AlignCenter)
	modal.SetFocus(1)

	form := modal.GetForm()
	form.SetButtonBackgroundColor(monitorHeaderColor)
	form.SetButtonBackgroundColorFocused(monitorAccentColor)
	form.SetButtonTextColor(monitorBaseColor)
	form.SetButtonTextColorFocused(monitorBaseColor)

	frame := modal.GetFrame()
	frame.Clear()
	frame.AddText(" y: yes / n: no / esc: cancel ", false, mview.AlignCenter, monitorMutedColor)

	modal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		switch buttonLabel {
		case "Yes":
			m.confirmExit()
		default:
			m.hideExitConfirm()
		}
	})

	return modal
}

func (m *Monitor) showExitConfirm() {
	if m.View == nil || m.exitModal == nil {
		return
	}

	if m.isExitConfirmVisible() {
		m.View.SetFocus(m.exitModal)
		return
	}

	m.setExitConfirmVisible(true, m.View.GetFocus())
	m.View.SetFocus(m.exitModal)
	m.DrawUpdate()
}

func (m *Monitor) hideExitConfirm() {
	if m.View == nil {
		return
	}

	focus := m.getExitConfirmFocus()
	m.setExitConfirmVisible(false, nil)

	if focus != nil {
		m.View.SetFocus(focus)
	} else if m.table != nil {
		m.View.SetFocus(m.table)
	}

	m.DrawUpdate()
}

func (m *Monitor) confirmExit() {
	if m.View == nil {
		return
	}

	m.View.Stop()
}
