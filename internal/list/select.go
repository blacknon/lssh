// Copyright (c) 2026 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package list

import (
	"fmt"

	conf "github.com/blacknon/lssh/internal/config"
	termbox "github.com/nsf/termbox-go"
)

// SelectHosts displays the classic list UI and returns selected hosts.
// ok is false when the selector was canceled.
func SelectHosts(prompt string, names []string, data conf.Config, multi bool) (selected []string, ok bool, err error) {
	l := &ListInfo{
		Prompt:    prompt,
		NameList:  names,
		DataList:  data,
		MultiFlag: multi,
	}

	l.getText()
	if len(l.DataText) == 1 {
		return nil, false, nil
	}

	if err := termbox.Init(); err != nil {
		return nil, false, fmt.Errorf("termbox init error: %w", err)
	}
	defer termbox.Close()

	termbox.SetInputMode(termbox.InputMouse)
	return l.keyEventSelectable()
}
