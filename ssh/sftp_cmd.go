package ssh

import (
	"fmt"

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

}

// TODO(blacknon): 転送時の進捗状況を表示するプログレスバーの表示はさせること
func (r *RunSftp) get(args []string) {
	// pathがディレクトリかどうかのチェックが必要
	// remoteFile, err := sftp.Create("hello.txt")
	// localFile, err := os.Open("hello.txt")
	// io.Copy(remoteFile, localFile)

	// f, err := sftp.Create("hello.txt")
	// TODO(blacknon): io.Copy使うとよさそう？？
}

// list is stfp ls command.
func (r *RunSftp) ls(args []string) (err error) {
	// create app
	app := cli.NewApp()

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
	app.HideHelp = true

	// action
	app.Action = func(c *cli.Context) error {
		// set path
		path := "./"
		if len(c.Args()) > 0 {
			path = c.Args().First()
		}

		// get terminal width
		// width, _, err := terminal.GetSize(int(os.Stdout.Fd()))
		// if err != nil {
		// 	return err
		// }

		// get directory files
		for server, client := range r.Client {
			// get writer
			client.Output.Create(server)
			w := client.Output.NewWriter()

			// get directory list data
			data, err := client.Connect.ReadDir(path)
			if err != nil {
				fmt.Fprintf(w, "Error: %s\n", err)
				continue
			}

			// sort
			switch {
			case c.Bool("f"): // do not sort
			case c.Bool("S"): // sort by file size
			case c.Bool("t"): // sort by mod time
			default:
			}

			// reverse order sort

			// print
			switch {
			case c.Bool("l"): // long list format
				// for printout
				for _, f := range data {
					name := f.Name()
					fmt.Fprintf(w, "%s\n", name)
				}

			case c.Bool("1"): // list 1 file per line
				// for list
				for _, f := range data {
					name := f.Name()
					fmt.Fprintf(w, "%s\n", name)
				}

			default: // default

			}
		}

		return nil
	}

	app.Run(args)

	return
}

//
func (r *RunSftp) mkdir(args []string) {

}

// TODO(blacknon): 転送時の進捗状況を表示するプログレスバーの表示はさせること
func (r *RunSftp) put(args []string) {
	// pathがディレクトリかどうかのチェックが必要
	// f, err := sftp.Open(path)
	// TODO(blacknon): io.Copy使うとよさそう？？
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

}

func (r *RunSftp) rm(args []string) {

}

func (r *RunSftp) rmdir(args []string) {

}

func (r *RunSftp) symlink(args []string) {

}

func (r *RunSftp) tree(args []string) {

}

func (r *RunSftp) version(args []string) {

}
