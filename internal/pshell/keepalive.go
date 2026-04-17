// Copyright (c) 2024 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package pshell

import (
	"fmt"
	"io"
	"os"
)

func (s *shell) probeConnection(client *sConnect, forceRemote bool) error {
	if client == nil || client.Connect == nil {
		return fmt.Errorf("connection is nil")
	}

	if !forceRemote && client.Connect.ControlMaster != "" && client.Connect.ControlMaster != "no" {
		return nil
	}

	if client.Connect.IsControlClient() && forceRemote {
		clone := *client.Connect
		clone.Stdin = nil
		clone.Stdout = io.Discard
		clone.Stderr = io.Discard
		clone.TTY = false
		return clone.Command("true")
	}

	return client.Connect.CheckClientAlive()
}

func (s *shell) checkKeepalive(forceRemote bool) {
	ch := make(chan bool)
	clients := s.Connects

	for _, client := range clients {
		go func(client *sConnect) {
			if client == nil {
				ch <- true
				return
			}
			if client.Connector {
				client.Connected = true
				client.LastError = ""
				ch <- true
				return
			}
			if client.Connect == nil {
				ch <- true
				return
			}
			if !client.Connected {
				ch <- true
				return
			}

			// keepalive
			// Note: client.Client may be nil for ControlMaster connections;
			// CheckClientAlive() handles that case via controlClient.Ping().
			err := s.probeConnection(client, forceRemote)

			if err != nil {
				// error
				if client.Connected {
					fmt.Fprintf(os.Stderr, "Exit Connect %s, Error: %s\n", client.Name, err)
				}
				client.Connected = false
				client.LastError = err.Error()

				// close underlying ssh client if present (nil for ControlMaster connections)
				if client.Client != nil {
					client.Client.Close()
				}
			} else {
				client.Connected = true
				client.LastError = ""
			}

			ch <- true
		}(client)
	}

	// wait
	for i := 0; i < len(clients); i++ {
		<-ch
	}

	connectedCount := 0
	for _, client := range s.Connects {
		if client != nil && client.Connected {
			connectedCount++
		}
	}

	if connectedCount == 0 {
		s.exit(1, "Error: No valid connections\n")
	}

	return
}
