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

    // ControlMaster info
    if connect != nil && connect.ControlMaster != "" && connect.ControlMaster != "no" {
        if connect.IsControlClient() {
            fmt.Fprintln(os.Stderr, "Information   :ControlMaster enabled (connected to existing master)")
        } else if connect.SpawnedControlMaster() {
            fmt.Fprintln(os.Stderr, "Information   :ControlMaster enabled (started detached master)")
        } else {
            fmt.Fprintln(os.Stderr, "Information   :ControlMaster enabled (started master)")
        }
    }
}
