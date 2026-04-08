// Copyright (c) 2026 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package mux

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/pkg/sftp"
	"github.com/rivo/tview"
)

type transferFileEntry struct {
	Name  string
	Path  string
	IsDir bool
}

type transferFileBrowser struct {
	root       *tview.Flex
	headerView *tview.TextView
	pathView   *tview.TextView
	helpView   *tview.TextView
	table      *tview.Table

	load         func(string) (string, []transferFileEntry, error)
	onCancel     func()
	onChange     func([]string)
	onTabNav     func(int)
	onFocusNav   func(int)
	onSelectPath func(string)

	current   string
	entries   []transferFileEntry
	selected  map[string]transferFileEntry
	selecting bool
	title     string
	pathLabel string
	hostLabel string
}

type transferTargetPicker struct {
	*tview.List
	all        []string
	selected   map[string]struct{}
	onChange   func([]string)
	onFocusNav func(int)
}

func newTransferFileBrowser(
	load func(string) (string, []transferFileEntry, error),
	onCancel func(),
	onChange func([]string),
	onTabNav func(int),
) *transferFileBrowser {
	b := &transferFileBrowser{
		root:       tview.NewFlex().SetDirection(tview.FlexRow),
		headerView: tview.NewTextView().SetDynamicColors(true).SetWrap(false),
		pathView:   tview.NewTextView().SetDynamicColors(true).SetWrap(false),
		helpView:   tview.NewTextView().SetDynamicColors(true).SetWrap(true),
		table:      tview.NewTable().SetSelectable(true, false).SetBorders(false),
		load:       load,
		onCancel:   onCancel,
		onChange:   onChange,
		onTabNav:   onTabNav,
		selected:   map[string]transferFileEntry{},
		title:      "browser",
		pathLabel:  "path",
	}

	b.root.SetBorder(true)
	b.root.SetBorderColor(tcell.ColorGreen)
	b.root.SetTitleColor(tcell.ColorYellow)
	b.root.SetTitle("browser")
	b.helpView.SetText("[yellow]Space[-]: toggle  [yellow]Enter[-]: select/open  [yellow]BS/Left[-]: parent  [yellow]Esc[-]: close")
	b.table.SetInputCapture(b.handleInput)
	b.table.SetSelectedFunc(func(row, column int) {
		b.activateCurrent(false)
	})
	b.reload("")

	b.root.AddItem(b.headerView, 1, 0, false)
	b.root.AddItem(b.pathView, 1, 0, false)
	b.root.AddItem(b.table, 0, 1, true)
	b.root.AddItem(b.helpView, 1, 0, false)

	return b
}

func newTransferTargetPicker(all []string, initial []string, onChange func([]string)) *transferTargetPicker {
	p := &transferTargetPicker{
		List:     tview.NewList().ShowSecondaryText(false),
		all:      append([]string(nil), all...),
		selected: map[string]struct{}{},
		onChange: onChange,
	}
	p.SetMainTextColor(tcell.ColorWhite)
	p.SetSelectedTextColor(tcell.ColorBlack)
	p.SetSelectedBackgroundColor(tcell.ColorGreen)
	p.SetHighlightFullLine(true)
	p.SetBackgroundColor(tcell.ColorBlack)
	p.SetInputCapture(p.handleInput)
	p.SetSelectedFunc(func(index int, mainText, secondaryText string, shortcut rune) {
		p.toggleCurrent()
	})
	p.SetSelected(initial)
	return p
}

func (b *transferFileBrowser) setTitle(title string) {
	if title == "" {
		title = "browser"
	}
	b.title = title
	b.root.SetTitle(title)
}

func (b *transferFileBrowser) setPathLabel(label string) {
	if label == "" {
		label = "path"
	}
	b.pathLabel = label
	b.updateHeader()
}

func (b *transferFileBrowser) setHostLabel(host string) {
	b.hostLabel = host
	b.updateHeader()
}

func (b *transferFileBrowser) updateHeader() {
	if b.headerView != nil {
		b.headerView.SetText(fmt.Sprintf("[yellow]host[-]: %s", tview.Escape(b.hostLabel)))
	}
	if b.pathView != nil {
		b.pathView.SetText(fmt.Sprintf("[yellow]%s[-]: %s", tview.Escape(b.pathLabel), tview.Escape(b.current)))
	}
}

func (p *transferTargetPicker) handleInput(event *tcell.EventKey) *tcell.EventKey {
	if event == nil {
		return nil
	}
	switch event.Key() {
	case tcell.KeyTab:
		if p.onFocusNav != nil {
			p.onFocusNav(1)
			return nil
		}
	case tcell.KeyBacktab:
		if p.onFocusNav != nil {
			p.onFocusNav(-1)
			return nil
		}
	case tcell.KeyRune:
		if event.Rune() == ' ' {
			p.toggleCurrent()
			return nil
		}
	case tcell.KeyEnter:
		p.toggleCurrent()
		return nil
	}
	return event
}

