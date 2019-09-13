// Copyright (c) 2019 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

// This file describes the code of the built-in command used by lsftp.
// It is quite big in that relationship. Maybe it will be separated or repaired soon.

package ssh

import (
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"sort"
	"strconv"
	"time"

	"text/tabwriter"

	"github.com/blacknon/lssh/common"
	"github.com/blacknon/textcol"
	"github.com/dustin/go-humanize"
	"github.com/pkg/sftp"
	"github.com/urfave/cli"
)

//
// NOTE: カレントディレクトリの移動の仕組みを別途作成すること(保持する仕組みがないので)
func (r *RunSftp) cd(args []string) {
	// for _, c := range r.Client {

	// }
}

func (r *RunSftp) chgrp(args []string) {

}

func (r *RunSftp) chown(args []string) {

}

// sftp put/pull function
// NOTE: リモートマシンからリモートマシンにコピーさせるような処理や、対象となるホストを個別に指定してコピーできるような仕組みをつくること！
// TODO(blacknon): 転送時の進捗状況を表示するプログレスバーの表示はさせること
func (r *RunSftp) cp(args []string) {
	// finished := make(chan bool)

	// // set target list
	// targetList := []string{}
	// switch mode {
	// case "push":
	//  targetList = r.To.Server
	// case "pull":
	//  targetList = r.From.Server
	// }

	// for _, value := range targetList {
	//  target := value
	// }
}

