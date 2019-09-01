// Copyright (c) 2019 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package ssh

import (
	"fmt"
	"os"
	"sync"

	"github.com/blacknon/lssh/conf"
	"github.com/pkg/sftp"
	"github.com/vbauerster/mpb"
)

// TODO(blacknon): 転送時の進捗状況を表示するプログレスバーの表示はさせること

type RunSftp struct {
	// select server
	SelectServer []string

	// config
	Config conf.Config

	// Client
	Client map[string]*SftpConnect

	// ssh Run
	Run *Run

	// progress bar
	Progress   *mpb.Progress
	ProgressWG *sync.WaitGroup
}

type SftpConnect struct {
	// ssh connect
	Connect *sftp.Client

	// Output
	Output *Output

	// Current Directory
	PWD string
}

func (r *RunSftp) Start() {
	// Create AuthMap
	r.Run = new(Run)
	r.Run.ServerList = r.SelectServer
	r.Run.Conf = r.Config
	r.Run.createAuthMethodMap()

	// Create Sftp Connect
	r.Client = r.createSftpConnect(r.Run.ServerList)

	// Start sftp shell
	r.shell()
}

//
func (r *RunSftp) createSftpConnect(targets []string) (result map[string]*SftpConnect) {
	// init
	result = map[string]*SftpConnect{}

	ch := make(chan bool)
	m := new(sync.Mutex)
	for _, target := range targets {
		server := target
		go func() {
			// ssh connect
			conn, err := r.Run.createSshConnect(server)
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
				Conf:       r.Config.Server[server],
				AutoColor:  true,
				Progress:   r.Progress,
				ProgressWG: r.ProgressWG,
			}

			// create SftpConnect
			sftpCon := &SftpConnect{
				Connect: ftp,
				Output:  o,
			}

			// append result
			m.Lock()
			result[server] = sftpCon
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

//
// NOTE: リモートマシンからリモートマシンにコピーさせるような処理や、対象となるホストを個別に指定してコピーできるような仕組みをつくること！
func (r *RunSftp) cd(args string) {
	// for _, c := range r.Client {

	// }
}

func (r *RunSftp) chown(target, path string) {

}

// sftp put/pull function
// NOTE: リモートマシンからリモートマシンにコピーさせるような処理や、対象となるホストを個別に指定してコピーできるような仕組みをつくること！
func (r *RunSftp) cp(args string) {
	// finished := make(chan bool)

	// // set target list
	// targetList := []string{}
	// switch mode {
	// case "push":
	// 	targetList = r.To.Server
	// case "pull":
	// 	targetList = r.From.Server
	// }

	// for _, value := range targetList {
	// 	target := value
	// }
}

//
func (r *RunSftp) get(args string) {
	// pathがディレクトリかどうかのチェックが必要
	// remoteFile, err := sftp.Create("hello.txt")
	// localFile, err := os.Open("hello.txt")
	// io.Copy(remoteFile, localFile)

	// f, err := sftp.Create("hello.txt")
	// TODO(blacknon): io.Copy使うとよさそう？？
}

// list is stfp ls command.
func (r *RunSftp) ls(args string) (err error) {
	// TODO(blacknon): パース処理を行い、オプションが利用できるようにする
	// TODO(blacknon): PATH追加等の対応

	// set path
	path := "./"

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

		for _, f := range data {
			name := f.Name()
			fmt.Fprintf(w, "%s\n", name)
		}
	}
	// data, err := sftp.ReadDir(path)

	// // TODO(blacknon): lsのオプション相当のものを受け付けるようにする必要がある。
	// //                 それとも、単にdataを返すだけになるかも？？
	// for _, f := range data {
	// 	name := f.Name()
	// 	fmt.Println(name)
	// }
	return
}

//
func (r *RunSftp) put(args string) {
	// pathがディレクトリかどうかのチェックが必要
	// f, err := sftp.Open(path)
	// TODO(blacknon): io.Copy使うとよさそう？？
}