func (p *transferTargetPicker) SetSelected(targets []string) {
	p.selected = map[string]struct{}{}
	for _, target := range targets {
		p.selected[target] = struct{}{}
	}
	p.refresh()
}

func (p *transferTargetPicker) Selected() []string {
	result := make([]string, 0, len(p.selected))
	for target := range p.selected {
		result = append(result, target)
	}
	sort.Strings(result)
	return result
}

func (p *transferTargetPicker) refresh() {
	current := p.GetCurrentItem()
	p.Clear()
	for _, name := range p.all {
		prefix := "[ ]"
		if _, ok := p.selected[name]; ok {
			prefix = "[x]"
		}
		p.AddItem(prefix+" "+name, "", 0, nil)
	}
	if len(p.all) == 0 {
		p.AddItem("(no connected panes)", "", 0, nil)
	}
	if current >= 0 && current < p.GetItemCount() {
		p.SetCurrentItem(current)
	}
	if p.onChange != nil {
		p.onChange(p.Selected())
	}
}

func (p *transferTargetPicker) toggleCurrent() {
	index := p.GetCurrentItem()
	if index < 0 || index >= len(p.all) {
		return
	}
	name := p.all[index]
	if _, ok := p.selected[name]; ok {
		delete(p.selected, name)
	} else {
		p.selected[name] = struct{}{}
	}
	p.refresh()
}

func (b *transferFileBrowser) selectedPaths() []string {
	result := make([]string, 0, len(b.selected))
	for _, entry := range b.selected {
		result = append(result, entry.Path)
	}
	sort.Strings(result)
	return result
}

func (b *transferFileBrowser) reload(current string) {
	currentPath, entries, err := b.load(current)
	b.current = currentPath
	b.entries = entries
	b.updateHeader()
	b.table.Clear()

	if err != nil {
		b.table.SetCell(0, 0, tview.NewTableCell(fmt.Sprintf("load error: %v", err)).SetTextColor(tcell.ColorRed))
		return
	}

	for i, entry := range entries {
		b.table.SetCell(i, 0, b.entryCell(entry))
	}

	if len(entries) > 0 {
		b.selecting = true
		b.table.Select(0, 0)
		b.selecting = false
	}
}

func (b *transferFileBrowser) entryCell(entry transferFileEntry) *tview.TableCell {
	mark := "  "
	selected := false
	if _, ok := b.selected[entry.Path]; ok {
		mark = "x "
		selected = true
	}

	label := entry.Name
	if entry.IsDir && label != ".." {
		label += "/"
	}

	cell := tview.NewTableCell(mark + label).SetExpansion(1)
	if selected {
		cell.SetBackgroundColor(tcell.ColorTeal)
		cell.SetTextColor(tcell.ColorBlack)
		cell.SetSelectedStyle(tcell.StyleDefault.Foreground(tcell.ColorBlack).Background(tcell.ColorGreen))
		return cell
	}
	cell.SetTextColor(tcell.ColorWhite)
	cell.SetSelectedStyle(tcell.StyleDefault.Foreground(tcell.ColorBlack).Background(tcell.ColorGreen))
	return cell
}

func (b *transferFileBrowser) refreshEntries() {
	for i, entry := range b.entries {
		b.table.SetCell(i, 0, b.entryCell(entry))
	}
	if b.onChange != nil {
		b.onChange(b.selectedPaths())
	}
}

func (b *transferFileBrowser) handleInput(event *tcell.EventKey) *tcell.EventKey {
	if event == nil {
		return nil
	}
	switch event.Key() {
	case tcell.KeyTab:
		if b.onFocusNav != nil {
			b.onFocusNav(1)
			return nil
		}
	case tcell.KeyBacktab:
		if b.onFocusNav != nil {
			b.onFocusNav(-1)
			return nil
		}
	case tcell.KeyEsc, tcell.KeyCtrlC:
		if b.onCancel != nil {
			b.onCancel()
		}
		return nil
	case tcell.KeyLeft:
		if event.Modifiers()&tcell.ModShift != 0 && b.onTabNav != nil {
			b.onTabNav(-1)
			return nil
		}
		b.goParent()
		return nil
	case tcell.KeyRight:
		if event.Modifiers()&tcell.ModShift != 0 && b.onTabNav != nil {
			b.onTabNav(1)
			return nil
		}
		b.enterCurrentDir()
		return nil
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		b.goParent()
		return nil
	case tcell.KeyEnter:
		b.activateCurrent(false)
		return nil
	case tcell.KeyRune:
		if event.Rune() == ' ' {
			b.activateCurrent(true)
			return nil
		}
	}
	return event
}

func (b *transferFileBrowser) goParent() {
	if b.current == "" {
		return
	}
	next := parentPathOf(b.current)
	if next == b.current {
		return
	}
	b.reload(next)
}

func (b *transferFileBrowser) enterCurrentDir() {
	row, _ := b.table.GetSelection()
	if row < 0 || row >= len(b.entries) {
		return
	}
	entry := b.entries[row]
	if entry.Name == ".." || entry.IsDir {
		b.reload(entry.Path)
	}
}

