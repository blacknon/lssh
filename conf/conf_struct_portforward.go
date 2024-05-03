// Copyright (c) 2024 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package conf

// PortForward
type PortForward struct {
	Mode   string // L or R.
	Local  string // localhost:8080
	Remote string // localhost:80
}
