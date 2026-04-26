package lspipe

import (
	"fmt"
	"strconv"
	"strings"
)

func ResolveExecHosts(sessionHosts, selectors []string) ([]string, error) {
	if len(selectors) == 0 {
		return nil, nil
	}

	available := make(map[string]struct{}, len(sessionHosts))
	for _, host := range sessionHosts {
		available[host] = struct{}{}
	}

	resolved := make([]string, 0, len(selectors))
	seen := make(map[string]struct{}, len(selectors))
	for _, selector := range selectors {
		host, err := resolveSelector(sessionHosts, strings.TrimSpace(selector))
		if err != nil {
			return nil, err
		}
		if _, ok := available[host]; !ok {
			return nil, fmt.Errorf("host %q is not part of session", host)
		}
		if _, ok := seen[host]; ok {
			continue
		}
		seen[host] = struct{}{}
		resolved = append(resolved, host)
	}

	return resolved, nil
}

func resolveSelector(sessionHosts []string, selector string) (string, error) {
	if selector == "" {
		return "", fmt.Errorf("empty host selector")
	}

	index, err := strconv.Atoi(selector)
	if err == nil {
		if index < 1 || index > len(sessionHosts) {
			return "", fmt.Errorf("host selector index %d is out of range", index)
		}
		return sessionHosts[index-1], nil
	}

	return selector, nil
}
