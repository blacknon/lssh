package pshell

import (
	"fmt"
	"strings"

	"github.com/blacknon/lssh/internal/common"
)

func knownHostsFromConnects(connects []*sConnect) []string {
	hosts := make([]string, 0, len(connects))
	for _, connect := range connects {
		if connect == nil || strings.TrimSpace(connect.Name) == "" {
			continue
		}
		hosts = append(hosts, connect.Name)
	}

	return hosts
}

func parseTargetedCommand(command string, knownHosts []string) ([]string, string, error) {
	if !isTargetedRemoteCommand(command) {
		return nil, "", fmt.Errorf("not targeted command")
	}

	targets, commandHead := common.ParseHostPathWithHosts(strings.TrimPrefix(command, "@"), knownHosts)
	commandHead = strings.TrimSpace(commandHead)
	if len(targets) == 0 || commandHead == "" {
		return nil, "", fmt.Errorf("invalid server selector")
	}

	for i, target := range targets {
		target = strings.TrimSpace(target)
		if target == "" {
			return nil, "", fmt.Errorf("invalid server selector")
		}
		targets[i] = target
	}

	return targets, commandHead, nil
}
