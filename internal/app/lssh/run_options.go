package lssh

import (
	"fmt"
	"os"
	"regexp"
	"runtime"

	"github.com/blacknon/lssh/internal/common"
	conf "github.com/blacknon/lssh/internal/config"
	sshcmd "github.com/blacknon/lssh/internal/ssh"
	"github.com/urfave/cli"
)

var numericPortPattern = regexp.MustCompile(`^[0-9]+$`)

type runForwardSettings struct {
	PortForward                   []*conf.PortForward
	DynamicPortForward            string
	HTTPDynamicPortForward        string
	ReverseDynamicPortForward     string
	HTTPReverseDynamicPortForward string
	NFSDynamicForwardPort         string
	NFSDynamicForwardPath         string
	NFSReverseDynamicForwardPort  string
	NFSReverseDynamicForwardPath  string
	SMBDynamicForwardPort         string
	SMBDynamicForwardPath         string
	SMBReverseDynamicForwardPort  string
	SMBReverseDynamicForwardPath  string
	TunnelEnabled                 bool
	TunnelLocal                   int
	TunnelRemote                  int
}

func parsePortForwards(localSpecs, remoteSpecs []string) ([]*conf.PortForward, string, error) {
	var (
		err            error
		forwards       []*conf.PortForward
		reverseDynamic string
	)

	for _, forwardargs := range localSpecs {
		f := new(conf.PortForward)
		f.Mode = "L"
		f.LocalNetwork, f.Local, f.RemoteNetwork, f.Remote, err = common.ParseForwardSpec(forwardargs)
		if err != nil {
			return nil, "", err
		}
		forwards = append(forwards, f)
	}

	for _, forwardargs := range remoteSpecs {
		if numericPortPattern.MatchString(forwardargs) {
			reverseDynamic = forwardargs
			continue
		}

		f := new(conf.PortForward)
		f.Mode = "R"
		f.Local, f.Remote, err = common.ParseForwardPort(forwardargs)
		if err != nil {
			return nil, "", err
		}
		forwards = append(forwards, f)
	}

	return forwards, reverseDynamic, nil
}

func parseForwardPathOption(spec string, fullPath bool) (string, string, error) {
	if spec == "" {
		return "", "", nil
	}

	port, path, err := common.ParseNFSForwardPortPath(spec)
	if err != nil {
		return "", "", err
	}
	if fullPath {
		path = common.GetFullPath(path)
	}

	return port, path, nil
}

func parseRunForwardSettings(c *cli.Context) (runForwardSettings, error) {
	settings := runForwardSettings{
		DynamicPortForward:            c.String("D"),
		HTTPDynamicPortForward:        c.String("d"),
		HTTPReverseDynamicPortForward: c.String("r"),
	}

	var err error
	settings.PortForward, settings.ReverseDynamicPortForward, err = parsePortForwards(c.StringSlice("L"), c.StringSlice("R"))
	if err != nil {
		return runForwardSettings{}, err
	}

	settings.NFSDynamicForwardPort, settings.NFSDynamicForwardPath, err = parseForwardPathOption(c.String("M"), false)
	if err != nil {
		return runForwardSettings{}, err
	}
	settings.SMBDynamicForwardPort, settings.SMBDynamicForwardPath, err = parseForwardPathOption(c.String("S"), false)
	if err != nil {
		return runForwardSettings{}, err
	}
	settings.NFSReverseDynamicForwardPort, settings.NFSReverseDynamicForwardPath, err = parseForwardPathOption(c.String("m"), true)
	if err != nil {
		return runForwardSettings{}, err
	}
	settings.SMBReverseDynamicForwardPort, settings.SMBReverseDynamicForwardPath, err = parseForwardPathOption(c.String("s"), true)
	if err != nil {
		return runForwardSettings{}, err
	}

	if enabled, local, remote, err := resolveTunnelOption(runtime.GOOS, c.String("tunnel")); err != nil {
		return runForwardSettings{}, err
	} else if enabled {
		settings.TunnelEnabled = true
		settings.TunnelLocal = local
		settings.TunnelRemote = remote
	}

	return settings, nil
}

func buildRun(c *cli.Context, data conf.Config, selected []string, controlMasterOverride *bool, connectorAttachSession string, connectorDetach, enableX11, enableTrustedX11 bool) (*sshcmd.Run, error) {
	forwardSettings, err := parseRunForwardSettings(c)
	if err != nil {
		return nil, err
	}

	r := &sshcmd.Run{
		ServerList: selected,
		Conf:       data,
		RunSessionConfig: sshcmd.RunSessionConfig{
			ControlMasterOverride:  controlMasterOverride,
			ConnectorAttachSession: connectorAttachSession,
			ConnectorDetach:        connectorDetach,
			X11:                    enableX11 || enableTrustedX11,
			X11Trusted:             enableTrustedX11,
			IsBashrc:               c.Bool("localrc"),
			IsNotBashrc:            c.Bool("not-localrc"),
		},
		RunCommandConfig: sshcmd.RunCommandConfig{
			Mode:          "shell",
			IsTerm:        c.Bool("term"),
			IsParallel:    c.Bool("parallel"),
			IsNone:        c.Bool("not-execute"),
			ExecCmd:       c.Args(),
			EnableHeader:  c.Bool("w"),
			DisableHeader: c.Bool("W"),
		},
		RunForwardConfig: sshcmd.RunForwardConfig{
			PortForward:                   forwardSettings.PortForward,
			DynamicPortForward:            forwardSettings.DynamicPortForward,
			HTTPDynamicPortForward:        forwardSettings.HTTPDynamicPortForward,
			ReverseDynamicPortForward:     forwardSettings.ReverseDynamicPortForward,
			HTTPReverseDynamicPortForward: forwardSettings.HTTPReverseDynamicPortForward,
			NFSDynamicForwardPort:         forwardSettings.NFSDynamicForwardPort,
			NFSDynamicForwardPath:         forwardSettings.NFSDynamicForwardPath,
			NFSReverseDynamicForwardPort:  forwardSettings.NFSReverseDynamicForwardPort,
			NFSReverseDynamicForwardPath:  forwardSettings.NFSReverseDynamicForwardPath,
			SMBDynamicForwardPort:         forwardSettings.SMBDynamicForwardPort,
			SMBDynamicForwardPath:         forwardSettings.SMBDynamicForwardPath,
			SMBReverseDynamicForwardPort:  forwardSettings.SMBReverseDynamicForwardPort,
			SMBReverseDynamicForwardPath:  forwardSettings.SMBReverseDynamicForwardPath,
			TunnelEnabled:                 forwardSettings.TunnelEnabled,
			TunnelLocal:                   forwardSettings.TunnelLocal,
			TunnelRemote:                  forwardSettings.TunnelRemote,
		},
	}

	if len(c.Args()) > 0 && !c.Bool("not-execute") {
		r.Mode = "cmd"
	}

	if r.EnableHeader {
		fmt.Println("enable w")
	}
	if r.DisableHeader {
		fmt.Println("enable W")
	}

	return r, nil
}

func exitOnRunError(err error) {
	if err == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "Error: %s\n", err)
	os.Exit(1)
}
