// Copyright (c) 2019 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package sftp

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/blacknon/lssh/common"
	"github.com/urfave/cli"
	"github.com/vbauerster/mpb"
)

// TODO(blacknon): リファクタリング(v0.6.1)

//
func (r *RunSftp) put(args []string) {
	// create app
	app := cli.NewApp()
	// app.UseShortOptionHandling = true

	// set help message
	app.CustomAppHelpTemplate = helptext

	// set parameter
	app.Name = "put"
	app.Usage = "lsftp build-in command: put"
	app.ArgsUsage = "[source(local) target(remote)]"
	app.HideHelp = true
	app.HideVersion = true
	app.EnableBashCompletion = true

	// action
	app.Action = func(c *cli.Context) error {
		if len(c.Args()) != 2 {
			fmt.Println("Requires two arguments")
			fmt.Println("put source(local) target(remote)")
			return nil
		}

		// Create Progress
		r.ProgressWG = new(sync.WaitGroup)
		r.Progress = mpb.New(mpb.WithWaitGroup(r.ProgressWG))

		// set path
		source := c.Args()[0]
		target := c.Args()[1]

		// get local host directory walk data
		pathset := []PathSet{}
		data, err := common.WalkDir(source)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			return nil
		}

		sort.Strings(data)
		dataset := PathSet{
			Base:      filepath.Dir(source),
			PathSlice: data,
		}
		pathset = append(pathset, dataset)

		// parallel push data
		exit := make(chan bool)
		for s, c := range r.Client {
			server := s
			client := c
			go func() {
				// set Progress
				client.Output.Progress = r.Progress
				client.Output.ProgressWG = r.ProgressWG

				// create output
				client.Output.Create(server)

				// push path
				for _, p := range pathset {
					base := p.Base
					data := p.PathSlice

					for _, path := range data {
						r.pushPath(client, target, base, path)
					}
				}

				// exit
				exit <- true
			}()
		}

		// wait exit
		for i := 0; i < len(r.Client); i++ {
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
func (r *RunSftp) pushPath(client *SftpConnect, target, base, path string) (err error) {
	// set arg path
	rpath, _ := filepath.Rel(base, path)
	switch {
	case filepath.IsAbs(target):
		rpath = filepath.Join(target, rpath)
	case !filepath.IsAbs(target):
		target = filepath.Join(client.Pwd, target)
		rpath = filepath.Join(target, rpath)
	}

	// get local file info
	fInfo, _ := os.Lstat(path)
	if fInfo.IsDir() { // directory
		client.Connect.Mkdir(rpath)
	} else { //file
		// open local file
		localfile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer localfile.Close()

		// get file size
		lstat, _ := os.Lstat(path)
		size := lstat.Size()

		// copy file
		err = r.pushFile(client, localfile, rpath, size)
		if err != nil {
			return err
		}
	}

	// set mode
	if r.Permission {
		client.Connect.Chmod(rpath, fInfo.Mode())
	}

	return
}

// pushfile put file to path.
func (r *RunSftp) pushFile(client *SftpConnect, localfile io.Reader, path string, size int64) (err error) {
	// mkdir all
	dir := filepath.Dir(path)
	err = client.Connect.MkdirAll(dir)
	if err != nil {
		return
	}

	// open remote file
	remotefile, err := client.Connect.OpenFile(path, os.O_RDWR|os.O_CREATE)
	if err != nil {
		return
	}

	// set tee reader
	rd := io.TeeReader(localfile, remotefile)

	// copy to data
	r.ProgressWG.Add(1)
	client.Output.ProgressPrinter(size, rd, path)

	return
}
