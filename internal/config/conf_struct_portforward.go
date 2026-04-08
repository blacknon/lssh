// Copyright (c) 2024 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package conf

// PortForward
type PortForward struct {
	Mode          string // L or R.
	Local         string // localhost:8080 or /tmp/local.sock
	Remote        string // localhost:80 or /tmp/remote.sock
	LocalNetwork  string // tcp or unix
	RemoteNetwork string // tcp or unix
}
