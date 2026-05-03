package apputil

import (
	"errors"
	"fmt"

	"github.com/blacknon/lssh/internal/check"
	conf "github.com/blacknon/lssh/internal/config"
	"github.com/blacknon/lssh/internal/core/list"
)

type ServerSelectionPrompt func(prompt string, names []string, data conf.Config, isMulti bool) ([]string, error)

func PromptServerSelection(prompt string, names []string, data conf.Config, isMulti bool) ([]string, error) {
	l := new(list.ListInfo)
	l.Prompt = prompt
	l.NameList = names
	l.DataList = data
	l.MultiFlag = isMulti
	l.View()

	return l.SelectName, nil
}

func SelectOperationHosts(hosts, allNames, names []string, data conf.Config, operation, emptyMessage, unsupportedMessage, promptName string, isMulti bool, prompt ServerSelectionPrompt) ([]string, error) {
	if len(hosts) > 0 {
		filteredHosts, err := data.FilterServersByOperation(hosts, operation)
		if err != nil {
			return nil, err
		}
		if !check.ExistServer(hosts, allNames) {
			return nil, fmt.Errorf("Input Server not found from list.")
		}
		if len(filteredHosts) != len(hosts) {
			return nil, errors.New(unsupportedMessage)
		}
		return hosts, nil
	}

	if len(names) == 0 {
		return nil, errors.New(emptyMessage)
	}

	selected, err := prompt(promptName, names, data, isMulti)
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
