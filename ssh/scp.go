// Copyright (c) 2019 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package ssh

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/blacknon/lssh/common"
	"github.com/blacknon/lssh/conf"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

var (
	oprompt = "${SERVER} :: "
)

type Scp struct {
	Run         *Run
	From        ScpInfo
	To          ScpInfo
	Permission  bool
	Parallel    bool
	ParallelNum int
	Config      conf.Config
	AuthMap     map[AuthKey][]ssh.AuthMethod
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
	Output *Output
}

// TODO(blacknon): scpプロトコルは使用せず、sftpプロトコルを利用する。(これにより、リモートにscpコマンドがなくても動作するようになる)
// TODO(blacknon):
//     scp時のプログレスバーの表示について検討する(リモートについては、リモートで実行しているscpコマンドの出力をそのまま出力すればいけそうな気がする)
//     https://github.com/cheggaaa/pb
//     https://github.com/vbauerster/mpb // パラレルのバーを使うので、使うならこっち！
// TODO(blacknon): Reader/Writerでの処理に切り替えたほうが良さそう

// Start scp, switching process.
func (cp *Scp) Start() {
	// Create server list
	slist := append(cp.To.Server, cp.From.Server...)

	// Create AuthMap
	cp.Run = new(Run)
	cp.Run.ServerList = slist
	cp.Run.Conf = cp.Config
	cp.Run.createAuthMethodMap()

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
	paths := []string{}
	for _, p := range cp.From.Path {
		data, err := common.WalkDir(p)
		if err != nil {
			continue
		}

		paths = append(paths, data...)
	}

	// sort paths
	sort.Strings(paths)

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
			for _, path := range paths {
				cp.pushPath(ftp, ow, path)
			}

			// exit messages
			fmt.Fprintf(ow, "all push exit.\n")

			// exit
			exit <- true
		}()
	}

	// wait send data
	for i := 0; i < len(clients); i++ {
		<-exit
	}
	close(exit)
}

//
func (cp *Scp) pushPath(ftp *sftp.Client, ow *io.PipeWriter, path string) (err error) {
	// get rel path
	baseDir, _ := os.Getwd()
	relpath, _ := filepath.Rel(baseDir, path)
	relpath = strings.Replace(relpath, "../", "", -1) // test(delete `../`)
	relpath = strings.Replace(relpath, "//", "/", -1) // test(replace `/` => `/`)
	rpath := cp.To.Path[0] + "/" + relpath

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

		// copy file
		// TODO(blacknon): Outputからプログレスバーで出力できるようにする(io.MultiWriterを利用して書き込み？)
		err = cp.pushFile(lf, ftp, rpath)
		if err != nil {
			return err
		}
	}

	// set mode
	if cp.Permission {
		ftp.Chmod(rpath, fInfo.Mode())
	}

	fmt.Fprintf(ow, "%s => %s exit.\n", path, rpath)

	return
}

// pushfile put file to path.
func (cp *Scp) pushFile(file io.Reader, ftp *sftp.Client, path string) (err error) {
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

	// copy file
	io.Copy(rf, file)

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
		// TODO(): つくる。やってるさいちゅう。
		cp.viaPushPath(path, fclient[0], tclient)
	}
}

//
func (cp *Scp) viaPushPath(path string, fclient *ScpConnect, tclients []*ScpConnect) {
	// from ftp client
	fftp := fclient.Connect

	// create from sftp walker
	walker := fftp.Walk(path)

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
			file, err := fftp.Open(p)
			if err != nil {
				fmt.Fprintf(fow, "Error: %s\n", err)
				continue
			}

			exit := make(chan bool)
			for _, tc := range tclients {
				tclient := tc
				go func() {
					tclient.Output.Create(tclient.Server)
					tow := tclient.Output.NewWriter()

					cp.pushFile(file, tclient.Connect, p)
					exit <- true

					fmt.Fprintf(tow, "exit: %s\n", p)
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
			// set ftp client
			ftp := client.Connect

			// get output writer
			client.Output.Create(client.Server)
			ow := client.Output.NewWriter()

			// pull data
			cp.pullPath(ftp, ow, client.Server)

			exit <- true
		}()
	}

	// wait send data
	for i := 0; i < len(clients); i++ {
		<-exit
	}
	close(exit)
}

