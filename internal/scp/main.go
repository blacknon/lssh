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

	"github.com/blacknon/lssh/internal/common"
	conf "github.com/blacknon/lssh/internal/config"
	"github.com/blacknon/lssh/internal/output"
	sshl "github.com/blacknon/lssh/internal/ssh"
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

	parallelNum := 1
	if cp.ParallelNum > 0 {
		parallelNum = cp.ParallelNum
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
	for _, target := range targets {
		server := target
		go func() {
			type pushTask struct {
				base string
				path string
			}

			tasks := make(chan pushTask)
			workerExit := make(chan bool)
			workerCount := 0

			for i := 0; i < parallelNum; i++ {
				client := cp.createScpConnect(server, targets)
				if client == nil {
					continue
				}
				workerCount++
				go func(client *ScpConnect) {
					client.Output.Create(client.Server)
					ow := client.Output.NewWriter()
					defer ow.Close()
					for task := range tasks {
						cp.pushPath(client.Connect, ow, client.Output, task.base, task.path)
					}
					workerExit <- true
				}(client)
			}

			if workerCount == 0 {
				exit <- true
				return
			}

			for _, p := range pathset {
				base := p.Base
				data := p.PathSlice
				for _, path := range data {
					tasks <- pushTask{base: base, path: path}
				}
			}
			close(tasks)

			for i := 0; i < workerCount; i++ {
				<-workerExit
			}

			// exit
			exit <- true
		}()
	}

	// wait send data
	for i := 0; i < len(targets); i++ {
		<-exit
	}
	close(exit)

	// wait 0.3 sec
	time.Sleep(300 * time.Millisecond)

	// exit messages
	fmt.Println("all push exit.")
}

