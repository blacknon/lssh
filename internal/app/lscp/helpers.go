package lscp

import (
	"fmt"

	"github.com/blacknon/lssh/internal/app/apputil"
	conf "github.com/blacknon/lssh/internal/config"
)

func selectSCPServers(hosts, allNames, names []string, data conf.Config, isFromRemote, isToRemote bool) (fromServer, toServer []string, err error) {
	switch {
	case len(hosts) != 0:
		selected, selectErr := apputil.SelectOperationHosts(
			hosts,
			allNames,
			names,
			data,
			"sftp_transport",
			"No servers matched the current config conditions.",
			"Input Server does not support SFTP-based transfer.",
			"lscp>>",
			true,
			apputil.PromptServerSelection,
		)
		if selectErr != nil {
			return nil, nil, selectErr
		}
		toServer = selected
	case isFromRemote && isToRemote:
		fromServer, err = apputil.PromptServerSelection("lscp(from)>>", names, data, false)
		if err != nil {
			return nil, nil, err
		}
		if len(fromServer) == 0 {
			return nil, nil, fmt.Errorf("Selection cancelled.")
		}
		if fromServer[0] == "ServerName" {
			return nil, nil, fmt.Errorf("Server not selected.")
		}

		toServer, err = apputil.PromptServerSelection("lscp(to)>>", names, data, true)
		if err != nil {
			return nil, nil, err
		}
		if len(toServer) == 0 {
			return nil, nil, fmt.Errorf("Selection cancelled.")
		}
		if toServer[0] == "ServerName" {
			return nil, nil, fmt.Errorf("Server not selected.")
		}
	default:
		selected, selectErr := apputil.SelectOperationHosts(
			nil,
			allNames,
			names,
			data,
			"sftp_transport",
			"No servers matched the current config conditions.",
			"Input Server does not support SFTP-based transfer.",
			"lscp>>",
			true,
			apputil.PromptServerSelection,
		)
		if selectErr != nil {
			return nil, nil, selectErr
		}
		if isFromRemote {
			fromServer = selected
		} else {
			toServer = selected
		}
	}

	return fromServer, toServer, nil
}
