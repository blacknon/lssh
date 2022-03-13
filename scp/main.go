// Copyright (c) 2022 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package scp

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/blacknon/lssh/common"
	"github.com/blacknon/lssh/conf"
	"github.com/blacknon/lssh/output"
	sshl "github.com/blacknon/lssh/ssh"
	"github.com/pkg/sftp"
	"github.com/vbauerster/mpb"
	"golang.org/x/crypto/ssh"
)

var (
	oprompt = "${SERVER} :: "
)

type Scp struct {
	// ssh Run
	Run *sshl.Run

	// From and To data
	From ScpInfo
	To   ScpInfo

	Config  conf.Config
	AuthMap map[sshl.AuthKey][]ssh.AuthMethod

	// copy with permission flag
	Permission bool

	// send parallel flag
	Parallel    bool
	ParallelNum int

	// progress bar
	Progress   *mpb.Progress
	ProgressWG *sync.WaitGroup
}

type ScpInfo struct {
	// is remote flag
	IsRemote bool

	// connect server list
	Server []string

	// path list
	Path []string
}

type ScpConnect struct {
	// server name
	Server string

	// ssh connect
	Connect *sftp.Client

	// Output
	Output *output.Output
}

type PathSet struct {
	Base      string
	PathSlice []string
}

// Start scp, switching process.
func (cp *Scp) Start() {
	// Create server list
	slist := append(cp.To.Server, cp.From.Server...)

	// Create AuthMap
	cp.Run = new(sshl.Run)
	cp.Run.ServerList = slist
	cp.Run.Conf = cp.Config
	cp.Run.CreateAuthMethodMap()

	// Create Progress bar struct
	cp.ProgressWG = new(sync.WaitGroup)
	cp.Progress = mpb.New(mpb.WithWaitGroup(cp.ProgressWG))

	switch {
	// remote to remote
	case cp.From.IsRemote && cp.To.IsRemote:
		cp.viaPush()

	// remote to local
	case cp.From.IsRemote && !cp.To.IsRemote:
		cp.pull()

	// local to remote
	case !cp.From.IsRemote && cp.To.IsRemote:
		cp.push()
	}
}

// local machine to remote machine push data
func (cp *Scp) push() {
	// set target hosts
	targets := cp.To.Server

	// create channel
	exit := make(chan bool)

	// create connection parallel
	clients := cp.createScpConnects(targets)
	if len(clients) == 0 {
		fmt.Fprintf(os.Stderr, "There is no host to connect to\n")
		return
	}

	// get local host directory walk data
	pathset := []PathSet{}
	for _, p := range cp.From.Path {
		data, err := common.WalkDir(p)
		if err != nil {
			continue
		}

		sort.Strings(data)

		dataset := PathSet{
			Base:      path.Dir(p),
			PathSlice: data,
		}

		pathset = append(pathset, dataset)
	}

	// parallel push data
	for _, c := range clients {
		client := c
		go func() {
			// TODO(blacknon): Parallelで指定した数までは同時コピーできるようにする

			// set ftp client
			ftp := client.Connect

			// get output writer
			client.Output.Create(client.Server)
			ow := client.Output.NewWriter()

			// push path
			for _, p := range pathset {
				base := p.Base
				data := p.PathSlice
				for _, path := range data {
					cp.pushPath(ftp, ow, client.Output, base, path)
				}
			}

			// exit
			exit <- true
		}()
	}

	// wait send data
	for i := 0; i < len(clients); i++ {
		<-exit
	}
	close(exit)

	// wait 0.3 sec
	time.Sleep(300 * time.Millisecond)

	// exit messages
	fmt.Println("all push exit.")
}

