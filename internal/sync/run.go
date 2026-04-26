package sync

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	syncpkg "sync"
	"syscall"
	"time"

	conf "github.com/blacknon/lssh/internal/config"
	"github.com/blacknon/lssh/internal/output"
	sshl "github.com/blacknon/lssh/internal/ssh"
	"github.com/pkg/sftp"
	"github.com/vbauerster/mpb/v8"
)

var (
	oprompt = "${SERVER} :: "
)

type Sync struct {
	Run *sshl.Run

	// ControlMasterOverride temporarily overrides the config value for this
	// command execution.
	ControlMasterOverride *bool
	DryRun                bool

	From SyncInfo
	To   SyncInfo

	Config         conf.Config
	Permission     bool
	Delete         bool
	Daemon         bool
	Bidirectional  bool
	DaemonInterval time.Duration
	ParallelNum    int

	Progress   *mpb.Progress
	ProgressWG *syncpkg.WaitGroup
}

type SyncInfo struct {
	IsRemote    bool
	Server      []string
	Path        []string
	DisplayPath []string
}

type SyncConnect struct {
	Server  string
	Connect *sftp.Client
	Closer  io.Closer
	Output  *output.Output
	Pwd     string
}

func (s *SyncConnect) Close() error {
	if s == nil || s.Closer == nil {
		return nil
	}
	return s.Closer.Close()
}

func (s *Sync) Start() {
	slist := append([]string{}, s.To.Server...)
	slist = append(slist, s.From.Server...)

	s.Run = new(sshl.Run)
	s.Run.ServerList = slist
	s.Run.Conf = s.Config
	s.Run.ControlMasterOverride = s.ControlMasterOverride
	s.Run.CreateAuthMethodMap()

	s.ProgressWG = new(syncpkg.WaitGroup)
	s.Progress = mpb.New(
		mpb.WithWaitGroup(s.ProgressWG),
		mpb.WithRefreshRate(40*time.Millisecond),
		mpb.PopCompletedMode(),
	)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	switch {
	case s.Bidirectional:
		s.bidirectional(ctx)
	case s.From.IsRemote && s.To.IsRemote:
		s.remoteToRemote(ctx)
	case s.From.IsRemote && !s.To.IsRemote:
		s.remoteToLocal(ctx)
	case !s.From.IsRemote && s.To.IsRemote:
		s.localToRemote(ctx)
	}
}

func (s *Sync) localToRemote(ctx context.Context) {
	localFS, err := NewLocalFS()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		return
	}

	exit := make(chan bool, len(s.To.Server))
	for _, server := range s.To.Server {
		server := server
		go func() {
			s.runLoop(ctx, func(loopCtx context.Context) error {
				return s.localToRemoteOnce(loopCtx, localFS, server)
			})
			exit <- true
		}()
	}

	for i := 0; i < len(s.To.Server); i++ {
		<-exit
	}

	s.Progress.Wait()
}

func (s *Sync) remoteToLocal(ctx context.Context) {
	localFS, err := NewLocalFS()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		return
	}

	exit := make(chan bool, len(s.From.Server))
	for _, server := range s.From.Server {
		server := server
		go func() {
			s.runLoop(ctx, func(loopCtx context.Context) error {
				return s.remoteToLocalOnce(loopCtx, localFS, server)
			})
			exit <- true
		}()
	}

	for i := 0; i < len(s.From.Server); i++ {
		<-exit
	}

	s.Progress.Wait()
}

func (s *Sync) remoteToRemote(ctx context.Context) {
	if len(s.From.Server) == 0 {
		fmt.Fprintln(os.Stderr, "Error: source server is not selected")
		return
	}

	sourceServer := s.From.Server[0]
	exit := make(chan bool, len(s.To.Server))
	for _, server := range s.To.Server {
		server := server
		go func() {
			s.runLoop(ctx, func(loopCtx context.Context) error {
				return s.remoteToRemoteOnce(loopCtx, sourceServer, server)
			})
			exit <- true
		}()
	}

	for i := 0; i < len(s.To.Server); i++ {
		<-exit
	}

	s.Progress.Wait()
}

