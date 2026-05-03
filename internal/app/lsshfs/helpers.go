package lsshfs

import (
	"fmt"

	"github.com/blacknon/lssh/internal/app/apputil"
	conf "github.com/blacknon/lssh/internal/config"
)

func selectMountHost(flagHosts, names []string, data conf.Config, specHost string) (selectedHost string, appendHostFlag bool, err error) {
	if len(flagHosts) > 1 {
		return "", false, fmt.Errorf("lsshfs only supports a single host")
	}

	switch {
	case specHost != "" && len(flagHosts) == 1 && specHost != flagHosts[0]:
		return "", false, fmt.Errorf("host in remote path and --host do not match")
	case specHost != "":
		selected, err := apputil.SelectOperationHosts(
			[]string{specHost},
			names,
			names,
			data,
			"shell",
			"no servers matched the current config conditions",
			"input server not found from list",
			"lsshfs>>",
			false,
			apputil.PromptServerSelection,
		)
		if err != nil {
			return "", false, err
		}
		return selected[0], false, nil
	case len(flagHosts) == 1:
		selected, err := apputil.SelectOperationHosts(
			flagHosts,
			names,
			names,
			data,
			"shell",
			"no servers matched the current config conditions",
			"input server not found from list",
			"lsshfs>>",
			false,
			apputil.PromptServerSelection,
		)
		if err != nil {
			return "", false, err
		}
		return selected[0], false, nil
	default:
		selected, err := apputil.SelectOperationHosts(
			nil,
			names,
			names,
			data,
			"shell",
			"no servers matched the current config conditions",
			"input server not found from list",
			"lsshfs>>",
			false,
			apputil.PromptServerSelection,
		)
		if err != nil {
			return "", false, err
		}
		return selected[0], true, nil
	}
}
