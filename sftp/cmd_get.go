// Copyright (c) 2021 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package sftp

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/blacknon/lssh/common"
	"github.com/urfave/cli"
	"github.com/vbauerster/mpb"
)

// TODO(blacknon): リファクタリング(v0.6.2)

//
func (r *RunSftp) get(args []string) {
	// create app
	app := cli.NewApp()

	// set help message
	app.CustomAppHelpTemplate = helptext

	// set parameter
	app.Name = "get"
	app.Usage = "lsftp build-in command: get"
	app.ArgsUsage = "[source(remote)...] target(local)"
	app.HideHelp = true
	app.HideVersion = true
	app.EnableBashCompletion = true

	// action
	app.Action = func(c *cli.Context) error {
		if len(c.Args()) != 2 {
			fmt.Println("Requires over two arguments")
			fmt.Println("get source(remote)... target(local)")
			return nil
		}

		// Create Progress
		r.ProgressWG = new(sync.WaitGroup)
		r.Progress = mpb.New(mpb.WithWaitGroup(r.ProgressWG))

		// set pathlist
		argsSize := len(c.Args()) - 1
		source := c.Args()[:argsSize]
		destination := c.Args()[1]

		// get destination directory abs
		destination, err := filepath.Abs(destination)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			return nil
		}

		// mkdir local destination directory
		err = os.MkdirAll(destination, 0755)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			return nil
		}

		targetmap := map[string]*TargetConnectMap{}
		for _, spath := range source {
			targetmap = r.createTargetMap(targetmap, spath)
		}

		// get directory data, copy remote to local
		exit := make(chan bool)
		for s, c := range targetmap {
			server := s
			client := c

			targetDestinationDir := destination
			if len(targetmap) > 1 {
				targetDestinationDir = filepath.Join(targetDestinationDir, server)
				// mkdir local target directory
				err = os.MkdirAll(targetDestinationDir, 0755)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: %s\n", err)
					return nil
				}
			}

			go func() {
				// set Progress
				client.Output.Progress = r.Progress
				client.Output.ProgressWG = r.ProgressWG

				// create output
				client.Output.Create(server)

				err = r.pullPath(client, targetDestinationDir)

				exit <- true
			}()
		}

		// wait exit
		for i := 0; i < len(targetmap); i++ {
			<-exit
		}
		close(exit)

		// wait Progress
		r.Progress.Wait()

		// wait 0.3 sec
		time.Sleep(300 * time.Millisecond)

		return nil
	}

	// parse short options
	args = common.ParseArgs(app.Flags, args)
	app.Run(args)

	return
}

//
func (r *RunSftp) pullPath(client *TargetConnectMap, targetdir string) (err error) {
	for _, path := range client.Path {
		// set arg path
		var rpath string
		switch {
		case filepath.IsAbs(path):
			rpath = path
		case !filepath.IsAbs(path):
			rpath = filepath.Join(client.Pwd, path)
		}
		base := filepath.Dir(rpath)

		// get writer
		ow := client.Output.NewWriter()

		// expantion path
		epath, _ := client.Connect.Glob(rpath)

		if len(epath) == 0 {
			fmt.Fprintf(ow, "Error: File Not founds.\n")
			return
		}

		// for walk
		for _, ep := range epath {

			walker := client.Connect.Walk(ep)

			for walker.Step() {
				err := walker.Err()
				if err != nil {
					fmt.Fprintf(ow, "Error: %s\n", err)
					continue
				}

				p := walker.Path()
				relpath, _ := filepath.Rel(base, p)
				relpath = strings.Replace(relpath, "../", "", 1)
				if strings.Contains(relpath, "/") {
					os.MkdirAll(filepath.Join(targetdir, filepath.Dir(relpath)), 0755)
				}

				stat := walker.Stat()
				localpath := filepath.Join(targetdir, relpath)

				//
				if stat.IsDir() { // is directory
					os.MkdirAll(localpath, 0755)
				} else { // is not directory
					// get size
					size := stat.Size()

					// open remote file
					remotefile, err := client.Connect.Open(p)
					if err != nil {
						fmt.Fprintf(ow, "Error: %s\n", err)
						continue
					}

					// open local file
					localfile, err := os.OpenFile(localpath, os.O_RDWR|os.O_CREATE, 0644)
					if err != nil {
						fmt.Fprintf(ow, "Error: %s\n", err)
						continue
					}

					// set tee reader
					rd := io.TeeReader(remotefile, localfile)

					r.ProgressWG.Add(1)
					client.Output.ProgressPrinter(size, rd, p)
				}

				// set mode
				if r.Permission {
					os.Chmod(localpath, stat.Mode())
				}
			}
		}
	}

	return
}