func (s *Sync) bidirectional(ctx context.Context) {
	if s.Delete {
		fmt.Fprintln(os.Stderr, "Error: --delete is not supported with bidirectional sync.")
		return
	}
	if len(s.From.Path) != 1 || len(s.To.Path) != 1 {
		fmt.Fprintln(os.Stderr, "Error: bidirectional sync requires exactly one source path and one destination path.")
		return
	}

	switch {
	case !s.From.IsRemote && s.To.IsRemote:
		s.bidirectionalLocalRemote(ctx)
	case s.From.IsRemote && !s.To.IsRemote:
		s.bidirectionalRemoteLocal(ctx)
	case s.From.IsRemote && s.To.IsRemote:
		s.bidirectionalRemoteRemote(ctx)
	default:
		fmt.Fprintln(os.Stderr, "Error: LOCAL to LOCAL sync is not supported.")
	}
}

func (s *Sync) bidirectionalLocalRemote(ctx context.Context) {
	localFS, err := NewLocalFS()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		return
	}

	s.runBidirectionalLoop(ctx, s.To.Server, func(loopCtx context.Context, server string) error {
		return s.bidirectionalLocalRemoteOnce(loopCtx, localFS, server)
	})

	s.Progress.Wait()
}

func (s *Sync) bidirectionalRemoteLocal(ctx context.Context) {
	localFS, err := NewLocalFS()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		return
	}

	s.runBidirectionalLoop(ctx, s.From.Server, func(loopCtx context.Context, server string) error {
		return s.bidirectionalRemoteLocalOnce(loopCtx, localFS, server)
	})

	s.Progress.Wait()
}

func (s *Sync) bidirectionalRemoteRemote(ctx context.Context) {
	if len(s.From.Server) == 0 {
		fmt.Fprintln(os.Stderr, "Error: source server is not selected")
		return
	}
	sourceServer := s.From.Server[0]

	s.runBidirectionalLoop(ctx, s.To.Server, func(loopCtx context.Context, server string) error {
		return s.bidirectionalRemoteRemoteOnce(loopCtx, sourceServer, server)
	})

	s.Progress.Wait()
}

func (s *Sync) runLoop(ctx context.Context, fn func(context.Context) error) {
	run := func(loopCtx context.Context) error {
		if err := fn(loopCtx); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		}
		return nil
	}

	if s.Daemon {
		_ = RunDaemonLoop(ctx, s.DaemonInterval, run)
		return
	}

	_ = run(ctx)
}

func (s *Sync) runBidirectionalLoop(ctx context.Context, servers []string, fn func(context.Context, string) error) {
	runCycle := func(loopCtx context.Context) error {
		for _, server := range servers {
			if err := fn(loopCtx, server); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			}
		}
		return nil
	}

	if s.Daemon {
		_ = RunDaemonLoop(ctx, s.DaemonInterval, runCycle)
		return
	}

	_ = runCycle(ctx)
}

func (s *Sync) localToRemoteOnce(ctx context.Context, localFS FileSystem, server string) error {
	conn := s.createSyncConnect(server, s.To.Server)
	if conn == nil {
		return fmt.Errorf("%s connect error", server)
	}
	defer conn.Close()

	remoteFS := NewRemoteFS(conn.Connect, conn.Pwd)
	plan, err := BuildPlan(localFS, remoteFS, s.From.Path, s.To.Path[0])
	if err != nil {
		return err
	}

	return ApplyPlan(ctx, localFS, remoteFS, plan, ApplyOptions{
		Delete:            s.Delete,
		DryRun:            s.DryRun,
		Permission:        s.Permission,
		ParallelNum:       s.ParallelNum,
		Output:            conn.Output,
		SourceLabel:       "local",
		TargetLabel:       server,
		SourcePathDisplay: newDisplayPathResolver(s.From.Path, s.From.DisplayPath),
	})
}

func (s *Sync) remoteToLocalOnce(ctx context.Context, localFS FileSystem, server string) error {
	conn := s.createSyncConnect(server, s.From.Server)
	if conn == nil {
		return fmt.Errorf("%s connect error", server)
	}
	defer conn.Close()

	destination := s.To.Path[0]
	if len(s.From.Server) > 1 {
		destination = localFS.Join(destination, server)
	}

	remoteFS := NewRemoteFS(conn.Connect, conn.Pwd)
	plan, err := BuildPlan(remoteFS, localFS, s.From.Path, destination)
	if err != nil {
		return err
	}

	return ApplyPlan(ctx, remoteFS, localFS, plan, ApplyOptions{
		Delete:            s.Delete,
		DryRun:            s.DryRun,
		Permission:        s.Permission,
		ParallelNum:       s.ParallelNum,
		Output:            conn.Output,
		SourceLabel:       server,
		TargetLabel:       "local",
		TargetPathDisplay: destinationDisplayResolver(s.To.Path[0], s.displayPathAt(s.To.DisplayPath, 0, s.To.Path[0]), len(s.From.Server) > 1, server),
	})
}

