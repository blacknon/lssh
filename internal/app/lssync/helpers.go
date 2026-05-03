package lssync

import (
	conf "github.com/blacknon/lssh/internal/config"
	"github.com/blacknon/lssh/internal/core/apputil"
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
	sourcePaths := make([]apputil.TransferPathSpec, 0, len(sourceSpecs))
	for _, spec := range sourceSpecs {
		sourcePaths = append(sourcePaths, apputil.TransferPathSpec{
			IsRemote: spec.IsRemote,
			Path:     spec.Path,
		})
	}
	preparedFrom, err := apputil.PrepareTransferSourcePaths(sourcePaths)
	if err != nil {
		return nil, err
	}
	for _, from := range preparedFrom {
		s.From.IsRemote = from.IsRemote
		s.From.Path = append(s.From.Path, from.Path)
		s.From.DisplayPath = append(s.From.DisplayPath, from.DisplayPath)
	}
	s.From.Server = fromServer

	preparedTo, err := apputil.PrepareTransferDestinationPath(apputil.TransferPathSpec{
		IsRemote: isToRemote,
		Path:     targetSpec.Path,
	})
	if err != nil {
		return nil, err
	}
	s.To.IsRemote = preparedTo.IsRemote
	s.To.Path = []string{preparedTo.Path}
	s.To.DisplayPath = []string{preparedTo.DisplayPath}
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
