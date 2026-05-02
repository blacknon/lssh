// Copyright (c) 2024 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package sshproc

import (
	"fmt"

	proc "github.com/c9s/goprocinfo/linux"
)

type SocketStateCount map[string]uint64

func TCPStateName(state uint8) string {
	switch state {
	case 0x01:
		return "ESTABLISHED"
	case 0x02:
		return "SYN_SENT"
	case 0x03:
		return "SYN_RECV"
	case 0x04:
		return "FIN_WAIT1"
	case 0x05:
		return "FIN_WAIT2"
	case 0x06:
		return "TIME_WAIT"
	case 0x07:
		return "CLOSE"
	case 0x08:
		return "CLOSE_WAIT"
	case 0x09:
		return "LAST_ACK"
	case 0x0A:
		return "LISTEN"
	case 0x0B:
		return "CLOSING"
	case 0x0C:
		return "NEW_SYN_RECV"
	default:
		return fmt.Sprintf("UNKNOWN_%02X", state)
	}
}

func CountTCPSocketStates(sockets *proc.NetTCPSockets) SocketStateCount {
	counts := SocketStateCount{}
	if sockets == nil {
		return counts
	}

	for _, socket := range sockets.Sockets {
		counts[TCPStateName(socket.Status)]++
	}

	return counts
}

func CountUDPSocketStates(sockets *proc.NetUDPSockets) SocketStateCount {
	counts := SocketStateCount{}
	if sockets == nil {
		return counts
	}

	for _, socket := range sockets.Sockets {
		counts[TCPStateName(socket.Status)]++
	}

	return counts
}
