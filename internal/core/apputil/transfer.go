package apputil

import (
	"errors"
	"fmt"

	conf "github.com/blacknon/lssh/internal/config"
)

func ValidateTransferCopyTypes(isFromInRemote, isFromInLocal, isToRemote bool, countHosts int) error {
	if isFromInRemote && isFromInLocal {
		return fmt.Errorf("Can not set LOCAL and REMOTE to FROM path.")
	}
	if !isFromInRemote && !isToRemote {
		return fmt.Errorf("It does not correspond LOCAL to LOCAL copy.")
	}
	if isFromInRemote && isToRemote && countHosts != 0 {
		return fmt.Errorf("In the case of REMOTE to REMOTE copy, it does not correspond to host option.")
	}

	return nil
}

type TransferHostSelectionOptions struct {
	Operation          string
	EmptyMessage       string
	UnsupportedMessage string
	SinglePrompt       string
	FromPrompt         string
	ToPrompt           string
	Prompt             ServerSelectionPrompt
}

func SelectTransferHosts(hosts, allNames, names []string, data conf.Config, explicitSourceHosts, explicitTargetHosts []string, isFromInRemote, isToRemote bool, options TransferHostSelectionOptions) (fromServer, toServer []string, err error) {
	switch {
	case len(hosts) != 0:
		selected, selectErr := SelectOperationHosts(
			hosts,
			allNames,
			names,
			data,
			options.Operation,
			options.EmptyMessage,
			options.UnsupportedMessage,
			options.SinglePrompt,
			true,
			options.Prompt,
		)
		if selectErr != nil {
			return nil, nil, selectErr
		}
		if isFromInRemote {
			fromServer = selected
		} else {
			toServer = selected
		}
	case len(explicitSourceHosts) > 0 || len(explicitTargetHosts) > 0:
		if len(explicitSourceHosts) > 0 {
			selected, selectErr := SelectOperationHosts(
				explicitSourceHosts,
				allNames,
				names,
				data,
				options.Operation,
				options.EmptyMessage,
				options.UnsupportedMessage,
				options.SinglePrompt,
				true,
				options.Prompt,
			)
			if selectErr != nil {
				return nil, nil, selectErr
			}
			fromServer = append(fromServer, selected...)
		}
		if len(explicitTargetHosts) > 0 {
			selected, selectErr := SelectOperationHosts(
				explicitTargetHosts,
				allNames,
				names,
				data,
				options.Operation,
				options.EmptyMessage,
				options.UnsupportedMessage,
				options.SinglePrompt,
				true,
				options.Prompt,
			)
			if selectErr != nil {
				return nil, nil, selectErr
			}
			toServer = append(toServer, selected...)
		}
	case isFromInRemote && isToRemote:
		if len(names) == 0 {
			return nil, nil, errors.New(options.EmptyMessage)
		}

		fromServer, err = options.Prompt(options.FromPrompt, names, data, false)
		if err != nil {
			return nil, nil, err
		}
		if len(fromServer) == 0 || fromServer[0] == "ServerName" {
			return nil, nil, fmt.Errorf("Selection cancelled.")
		}

		toServer, err = options.Prompt(options.ToPrompt, names, data, true)
		if err != nil {
			return nil, nil, err
		}
		if len(toServer) == 0 || toServer[0] == "ServerName" {
			return nil, nil, fmt.Errorf("Selection cancelled.")
		}
	default:
		selected, selectErr := SelectOperationHosts(
			nil,
			allNames,
			names,
			data,
			options.Operation,
			options.EmptyMessage,
			options.UnsupportedMessage,
			options.SinglePrompt,
			true,
			options.Prompt,
		)
		if selectErr != nil {
			return nil, nil, selectErr
		}
		if isFromInRemote {
			fromServer = selected
		} else {
			toServer = selected
		}
	}

	return fromServer, toServer, nil
}
