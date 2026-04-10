// Copyright (c) 2026 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package pshell

import (
	"fmt"
	"sort"
	"strings"

	"github.com/c-bata/go-prompt"
)

func (s *shell) expandAliases(pslice [][]pipeLine) [][]pipeLine {
	if len(s.Config.Alias) == 0 {
		return pslice
	}

	result := make([][]pipeLine, 0, len(pslice))
	for _, pline := range pslice {
		result = append(result, s.expandPipeLineAliases(pline))
	}

	return result
}

func (s *shell) expandPipeLineAliases(pline []pipeLine) []pipeLine {
	result := make([]pipeLine, 0, len(pline))

	for _, p := range pline {
		cloned := pipeLine{
			Args:    append([]string{}, p.Args...),
			Oprator: p.Oprator,
		}
		if len(cloned.Args) == 0 {
			result = append(result, cloned)
			continue
		}

		command := cloned.Args[0]
		if checkLocalBuildInCommand(command) {
			result = append(result, cloned)
			continue
		}

		targetPrefix := ""
		aliasName := command
		if isTargetedRemoteCommand(command) {
			targets, cmd, _, inSelector := parseLeadingTargetSelector(command)
			if inSelector || cmd == "" {
				result = append(result, cloned)
				continue
			}
			targetPrefix = "@" + strings.Join(targets, ",") + ":"
			aliasName = cmd
		}

		aliasArgs, ok := s.aliasArgs(aliasName)
		if !ok || len(aliasArgs) == 0 {
			result = append(result, cloned)
			continue
		}

		expanded := make([]string, 0, len(aliasArgs)+len(cloned.Args)-1)
		first := aliasArgs[0]
		if targetPrefix != "" {
			first = targetPrefix + first
		}
		expanded = append(expanded, first)
		expanded = append(expanded, aliasArgs[1:]...)
		expanded = append(expanded, cloned.Args[1:]...)
		cloned.Args = expanded

		result = append(result, cloned)
	}

	return result
}

func (s *shell) aliasArgs(name string) ([]string, bool) {
	cfg, ok := s.Config.Alias[name]
	if !ok || strings.TrimSpace(cfg.Command) == "" {
		return nil, false
	}

	pslice, err := parsePipeLine(cfg.Command)
	if err != nil || len(pslice) != 1 || len(pslice[0]) != 1 || len(pslice[0][0].Args) == 0 {
		return nil, false
	}

	expanded := append([]string{}, pslice[0][0].Args...)
	if checkLocalBuildInCommand(expanded[0]) || isTargetedRemoteCommand(expanded[0]) {
		return nil, false
	}

	return expanded, true
}

func (s *shell) aliasSuggests() []prompt.Suggest {
	if len(s.Config.Alias) == 0 {
		return nil
	}

	keys := make([]string, 0, len(s.Config.Alias))
	for key := range s.Config.Alias {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	suggests := make([]prompt.Suggest, 0, len(keys))
	for _, key := range keys {
		cfg := s.Config.Alias[key]
		suggests = append(suggests, prompt.Suggest{
			Text:        key,
			Description: fmt.Sprintf("Alias. command:%s", cfg.Command),
		})
	}

	return suggests
}