func (s *Sync) remoteToRemoteOnce(ctx context.Context, sourceServer string, targetServer string) error {
	srcConn := s.createSyncConnect(sourceServer, []string{sourceServer})
	dstConn := s.createSyncConnect(targetServer, s.To.Server)
	if srcConn == nil || dstConn == nil {
		return fmt.Errorf("remote connection error")
	}
	defer srcConn.Close()
	defer dstConn.Close()

	srcFS := NewRemoteFS(srcConn.Connect, srcConn.Pwd)
	dstFS := NewRemoteFS(dstConn.Connect, dstConn.Pwd)
	plan, err := BuildPlan(srcFS, dstFS, s.From.Path, s.To.Path[0])
	if err != nil {
		return err
	}

	return ApplyPlan(ctx, srcFS, dstFS, plan, ApplyOptions{
		Delete:      s.Delete,
		DryRun:      s.DryRun,
		Permission:  s.Permission,
		ParallelNum: s.ParallelNum,
		Output:      dstConn.Output,
		SourceLabel: sourceServer,
		TargetLabel: targetServer,
	})
}

func (s *Sync) bidirectionalLocalRemoteOnce(ctx context.Context, localFS FileSystem, server string) error {
	conn := s.createSyncConnect(server, s.To.Server)
	if conn == nil {
		return fmt.Errorf("%s connect error", server)
	}
	defer conn.Close()

	remoteFS := NewRemoteFS(conn.Connect, conn.Pwd)
	leftToRight, rightToLeft, err := BuildBidirectionalPlans(localFS, remoteFS, s.From.Path[0], s.To.Path[0])
	if err != nil {
		return err
	}

	if err := ApplyPlan(ctx, localFS, remoteFS, leftToRight, ApplyOptions{
		DryRun:            s.DryRun,
		Permission:        s.Permission,
		ParallelNum:       s.ParallelNum,
		Output:            conn.Output,
		SourceLabel:       "local",
		TargetLabel:       server,
		SourcePathDisplay: newDisplayPathResolver(s.From.Path, s.From.DisplayPath),
	}); err != nil {
		return err
	}

	return ApplyPlan(ctx, remoteFS, localFS, rightToLeft, ApplyOptions{
		DryRun:            s.DryRun,
		Permission:        s.Permission,
		ParallelNum:       s.ParallelNum,
		Output:            conn.Output,
		SourceLabel:       server,
		TargetLabel:       "local",
		TargetPathDisplay: newDisplayPathResolver(s.From.Path, s.From.DisplayPath),
	})
}

func (s *Sync) bidirectionalRemoteLocalOnce(ctx context.Context, localFS FileSystem, server string) error {
	conn := s.createSyncConnect(server, s.From.Server)
	if conn == nil {
		return fmt.Errorf("%s connect error", server)
	}
	defer conn.Close()

	destination := s.To.Path[0]
	if len(s.From.Server) > 1 {
		destination = localFS.Join(destination, server)
	}

	remoteFS := NewRemoteFS(conn.Connect, conn.Pwd)
	leftToRight, rightToLeft, err := BuildBidirectionalPlans(remoteFS, localFS, s.From.Path[0], destination)
	if err != nil {
		return err
	}

	if err := ApplyPlan(ctx, remoteFS, localFS, leftToRight, ApplyOptions{
		DryRun:            s.DryRun,
		Permission:        s.Permission,
		ParallelNum:       s.ParallelNum,
		Output:            conn.Output,
		SourceLabel:       server,
		TargetLabel:       "local",
		TargetPathDisplay: destinationDisplayResolver(s.To.Path[0], s.displayPathAt(s.To.DisplayPath, 0, s.To.Path[0]), len(s.From.Server) > 1, server),
	}); err != nil {
		return err
	}

	return ApplyPlan(ctx, localFS, remoteFS, rightToLeft, ApplyOptions{
		DryRun:            s.DryRun,
		Permission:        s.Permission,
		ParallelNum:       s.ParallelNum,
		Output:            conn.Output,
		SourceLabel:       "local",
		TargetLabel:       server,
		SourcePathDisplay: destinationDisplayResolver(s.To.Path[0], s.displayPathAt(s.To.DisplayPath, 0, s.To.Path[0]), len(s.From.Server) > 1, server),
	})
}

