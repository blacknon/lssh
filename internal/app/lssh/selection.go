package lssh

import (
	"fmt"

	"github.com/blacknon/lssh/internal/check"
	conf "github.com/blacknon/lssh/internal/config"
	"github.com/blacknon/lssh/internal/core/list"
)

type serverSelectionPrompt func(names []string, data conf.Config, isMulti bool) ([]string, error)

func validateKnownHosts(hosts, names []string) error {
	if len(hosts) == 0 {
		return nil
	}
	if !check.ExistServer(hosts, names) {
		return fmt.Errorf("Input Server not found from list.")
	}
	return nil
}

func promptServerSelection(names []string, data conf.Config, isMulti bool) ([]string, error) {
	l := new(list.ListInfo)
	l.Prompt = "lssh>>"
	l.NameList = names
	l.DataList = data
	l.MultiFlag = isMulti

	l.View()

	return l.SelectName, nil
}

func selectServers(hosts, names []string, data conf.Config, isMulti, background bool, prompt serverSelectionPrompt) ([]string, error) {
	if len(hosts) > 0 {
		if err := validateKnownHosts(hosts, names); err != nil {
			return nil, err
		}
		return hosts, nil
	}

	if len(names) == 0 {
		return nil, fmt.Errorf("No servers matched the current config conditions.")
	}

	selected, err := prompt(names, data, isMulti)
	if err != nil {
		return nil, err
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("Selection cancelled.")
	}
	if selected[0] == "ServerName" {
		return nil, fmt.Errorf("Server not selected.")
	}
	if background && len(selected) > 1 {
		return nil, fmt.Errorf("Error: -f cannot be used with multiple hosts. Select a single host.")
	}

	return selected, nil
}
