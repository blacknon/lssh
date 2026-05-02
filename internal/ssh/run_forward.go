// Copyright (c) 2022 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package ssh

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/blacknon/go-sshlib"
	"github.com/blacknon/lssh/internal/common"
	conf "github.com/blacknon/lssh/internal/config"
)

// setPortForwards is Add local/remote port forward to Run.PortForward
func (r *Run) setPortForwards(server string, config conf.ServerConfig) (c conf.ServerConfig) {
	c = config

	// append single port forward settings (Backward compatibility).
	if c.PortForwardLocal != "" && c.PortForwardRemote != "" {
		fw := new(conf.PortForward)
		fw.Mode = c.PortForwardMode
		fw.Local = c.PortForwardLocal
		fw.Remote = c.PortForwardRemote

		c.Forwards = append(c.Forwards, fw)
	}

	// append port forwards from c, to r.PortForward
	for _, f := range c.PortForwards {
		var err error
		fw := new(conf.PortForward)
		farray := strings.SplitN(f, ":", 2)

		if len(farray) == 1 {
			fmt.Fprintf(os.Stderr, "port forward format is incorrect: %s: \"%s\"", server, f)
			continue
		}

		mode := strings.ToLower(farray[0])
		switch mode {
		case "local", "l":
			fw.Mode = "L"
			fw.Local, fw.Remote, err = common.ParseForwardPort(farray[1])
		case "remote", "r":
			fw.Mode = "R"
			fw.Local, fw.Remote, err = common.ParseForwardPort(farray[1])
		default:
			fmt.Fprintf(os.Stderr, "port forward format is incorrect: %s: \"%s\"", server, f)
			continue
		}

		if err != nil {
			fmt.Fprintf(os.Stderr, "port forward format is incorrect: %s: \"%s\"", server, f)
			continue
		}

		c.Forwards = append(c.Forwards, fw)
	}

	c.Forwards = append(c.Forwards, r.PortForward...)

	return
}

func (r *Run) startPortForward(connect *sshlib.Connect, fw *conf.PortForward) error {
	if fw == nil {
		return nil
	}

	localNetwork := fw.LocalNetwork
	if localNetwork == "" {
		localNetwork = "tcp"
	}
	remoteNetwork := fw.RemoteNetwork
	if remoteNetwork == "" {
		remoteNetwork = "tcp"
	}

	if strings.ToUpper(fw.Mode) == "R" {
		return connect.RemoteForward(localNetwork, fw.Local, remoteNetwork, fw.Remote)
	}

	return connect.LocalForward(localNetwork, fw.Local, remoteNetwork, fw.Remote)
}

// ParallelIgnoredFeatures lists forwarding settings that are intentionally
// skipped for per-host parallel sessions because they require local listeners.
func (r *Run) ParallelIgnoredFeatures(server string) []string {
	config := r.Conf.Server[server]
	config = r.setPortForwards(server, config)

	if r.DynamicPortForward != "" {
		config.DynamicPortForward = r.DynamicPortForward
	}
	if r.HTTPDynamicPortForward != "" {
		config.HTTPDynamicPortForward = r.HTTPDynamicPortForward
	}
	if r.NFSDynamicForwardPort != "" {
		config.NFSDynamicForwardPort = r.NFSDynamicForwardPort
	}
	if r.NFSDynamicForwardPath != "" {
		config.NFSDynamicForwardPath = r.NFSDynamicForwardPath
	}
	if r.SMBDynamicForwardPort != "" {
		config.SMBDynamicForwardPort = r.SMBDynamicForwardPort
	}
	if r.SMBDynamicForwardPath != "" {
		config.SMBDynamicForwardPath = r.SMBDynamicForwardPath
	}

	notices := []string{}
	for _, fw := range config.Forwards {
		if fw == nil || strings.ToUpper(fw.Mode) == "R" {
			continue
		}
		notices = append(notices, fmt.Sprintf("-L %s:%s", fw.Local, fw.Remote))
	}
	if config.DynamicPortForward != "" {
		notices = append(notices, fmt.Sprintf("-D %s", config.DynamicPortForward))
	}
	if config.HTTPDynamicPortForward != "" {
		notices = append(notices, fmt.Sprintf("-d %s", config.HTTPDynamicPortForward))
	}
	if config.NFSDynamicForwardPort != "" && config.NFSDynamicForwardPath != "" {
		notices = append(notices, fmt.Sprintf("-M %s:%s", config.NFSDynamicForwardPort, config.NFSDynamicForwardPath))
	}
	if config.SMBDynamicForwardPort != "" && config.SMBDynamicForwardPath != "" {
		notices = append(notices, fmt.Sprintf("-S %s:%s", config.SMBDynamicForwardPort, config.SMBDynamicForwardPath))
	}
	if r.TunnelEnabled {
		notices = append(notices, fmt.Sprintf("--tunnel %d:%d", r.TunnelLocal, r.TunnelRemote))
	}

	return notices
}

