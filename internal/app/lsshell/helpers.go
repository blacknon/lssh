package lsshell

import (
	"fmt"
	"regexp"
	"runtime"

	"github.com/blacknon/lssh/internal/check"
	"github.com/blacknon/lssh/internal/common"
	conf "github.com/blacknon/lssh/internal/config"
	"github.com/blacknon/lssh/internal/core/list"
	sshcmd "github.com/blacknon/lssh/internal/ssh"
	"github.com/urfave/cli"
	"golang.org/x/crypto/ssh/terminal"
)

var numericPortPattern = regexp.MustCompile(`^[0-9]+$`)

type serverSelectionPrompt func(names []string, data conf.Config, isMulti bool) ([]string, error)

func promptServerSelection(names []string, data conf.Config, isMulti bool) ([]string, error) {
	l := new(list.ListInfo)
	l.Prompt = "lssh>>"
	l.NameList = names
	l.DataList = data
	l.MultiFlag = isMulti
	l.View()

	return l.SelectName, nil
}

func selectServers(hosts, names []string, data conf.Config, isMulti bool, prompt serverSelectionPrompt) ([]string, error) {
	if len(hosts) > 0 {
		if !check.ExistServer(hosts, names) {
			return nil, fmt.Errorf("Input Server not found from list.")
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

	return selected, nil
}

func buildRun(c *cli.Context, data conf.Config, selected []string, controlMasterOverride *bool) (*sshcmd.Run, error) {
	run := &sshcmd.Run{
		ServerList: selected,
		Conf:       data,
		RunSessionConfig: sshcmd.RunSessionConfig{
			ControlMasterOverride: controlMasterOverride,
		},
		RunCommandConfig: sshcmd.RunCommandConfig{
			IsTerm: c.Bool("term"),
		},
	}

	var (
		err            error
		forwards       []*conf.PortForward
		reverseDynamic string
	)
	for _, forwardargs := range c.StringSlice("R") {
		if numericPortPattern.MatchString(forwardargs) {
			reverseDynamic = forwardargs
			continue
		}

		f := new(conf.PortForward)
		f.Mode = "R"
		f.Local, f.Remote, err = common.ParseForwardPort(forwardargs)
		if err != nil {
			return nil, err
		}
		forwards = append(forwards, f)
	}

	run.PortForward = forwards
	run.ReverseDynamicPortForward = reverseDynamic
	run.HTTPReverseDynamicPortForward = c.String("r")
	if nfsReverseForwarding := c.String("m"); nfsReverseForwarding != "" {
		port, path, err := common.ParseNFSForwardPortPath(nfsReverseForwarding)
		if err != nil {
			return nil, err
		}
		run.NFSReverseDynamicForwardPort = port
		run.NFSReverseDynamicForwardPath = common.GetFullPath(path)
	}

	if runtime.GOOS != "windows" && !terminal.IsTerminal(0) {
		run.IsStdinPipe = true
	}

	return run, nil
}
