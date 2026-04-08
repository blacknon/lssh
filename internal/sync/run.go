package sync

import (
	"context"
	"fmt"
	"os"
	syncpkg "sync"
	"time"

	conf "github.com/blacknon/lssh/internal/config"
	"github.com/blacknon/lssh/internal/output"
	sshl "github.com/blacknon/lssh/internal/ssh"
	"github.com/pkg/sftp"
	"github.com/vbauerster/mpb"
)

var (
	oprompt = "${SERVER} :: "
)

type Sync struct {
	Run *sshl.Run

	From SyncInfo
	To   SyncInfo

	Config      conf.Config
	Permission  bool
	Delete      bool
	ParallelNum int

	Progress   *mpb.Progress
	ProgressWG *syncpkg.WaitGroup
}

type SyncInfo struct {
	IsRemote bool
	Server   []string
	Path     []string
}

type SyncConnect struct {
	Server  string
	Connect *sftp.Client
	Output  *output.Output
	Pwd     string
}

func (s *Sync) Start() {
	slist := append([]string{}, s.To.Server...)
	slist = append(slist, s.From.Server...)

	s.Run = new(sshl.Run)
	s.Run.ServerList = slist
	s.Run.Conf = s.Config
	s.Run.CreateAuthMethodMap()

	s.ProgressWG = new(syncpkg.WaitGroup)
	s.Progress = mpb.New(mpb.WithWaitGroup(s.ProgressWG))

	switch {
	case s.From.IsRemote && s.To.IsRemote:
		s.remoteToRemote()
	case s.From.IsRemote && !s.To.IsRemote:
		s.remoteToLocal()
	case !s.From.IsRemote && s.To.IsRemote:
		s.localToRemote()
	}
}

func (s *Sync) localToRemote() {
	localFS, err := NewLocalFS()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		return
	}

	exit := make(chan bool, len(s.To.Server))
	for _, server := range s.To.Server {
		server := server
		go func() {
			conn := s.createSyncConnect(server, s.To.Server)
			if conn == nil {
				exit <- true
				return
			}
			defer conn.Connect.Close()

			remoteFS := NewRemoteFS(conn.Connect, conn.Pwd)
			plan, err := BuildPlan(localFS, remoteFS, s.From.Path, s.To.Path[0])
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %s\n", err)
				exit <- true
				return
			}

			if err := ApplyPlan(context.Background(), localFS, remoteFS, plan, ApplyOptions{
				Delete:      s.Delete,
				Permission:  s.Permission,
				ParallelNum: s.ParallelNum,
				Output:      conn.Output,
			}); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			}

			exit <- true
		}()
	}

	for i := 0; i < len(s.To.Server); i++ {
		<-exit
	}

	s.Progress.Wait()
	time.Sleep(300 * time.Millisecond)
}

func (s *Sync) remoteToLocal() {
	localFS, err := NewLocalFS()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		return
	}

	exit := make(chan bool, len(s.From.Server))
	for _, server := range s.From.Server {
		server := server
		go func() {
			conn := s.createSyncConnect(server, s.From.Server)
			if conn == nil {
				exit <- true
				return
			}
			defer conn.Connect.Close()

			destination := s.To.Path[0]
			if len(s.From.Server) > 1 {
				destination = localFS.Join(destination, server)
			}

			remoteFS := NewRemoteFS(conn.Connect, conn.Pwd)
			plan, err := BuildPlan(remoteFS, localFS, s.From.Path, destination)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %s\n", err)
				exit <- true
				return
			}

			if err := ApplyPlan(context.Background(), remoteFS, localFS, plan, ApplyOptions{
				Delete:      s.Delete,
				Permission:  s.Permission,
				ParallelNum: s.ParallelNum,
				Output:      conn.Output,
			}); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			}

			exit <- true
		}()
	}

	for i := 0; i < len(s.From.Server); i++ {
		<-exit
	}

	s.Progress.Wait()
	time.Sleep(300 * time.Millisecond)
}

func (s *Sync) remoteToRemote() {
	if len(s.From.Server) == 0 {
		fmt.Fprintln(os.Stderr, "Error: source server is not selected")
		return
	}

	sourceServer := s.From.Server[0]
	exit := make(chan bool, len(s.To.Server))
	for _, server := range s.To.Server {
		server := server
		go func() {
			srcConn := s.createSyncConnect(sourceServer, []string{sourceServer})
			dstConn := s.createSyncConnect(server, s.To.Server)
			if srcConn == nil || dstConn == nil {
				exit <- true
				return
			}
			defer srcConn.Connect.Close()
			defer dstConn.Connect.Close()

			srcFS := NewRemoteFS(srcConn.Connect, srcConn.Pwd)
			dstFS := NewRemoteFS(dstConn.Connect, dstConn.Pwd)
			plan, err := BuildPlan(srcFS, dstFS, s.From.Path, s.To.Path[0])
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %s\n", err)
				exit <- true
				return
			}

			if err := ApplyPlan(context.Background(), srcFS, dstFS, plan, ApplyOptions{
				Delete:      s.Delete,
				Permission:  s.Permission,
				ParallelNum: s.ParallelNum,
				Output:      dstConn.Output,
			}); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			}

			exit <- true
		}()
	}

	for i := 0; i < len(s.To.Server); i++ {
		<-exit
	}

	s.Progress.Wait()
	time.Sleep(300 * time.Millisecond)
}

func (s *Sync) createSyncConnect(server string, serverList []string) *SyncConnect {
	conn, err := s.Run.CreateSshConnectDirect(server)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s connect error: %s\n", server, err)
		return nil
	}
	if conn == nil || conn.Client == nil {
		fmt.Fprintf(os.Stderr, "%s connect error: ssh client is not available for sftp\n", server)
		return nil
	}

	client, err := sftp.NewClient(conn.Client)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s create client error: %s\n", server, err)
		return nil
	}

	pwd, err := client.Getwd()
	if err != nil {
		pwd = "."
	}

	o := &output.Output{
		Templete:   oprompt,
		ServerList: serverList,
		Conf:       s.Config.Server[server],
		AutoColor:  true,
		Progress:   s.Progress,
		ProgressWG: s.ProgressWG,
	}
	o.Create(server)

	return &SyncConnect{
		Server:  server,
		Connect: client,
		Output:  o,
		Pwd:     pwd,
	}
}
