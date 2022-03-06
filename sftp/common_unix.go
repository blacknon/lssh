//go:build !windows
// +build !windows

// Copyright (c) 2021 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

/*
common is a package that summarizes the common processing of lssh package.
*/

package sftp

import "golang.org/x/sys/unix"

func getFileStat(i interface{}) (uid, gid uint32, size int64) {
	if stat, ok := i.(*unix.Stat_t); ok {

		uid = stat.Uid
		gid = stat.Gid
		size = stat.Size
	}

	return
}
