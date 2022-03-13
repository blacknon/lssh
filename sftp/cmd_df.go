// Copyright (c) 2022 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

// This file describes the code of the built-in command used by lsftp.
// It is quite big in that relationship. Maybe it will be separated or repaired soon.

package sftp

import (
	"fmt"
	"os"
	"strconv"
	"text/tabwriter"

	"github.com/blacknon/lssh/common"
	"github.com/dustin/go-humanize"
	"github.com/pkg/sftp"
	"github.com/urfave/cli"
)

// df exec and print out remote df.
func (r *RunSftp) df(args []string) {
	// create app
	app := cli.NewApp()
	// app.UseShortOptionHandling = true

	// set help message
	app.CustomAppHelpTemplate = helptext

	// set parameter
	app.Flags = []cli.Flag{
		cli.BoolFlag{Name: "h", Usage: "print sizes in powers of 1024 (e.g., 1023M)"},
		cli.BoolFlag{Name: "i", Usage: "list inode information instead of block usage"},
	}
	app.Name = "df"
	app.Usage = "lsftp build-in command: df [remote machine df]"
	app.ArgsUsage = "PATH..."
	app.HideHelp = true
	app.HideVersion = true
	app.EnableBashCompletion = true

	// action
	app.Action = func(c *cli.Context) error {
		argpathlist := c.Args()

		// get remote stat data
		stats := map[string]map[string]*sftp.StatVFS{}

		// set tabwriter
		tabw := new(tabwriter.Writer)
		tabw.Init(os.Stdout, 0, 8, 4, ' ', tabwriter.AlignRight)

		if len(argpathlist) == 0 {
			argpathlist = append(argpathlist, "./")
		}

		targetmap := map[string]*TargetConnectMap{}
		for _, p := range argpathlist {
			targetmap = r.createTargetMap(targetmap, p)
		}

		for server, client := range targetmap {
			// set map
			stats[server] = map[string]*sftp.StatVFS{}

			// set ftp client
			ftp := client.Connect

			// set path
			pathlist := []string{}
			if len(client.Path) > 0 {
				pathlist = append(pathlist, client.Path...)
			} else {
				pathlist = append(pathlist, client.Pwd)
			}

			// get StatVFS
			for _, path := range pathlist {
				stat, err := ftp.StatVFS(path)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: %s: %s\n", server, err)
					continue
				}
				stats[server][path] = stat
			}

		}

		// print header
		headerTotal := "TotalSize"
		if c.Bool("i") {
			headerTotal = "Inodes"
		}
		fmt.Fprintf(tabw, "%s\t%s\t%s\t%s\t%s\t%s\t\n", "Server", "Path", headerTotal, "Used", "(root)", "Capacity")

		// print stat
		for server, statlist := range stats {
			for path, stat := range statlist {
				// set data in columns
				var column1, column2, column3, column4, column5, column6 string
				switch {
				case c.Bool("i"):
					totals := stat.Files
					frees := stat.Ffree
					useds := totals - frees

					column1 = server
					column2 = path
					column3 = strconv.FormatUint(totals, 10)
					column4 = strconv.FormatUint(useds, 10)
					column5 = strconv.FormatUint(frees, 10)
					column6 = fmt.Sprintf("%0.2f", (float64(useds)/float64(totals))*100)

				case c.Bool("h"):
					totals := stat.TotalSpace()
					frees := stat.FreeSpace()
					useds := stat.TotalSpace() - stat.FreeSpace()

					column1 = server
					column2 = path
					column3 = humanize.IBytes(totals)
					column4 = humanize.IBytes(useds)
					column5 = humanize.IBytes(frees)
					column6 = fmt.Sprintf("%0.2f", (float64(useds)/float64(totals))*100)

				default:
					totals := stat.TotalSpace()
					frees := stat.FreeSpace()
					useds := stat.TotalSpace() - stat.FreeSpace()

					column1 = server
					column2 = path
					column3 = strconv.FormatUint(totals/1024, 10)
					column4 = strconv.FormatUint(useds/1024, 10)
					column5 = strconv.FormatUint(frees/1024, 10)
					column6 = fmt.Sprintf("%0.2f", (float64(useds)/float64(totals))*100)
				}

				fmt.Fprintf(tabw, "%s\t%s\t%s\t%s\t%s\t%s%%\t\n", column1, column2, column3, column4, column5, column6)
			}
		}

		// write tabwriter
		tabw.Flush()

		return nil
	}

	// parse short options
	args = common.ParseArgs(app.Flags, args)
	app.Run(args)

	return
}