func (cp *Scp) pushPath(ftp *sftp.Client, ow io.Writer, output *output.Output, base, p string) (err error) {
	var rpath string

	// Set remote path
	relpath, _ := filepath.Rel(base, p)
	if common.IsDirPath(cp.To.Path[0]) || len(cp.From.Path) > 1 {
		rpath = filepath.Join(cp.To.Path[0], relpath)
	} else if len(cp.From.Path) == 1 {
		rpath = cp.To.Path[0]
		dInfo, _ := os.Lstat(cp.From.Path[0])
		if dInfo.IsDir() {
			ftp.Mkdir(cp.To.Path[0])
			rpath = filepath.Join(cp.To.Path[0], relpath)
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
	defer ow.Close()

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
	defer rf.Close()

	// empty the file
	err = rf.Truncate(0)
	if err != nil {
		fmt.Fprintf(ow, "%s\n", err)
		return
	}

	// stream file data to the remote file and the progress printer at the same time.
	pr, pw := io.Pipe()
	defer pr.Close()

	cp.ProgressWG.Add(1)
	go output.ProgressPrinter(size, pr, path)

	_, err = io.Copy(io.MultiWriter(rf, pw), lf)
	closeErr := pw.Close()
	if err != nil {
		fmt.Fprintf(ow, "%s\n", err)
		return
	}
	if closeErr != nil {
		fmt.Fprintf(ow, "%s\n", closeErr)
		err = closeErr
		return
	}

	return
}

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
	cp.viaPushPath(cp.From.Path, fclient[0], tclient)

	// wait 0.3 sec
	time.Sleep(300 * time.Millisecond)

	// exit messages
	fmt.Println("all push exit.")
}

func (cp *Scp) viaPushPath(paths []string, fclient *ScpConnect, tclients []*ScpConnect) {
	// from ftp client
	ftp := fclient.Connect

	// get from sftp output writer
	fclient.Output.Create(fclient.Server)
	fow := fclient.Output.NewWriter()
	defer fow.Close()

	type viaPushTask struct {
		path string
		size int64
	}

	parallelNum := 1
	if cp.ParallelNum > 0 {
		parallelNum = cp.ParallelNum
	}

	taskMap := map[string]chan viaPushTask{}
	exitMap := map[string]chan bool{}
	workerCountMap := map[string]int{}
	for _, tc := range tclients {
		tasks := make(chan viaPushTask)
		workerExit := make(chan bool)
		taskMap[tc.Server] = tasks
		exitMap[tc.Server] = workerExit
		workerCountMap[tc.Server] = 0

		for i := 0; i < parallelNum; i++ {
			tclient := cp.createScpConnect(tc.Server, cp.To.Server)
			sclient := cp.createScpConnect(fclient.Server, []string{fclient.Server})
			if tclient == nil || sclient == nil {
				continue
			}
			workerCountMap[tc.Server]++
			go func() {
				tclient.Output.Create(tclient.Server)
				for task := range tasks {
					file, err := sclient.Connect.Open(task.path)
					if err != nil {
						tow := tclient.Output.NewWriter()
						fmt.Fprintf(tow, "Error: %s\n", err)
						tow.Close()
						continue
					}

					cp.pushFile(file, tclient.Connect, tclient.Output, task.path, task.size)
					file.Close()
				}

				workerExit <- true
			}()
		}
	}
	defer func() {
		for _, tc := range tclients {
			close(taskMap[tc.Server])
		}
		for _, tc := range tclients {
			for i := 0; i < workerCountMap[tc.Server]; i++ {
				<-exitMap[tc.Server]
			}
		}
	}()

	for _, path := range paths {
		// create from sftp walker
		walker := ftp.Walk(path)

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
				for _, tc := range tclients {
					taskMap[tc.Server] <- viaPushTask{
						path: p,
						size: stat.Size(),
					}
				}
			}
		}
	}
}

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

	type pullTask struct {
		remotePath string
		localPath  string
		mode       os.FileMode
		size       int64
	}

	// basedir
	baseDir := filepath.Dir(cp.To.Path[0])
	fileName := filepath.Base(cp.To.Path[0])

	// if multi pull, servername add baseDir
	if len(cp.From.Server) > 1 {
		baseDir = filepath.Join(baseDir, client.Server)
		os.MkdirAll(baseDir, 0755)
	}

	// get abs path
	baseDir, _ = filepath.Abs(baseDir)
	baseDir = filepath.ToSlash(baseDir)

	parallelNum := 1
	if cp.ParallelNum > 0 {
		parallelNum = cp.ParallelNum
	}

	tasks := make(chan pullTask)
	workerExit := make(chan bool)
	workerCount := 0

	for i := 0; i < parallelNum; i++ {
		wclient := cp.createScpConnect(client.Server, cp.From.Server)
		if wclient == nil {
			continue
		}
		workerCount++
		go func(wclient *ScpConnect) {
			wclient.Output.Create(wclient.Server)
			wow := wclient.Output.NewWriter()
			defer wow.Close()
			for task := range tasks {
				// open remote file
				rf, err := wclient.Connect.Open(task.remotePath)
				if err != nil {
					fmt.Fprintf(wow, "Error: %s\n", err)
					continue
				}

				err = os.MkdirAll(filepath.Dir(task.localPath), 0755)
				if err != nil {
					rf.Close()
					fmt.Fprintf(wow, "Error: %s\n", err)
					continue
				}

				// open local file
				lf, err := os.OpenFile(task.localPath, os.O_RDWR|os.O_CREATE, 0644)
				if err != nil {
					rf.Close()
					fmt.Fprintf(wow, "Error: %s\n", err)
					continue
				}

				// empty the file
				err = lf.Truncate(0)
				if err != nil {
					rf.Close()
					lf.Close()
					fmt.Fprintf(wow, "Error: %s\n", err)
					continue
				}

				// set tee reader
				rd := io.TeeReader(rf, lf)

				cp.ProgressWG.Add(1)
				client.Output.ProgressPrinter(task.size, rd, task.remotePath)

				rf.Close()
				lf.Close()

				// set mode
				if cp.Permission {
					os.Chmod(task.localPath, task.mode)
				}
			}

			workerExit <- true
		}(wclient)
	}

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
				// rp, _ := filepath.Rel(remoteBase, fileName)
				rp, _ := filepath.Rel(remoteBase, p)
				if fileName == "" {
					rp, _ = filepath.Rel(remoteBase, fileName)
				}
				lpath := filepath.Join(baseDir, rp)

				stat := walker.Stat()
				if stat.IsDir() { // create dir
					os.MkdirAll(lpath, 0755)
				} else { // create file
					tasks <- pullTask{
						remotePath: p,
						localPath:  lpath,
						mode:       stat.Mode(),
						size:       stat.Size(),
					}
				}
			}
		}
	}

	close(tasks)
	for i := 0; i < workerCount; i++ {
		<-workerExit
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
			scpCon := cp.createScpConnect(server, targets)
			if scpCon == nil {
				ch <- true
				return
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

func (cp *Scp) createScpConnect(server string, serverList []string) (result *ScpConnect) {
	// ssh connect
	conn, err := cp.Run.CreateSshConnectDirect(server)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s connect error: %s\n", server, err)
		return nil
	}
	if conn == nil || conn.Client == nil {
		fmt.Fprintf(os.Stderr, "%s connect error: ssh client is not available for sftp\n", server)
		return nil
	}

	// create sftp client
	ftp, err := sftp.NewClient(conn.Client)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s create client error: %s\n", server, err)
		return nil
	}

	// create output
	o := &output.Output{
		Templete:   oprompt,
		ServerList: serverList,
		Conf:       cp.Config.Server[server],
		AutoColor:  true,
		Progress:   cp.Progress,
		ProgressWG: cp.ProgressWG,
	}

	return &ScpConnect{
		Server:  server,
		Connect: ftp,
		Output:  o,
	}
}