func (b *transferFileBrowser) activateCurrent(toggle bool) {
	row, _ := b.table.GetSelection()
	if row < 0 || row >= len(b.entries) {
		return
	}

	entry := b.entries[row]
	if entry.Name == ".." {
		b.reload(entry.Path)
		return
	}
	if entry.IsDir && !toggle {
		b.reload(entry.Path)
		return
	}
	if b.onSelectPath != nil {
		b.onSelectPath(entry.Path)
		return
	}

	if _, ok := b.selected[entry.Path]; ok {
		delete(b.selected, entry.Path)
	} else {
		b.selected[entry.Path] = entry
	}
	b.refreshEntries()
}

func loadLocalEntries(current string) (string, []transferFileEntry, error) {
	if current == "" {
		wd, err := os.Getwd()
		if err != nil {
			return "", nil, err
		}
		current = wd
	}
	current = filepath.Clean(current)

	entries := []transferFileEntry{}
	parent := filepath.Dir(current)
	if parent != current {
		entries = append(entries, transferFileEntry{Name: "..", Path: parent, IsDir: true})
	}

	contents, err := os.ReadDir(current)
	if err != nil {
		return current, nil, err
	}
	for _, entry := range sortDirEntries(contents) {
		entries = append(entries, transferFileEntry{
			Name:  entry.Name(),
			Path:  filepath.Join(current, entry.Name()),
			IsDir: entry.IsDir(),
		})
	}

	return current, entries, nil
}

func loadRemoteEntries(client *sftp.Client, current string) (string, []transferFileEntry, error) {
	if client == nil {
		return "", nil, fmt.Errorf("sftp unavailable")
	}
	if current == "" {
		wd, err := client.Getwd()
		if err != nil {
			return "", nil, err
		}
		current = wd
	}
	current, err := resolveRemotePath(client, current)
	if err != nil {
		return "", nil, err
	}

	entries := []transferFileEntry{}
	parent := path.Dir(current)
	if parent != current {
		entries = append(entries, transferFileEntry{Name: "..", Path: parent, IsDir: true})
	}

	contents, err := client.ReadDir(current)
	if err != nil {
		return current, nil, err
	}
	sort.Slice(contents, func(i, j int) bool {
		if contents[i].IsDir() != contents[j].IsDir() {
			return contents[i].IsDir()
		}
		return contents[i].Name() < contents[j].Name()
	})
	for _, entry := range contents {
		entries = append(entries, transferFileEntry{
			Name:  entry.Name(),
			Path:  path.Join(current, entry.Name()),
			IsDir: entry.IsDir(),
		})
	}

	return current, entries, nil
}

func sortDirEntries(entries []os.DirEntry) []os.DirEntry {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir() != entries[j].IsDir() {
			return entries[i].IsDir()
		}
		return entries[i].Name() < entries[j].Name()
	})
	return entries
}

func parentPathOf(current string) string {
	if strings.Contains(current, "/") && !strings.Contains(current, `\`) {
		return path.Dir(current)
	}
	return filepath.Dir(current)
}

func resolveRemotePath(client *sftp.Client, raw string) (string, error) {
	if raw == "" {
		return client.Getwd()
	}
	switch {
	case raw == "~":
		return client.Getwd()
	case strings.HasPrefix(raw, "~/"):
		wd, err := client.Getwd()
		if err != nil {
			return "", err
		}
		return path.Join(wd, raw[2:]), nil
	case !path.IsAbs(raw):
		wd, err := client.Getwd()
		if err != nil {
			return "", err
		}
		return path.Join(wd, raw), nil
	default:
		return path.Clean(raw), nil
	}
}

func styleTransferForm(form *tview.Form) {
	if form == nil {
		return
	}
	form.SetLabelColor(tcell.ColorYellow)
	form.SetFieldBackgroundColor(tcell.ColorDefault)
	form.SetFieldTextColor(tcell.ColorWhite)
	form.SetButtonStyle(tcell.StyleDefault.Background(tcell.ColorTeal).Foreground(tcell.ColorBlack))
	form.SetButtonActivatedStyle(tcell.StyleDefault.Background(tcell.ColorGreen).Foreground(tcell.ColorBlack))
}

func styleTransferDropDown(dropdown *tview.DropDown) {
	if dropdown == nil {
		return
	}
	dropdown.SetLabelColor(tcell.ColorYellow)
	dropdown.SetFieldBackgroundColor(tcell.ColorDefault)
	dropdown.SetFieldTextColor(tcell.ColorWhite)
	dropdown.SetPrefixTextColor(tcell.ColorYellow)
	dropdown.SetListStyles(
		tcell.StyleDefault.Background(tcell.ColorDefault).Foreground(tcell.ColorWhite),
		tcell.StyleDefault.Background(tcell.ColorGreen).Foreground(tcell.ColorBlack),
	)
}
