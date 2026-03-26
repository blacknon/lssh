package ssh

import (
	"fmt"
	"os"

	"github.com/blacknon/go-sshlib"
	"github.com/blacknon/lssh/conf"
)

// PrintConnectInfo centralizes informational output related to a created
// connection (ControlMaster state, local rc usage, etc.). Call this after
// CreateSshConnect returns a valid *sshlib.Connect.
func (r *Run) PrintConnectInfo(server string, connect *sshlib.Connect, cfg conf.ServerConfig) {
	// Local rc info
	if cfg.LocalRcUse == "yes" {
		fmt.Fprintf(os.Stderr, "Information   :This connect use local bashrc.\n")
	}

	// X11 info
	if cfg.X11 || r.X11 {
		if cfg.X11 && r.X11 {
			fmt.Fprintln(os.Stderr, "Information   :X11 forwarding enabled (local and global)")
		} else if cfg.X11 {
			fmt.Fprintln(os.Stderr, "Information   :X11 forwarding enabled (from config)")
		} else {
			fmt.Fprintln(os.Stderr, "Information   :X11 forwarding enabled (from CLI)")
		}
		if cfg.X11Trusted || r.X11Trusted {
			fmt.Fprintln(os.Stderr, "Information   :Trusted X11 forwarding enabled")
		}
	}

	// ControlMaster info
	if connect != nil && connect.ControlMaster != "" && connect.ControlMaster != "no" {
		if connect.SpawnedControlMaster() {
			fmt.Fprintln(os.Stderr, "Information   :ControlMaster enabled (started detached master)")
		} else if connect.IsControlClient() {
			fmt.Fprintln(os.Stderr, "Information   :ControlMaster enabled (connected to existing master)")
		} else {
			fmt.Fprintln(os.Stderr, "Information   :ControlMaster enabled (started master)")
		}
	}
}
