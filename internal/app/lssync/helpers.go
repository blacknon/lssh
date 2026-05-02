package lssync

import (
	"fmt"
	"os"

	"github.com/blacknon/lssh/internal/check"
	"github.com/blacknon/lssh/internal/common"
	conf "github.com/blacknon/lssh/internal/config"
	"github.com/blacknon/lssh/internal/list"
	lsync "github.com/blacknon/lssh/internal/sync"
	"github.com/urfave/cli"
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

func validateCopyTypes(isFromInRemote, isFromInLocal, isToRemote bool, countHosts int) error {
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

func parseSyncSpecs(fromArgs []string, toArg string, allNames []string) (sourceSpecs []lsync.PathSpec, targetSpec lsync.PathSpec, isFromInRemote, isFromInLocal bool, explicitSourceHosts []string, err error) {
	sourceSpecs = make([]lsync.PathSpec, 0, len(fromArgs))
	for _, from := range fromArgs {
		spec, parseErr := lsync.ParsePathSpecWithHosts(from, allNames)
		if parseErr != nil {
			err = parseErr
			return
		}
		if spec.IsRemote {
			isFromInRemote = true
			explicitSourceHosts = appendUniqueStrings(explicitSourceHosts, spec.Hosts...)
		} else {
			isFromInLocal = true
		}
		sourceSpecs = append(sourceSpecs, spec)
	}

	targetSpec, err = lsync.ParsePathSpecWithHosts(toArg, allNames)
	return
}

func selectSyncServers(hosts, names, allNames []string, data conf.Config, explicitSourceHosts []string, targetSpec lsync.PathSpec, isFromInRemote, isToRemote bool, prompt serverSelectionPrompt) (fromServer, toServer []string, err error) {
	switch {
	case len(hosts) != 0:
		filteredHosts, filterErr := data.FilterServersByOperation(hosts, "sftp_transport")
		if filterErr != nil {
			return nil, nil, filterErr
		}
		if !check.ExistServer(hosts, allNames) {
			return nil, nil, fmt.Errorf("Input Server not found from list.")
		}
		if len(filteredHosts) != len(hosts) {
			return nil, nil, fmt.Errorf("Input Server does not support SFTP-based transfer.")
		}
		if isFromInRemote {
			fromServer = append(fromServer, hosts...)
		} else {
			toServer = append(toServer, hosts...)
		}
	case len(explicitSourceHosts) > 0 || (targetSpec.IsRemote && len(targetSpec.Hosts) > 0):
		filteredSourceHosts, filterErr := data.FilterServersByOperation(explicitSourceHosts, "sftp_transport")
		if filterErr != nil {
			return nil, nil, filterErr
		}
		filteredTargetHosts, filterErr := data.FilterServersByOperation(targetSpec.Hosts, "sftp_transport")
		if filterErr != nil {
			return nil, nil, filterErr
		}
		if len(filteredSourceHosts) != len(explicitSourceHosts) || len(filteredTargetHosts) != len(targetSpec.Hosts) {
			return nil, nil, fmt.Errorf("Selected host does not support SFTP-based transfer.")
		}
		fromServer = append(fromServer, explicitSourceHosts...)
		toServer = append(toServer, targetSpec.Hosts...)
	case isFromInRemote && isToRemote:
		if len(names) == 0 {
			return nil, nil, fmt.Errorf("No servers matched the current config conditions.")
		}
		fromServer, err = prompt("lssync(from)>>", names, data, false)
		if err != nil {
			return nil, nil, err
		}
		if len(fromServer) == 0 || fromServer[0] == "ServerName" {
			return nil, nil, fmt.Errorf("Selection cancelled.")
		}

		toServer, err = prompt("lssync(to)>>", names, data, true)
		if err != nil {
			return nil, nil, err
		}
		if len(toServer) == 0 || toServer[0] == "ServerName" {
			return nil, nil, fmt.Errorf("Selection cancelled.")
		}
	default:
		if len(names) == 0 {
			return nil, nil, fmt.Errorf("No servers matched the current config conditions.")
		}
		selected, promptErr := prompt("lssync>>", names, data, true)
		if promptErr != nil {
			return nil, nil, promptErr
		}
		if len(selected) == 0 || selected[0] == "ServerName" {
			return nil, nil, fmt.Errorf("Selection cancelled.")
		}

		if isFromInRemote {
			fromServer = selected
		} else {
			toServer = selected
		}
	}

	return fromServer, toServer, nil
}

func buildSync(c *cli.Context, data conf.Config, sourceSpecs []lsync.PathSpec, targetSpec lsync.PathSpec, fromServer, toServer []string, isToRemote bool, controlMasterOverride *bool) (*lsync.Sync, error) {
	s := new(lsync.Sync)
	for _, spec := range sourceSpecs {
		fromPath := spec.Path
		displayFromPath := spec.Path
		if !spec.IsRemote {
			fullPath := common.GetFullPath(fromPath)
			if _, err := os.Stat(fullPath); err != nil {
				return nil, fmt.Errorf("not found path %s", fromPath)
			}
			fromPath = fullPath
		} else {
			fromPath = check.EscapePath(fromPath)
		}

		s.From.IsRemote = spec.IsRemote
		s.From.Path = append(s.From.Path, fromPath)
		s.From.DisplayPath = append(s.From.DisplayPath, displayFromPath)
	}
	s.From.Server = fromServer

	displayToPath := targetSpec.Path
	if isToRemote {
		targetSpec.Path = check.EscapePath(targetSpec.Path)
	}
	s.To.IsRemote = isToRemote
	s.To.Path = []string{targetSpec.Path}
	s.To.DisplayPath = []string{displayToPath}
	s.To.Server = toServer
	s.Daemon = c.Bool("daemon")
	s.DaemonInterval = c.Duration("daemon-interval")
	s.Bidirectional = c.Bool("bidirectional")
	s.Permission = c.Bool("permission")
	s.DryRun = c.Bool("dry-run")
	s.Delete = c.Bool("delete")
	s.ParallelNum = c.Int("parallel")
	s.Config = data
	s.ControlMasterOverride = controlMasterOverride

	return s, nil
}
