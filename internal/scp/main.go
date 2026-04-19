// Copyright (c) 2022 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package scp

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
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

	// ControlMasterOverride temporarily overrides the config value for this
	// command execution.
	ControlMasterOverride *bool

	// From and To data
	From ScpInfo
	To   ScpInfo

	Config  conf.Config
	AuthMap map[sshl.AuthKey][]ssh.AuthMethod

	// copy with permission flag
	Permission bool
	DryRun     bool

	// send parallel flag
	Parallel    bool
	ParallelNum int

	// progress bar
	Progress   *mpb.Progress
	ProgressWG *sync.WaitGroup
}

func (cp *Scp) printAction(printer *output.Output, action, target string) {
	if printer != nil {
		ow := printer.NewWriter()
		fmt.Fprintf(ow, "[DRY-RUN] %s: %s\n", action, target)
		ow.Close()
		return
	}

	fmt.Fprintf(os.Stderr, "[DRY-RUN] %s: %s\n", action, target)
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
	Closer  io.Closer

	// Output
	Output *output.Output
}

func (s *ScpConnect) Close() error {
	if s == nil || s.Closer == nil {
		return nil
	}
	return s.Closer.Close()
}

type PathSet struct {
	Root      string
	RootIsDir bool
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
	cp.Run.ControlMasterOverride = cp.ControlMasterOverride
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
		info, err := os.Lstat(p)
		if err != nil {
			fmt.Fprintf(os.Stderr, "scp: failed to stat source path %q: %v\n", p, err)
			return
		}

		data, err := common.WalkDir(p)
		if err != nil {
			continue
		}

		sort.Strings(data)

		dataset := PathSet{
			Root:      p,
			RootIsDir: info.IsDir(),
			PathSlice: data,
		}

		pathset = append(pathset, dataset)
	}

	// parallel push data
	for _, target := range targets {
		server := target
		go func() {
			type pushTask struct {
				root      string
				rootIsDir bool
				path      string
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
					defer func() {
						if err := client.Close(); err != nil {
							fmt.Fprintf(os.Stderr, "%s close error: %s\n", client.Server, err)
						}
					}()
					client.Output.Create(client.Server)
					ow := client.Output.NewWriter()
					defer ow.Close()
					for task := range tasks {
						cp.pushPath(client.Connect, ow, client.Output, task.root, task.rootIsDir, task.path)
					}
					workerExit <- true
				}(client)
			}

			if workerCount == 0 {
				exit <- true
				return
			}

			for _, p := range pathset {
				data := p.PathSlice
				for _, path := range data {
					tasks <- pushTask{root: p.Root, rootIsDir: p.RootIsDir, path: path}
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

func (cp *Scp) pushPath(ftp *sftp.Client, ow io.Writer, output *output.Output, root string, rootIsDir bool, p string) (err error) {
	fInfo, _ := os.Lstat(p)
	_, statErr := ftp.Lstat(cp.To.Path[0])
	targetExistsAsDir := statErr == nil
	if targetExistsAsDir {
		targetInfo, err := ftp.Lstat(cp.To.Path[0])
		targetExistsAsDir = err == nil && targetInfo.IsDir()
	}
	preserveSourceName := targetExistsAsDir || len(cp.From.Path) > 1 || strings.HasSuffix(cp.To.Path[0], "/")
	base := copySourceBase(root, rootIsDir, preserveSourceName)
	relpath, _ := filepath.Rel(base, p)
	rpath := resolveRemoteDestinationPath(
		cp.To.Path[0],
		relpath,
		shouldTreatRemoteDestinationAsDir(cp.To.Path[0], targetExistsAsDir, rootIsDir, len(cp.From.Path) > 1),
	)

	if fInfo.IsDir() { // directory
		if cp.DryRun {
			cp.printAction(output, "mkdir", fmt.Sprintf("%s:%s", output.Server, rpath))
		} else {
			ftp.Mkdir(rpath)
		}
	} else { //file
		if cp.DryRun {
			cp.printAction(output, "copy", fmt.Sprintf("local:%s -> %s:%s", p, output.Server, rpath))
			if cp.Permission {
				cp.printAction(output, "chmod", fmt.Sprintf("%s:%s", output.Server, rpath))
			}
			return nil
		}

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

		err = cp.pushFile(lf, ftp, output, p, rpath, size)
		if err != nil {
			fmt.Fprintf(ow, "%s\n", err)
			return err
		}
	}

	// set mode
	if cp.Permission {
		if cp.DryRun {
			cp.printAction(output, "chmod", fmt.Sprintf("%s:%s", output.Server, rpath))
		} else {
			ftp.Chmod(rpath, fInfo.Mode())
		}
	}

	return
}

// pushfile put file to path.
func (cp *Scp) pushFile(lf io.Reader, ftp *sftp.Client, output *output.Output, sourcePath, path string, size int64) (err error) {
	if cp.DryRun {
		cp.printAction(output, "copy", fmt.Sprintf("local:%s -> %s:%s", sourcePath, output.Server, path))
		return nil
	}

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
	go output.ProgressPrinter(size, pr, fmt.Sprintf("local:%s -> %s:%s", sourcePath, output.Server, path))

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
		closeScpClients(fclient)
		closeScpClients(tclient)
		fmt.Fprintf(os.Stderr, "There is no host to connect to\n")
		return
	}
	defer closeScpClients(fclient)
	defer closeScpClients(tclient)

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
				defer func() {
					if err := tclient.Close(); err != nil {
						fmt.Fprintf(os.Stderr, "%s close error: %s\n", tclient.Server, err)
					}
					if err := sclient.Close(); err != nil {
						fmt.Fprintf(os.Stderr, "%s close error: %s\n", sclient.Server, err)
					}
				}()
				tclient.Output.Create(tclient.Server)
				for task := range tasks {
					if cp.DryRun {
						cp.printAction(tclient.Output, "copy", fmt.Sprintf("%s:%s -> %s:%s", sclient.Server, task.path, tclient.Output.Server, task.path))
						continue
					}

					file, err := sclient.Connect.Open(task.path)
					if err != nil {
						tow := tclient.Output.NewWriter()
						fmt.Fprintf(tow, "Error: %s\n", err)
						tow.Close()
						continue
					}

					cp.pushFile(file, tclient.Connect, tclient.Output, task.path, task.path, task.size)
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
					if cp.DryRun {
						cp.printAction(tc.Output, "mkdir", fmt.Sprintf("%s:%s", tc.Output.Server, p))
					} else {
						tc.Connect.Mkdir(p)
					}
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
	defer closeScpClients(clients)

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

	destinationRoot := cp.To.Path[0]
	destinationIsDir := shouldTreatLocalDestinationAsDir(destinationRoot, len(cp.From.Server) > 1 || len(cp.From.Path) > 1)
	if len(cp.From.Server) > 1 {
		destinationRoot = filepath.Join(destinationRoot, client.Server)
		if cp.DryRun {
			cp.printAction(client.Output, "mkdir", fmt.Sprintf("local:%s", destinationRoot))
		} else {
			if err := os.MkdirAll(destinationRoot, 0755); err != nil {
				fmt.Fprintf(ow, "Error: %s\n", err)
				return
			}
		}
	}

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
			defer func() {
				if err := wclient.Close(); err != nil {
					fmt.Fprintf(os.Stderr, "%s close error: %s\n", wclient.Server, err)
				}
			}()
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

				if cp.DryRun {
					rf.Close()
					cp.printAction(wclient.Output, "copy", fmt.Sprintf("%s:%s -> local:%s", wclient.Output.Server, task.remotePath, task.localPath))
					if cp.Permission {
						cp.printAction(wclient.Output, "chmod", fmt.Sprintf("local:%s", task.localPath))
					}
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
				client.Output.ProgressPrinter(task.size, rd, fmt.Sprintf("%s:%s -> local:%s", wclient.Output.Server, task.remotePath, task.localPath))

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
			sourceInfo, err := ftp.Stat(gp)
			if err != nil {
				fmt.Fprintf(ow, "Error: %s\n", err)
				continue
			}

			preserveSourceName := destinationIsDir || len(globpath) > 1
			remoteBase := copySourceBase(gp, sourceInfo.IsDir(), preserveSourceName)
			walker := ftp.Walk(gp)
			for walker.Step() {
				err := walker.Err()
				if err != nil {
					fmt.Fprintf(ow, "Error: %s\n", err)
					continue
				}

				p := walker.Path()
				rp, _ := filepath.Rel(remoteBase, p)
				lpath := resolveLocalDestinationPath(destinationRoot, rp, preserveSourceName || sourceInfo.IsDir())

				stat := walker.Stat()
				if stat.IsDir() { // create dir
					if cp.DryRun {
						cp.printAction(client.Output, "mkdir", fmt.Sprintf("local:%s", lpath))
						if cp.Permission {
							cp.printAction(client.Output, "chmod", fmt.Sprintf("local:%s", lpath))
						}
					} else {
						os.MkdirAll(lpath, 0755)
					}
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
	client, closer, err := cp.Run.CreateSFTPClient(server)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s connect error: %s\n", server, err)
		return nil
	}
	if client == nil {
		_ = closeCloser(closer)
		fmt.Fprintf(os.Stderr, "%s connect error: sftp client is not available\n", server)
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
		Connect: client,
		Closer:  closer,
		Output:  o,
	}
}

func closeScpClients(clients []*ScpConnect) {
	for _, client := range clients {
		if err := closeCloser(client); err != nil {
			fmt.Fprintf(os.Stderr, "%s close error: %s\n", client.Server, err)
		}
	}
}

func closeCloser(closer io.Closer) error {
	if closer == nil {
		return nil
	}
	err := closer.Close()
	if errors.Is(err, io.ErrClosedPipe) {
		return nil
	}
	return err
}
