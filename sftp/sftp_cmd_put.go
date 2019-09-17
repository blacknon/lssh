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
	"time"

	"github.com/blacknon/lssh/common"
	"github.com/blacknon/lssh/output"
	"github.com/pkg/sftp"
	"github.com/urfave/cli"
)

// TODO(blacknon): 転送時の進捗状況を表示するプログレスバーの表示はさせること
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
			Base:      filepath.Dir(p),
			PathSlice: data,
		}

		pathset = append(pathset, dataset)

		// parallel push data
		exit := make(chan bool)
		for s, c := range r.Client {
			server := s
			client := c
			go func() {
				// set arg path
				if !filepath.IsAbs(target) {
					target = filepath.Join(client.Pwd, target)
				}

				// set ftp client
				ftp := client.Connect

				// get output writer
				client.Output.Create(server)
				ow := client.Output.NewWriter()

				// push path
				for _, p := range pathset {
					base := p.Base
					data := p.PathSlice
					for _, path := range data {
						r.pushPath(ftp, ow, client.Output, base, target)
					}
				}

				// exit
				exit <- true
			}()
		}

		//
		for i := 0; i < len(r.Client); i++ {
			<-exit
		}
		close(exit)

		// wait 0.3 sec
		time.Sleep(300 * time.Millisecond)

		return nil
	}

	// parse short options
	args = common.ParseArgs(app.Flags, args)
	app.Run(args)

	return
}

// local machine to remote machine push data
// func (r *RunSftp) put() {
// 	// set target hosts
// 	targets := cp.To.Server

// 	// create channel
// 	exit := make(chan bool)

// 	// create connection parallel
// 	clients := cp.createScpConnects(targets)
// 	if len(clients) == 0 {
// 		fmt.Fprintf(os.Stderr, "There is no host to connect to\n")
// 		return
// 	}

// 	// get local host directory walk data
// 	pathset := []PathSet{}
// 	for _, p := range cp.From.Path {
// 		data, err := common.WalkDir(p)
// 		if err != nil {
// 			continue
// 		}

// 		sort.Strings(data)
// 		dataset := PathSet{
// 			Base:      filepath.Dir(p),
// 			PathSlice: data,
// 		}

// 		pathset = append(pathset, dataset)
// 	}

// 	// parallel push data
// 	for _, c := range clients {
// 		client := c
// 		go func() {
// 			// TODO(blacknon): Parallelで指定した数までは同時コピーできるようにする

// 			// set ftp client
// 			ftp := client.Connect

// 			// get output writer
// 			client.Output.Create(client.Server)
// 			ow := client.Output.NewWriter()

// 			// push path
// 			for _, p := range pathset {
// 				base := p.Base
// 				data := p.PathSlice
// 				for _, path := range data {
// 					cp.pushPath(ftp, ow, client.Output, base, path)
// 				}
// 			}

// 			// exit
// 			exit <- true
// 		}()
// 	}

// 	// wait send data
// 	for i := 0; i < len(clients); i++ {
// 		<-exit
// 	}
// 	close(exit)

// 	// wait 0.3 sec
// 	time.Sleep(300 * time.Millisecond)

// 	// exit messages
// 	fmt.Println("all push exit.")
// }

//
func (r *RunSftp) pushPath(ftp *sftp.Client, ow *io.PipeWriter, output *output.Output, base, path string) (err error) {
	// TODO(blacknon): リモートのPATH(target)の推測等の処理をこっちに移す

	// get rel path
	relpath, _ := filepath.Rel(base, path)
	rpath := filepath.Join(cp.To.Path[0], relpath)

	// get local file info
	fInfo, _ := os.Lstat(path)
	if fInfo.IsDir() { // directory
		ftp.Mkdir(rpath)
	} else { //file
		// open local file
		lf, err := os.Open(path)
		if err != nil {
			return err
		}
		defer lf.Close()

		// get file size
		lstat, _ := os.Lstat(path)
		size := lstat.Size()

		// copy file
		// TODO(blacknon): Outputからプログレスバーで出力できるようにする(io.MultiWriterを利用して書き込み？)
		err = r.pushFile(lf, ftp, output, rpath, size)
		if err != nil {
			return err
		}
	}

	// set mode
	if r.Permission {
		ftp.Chmod(rpath, fInfo.Mode())
	}

	return
}

// pushfile put file to path.
func (r *RunSftp) pushFile(lf io.Reader, ftp *sftp.Client, output *output.Output, path string, size int64) (err error) {
	// mkdir all
	dir := filepath.Dir(path)
	err = ftp.MkdirAll(dir)
	if err != nil {
		return
	}

	// open remote file
	rf, err := ftp.OpenFile(path, os.O_RDWR|os.O_CREATE)
	if err != nil {
		return
	}

	// set tee reader
	rd := io.TeeReader(lf, rf)

	// copy to data
	r.ProgressWG.Add(1)
	output.ProgressPrinter(size, rd, path)

	return
}
