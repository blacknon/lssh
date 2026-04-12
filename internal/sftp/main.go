// Copyright (c) 2022 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package sftp

import (
	"fmt"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/blacknon/go-sshlib"
	"github.com/blacknon/lssh/internal/common"
	conf "github.com/blacknon/lssh/internal/config"
	"github.com/blacknon/lssh/internal/output"
	sshl "github.com/blacknon/lssh/internal/ssh"
	"github.com/c-bata/go-prompt"
	"github.com/pkg/sftp"
	"github.com/vbauerster/mpb"
)

// TODO(blacknon): Ctrl + Cでコマンドの処理をキャンセルできるようにする

// RunSftp struct sftp run
type RunSftp struct {
	// select server
	SelectServer []string

	// config
	Config conf.Config

	// Client
	Client map[string]*SftpConnect

	// Complete select client
	TargetClient map[string]*SftpConnect

	// ssh Run
	Run *sshl.Run

	// ControlMasterOverride temporarily overrides the config value for this
	// command execution.
	ControlMasterOverride *bool
	DryRun                bool
	AutoReconnect         bool

	//
	Permission bool

	// local umask. [000-777]
	LocalUmask []string

	// progress bar
	Progress   *mpb.Progress
	ProgressWG *sync.WaitGroup

	// PathComplete
	RemoteComplete []prompt.Suggest
	LocalComplete  []prompt.Suggest

	mu sync.RWMutex
}

// SftpConnect struct at sftp client
type SftpConnect struct {
	Server string

	// ssh connect
	SshConnect *sshlib.Connect

	// sftp connect
	Connect *sftp.Client

	// Output
	Output *output.Output

	// Current Directory
	Pwd string

	Connected bool
	LastError string
}

type TargetConnectMap struct {
	SftpConnect

	// Target Path list
	Path []string
}

// PathSet struct at path data
type PathSet struct {
	Root      string
	RootIsDir bool
	PathSlice []string
}

func (r *RunSftp) printAction(printer *output.Output, action, target string) {
	if printer != nil {
		ow := printer.NewWriter()
		fmt.Fprintf(ow, "[DRY-RUN] %s: %s\n", action, target)
		ow.Close()
		return
	}

	fmt.Fprintf(os.Stderr, "[DRY-RUN] %s: %s\n", action, target)
}

var (
	oprompt = "${SERVER} :: "
)

// Start sftp shell
func (r *RunSftp) Start() {
	// Create AuthMap
	r.Run = new(sshl.Run)
	r.Run.ServerList = r.SelectServer
	r.Run.Conf = r.Config
	r.Run.ControlMasterOverride = r.ControlMasterOverride
	r.Run.CreateAuthMethodMap()

	// Default local umask(022).
	r.LocalUmask = []string{"0", "2", "2"}

	// Create Sftp Connect
	r.Client = r.createSftpConnect(r.Run.ServerList)

	if len(r.Client) == 0 {
		fmt.Fprintf(os.Stderr, "Error: No server to connect.\n")
		return
	}

	// check keepalive
	go func() {
		for {
			r.checkKeepalive()
			time.Sleep(3 * time.Second)
		}
	}()

	// Start sftp shell
	r.shell()

}

