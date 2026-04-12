// Copyright (c) 2026 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package common

import "strings"

// NormalizeGenerateLSSHConfArgs rewrites --generate-lssh-conf without an
// explicit value to use the default OpenSSH config path.
func NormalizeGenerateLSSHConfArgs(args []string) []string {
	result := make([]string, 0, len(args))
	for _, arg := range args {
		switch {
		case arg == "--generate-lssh-conf":
			result = append(result, "--generate-lssh-conf=~/.ssh/config")
		case strings.HasPrefix(arg, "--generate-lssh-conf="):
			result = append(result, arg)
		default:
			result = append(result, arg)
		}
	}

	return result
}