//
func (cp *Scp) pushPath(ftp *sftp.Client, ow *io.PipeWriter, output *output.Output, base, p string) (err error) {
	var rpath string

	// Set remote path
	relpath, _ := filepath.Rel(base, p)
	if common.IsDirPath(cp.To.Path[0]) || len(cp.From.Path) > 1 {
		rpath = filepath.Join(cp.To.Path[0], relpath)
	} else if len(cp.From.Path) == 1 {
		rpath = filepath.Join(cp.To.Path[0], relpath)
		dInfo, _ := os.Lstat(cp.From.Path[0])
		if dInfo.IsDir() {
			ftp.Mkdir(cp.To.Path[0])
		}
	} else {
		rpath = filepath.Clean(cp.To.Path[0])
	}

	// get local file info
	fInfo, _ := os.Lstat(p)
	if fInfo.IsDir() { // directory
		ftp.Mkdir(rpath)
	} else { //file
		// open local file
		lf, err := os.Open(p)
		if err != nil {
			fmt.Fprintf(ow, "%s\n", err)
			return err
		}
		defer lf.Close()

		// get file size
		lstat, _ := os.Lstat(p)
		size := lstat.Size()

		// copy file
		// TODO(blacknon): Outputからプログレスバーで出力できるようにする(io.MultiWriterを利用して書き込み？)
		err = cp.pushFile(lf, ftp, output, rpath, size)
		if err != nil {
			fmt.Fprintf(ow, "%s\n", err)
			return err
		}
	}

	// set mode
	if cp.Permission {
		ftp.Chmod(rpath, fInfo.Mode())
	}

	return
}

// pushfile put file to path.
func (cp *Scp) pushFile(lf io.Reader, ftp *sftp.Client, output *output.Output, path string, size int64) (err error) {
	// get output writer
	ow := output.NewWriter()

	// set path
	dir := filepath.Dir(path)
	dir = filepath.ToSlash(dir)

	// mkdir all
	err = ftp.MkdirAll(dir)

	if err != nil {
		fmt.Fprintf(ow, "%s\n", err)
		return
	}

	// open remote file
	rf, err := ftp.OpenFile(path, os.O_RDWR|os.O_CREATE)
	if err != nil {
		fmt.Fprintf(ow, "%s\n", err)
		return
	}

	// set tee reader
	rd := io.TeeReader(lf, rf)

	// copy to data
	cp.ProgressWG.Add(1)
	output.ProgressPrinter(size, rd, path)

	return
}

//
func (cp *Scp) viaPush() {
	// get server name
	from := cp.From.Server[0] // string
	to := cp.To.Server        // []string

	// create client
	fclient := cp.createScpConnects([]string{from})
	tclient := cp.createScpConnects(to)
	if len(fclient) == 0 || len(tclient) == 0 {
		fmt.Fprintf(os.Stderr, "There is no host to connect to\n")
		return
	}

	// pull and push data
	for _, path := range cp.From.Path {
		cp.viaPushPath(path, fclient[0], tclient)
	}

	// wait 0.3 sec
	time.Sleep(300 * time.Millisecond)

	// exit messages
	fmt.Println("all push exit.")
}

//
func (cp *Scp) viaPushPath(path string, fclient *ScpConnect, tclients []*ScpConnect) {
	// from ftp client
	ftp := fclient.Connect

	// create from sftp walker
	walker := ftp.Walk(path)

	// get from sftp output writer
	fclient.Output.Create(fclient.Server)
	fow := fclient.Output.NewWriter()

	for walker.Step() {
		err := walker.Err()
		if err != nil {
			fmt.Fprintf(fow, "Error: %s\n", err)
			continue
		}

		p := walker.Path()
		stat := walker.Stat()
		if stat.IsDir() { // is directory
			for _, tc := range tclients {
				tc.Connect.Mkdir(p)
			}
		} else { // is file
			// open from server file
			file, err := ftp.Open(p)
			if err != nil {
				fmt.Fprintf(fow, "Error: %s\n", err)
				continue
			}
			size := stat.Size()

			exit := make(chan bool)
			for _, tc := range tclients {
				tclient := tc
				go func() {
					tclient.Output.Create(tclient.Server)

					cp.pushFile(file, tclient.Connect, tclient.Output, p, size)
					exit <- true
				}()
			}

			for i := 0; i < len(tclients); i++ {
				<-exit
			}
		}
	}
}

