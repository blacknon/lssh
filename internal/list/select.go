// Copyright (c) 2026 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package list

import (
	conf "github.com/blacknon/lssh/internal/config"
)

// SelectHosts displays the selector UI and returns selected hosts.
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

	return l.selectWithTview()
}
