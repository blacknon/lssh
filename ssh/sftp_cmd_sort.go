// Copyright (c) 2019 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

// NOTE:
// The file in which code for the sort function used mainly with the lsftp ls command is written.

package ssh

import (
	"os"
)

type FileInfos []os.FileInfo

// ByName is sort by name
type ByName struct{ FileInfos }

func (fi ByName) Len() int {
	return len(fi.FileInfos)
}
func (fi ByName) Swap(i, j int) {
	fi.FileInfos[i], fi.FileInfos[j] = fi.FileInfos[j], fi.FileInfos[i]
}
func (fi ByName) Less(i, j int) bool {
	return fi.FileInfos[j].Name() < fi.FileInfos[i].Name()
}

// ByName is sort by name
type BySize struct{ FileInfos }

func (fi BySize) Len() int {
	return len(fi.FileInfos)
}
func (fi BySize) Swap(i, j int) {
	fi.FileInfos[i], fi.FileInfos[j] = fi.FileInfos[j], fi.FileInfos[i]
}
func (fi BySize) Less(i, j int) bool {
	return fi.FileInfos[j].Size() < fi.FileInfos[i].Size()
}

// ByName is sort by name
type ByTime struct{ FileInfos }

func (fi ByTime) Len() int {
	return len(fi.FileInfos)
}
func (fi ByTime) Swap(i, j int) {
	fi.FileInfos[i], fi.FileInfos[j] = fi.FileInfos[j], fi.FileInfos[i]
}
func (fi ByTime) Less(i, j int) bool {
	return fi.FileInfos[j].ModTime().Unix() < fi.FileInfos[i].ModTime().Unix()
}
