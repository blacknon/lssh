package lssh

import (
	"fmt"

	conf "github.com/blacknon/lssh/internal/config"
)

type connectorFlagOptions struct {
	AttachSession string
	Detach        bool

	CommandArgs []string

	MuxMode      bool
	ParallelMode bool
	TermMode     bool
	NotExecute   bool
	LocalRc      bool
	NotLocalRc   bool
	X11          bool
	X11Trusted   bool
	Background   bool

	LocalForwards            int
	RemoteForwards           int
	DynamicForward           bool
	HTTPDynamicForward       bool
	HTTPReverseForward       bool
	NFSDynamicForward        bool
	NFSReverseDynamicForward bool
	SMBDynamicForward        bool
	SMBReverseDynamicForward bool
	Tunnel                   bool
}

func validateConnectorShellOptions(opts connectorFlagOptions, selected []string, data conf.Config) error {
	if opts.AttachSession == "" && !opts.Detach {
		return nil
	}

	if opts.AttachSession != "" && opts.Detach {
		return fmt.Errorf("--attach and --detach cannot be used together")
	}
	if opts.MuxMode {
		return fmt.Errorf("--attach/--detach cannot be used with -P")
	}
	if opts.ParallelMode {
		return fmt.Errorf("--attach/--detach cannot be used with -p")
	}
	if opts.TermMode {
		return fmt.Errorf("--attach/--detach cannot be used with -t")
	}
	if opts.NotExecute {
		return fmt.Errorf("--attach/--detach cannot be used with -N")
	}
	if len(opts.CommandArgs) > 0 {
		return fmt.Errorf("--attach/--detach cannot be used with command execution")
	}
	if opts.LocalRc || opts.NotLocalRc {
		return fmt.Errorf("--attach/--detach cannot be used with --localrc or --not-localrc")
	}
	if opts.X11 || opts.X11Trusted {
		return fmt.Errorf("--attach/--detach cannot be used with X11 forwarding")
	}
	if opts.Background {
		return fmt.Errorf("--attach/--detach cannot be used with -f")
	}
	if opts.LocalForwards > 0 || opts.RemoteForwards > 0 ||
		opts.DynamicForward || opts.HTTPDynamicForward || opts.HTTPReverseForward ||
		opts.NFSDynamicForward || opts.NFSReverseDynamicForward ||
		opts.SMBDynamicForward || opts.SMBReverseDynamicForward || opts.Tunnel {
		return fmt.Errorf("--attach/--detach cannot be used with forwarding options")
	}
	if len(selected) != 1 {
		return fmt.Errorf("--attach/--detach require selecting exactly one host")
	}
	if !data.ServerUsesConnector(selected[0]) {
		return fmt.Errorf("--attach/--detach can only be used with connector-backed hosts")
	}

	return nil
}