//
func (cp *Scp) pull() {
	// set target hosts
	targets := cp.From.Server

	// create channel
	exit := make(chan bool)

	// create connection parallel
	clients := cp.createScpConnects(targets)
	if len(clients) == 0 {
		fmt.Fprintf(os.Stderr, "There is no host to connect to\n")
		return
	}

	// parallel push data
	for _, c := range clients {
		client := c
		go func() {
			// pull data
			cp.pullPath(client)
			exit <- true
		}()
	}

	// wait send data
	for i := 0; i < len(clients); i++ {
		<-exit
	}
	close(exit)

	// wait 0.3 sec
	time.Sleep(300 * time.Millisecond)

	// exit messages
	fmt.Println("all pull exit.")
}

// walk return file path list ([]string).
func (cp *Scp) pullPath(client *ScpConnect) {
	// set ftp client
	ftp := client.Connect

	// get output writer
	client.Output.Create(client.Server)
	ow := client.Output.NewWriter()

	// basedir
	baseDir := cp.To.Path[0]

	// if multi pull, servername add baseDir
	if len(cp.From.Server) > 1 {
		baseDir = filepath.Join(baseDir, client.Server)
		os.MkdirAll(baseDir, 0755)
	}
	baseDir, _ = filepath.Abs(baseDir)
	baseDir = filepath.ToSlash(baseDir)

	// walk remote path
	for _, path := range cp.From.Path {
		globpath, err := ftp.Glob(path)
		if err != nil {
			fmt.Fprintf(ow, "Error: %s\n", err)
			continue
		}

		for _, gp := range globpath {
			walker := ftp.Walk(gp)
			for walker.Step() {
				// basedir
				remoteBase := filepath.Dir(gp)
				remoteBase = filepath.ToSlash(remoteBase)

				err := walker.Err()
				if err != nil {
					fmt.Fprintf(ow, "Error: %s\n", err)
					continue
				}

				p := walker.Path()
				rp, _ := filepath.Rel(remoteBase, p)
				lpath := filepath.Join(baseDir, rp)

				stat := walker.Stat()
				if stat.IsDir() { // create dir
					os.MkdirAll(lpath, 0755)
				} else { // create file
					// get size
					size := stat.Size()

					// open remote file
					rf, err := ftp.Open(p)
					if err != nil {
						fmt.Fprintf(ow, "Error: %s\n", err)
						continue
					}

					// open local file
					lf, err := os.OpenFile(lpath, os.O_RDWR|os.O_CREATE, 0644)
					if err != nil {
						fmt.Fprintf(ow, "Error: %s\n", err)
						continue
					}

					// set tee reader
					rd := io.TeeReader(rf, lf)

					cp.ProgressWG.Add(1)
					client.Output.ProgressPrinter(size, rd, p)
				}

				// set mode
				if cp.Permission {
					os.Chmod(lpath, stat.Mode())
				}
			}
		}
	}

	return
}

// createScpConnects return []*ScpConnect.
func (cp *Scp) createScpConnects(targets []string) (result []*ScpConnect) {
	ch := make(chan bool)
	m := new(sync.Mutex)
	for _, target := range targets {
		server := target
		go func() {
			// ssh connect
			conn, err := cp.Run.CreateSshConnect(server)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s connect error: %s\n", server, err)
				ch <- true
				return
			}

			// create sftp client
			ftp, err := sftp.NewClient(conn.Client)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s create client error: %s\n", server, err)
				ch <- true
				return
			}

			// create output
			o := &output.Output{
				Templete:   oprompt,
				ServerList: targets,
				Conf:       cp.Config.Server[server],
				AutoColor:  true,
				Progress:   cp.Progress,
				ProgressWG: cp.ProgressWG,
			}

			// create ScpConnect
			scpCon := &ScpConnect{
				Server:  server,
				Connect: ftp,
				Output:  o,
			}

			// append result
			m.Lock()
			result = append(result, scpCon)
			m.Unlock()

			ch <- true
		}()
	}

	// wait
	for i := 0; i < len(targets); i++ {
		<-ch
	}

	return result
}
