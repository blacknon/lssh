package lssync

import (
	"fmt"
	"os"

	"github.com/blacknon/lssh/internal/app/apputil"
	"github.com/blacknon/lssh/internal/check"
	"github.com/blacknon/lssh/internal/common"
	conf "github.com/blacknon/lssh/internal/config"
	lsync "github.com/blacknon/lssh/internal/sync"
	"github.com/urfave/cli"
)

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

func selectSyncServers(hosts, names, allNames []string, data conf.Config, explicitSourceHosts []string, targetSpec lsync.PathSpec, isFromInRemote, isToRemote bool, prompt apputil.ServerSelectionPrompt) (fromServer, toServer []string, err error) {
	return apputil.SelectTransferHosts(
		hosts,
		allNames,
		names,
		data,
		explicitSourceHosts,
		targetSpec.Hosts,
		isFromInRemote,
		isToRemote,
		apputil.TransferHostSelectionOptions{
			Operation:          "sftp_transport",
			EmptyMessage:       "No servers matched the current config conditions.",
			UnsupportedMessage: "Selected host does not support SFTP-based transfer.",
			SinglePrompt:       "lssync>>",
			FromPrompt:         "lssync(from)>>",
			ToPrompt:           "lssync(to)>>",
			Prompt:             prompt,
		},
	)
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
