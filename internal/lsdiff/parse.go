// Copyright (c) 2026 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package lsdiff

import (
	"fmt"
	"strings"

	"github.com/blacknon/lssh/internal/common"
)

func ParseTargetSpec(value string) (Target, error) {
	return ParseTargetSpecWithHosts(value, nil)
}

func ParseTargetSpecWithHosts(value string, knownHosts []string) (Target, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return Target{}, fmt.Errorf("target path is required")
	}

	if strings.HasPrefix(value, "@") {
		hosts, path := common.ParseHostPathWithHosts(strings.TrimPrefix(value, "@"), knownHosts)
		if len(hosts) != 1 || strings.TrimSpace(path) == "" {
			return Target{}, fmt.Errorf("invalid target format: expected @host:/path")
		}

		host := strings.TrimSpace(hosts[0])
		path = strings.TrimSpace(path)
		return Target{
			Host:       host,
			RemotePath: path,
			Title:      host + ":" + path,
		}, nil
	}

	if strings.Contains(value, ":") {
		hosts, path := common.ParseHostPathWithHosts(value, knownHosts)
		if len(hosts) == 1 && strings.TrimSpace(hosts[0]) != "" && strings.HasPrefix(strings.TrimSpace(path), "/") {
			host := strings.TrimSpace(hosts[0])
			path = strings.TrimSpace(path)
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
