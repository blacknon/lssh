// Copyright (c) 2022 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

// NOTE:
// The file in which code for the sort function used mainly with the lsftp ls command is written.

package sftp

import (
	"fmt"
	"io/ioutil"
	"os"
	pkguser "os/user"
	"runtime"
	"strconv"
	"text/tabwriter"

	"github.com/blacknon/lssh/common"
	"github.com/blacknon/textcol"
	"github.com/dustin/go-humanize"
	"github.com/urfave/cli"
)

// lls exec and print out local ls data.
func (r *RunSftp) lls(args []string) (err error) {
	// create app
	app := cli.NewApp()
	// app.UseShortOptionHandling = true

	// set help message
	app.CustomAppHelpTemplate = helptext

	// set parameter
	app.Flags = []cli.Flag{
		cli.BoolFlag{Name: "1", Usage: "list one file per line"},
		cli.BoolFlag{Name: "a", Usage: "do not ignore entries starting with"},
		cli.BoolFlag{Name: "f", Usage: "do not sort"},
		cli.BoolFlag{Name: "h", Usage: "with -l, print sizes like 1K 234M 2G etc."},
		cli.BoolFlag{Name: "l", Usage: "use a long listing format"},
		cli.BoolFlag{Name: "n", Usage: "list numeric user and group IDs"},
		cli.BoolFlag{Name: "r", Usage: "reverse order while sorting"},
		cli.BoolFlag{Name: "S", Usage: "sort by file size, largest first"},
		cli.BoolFlag{Name: "t", Usage: "sort by modification time, newest first"},
	}
	app.Name = "lls"
	app.Usage = "lsftp build-in command: lls [local machine ls]"
	app.ArgsUsage = "[PATH]"
	app.HideHelp = true
	app.HideVersion = true
	app.EnableBashCompletion = true

	// action
	app.Action = func(c *cli.Context) error {
		// argpath
		argpath := c.Args().First()
		if argpath == "" {
			argpath = "./"
		}

		stat, err := os.Stat(argpath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			return nil
		}

		// check is directory
		var data []os.FileInfo
		if stat.IsDir() {
			data, err = ioutil.ReadDir(argpath)
		} else {
			data = append(data, stat)
		}

		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			return nil
		}

		switch {
		case c.Bool("l"): // long list format
			// set tabwriter
			tabw := new(tabwriter.Writer)
			tabw.Init(os.Stdout, 0, 1, 1, ' ', 0)

			// get maxSizeWidth
			var maxSizeWidth int
			var user, group, timestr, sizestr string
			for _, f := range data {
				if c.Bool("h") {
					sizestr = humanize.Bytes(uint64(f.Size()))
				} else {
					sizestr = strconv.FormatUint(uint64(f.Size()), 10)
				}

				// set sizestr max length
				if maxSizeWidth < len(sizestr) {
					maxSizeWidth = len(sizestr)
				}
			}

			//
			datas := []*sftpLsData{}
			for _, f := range data {
				var uid, gid uint32
				var size int64

				timestamp := f.ModTime()
				timestr = timestamp.Format("2006 01-02 15:04:05")

				if runtime.GOOS != "windows" {
					system := f.Sys()
					uid, gid, size = getFileStat(system)
				}

				// Switch with or without -n option.
				if c.Bool("n") {
					user = strconv.FormatUint(uint64(uid), 10)
					group = strconv.FormatUint(uint64(gid), 10)
				} else {
					userdata, _ := pkguser.LookupId(strconv.FormatUint(uint64(uid), 10))
					user = userdata.Username

					groupdata, _ := pkguser.LookupGroupId(strconv.FormatUint(uint64(gid), 10))
					group = groupdata.Name
				}

				// Switch with or without -h option.
				if c.Bool("h") {
					sizestr = humanize.Bytes(uint64(size))
				} else {
					sizestr = strconv.FormatUint(uint64(size), 10)
				}

				// set data
				lsdata := new(sftpLsData)
				lsdata.Mode = f.Mode().String()
				lsdata.User = user
				lsdata.Group = group
				lsdata.Size = sizestr
				lsdata.Time = timestr
				lsdata.Path = f.Name()

				// append data
				datas = append(datas, lsdata)

				// set print format
				format := "%s\t%s\t%s\t%" + strconv.Itoa(maxSizeWidth) + "s\t%s\t%s\n"

				// write data
				fmt.Fprintf(tabw, format, lsdata.Mode, lsdata.User, lsdata.Group, lsdata.Size, lsdata.Time, lsdata.Path)
			}

			tabw.Flush()

		case c.Bool("1"): // list 1 file per line
			for _, f := range data {
				fmt.Println(f.Name())
			}

		default: // default
			var item []string
			for _, f := range data {
				item = append(item, f.Name())
			}

			textcol.Output = os.Stdout
			textcol.Padding = 0
			textcol.PrintColumns(&item, 2)
		}

		return nil
	}

	// parse short options
	args = common.ParseArgs(app.Flags, args)
	app.Run(args)

	return
}
