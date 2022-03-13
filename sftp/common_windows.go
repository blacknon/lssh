//go:build windows
// +build windows

// Copyright (c) 2022 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

/*
common is a package that summarizes the common processing of lssh package.
*/

package sftp

import "golang.org/x/sys/windows"

func getFileStat(i interface{}) (uid, gid uint32, size int64) {
	if stat, ok := i.(*windows.Win32FileAttributeData); ok {
		size = int64(stat.FileSizeHigh)
	}

	uid = 0
	gid = 0

	return
}
