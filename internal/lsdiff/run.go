// Copyright (c) 2026 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package lsdiff

import (
	"fmt"
	"sort"

	"github.com/blacknon/lssh/internal/check"
	conf "github.com/blacknon/lssh/internal/config"
	"github.com/blacknon/lssh/internal/list"
)

func ResolveTargets(config conf.Config, args []string) ([]Target, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("lsdiff requires at least one remote path")
	}

	explicitTargets := make([]Target, 0, len(args))
	for _, arg := range args {
		target, err := ParseTargetSpec(arg)
		if err != nil {
			return nil, err
		}
		explicitTargets = append(explicitTargets, target)
	}

	allExplicit := true
	for _, target := range explicitTargets {
		if target.Host == "" {
			allExplicit = false
			break
		}
	}

	if allExplicit {
		if len(explicitTargets) < 2 {
			return nil, fmt.Errorf("lsdiff requires at least two targets to compare")
		}
		return explicitTargets, nil
	}

	if len(explicitTargets) != 1 || explicitTargets[0].Host != "" {
		return nil, fmt.Errorf("use a single common remote path or explicit @host:/path targets")
	}

	names := conf.GetNameList(config)
	names, err := config.FilterServersByOperation(names, "sftp_transport")
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	if len(names) == 0 {
		return nil, fmt.Errorf("no servers matched the current config conditions")
	}
	l := &list.ListInfo{
		Prompt:    "lsdiff>>",
		NameList:  names,
		DataList:  config,
		MultiFlag: true,
	}
	l.View()
	if len(l.SelectName) < 2 {
		return nil, fmt.Errorf("select at least two hosts")
	}
	if !check.ExistServer(l.SelectName, names) {
		return nil, fmt.Errorf("selected host not found from list")
	}

	targets := make([]Target, 0, len(l.SelectName))
	for _, host := range l.SelectName {
		targets = append(targets, Target{
			Host:       host,
			RemotePath: explicitTargets[0].RemotePath,
			Title:      host + ":" + explicitTargets[0].RemotePath,
		})
	}

	return targets, nil
}