// walk return file path list ([]string).
func (cp *Scp) pullPath(ftp *sftp.Client, ow *io.PipeWriter, server string) (result []string) {
	// basedir
	baseDir := cp.To.Path[0]

	// if multi pull, servername add baseDir
	if len(cp.From.Server) > 1 {
		baseDir = filepath.Join(baseDir, server)
		os.Mkdir(baseDir, 0755)
	}

	// walk remote path
	for _, path := range cp.From.Path {
		walker := ftp.Walk(path)
		for walker.Step() {
			err := walker.Err()
			if err != nil {
				fmt.Fprintf(ow, "Error: %s\n", err)
				continue
			}

			p := walker.Path()
			lpath := filepath.Join(baseDir, p)

			stat := walker.Stat()
			if stat.IsDir() { // create dir
				os.Mkdir(lpath, 0755)
			} else { // create file
				rf, err := ftp.Open(p)
				if err != nil {
					fmt.Fprintf(ow, "Error: %s\n", err)
					continue
				}

				lf, err := os.OpenFile(lpath, os.O_RDWR|os.O_CREATE, 0644)
				if err != nil {
					fmt.Fprintf(ow, "Error: %s\n", err)
					continue
				}

				io.Copy(lf, rf)
			}

			// set mode
			if cp.Permission {
				os.Chmod(lpath, stat.Mode())
			}

			fmt.Fprintf(ow, "%s => %s exit.\n", path, lpath)
		}
	}

	// sort result
	sort.Strings(result)

	return result
}

// createScpConnects return []*ScpConnect.
func (cp *Scp) createScpConnects(targets []string) (result []*ScpConnect) {
	ch := make(chan bool)

	m := new(sync.Mutex)
	for _, target := range targets {
		server := target
		go func() {
			// ssh connect
			conn, err := cp.Run.createSshConnect(server)
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
			o := &Output{
				Templete:   oprompt,
				ServerList: targets,
				Conf:       cp.Config.Server[server],
				AutoColor:  true,
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

// // Run execute scp according to mode.
// func (r *RunScp) run(mode string, authMap map[string][]ssh.AuthMethod) {
// 	finished := make(chan bool)

// 	// set target list
// 	targetList := []string{}
// 	switch mode {
// 	case "push":
// 		targetList = r.To.Server
// 	case "pull":
// 		targetList = r.From.Server
// 	}

// 	for _, value := range targetList {
// 		target := value

// 		go func() {
// 			// config
// 			config := r.Config.Server[target]

// 			// create ssh connect
// 			con := new(sshlib.Connect)
// 			addr := config.Addr
// 			port := config.Port
// 			user := config.User
// 			err := con.CreateClient(addr, port, user, authMap[target])
// 			if err != nil {
// 				fmt.Fprintf(os.Stderr, "cannot connect %v, %v \n", target, err)
// 				finished <- true
// 				return
// 			}

// 			// create ssh session
// 			session, err := con.CreateSession()
// 			if err != nil {
// 				fmt.Fprintf(os.Stderr, "cannot connect %v, %v \n", target, err)
// 				finished <- true
// 				return
// 			}
// 			defer session.Close()

// 			// create scp client
// 			scp := new(scplib.SCPClient)
// 			scp.Permission = r.Permission
// 			scp.Session = session

// 			switch mode {
// 			case "push":
// 				r.push(target, scp)
// 			case "pull":
// 				r.pull(target, scp)
// 			}

// 			fmt.Fprintf(os.Stderr, "%v(%v) is finished.\n", target, mode)
// 			finished <- true
// 		}()
// 	}

// 	for i := 1; i <= len(targetList); i++ {
// 		<-finished
// 	}
// }

// // push file scp
// // TODO(blacknon): targetいらない気がする…
// func (r *RunScp) push(target string, scp *scplib.SCPClient) {
// 	var err error
// 	if r.From.IsRemote && r.To.IsRemote {
// 		err = scp.PutData(r.CopyData, r.To.Path[0])
// 		if err != nil {
// 			fmt.Fprintf(os.Stderr, "Failed to run %v \n", err)
// 		}
// 	} else {
// 		err = scp.PutFile(r.From.Path, r.To.Path[0])
// 		if err != nil {
// 			fmt.Fprintf(os.Stderr, "Failed to run %v \n", err)
// 		}
// 	}
// }

// // pull file scp
// func (r *RunScp) pull(target string, scp *scplib.SCPClient) {
// 	var err error

// 	// scp pull
// 	if r.From.IsRemote && r.To.IsRemote {
// 		r.CopyData, err = scp.GetData(r.From.Path)
// 	} else {
// 		toPath := createServersDir(target, r.From.Server, r.To.Path[0])
// 		err = scp.GetFile(r.From.Path, toPath)
// 	}

// 	if err != nil {
// 		fmt.Fprintf(os.Stderr, "Failed to run %v \n", err.Error())
// 	}
// }

// func createServersDir(target string, serverList []string, toPath string) (path string) {
// 	if len(serverList) > 1 {
// 		toDir := filepath.Dir(toPath)
// 		toBase := filepath.Base(toPath)
// 		serverDir := toDir + "/" + target

// 		err := os.Mkdir(serverDir, os.FileMode(uint32(0755)))
// 		if err != nil {
// 			fmt.Fprintln(os.Stderr, "Failed to run: "+err.Error())
// 		}

// 		if toDir != toBase {
// 			toPath = serverDir + "/" + toBase
// 		} else {
// 			toPath = serverDir + "/"
// 		}
// 	}

// 	path = toPath
// 	return
// }
