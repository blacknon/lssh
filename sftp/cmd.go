// Copyright (c) 2019 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

// This file describes the code of the built-in command used by lsftp.

package sftp

import (
	"os"
	"sort"

	"github.com/urfave/cli"
)

// FileInfos is []os.FileInfo.
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
	return fi.FileInfos[j].Name() > fi.FileInfos[i].Name()
}

// BySize is sort by size
type BySize struct{ FileInfos }

func (fi BySize) Len() int {
	return len(fi.FileInfos)
}
func (fi BySize) Swap(i, j int) {
	fi.FileInfos[i], fi.FileInfos[j] = fi.FileInfos[j], fi.FileInfos[i]
}
func (fi BySize) Less(i, j int) bool {
	return fi.FileInfos[j].Size() > fi.FileInfos[i].Size()
}

// ByTime is sort by time
type ByTime struct{ FileInfos }

func (fi ByTime) Len() int {
	return len(fi.FileInfos)
}
func (fi ByTime) Swap(i, j int) {
	fi.FileInfos[i], fi.FileInfos[j] = fi.FileInfos[j], fi.FileInfos[i]
}
func (fi ByTime) Less(i, j int) bool {
	return fi.FileInfos[j].ModTime().Unix() > fi.FileInfos[i].ModTime().Unix()
}

var (
	helptext = `{{.Name}} - {{.Usage}}

	{{.HelpName}} {{if .VisibleFlags}}[options]{{end}} {{if .ArgsUsage}}{{.ArgsUsage}}{{end}}
	{{range .VisibleFlags}}	{{.}}
	{{end}}
	`
)

// sftpLsData struct by sftp ls command list data.
type sftpLsData struct {
	Mode  string
	User  string
	Group string
	Size  string
	Time  string
	Path  string
}

// SortLsData is sort []os.FileInfo.
func (r *RunSftp) SortLsData(c *cli.Context, files []os.FileInfo) {
	// sort
	switch {
	case c.Bool("f"): // do not sort
		// If the l flag is enabled, sort by name
		if c.Bool("l") {
			// check reverse
			if c.Bool("r") {
				sort.Sort(sort.Reverse(ByName{files}))
			} else {
				sort.Sort(ByName{files})
			}
		}

	case c.Bool("S"): // sort by file size
		// check reverse
		if c.Bool("r") {
			sort.Sort(sort.Reverse(BySize{files}))
		} else {
			sort.Sort(BySize{files})
		}

	case c.Bool("t"): // sort by mod time
		// check reverse
		if c.Bool("r") {
			sort.Sort(sort.Reverse(ByTime{files}))
		} else {
			sort.Sort(ByTime{files})
		}

	default: // sort by name (default).
		// check reverse
		if c.Bool("r") {
			sort.Sort(sort.Reverse(ByName{files}))
		} else {
			sort.Sort(ByName{files})
		}
	}
}