func (r *RunSftp) createSftpConnect(targets []string) (result map[string]*SftpConnect) {
	// init
	result = map[string]*SftpConnect{}

	ch := make(chan bool)
	m := new(sync.Mutex)
	for _, target := range targets {
		server := target
		go func() {
			// ssh connect
			conn, err := r.Run.CreateSshConnectDirect(server)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s connect error: %s\n", server, err)
				ch <- true
				return
			}
			if conn == nil || conn.Client == nil {
				fmt.Fprintf(os.Stderr, "%s connect error: ssh client is not available for sftp\n", server)
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
				Conf:       r.Config.Server[server],
				AutoColor:  true,
			}

			// create SftpConnect
			sftpCon := &SftpConnect{
				Server:     server,
				SshConnect: conn,
				Connect:    ftp,
				Output:     o,
				Pwd:        "./",
				Connected:  true,
			}

			if err := validateSFTPConnection(sftpCon); err != nil {
				fmt.Fprintf(os.Stderr, "%s create client error: %s\n", server, err)
				_ = ftp.Close()
				_ = conn.Close()
				ch <- true
				return
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

func (r *RunSftp) getClient(server string) (*SftpConnect, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	client, ok := r.Client[server]
	return client, ok
}

func (r *RunSftp) setClient(client *SftpConnect) {
	if client == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Client == nil {
		r.Client = map[string]*SftpConnect{}
	}
	r.Client[client.Server] = client
}

func (r *RunSftp) setDisconnected(server string, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	client, ok := r.Client[server]
	if !ok || client == nil {
		return
	}
	client.Connected = false
	if err != nil {
		client.LastError = err.Error()
	}
}

func (r *RunSftp) listClients() []*SftpConnect {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*SftpConnect, 0, len(r.Client))
	for _, client := range r.Client {
		if client == nil {
			continue
		}
		result = append(result, client)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Server < result[j].Server
	})
	return result
}

func (r *RunSftp) reconnect(server string) error {
	old, ok := r.getClient(server)
	if !ok || old == nil {
		return fmt.Errorf("host %s not found", server)
	}

	pwd := old.Pwd
	connMap := r.createSftpConnect([]string{server})
	client, ok := connMap[server]
	if !ok || client == nil {
		return fmt.Errorf("reconnect failed: %s", server)
	}
	if pwd != "" {
		client.Pwd = pwd
	}
	client.Output = old.Output
	client.Connected = true
	client.LastError = ""
	if old.Connect != nil {
		_ = old.Connect.Close()
	}
	if old.SshConnect != nil {
		_ = old.SshConnect.Close()
	}
	r.setClient(client)
	return nil
}

func (r *RunSftp) reconnectDisconnected() {
	for _, client := range r.listClients() {
		if client == nil || client.Connected {
			continue
		}
		if err := r.reconnect(client.Server); err != nil {
			fmt.Fprintf(os.Stderr, "%s reconnect error: %s\n", client.Server, err)
		}
	}
}

func (r *RunSftp) status(args []string) {
	fmt.Printf("AutoReconnect: %t\n", r.AutoReconnect)
	for _, client := range r.listClients() {
		state := "connected"
		if !client.Connected {
			state = "disconnected"
		}
		if client.LastError != "" {
			fmt.Printf("%s\t%s\t%s\n", client.Server, state, client.LastError)
		} else {
			fmt.Printf("%s\t%s\n", client.Server, state)
		}
	}
}

func (r *RunSftp) reconnectCommand(args []string) {
	targets := args[1:]
	if len(targets) == 0 {
		for _, client := range r.listClients() {
			if client != nil && !client.Connected {
				targets = append(targets, client.Server)
			}
		}
	}

	if len(targets) == 0 {
		fmt.Println("No disconnected hosts.")
		return
	}

	for _, server := range targets {
		if err := r.reconnect(server); err != nil {
			fmt.Fprintf(os.Stderr, "%s reconnect error: %s\n", server, err)
			continue
		}
		fmt.Printf("%s reconnected\n", server)
	}
}

// createTargetMap is a function that adds elements to the passed TargetConnectMap as a set (map) of connection destination host and target path to regenerate and return TargetConnectMap.
func (r *RunSftp) createTargetMap(srcTargetMap map[string]*TargetConnectMap, pathline string) (targetMap map[string]*TargetConnectMap) {
	// sftp target host
	targetMap = srcTargetMap

	// get r.Client keys
	clients := r.listClients()
	servers := make([]string, 0, len(clients))
	for _, client := range clients {
		if client == nil {
			continue
		}
		k := client.Server
		servers = append(servers, k)
	}

	// parse pathline
	targetList, path := common.ParseHostPath(pathline)

	if len(targetList) == 0 {
		targetList = servers
	}

	// check exist server.
	for _, t := range targetList {
		if !common.Contains(servers, t) {
			fmt.Fprintf(os.Stderr, "Error: host %s not found.\n", t)
			continue
		}
	}

	// create targetMap
	for _, client := range clients {
		if client == nil {
			continue
		}
		server := client.Server
		if common.Contains(targetList, server) {
			if err := ensureSFTPClientAvailable(client); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %s. Use reconnect or enable auto reconnect.\n", err)
				continue
			}
			if _, ok := targetMap[server]; !ok {
				targetMap[server] = &TargetConnectMap{}
				targetMap[server].SftpConnect = *client
			}

			// append path
			targetMap[server].Path = append(targetMap[server].Path, path)
		}
	}

	return targetMap
}

func (r *RunSftp) createConnectedTargetMapAll() map[string]*TargetConnectMap {
	targetMap := map[string]*TargetConnectMap{}
	for _, client := range r.listClients() {
		if err := ensureSFTPClientAvailable(client); err != nil {
			continue
		}

		targetMap[client.Server] = &TargetConnectMap{
			SftpConnect: *client,
		}
	}

	return targetMap
}

// checkKeepalive
func (r *RunSftp) checkKeepalive() {
	ch := make(chan bool)
	clients := r.listClients()

	for _, client := range clients {
		c := client
		go func() {
			if c == nil || c.SshConnect == nil {
				ch <- true
				return
			}
			if !c.Connected {
				ch <- true
				return
			}

			// keepalive
			err := c.SshConnect.CheckClientAlive()
			if err == nil {
				err = validateSFTPConnection(c)
			}

			// check error
			if err != nil {
				// error
				if c.Connected {
					fmt.Fprintf(os.Stderr, "Exit Connect %s, Error: %s\n", c.Server, err)
				}

				// close sftp client
				if c.Connect != nil {
					_ = c.Connect.Close()
					c.Connect = nil
				}
				if c.SshConnect != nil && c.SshConnect.Client != nil {
					_ = c.SshConnect.Client.Close()
				}
				r.setDisconnected(c.Server, err)
			} else {
				r.mu.Lock()
				if current, ok := r.Client[c.Server]; ok && current != nil {
					current.Connected = true
					current.LastError = ""
				}
				r.mu.Unlock()
			}

			ch <- true
		}()
	}

	// wait
	for i := 0; i < len(clients); i++ {
		<-ch
	}

	return
}

func validateSFTPConnection(client *SftpConnect) error {
	if client == nil || client.Connect == nil {
		return fmt.Errorf("sftp client is not available")
	}

	if _, err := client.Connect.Getwd(); err == nil {
		return nil
	}

	_, err := client.Connect.Stat(".")
	return err
}