func (s *Sync) bidirectionalRemoteRemoteOnce(ctx context.Context, sourceServer string, targetServer string) error {
	srcConn := s.createSyncConnect(sourceServer, []string{sourceServer})
	dstConn := s.createSyncConnect(targetServer, s.To.Server)
	if srcConn == nil || dstConn == nil {
		return fmt.Errorf("remote connection error")
	}
	defer srcConn.Close()
	defer dstConn.Close()

	srcFS := NewRemoteFS(srcConn.Connect, srcConn.Pwd)
	dstFS := NewRemoteFS(dstConn.Connect, dstConn.Pwd)
	leftToRight, rightToLeft, err := BuildBidirectionalPlans(srcFS, dstFS, s.From.Path[0], s.To.Path[0])
	if err != nil {
		return err
	}

	if err := ApplyPlan(ctx, srcFS, dstFS, leftToRight, ApplyOptions{
		DryRun:      s.DryRun,
		Permission:  s.Permission,
		ParallelNum: s.ParallelNum,
		Output:      dstConn.Output,
		SourceLabel: sourceServer,
		TargetLabel: targetServer,
	}); err != nil {
		return err
	}

	return ApplyPlan(ctx, dstFS, srcFS, rightToLeft, ApplyOptions{
		DryRun:      s.DryRun,
		Permission:  s.Permission,
		ParallelNum: s.ParallelNum,
		Output:      srcConn.Output,
		SourceLabel: targetServer,
		TargetLabel: sourceServer,
	})
}

func (s *Sync) createSyncConnect(server string, serverList []string) *SyncConnect {
	client, closer, err := s.Run.CreateSFTPClient(server)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s connect error: %s\n", server, err)
		return nil
	}
	if client == nil {
		if closer != nil {
			_ = closer.Close()
		}
		fmt.Fprintf(os.Stderr, "%s connect error: sftp client is not available\n", server)
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
		Closer:  closer,
		Output:  o,
		Pwd:     pwd,
	}
}

func (s *Sync) displayPathAt(displayPaths []string, index int, fallback string) string {
	if index >= 0 && index < len(displayPaths) && strings.TrimSpace(displayPaths[index]) != "" {
		return displayPaths[index]
	}
	return fallback
}

func newDisplayPathResolver(actuals, displays []string) func(string) string {
	type rule struct {
		actual  string
		display string
	}

	rules := make([]rule, 0, len(actuals))
	for i, actual := range actuals {
		display := actual
		if i < len(displays) && strings.TrimSpace(displays[i]) != "" {
			display = displays[i]
		}
		rules = append(rules, rule{actual: filepath.Clean(actual), display: display})
	}

	return func(path string) string {
		path = filepath.Clean(path)
		for _, rule := range rules {
			if got, ok := displayLocalPath(rule.actual, rule.display, path); ok {
				return got
			}
		}
		return path
	}
}

func destinationDisplayResolver(actualRoot, displayRoot string, appendServer bool, server string) func(string) string {
	actual := filepath.Clean(actualRoot)
	display := displayRoot
	if appendServer {
		actual = filepath.Join(actual, server)
		display = filepath.Join(display, server)
		if strings.HasPrefix(displayRoot, "."+string(filepath.Separator)) &&
			!strings.HasPrefix(display, "."+string(filepath.Separator)) {
			display = "." + string(filepath.Separator) + display
		}
	}

	return func(path string) string {
		if got, ok := displayLocalPath(actual, display, path); ok {
			return got
		}
		return path
	}
}

func displayLocalPath(root, displayRoot, current string) (string, bool) {
	root = filepath.Clean(root)
	current = filepath.Clean(current)
	displayRoot = strings.TrimSpace(displayRoot)
	if displayRoot == "" {
		return current, false
	}

	if current == root {
		return displayRoot, true
	}

	relpath, err := filepath.Rel(root, current)
	if err != nil || relpath == "." || strings.HasPrefix(relpath, ".."+string(filepath.Separator)) || relpath == ".." {
		return "", false
	}

	joined := filepath.Join(displayRoot, relpath)
	if strings.HasPrefix(displayRoot, "."+string(filepath.Separator)) &&
		!strings.HasPrefix(joined, "."+string(filepath.Separator)) &&
		!strings.HasPrefix(joined, ".."+string(filepath.Separator)) {
		return "." + string(filepath.Separator) + joined, true
	}

	return joined, true
}
