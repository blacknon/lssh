package lscp

import (
	conf "github.com/blacknon/lssh/internal/config"
	"github.com/blacknon/lssh/internal/core/apputil"
)

func selectSCPServers(hosts, allNames, names []string, data conf.Config, isFromRemote, isToRemote bool) (fromServer, toServer []string, err error) {
	return apputil.SelectTransferHosts(
		hosts,
		allNames,
		names,
		data,
		nil,
		nil,
		isFromRemote,
		isToRemote,
		apputil.TransferHostSelectionOptions{
			Operation:          "sftp_transport",
			EmptyMessage:       "No servers matched the current config conditions.",
			UnsupportedMessage: "Input Server does not support SFTP-based transfer.",
			SinglePrompt:       "lscp>>",
			FromPrompt:         "lscp(from)>>",
			ToPrompt:           "lscp(to)>>",
			Prompt:             apputil.PromptServerSelection,
		},
	)
}
