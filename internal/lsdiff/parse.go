// Copyright (c) 2026 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package lsdiff

import (
	"fmt"
	"strings"
)

func ParseTargetSpec(value string) (Target, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return Target{}, fmt.Errorf("target path is required")
	}

	if strings.HasPrefix(value, "@") {
		pair := strings.SplitN(value[1:], ":", 2)
		if len(pair) != 2 || strings.TrimSpace(pair[0]) == "" || strings.TrimSpace(pair[1]) == "" {
			return Target{}, fmt.Errorf("invalid target format: expected @host:/path")
		}

		host := strings.TrimSpace(pair[0])
		path := strings.TrimSpace(pair[1])
		return Target{
			Host:       host,
			RemotePath: path,
			Title:      host + ":" + path,
		}, nil
	}

	if strings.Contains(value, ":") {
		pair := strings.SplitN(value, ":", 2)
		if len(pair) == 2 && strings.TrimSpace(pair[0]) != "" && strings.HasPrefix(strings.TrimSpace(pair[1]), "/") {
			host := strings.TrimSpace(pair[0])
			path := strings.TrimSpace(pair[1])
			return Target{
				Host:       host,
				RemotePath: path,
				Title:      host + ":" + path,
			}, nil
		}
	}

	return Target{
		RemotePath: value,
		Title:      value,
	}, nil
}