// PrepareParallelForwardConfig returns only the forwarding settings that are safe
// to apply independently to each parallel connection.
func (r *Run) PrepareParallelForwardConfig(server string) (c conf.ServerConfig) {
	c = r.Conf.Server[server]
	c = r.setPortForwards(server, c)

	if r.ReverseDynamicPortForward != "" {
		c.ReverseDynamicPortForward = r.ReverseDynamicPortForward
	}
	if r.HTTPReverseDynamicPortForward != "" {
		c.HTTPReverseDynamicPortForward = r.HTTPReverseDynamicPortForward
	}
	if r.NFSReverseDynamicForwardPort != "" {
		c.NFSReverseDynamicForwardPort = r.NFSReverseDynamicForwardPort
	}
	if r.NFSReverseDynamicForwardPath != "" {
		c.NFSReverseDynamicForwardPath = r.NFSReverseDynamicForwardPath
	}
	if r.SMBReverseDynamicForwardPort != "" {
		c.SMBReverseDynamicForwardPort = r.SMBReverseDynamicForwardPort
	}
	if r.SMBReverseDynamicForwardPath != "" {
		c.SMBReverseDynamicForwardPath = r.SMBReverseDynamicForwardPath
	}

	forwards := make([]*conf.PortForward, 0, len(c.Forwards))
	for _, fw := range c.Forwards {
		if fw == nil || strings.ToUpper(fw.Mode) != "R" {
			continue
		}
		forwards = append(forwards, fw)
	}
	c.Forwards = forwards

	c.DynamicPortForward = ""
	c.HTTPDynamicPortForward = ""
	c.NFSDynamicForwardPort = ""
	c.NFSDynamicForwardPath = ""
	c.SMBDynamicForwardPort = ""
	c.SMBDynamicForwardPath = ""

	return
}

// StartParallelForwards starts only the reverse-side forwards that can be
// applied independently per connection in parallel workflows.
func StartParallelForwards(connect *sshlib.Connect, config conf.ServerConfig) error {
	var errs []error

	for _, fw := range config.Forwards {
		if fw == nil {
			continue
		}
		if err := (&Run{}).startPortForward(connect, fw); err != nil {
			errs = append(errs, err)
		}
	}

	if config.ReverseDynamicPortForward != "" {
		go connect.TCPReverseDynamicForward("localhost", config.ReverseDynamicPortForward)
	}
	if config.HTTPReverseDynamicPortForward != "" {
		go connect.HTTPReverseDynamicForward("localhost", config.HTTPReverseDynamicPortForward)
	}
	if config.NFSReverseDynamicForwardPort != "" && config.NFSReverseDynamicForwardPath != "" {
		go connect.NFSReverseForward("localhost", config.NFSReverseDynamicForwardPort, config.NFSReverseDynamicForwardPath)
	}
	if config.SMBReverseDynamicForwardPort != "" && config.SMBReverseDynamicForwardPath != "" {
		go connect.SMBReverseForward("localhost", config.SMBReverseDynamicForwardPort, "", config.SMBReverseDynamicForwardPath)
	}

	return errors.Join(errs...)
}