//
func (r *RunSftp) df(args []string) {
	// create app
	app := cli.NewApp()
	// app.UseShortOptionHandling = true

	// set help message
	app.CustomAppHelpTemplate = `	{{.Name}} - {{.Usage}}
	{{.HelpName}} {{if .VisibleFlags}}[options]{{end}} [PATH]
	{{range .VisibleFlags}}	{{.}}
	{{end}}
	`

	// set parameter
	app.Flags = []cli.Flag{
		cli.BoolFlag{Name: "h", Usage: "print sizes in powers of 1024 (e.g., 1023M)"},
		cli.BoolFlag{Name: "i", Usage: "list inode information instead of block usage"},
	}
	app.Name = "df"
	app.Usage = "lsftp build-in command: df [remote machine df]"
	app.ArgsUsage = "[path]"
	app.HideHelp = true
	app.HideVersion = true
	app.EnableBashCompletion = true

	// action
	app.Action = func(c *cli.Context) error {
		// set path
		// TODO(blacknon): cdでカレントディレクトリ変更した場合の処理に対応させる
		path := "./"
		if len(c.Args()) > 0 {
			path = c.Args().First()
		}

		// get remote stat data
		stats := map[string]*sftp.StatVFS{}
		for server, client := range r.Client {
			// set ftp client
			ftp := client.Connect

			// get StatVFS
			stat, err := ftp.StatVFS(path)
			if err != nil {
				fmt.Println(err)
				continue
			}
			stats[server] = stat
		}

		// set tabwriter
		tabw := new(tabwriter.Writer)
		tabw.Init(os.Stdout, 0, 8, 4, ' ', tabwriter.AlignRight)

		// print header
		headerTotal := "TotalSize"
		if c.Bool("i") {
			headerTotal = "Inodes"
		}
		fmt.Fprintf(tabw, "%s\t%s\t%s\t%s\t%s\t\n", "Server", headerTotal, "Used", "(root)", "Capacity")

		// print stat
		for server, stat := range stats {
			// set data in columns
			var column1, column2, column3, column4, column5 string
			switch {
			case c.Bool("i"):
				totals := stat.Files
				frees := stat.Ffree
				useds := totals - frees

				column1 = server
				column2 = strconv.FormatUint(totals, 10)
				column3 = strconv.FormatUint(useds, 10)
				column4 = strconv.FormatUint(frees, 10)
				column5 = fmt.Sprintf("%0.2f", (float64(useds)/float64(totals))*100)

			case c.Bool("h"):
				totals := stat.TotalSpace()
				frees := stat.FreeSpace()
				useds := stat.TotalSpace() - stat.FreeSpace()

				column1 = server
				column2 = humanize.IBytes(totals)
				column3 = humanize.IBytes(useds)
				column4 = humanize.IBytes(frees)
				column5 = fmt.Sprintf("%0.2f", (float64(useds)/float64(totals))*100)

			default:
				totals := stat.TotalSpace()
				frees := stat.FreeSpace()
				useds := stat.TotalSpace() - stat.FreeSpace()

				column1 = server
				column2 = strconv.FormatUint(totals/1024, 10)
				column3 = strconv.FormatUint(useds/1024, 10)
				column4 = strconv.FormatUint(frees/1024, 10)
				column5 = fmt.Sprintf("%0.2f", (float64(useds)/float64(totals))*100)
			}

			fmt.Fprintf(tabw, "%s\t%s\t%s\t%s\t%s%%\t\n", column1, column2, column3, column4, column5)
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

func (r *RunSftp) ln(args []string) (err error) {
	// create app
	app := cli.NewApp()
	// app.UseShortOptionHandling = true

	// set help message
	app.CustomAppHelpTemplate = `	{{.Name}} - {{.Usage}}
	{{.HelpName}} {{if .VisibleFlags}}[options]{{end}} [PATH]
	{{range .VisibleFlags}}	{{.}}
	{{end}}
	`
	app.Name = "ln"
	app.Usage = "lsftp build-in command: ln [remote machine ln]"
	app.ArgsUsage = "[path]"
	app.HideHelp = true
	app.HideVersion = true
	app.EnableBashCompletion = true

	// action
	app.Action = func(c *cli.Context) error {
		return nil
	}

	// parse short options
	args = common.ParseArgs(app.Flags, args)
	app.Run(args)

	return
}

// list is stfp ls command.
// TODO(blacknon): `ls -l`時に全サーバで表示を揃える
// TODO(blacknon): データをparallelで取得させる
func (r *RunSftp) ls(args []string) (err error) {
	// create app
	app := cli.NewApp()
	// app.UseShortOptionHandling = true

	// set help message
	app.CustomAppHelpTemplate = `	{{.Name}} - {{.Usage}}
	{{.HelpName}} {{if .VisibleFlags}}[options]{{end}} [PATH]
	{{range .VisibleFlags}}	{{.}}
	{{end}}
	`

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
	app.Name = "ls"
	app.Usage = "lsftp build-in command: ls [remote machine ls]"
	app.ArgsUsage = "[path]"
	app.HideHelp = true
	app.HideVersion = true
	app.EnableBashCompletion = true

	// action
	app.Action = func(c *cli.Context) error {
		// set path
		// TODO(blacknon): cdでカレントディレクトリ変更した場合の処理に対応させる
		path := "./"
		if len(c.Args()) > 0 {
			path = c.Args().First()
		}

		// get directory files
		for server, client := range r.Client {
			// get writer
			client.Output.Create(server)
			w := client.Output.NewWriter()
			headerWidth := len(client.Output.prompt)

			// get stat
			lstat, err := client.Connect.Lstat(path)
			if err != nil {
				fmt.Fprintf(w, "Error: %s\n", err)
				continue
			}

			var data []os.FileInfo
			if lstat.IsDir() {
				// get directory list data
				data, err = client.Connect.ReadDir(path)
				if err != nil {
					fmt.Fprintf(w, "Error: %s\n", err)
					continue
				}
			} else {
				data = []os.FileInfo{lstat}
			}

			// if `a` flag disable, delete Hidden files...
			if !c.Bool("a") {
				// hidden delete data slice
				hddata := []os.FileInfo{}

				// regex
				rgx := regexp.MustCompile(`^\.`)

				for _, f := range data {
					if !rgx.MatchString(f.Name()) {
						hddata = append(hddata, f)
					}
				}

				data = hddata
			}

			// sort
			switch {
			case c.Bool("f"): // do not sort
				// If the l flag is enabled, sort by name
				if c.Bool("l") {
					// check reverse
					if c.Bool("r") {
						sort.Sort(sort.Reverse(ByName{data}))
					} else {
						sort.Sort(ByName{data})
					}
				}

			case c.Bool("S"): // sort by file size
				// check reverse
				if c.Bool("r") {
					sort.Sort(sort.Reverse(BySize{data}))
				} else {
					sort.Sort(BySize{data})
				}

			case c.Bool("t"): // sort by mod time
				// check reverse
				if c.Bool("r") {
					sort.Sort(sort.Reverse(ByTime{data}))
				} else {
					sort.Sort(ByTime{data})
				}

			default: // sort by name (default).
				// check reverse
				if c.Bool("r") {
					sort.Sort(sort.Reverse(ByName{data}))
				} else {
					sort.Sort(ByName{data})
				}
			}

			// read /etc/passwd
			passwdFile, _ := client.Connect.Open("/etc/passwd")
			passwdByte, _ := ioutil.ReadAll(passwdFile)
			passwd := string(passwdByte)

			// read /etc/group
			groupFile, _ := client.Connect.Open("/etc/group")
			groupByte, _ := ioutil.ReadAll(groupFile)
			groups := string(groupByte)

			// print
			switch {
			case c.Bool("l"): // long list format
				// set tabwriter
				tabw := new(tabwriter.Writer)
				tabw.Init(w, 0, 1, 1, ' ', 0)

				// for get data
				datas := []*SftpLsData{}
				var maxSizeWidth int
				for _, f := range data {
					sys := f.Sys()

					// TODO(blacknon): count hardlink (2列目)の取得方法がわからないため、わかったら追加。
					var uid, gid uint32
					var size uint64
					var user, group, timestr, sizestr string

					if stat, ok := sys.(*sftp.FileStat); ok {
						uid = stat.UID
						gid = stat.GID
						size = stat.Size
						timestamp := time.Unix(int64(stat.Mtime), 0)
						timestr = timestamp.Format("2006 01-02 15:04:05")
					}

					// Switch with or without -n option.
					if c.Bool("n") {
						user = strconv.FormatUint(uint64(uid), 10)
						group = strconv.FormatUint(uint64(gid), 10)
					} else {
						user = common.GetNameFromId(passwd, uid)
						group = common.GetNameFromId(groups, gid)
					}

					// Switch with or without -h option.
					if c.Bool("h") {
						sizestr = humanize.Bytes(size)
					} else {
						sizestr = strconv.FormatUint(size, 10)
					}

					// set sizestr max length
					if maxSizeWidth < len(sizestr) {
						maxSizeWidth = len(sizestr)
					}

					// set data
					data := new(SftpLsData)
					data.Mode = f.Mode().String()
					data.User = user
					data.Group = group
					data.Size = sizestr
					data.Time = timestr
					data.Path = f.Name()

					// append data
					datas = append(datas, data)
				}

				// set print format
				format := "%s\t%s\t%s\t%" + strconv.Itoa(maxSizeWidth) + "s\t%s\t%s\n"

				// print ls
				for _, d := range datas {
					fmt.Fprintf(tabw, format, d.Mode, d.User, d.Group, d.Size, d.Time, d.Path)
				}

				tabw.Flush()

			case c.Bool("1"): // list 1 file per line
				// for list
				for _, f := range data {
					name := f.Name()
					fmt.Fprintf(w, "%s\n", name)
				}

			default: // default
				var item []string
				for _, f := range data {
					item = append(item, f.Name())
				}

				textcol.Output = w
				textcol.Padding = headerWidth
				textcol.PrintColumns(&item, 2)
			}
		}

		return nil
	}

	// parse short options
	args = common.ParseArgs(app.Flags, args)
	app.Run(args)

	return
}

//
func (r *RunSftp) mkdir(args []string) {
	// create app
	app := cli.NewApp()
	// app.UseShortOptionHandling = true

	// set parameter
	app.Flags = []cli.Flag{
		cli.BoolFlag{Name: "p", Usage: "no error if existing, make parent directories as needed"},
	}

	// set help message
	app.CustomAppHelpTemplate = `	{{.Name}} - {{.Usage}}
	{{.HelpName}} {{if .VisibleFlags}}[options]{{end}} [PATH]
	{{range .VisibleFlags}}	{{.}}
	{{end}}
	`
	app.Name = "mkdir"
	app.Usage = "lsftp build-in command: mkdir [remote machine mkdir]"
	app.ArgsUsage = "[path]"
	app.HideHelp = true
	app.HideVersion = true
	app.EnableBashCompletion = true

	// action
	app.Action = func(c *cli.Context) error {
		// TODO(blacknon): 複数のディレクトリ受付(v0.6.1)
		if len(c.Args()) != 1 {
			fmt.Println("Requires one arguments")
			fmt.Println("mkdir [path]")
			return nil
		}

		for server, client := range r.Client {
			// get writer
			client.Output.Create(server)
			w := client.Output.NewWriter()

			// create directory
			var err error
			if c.Bool("p") {
				err = client.Connect.MkdirAll(c.Args()[0])
			} else {
				err = client.Connect.Mkdir(c.Args()[0])
			}

			// check error
			if err != nil {
				fmt.Fprintf(w, "%s\n", err)
			}

			fmt.Fprintf(w, "make directory: %s\n", c.Args()[0])
		}

		return nil
	}

	// parse short options
	args = common.ParseArgs(app.Flags, args)
	app.Run(args)

	return
}

//
func (r *RunSftp) pwd(args []string) {
	exit := make(chan bool)

	go func() {
		for server, client := range r.Client {
			// get writer
			client.Output.Create(server)
			w := client.Output.NewWriter()

			// get current directory
			pwd, _ := client.Connect.Getwd()

			fmt.Fprintf(w, "%s\n", pwd)

			exit <- true
		}
	}()

	for i := 0; i < len(r.Client); i++ {
		<-exit
	}

	return
}

func (r *RunSftp) rename(args []string) {
	// create app
	app := cli.NewApp()
	// app.UseShortOptionHandling = true

	// set help message
	app.CustomAppHelpTemplate = `	{{.Name}} - {{.Usage}}
	{{.HelpName}} {{if .VisibleFlags}}[options]{{end}} [PATH] [PATH]
	{{range .VisibleFlags}}	{{.}}
	{{end}}
	`
	app.Name = "rename"
	app.Usage = "lsftp build-in command: rename [remote machine rename]"
	app.ArgsUsage = "[path path]"
	app.HideHelp = true
	app.HideVersion = true
	app.EnableBashCompletion = true

	// action
	app.Action = func(c *cli.Context) error {
		if len(c.Args()) != 2 {
			fmt.Println("Requires two arguments")
			fmt.Println("rename [old] [new]")
			return nil
		}

		for server, client := range r.Client {
			// get writer
			client.Output.Create(server)
			w := client.Output.NewWriter()

			// get current directory
			err := client.Connect.Rename(c.Args()[0], c.Args()[1])
			if err != nil {
				fmt.Fprintf(w, "%s\n", err)
			}

			fmt.Fprintf(w, "rename: %s => %s\n", c.Args()[0], c.Args()[1])
		}

		return nil
	}

	// parse short options
	args = common.ParseArgs(app.Flags, args)
	app.Run(args)

	return
}

func (r *RunSftp) rm(args []string) {

}

//
func (r *RunSftp) rmdir(args []string) {
	// create app
	app := cli.NewApp()
	// app.UseShortOptionHandling = true

	// set help message
	app.CustomAppHelpTemplate = `	{{.Name}} - {{.Usage}}
	{{.HelpName}} {{if .VisibleFlags}}[options]{{end}} [PATH]
	{{range .VisibleFlags}}	{{.}}
	{{end}}
	`
	app.Name = "rmdir"
	app.Usage = "lsftp build-in command: rmdir [remote machine rmdir]"
	app.ArgsUsage = "[path]"
	app.HideHelp = true
	app.HideVersion = true
	app.EnableBashCompletion = true

	// action
	app.Action = func(c *cli.Context) error {
		if len(c.Args()) != 1 {
			fmt.Println("Requires one arguments")
			fmt.Println("rmdir [path]")
			return nil
		}

		for server, client := range r.Client {
			// get writer
			client.Output.Create(server)
			w := client.Output.NewWriter()

			// remove directory
			err := client.Connect.RemoveDirectory(c.Args()[0])
			if err != nil {
				fmt.Fprintf(w, "%s\n", err)
			}

			fmt.Fprintf(w, "remove dir: %s\n", c.Args()[0])
		}

		return nil
	}

	// parse short options
	args = common.ParseArgs(app.Flags, args)
	app.Run(args)

	return
}

func (r *RunSftp) tree(args []string) {

}

func (r *RunSftp) version(args []string) {

}
