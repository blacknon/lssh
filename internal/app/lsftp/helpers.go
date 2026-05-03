package lsftp

import (
	"fmt"

	"github.com/blacknon/lssh/internal/check"
	conf "github.com/blacknon/lssh/internal/config"
	"github.com/blacknon/lssh/internal/list"
)

type serverSelectionPrompt func(prompt string, names []string, data conf.Config, isMulti bool) ([]string, error)

func promptServerSelection(prompt string, names []string, data conf.Config, isMulti bool) ([]string, error) {
	l := new(list.ListInfo)
	l.Prompt = prompt
	l.NameList = names
	l.DataList = data
	l.MultiFlag = isMulti
	l.View()

	return l.SelectName, nil
}

func selectSFTPServers(hosts, allNames, names []string, data conf.Config, prompt serverSelectionPrompt) ([]string, error) {
	if len(hosts) > 0 {
		filteredHosts, err := data.FilterServersByOperation(hosts, "sftp_transport")
		if err != nil {
			return nil, err
		}
		if !check.ExistServer(hosts, allNames) {
			return nil, fmt.Errorf("Input Server not found from list.")
		}
		if len(filteredHosts) != len(hosts) {
			return nil, fmt.Errorf("Input Server does not support SFTP-based transfer.")
		}
		return hosts, nil
	}

	if len(names) == 0 {
		return nil, fmt.Errorf("No servers matched the current config conditions.")
	}

	selected, err := prompt("lsftp>>", names, data, true)
	if err != nil {
		return nil, err
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("Selection cancelled.")
	}
	if selected[0] == "ServerName" {
		return nil, fmt.Errorf("Server not selected.")
	}

	return selected, nil
}
