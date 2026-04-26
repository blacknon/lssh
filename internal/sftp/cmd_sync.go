package sftp

import (
	"context"
	"fmt"
	"os"
	"strings"
	syncpkg "sync"
	"time"

	"github.com/blacknon/lssh/internal/common"
	lsync "github.com/blacknon/lssh/internal/sync"
	"github.com/urfave/cli"
	"github.com/vbauerster/mpb/v8"
)

func (r *RunSftp) sync(args []string) {
	app := cli.NewApp()

	app.CustomAppHelpTemplate = helptext
	app.Name = "sync"
	app.Usage = "lsftp build-in command: one-way sync"
	app.ArgsUsage = "[options] (local|remote):source... (local|remote):target"
	app.HideHelp = true
	app.HideVersion = true
	app.EnableBashCompletion = true
	app.Flags = []cli.Flag{
		cli.BoolFlag{Name: "daemon,D", Usage: "run as a daemon and repeat sync at each interval"},
		cli.DurationFlag{Name: "daemon-interval", Value: 5 * time.Second, Usage: "daemon sync interval"},
		cli.BoolFlag{Name: "bidirectional,B", Usage: "sync both sides and copy newer changes in either direction"},
		cli.IntFlag{Name: "parallel,P", Value: 1, Usage: "parallel file sync count per host"},
		cli.BoolFlag{Name: "permission,p", Usage: "copy file permission"},
		cli.BoolFlag{Name: "dry-run", Usage: "show sync actions without modifying files"},
		cli.BoolFlag{Name: "delete", Usage: "delete destination entries that do not exist in source"},
	}

	app.Action = func(c *cli.Context) error {
		knownHosts := make([]string, 0, len(r.Client))
		for host := range r.Client {
			knownHosts = append(knownHosts, host)
		}

		parsed, err := lsync.ParseCommandArgs(args)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			fmt.Println("sync [options] (local|remote):source... (local|remote):target")
			return nil
		}

		sourceSpecs := make([]lsync.PathSpec, 0, len(parsed.Sources))
		isSourceRemote := false
		isSourceLocal := false
		for _, raw := range parsed.Sources {
			spec, err := lsync.ParsePathSpecWithHosts(raw, knownHosts)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %s\n", err)
				return nil
			}
			if spec.IsRemote {
				isSourceRemote = true
			} else {
				isSourceLocal = true
			}
			sourceSpecs = append(sourceSpecs, spec)
		}

		targetSpec, err := lsync.ParsePathSpecWithHosts(parsed.Destination, knownHosts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			return nil
		}

		if isSourceRemote && isSourceLocal {
			fmt.Fprintln(os.Stderr, "Error: can not mix LOCAL and REMOTE in source paths.")
			return nil
		}
		if !isSourceRemote && !targetSpec.IsRemote {
			fmt.Fprintln(os.Stderr, "Error: LOCAL to LOCAL sync is not supported.")
			return nil
		}

		permission := c.Bool("permission") || parsed.Permission
		parallelNum := c.Int("parallel")
		if parallelNum < 1 {
			parallelNum = parsed.ParallelNum
		}
		if parallelNum < 1 {
			parallelNum = 1
		}
		deleteExtra := c.Bool("delete") || parsed.Delete
		r.DryRun = c.Bool("dry-run") || parsed.DryRun
		bidirectional := c.Bool("bidirectional") || parsed.Bidirectional
		daemon := c.Bool("daemon") || parsed.Daemon
		daemonInterval := c.Duration("daemon-interval")
		if daemonInterval <= 0 {
			daemonInterval = parsed.DaemonInterval
		}
		if daemonInterval <= 0 {
			daemonInterval = 5 * time.Second
		}

		r.ProgressWG = new(syncpkg.WaitGroup)
		r.Progress = mpb.New(
			mpb.WithWaitGroup(r.ProgressWG),
			mpb.WithRefreshRate(40*time.Millisecond),
			mpb.PopCompletedMode(),
		)

		if bidirectional {
			if deleteExtra {
				err = fmt.Errorf("--delete is not supported with bidirectional sync")
			} else if len(sourceSpecs) != 1 {
				err = fmt.Errorf("bidirectional sync requires exactly one source path")
			} else {
				switch {
				case !isSourceRemote && targetSpec.IsRemote:
					err = r.syncBidirectionalLocalToRemote(sourceSpecs[0], targetSpec, parallelNum, permission, daemon, daemonInterval)
				case isSourceRemote && !targetSpec.IsRemote:
					err = r.syncBidirectionalRemoteToLocal(sourceSpecs[0], targetSpec, parallelNum, permission, daemon, daemonInterval)
				case isSourceRemote && targetSpec.IsRemote:
					err = r.syncBidirectionalRemoteToRemote(sourceSpecs[0], targetSpec, parallelNum, permission, daemon, daemonInterval)
				default:
					err = fmt.Errorf("LOCAL to LOCAL sync is not supported")
				}
			}
		} else {
			switch {
			case !isSourceRemote && targetSpec.IsRemote:
				err = r.syncLocalToRemote(sourceSpecs, targetSpec, parallelNum, deleteExtra, permission, daemon, daemonInterval)
			case isSourceRemote && !targetSpec.IsRemote:
				err = r.syncRemoteToLocal(sourceSpecs, targetSpec, parallelNum, deleteExtra, permission, daemon, daemonInterval)
			case isSourceRemote && targetSpec.IsRemote:
				err = r.syncRemoteToRemote(sourceSpecs, targetSpec, parallelNum, deleteExtra, permission, daemon, daemonInterval)
			}
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		}

		r.Progress.Wait()
		return nil
	}

	args = common.ParseArgs(app.Flags, args)
	app.Run(args)
}

func (r *RunSftp) syncLocalToRemote(sourceSpecs []lsync.PathSpec, targetSpec lsync.PathSpec, parallelNum int, deleteExtra, permission bool, daemon bool, daemonInterval time.Duration) error {
	localFS, err := lsync.NewLocalFS()
	if err != nil {
		return err
	}

	targets := r.syncCreateTargetMap(targetSpec)
	if len(targets) == 0 {
		return fmt.Errorf("target host not found")
	}

	sourcePaths := make([]string, 0, len(sourceSpecs))
	for _, spec := range sourceSpecs {
		sourcePaths = append(sourcePaths, spec.Path)
	}

	for server, target := range targets {
		target := target
		if err := r.syncBidirectionalLoop(daemon, daemonInterval, []string{server}, func(ctx context.Context, _ string) error {
			target.Output.Create(server)
			target.Output.Progress = r.Progress
			target.Output.ProgressWG = r.ProgressWG

			remoteFS := lsync.NewRemoteFS(target.Connect, target.Pwd)
			plan, err := lsync.BuildPlan(localFS, remoteFS, sourcePaths, target.Path[0])
			if err != nil {
				return err
			}

			return lsync.ApplyPlan(ctx, localFS, remoteFS, plan, lsync.ApplyOptions{
				Delete:      deleteExtra,
				DryRun:      r.DryRun,
				Permission:  permission,
				ParallelNum: parallelNum,
				Output:      target.Output,
				SourceLabel: "local",
				TargetLabel: server,
			})
		}); err != nil {
			return err
		}
	}

	return nil
}

func (r *RunSftp) syncRemoteToLocal(sourceSpecs []lsync.PathSpec, targetSpec lsync.PathSpec, parallelNum int, deleteExtra, permission bool, daemon bool, daemonInterval time.Duration) error {
	localFS, err := lsync.NewLocalFS()
	if err != nil {
		return err
	}

	sources := map[string]*TargetConnectMap{}
	for _, spec := range sourceSpecs {
		sources = r.syncCreateTargetMapWithSeed(sources, spec)
	}
	if len(sources) == 0 {
		return fmt.Errorf("source host not found")
	}

	for server, source := range sources {
		source := source
		if err := r.syncBidirectionalLoop(daemon, daemonInterval, []string{server}, func(ctx context.Context, _ string) error {
			source.Output.Create(server)
			source.Output.Progress = r.Progress
			source.Output.ProgressWG = r.ProgressWG

			destination := targetSpec.Path
			if len(sources) > 1 {
				destination = localFS.Join(destination, server)
			}

			remoteFS := lsync.NewRemoteFS(source.Connect, source.Pwd)
			plan, err := lsync.BuildPlan(remoteFS, localFS, source.Path, destination)
			if err != nil {
				return err
			}

			return lsync.ApplyPlan(ctx, remoteFS, localFS, plan, lsync.ApplyOptions{
				Delete:      deleteExtra,
				DryRun:      r.DryRun,
				Permission:  permission,
				ParallelNum: parallelNum,
				Output:      source.Output,
				SourceLabel: server,
				TargetLabel: "local",
			})
		}); err != nil {
			return err
		}
	}

	return nil
}

func (r *RunSftp) syncRemoteToRemote(sourceSpecs []lsync.PathSpec, targetSpec lsync.PathSpec, parallelNum int, deleteExtra, permission bool, daemon bool, daemonInterval time.Duration) error {
	sources := map[string]*TargetConnectMap{}
	for _, spec := range sourceSpecs {
		sources = r.syncCreateTargetMapWithSeed(sources, spec)
	}
	if len(sources) == 0 {
		return fmt.Errorf("source host not found")
	}
	if len(sources) > 1 {
		return fmt.Errorf("remote to remote sync requires source paths from a single host; use remote:@host:/path")
	}

	targets := r.syncCreateTargetMap(targetSpec)
	if len(targets) == 0 {
		return fmt.Errorf("target host not found")
	}

	var source *TargetConnectMap
	for server, client := range sources {
		client.Output.Create(server)
		client.Output.Progress = r.Progress
		client.Output.ProgressWG = r.ProgressWG
		source = client
		break
	}

	if source == nil {
		return fmt.Errorf("source host not found")
	}

	sourceFS := lsync.NewRemoteFS(source.Connect, source.Pwd)
	for server, target := range targets {
		target := target
		if err := r.syncLoop(daemon, daemonInterval, func(ctx context.Context) error {
			target.Output.Create(server)
			target.Output.Progress = r.Progress
			target.Output.ProgressWG = r.ProgressWG

			targetFS := lsync.NewRemoteFS(target.Connect, target.Pwd)
			plan, err := lsync.BuildPlan(sourceFS, targetFS, source.Path, target.Path[0])
			if err != nil {
				return err
			}

			return lsync.ApplyPlan(ctx, sourceFS, targetFS, plan, lsync.ApplyOptions{
				Delete:      deleteExtra,
				DryRun:      r.DryRun,
				Permission:  permission,
				ParallelNum: parallelNum,
				Output:      target.Output,
				SourceLabel: source.Output.Server,
				TargetLabel: server,
			})
		}); err != nil {
			return err
		}
	}

	return nil
}

func (r *RunSftp) syncBidirectionalLocalToRemote(sourceSpec lsync.PathSpec, targetSpec lsync.PathSpec, parallelNum int, permission bool, daemon bool, daemonInterval time.Duration) error {
	localFS, err := lsync.NewLocalFS()
	if err != nil {
		return err
	}

	targets := r.syncCreateTargetMap(targetSpec)
	if len(targets) == 0 {
		return fmt.Errorf("target host not found")
	}

	for server, target := range targets {
		target := target
		if err := r.syncLoop(daemon, daemonInterval, func(ctx context.Context) error {
			target.Output.Create(server)
			target.Output.Progress = r.Progress
			target.Output.ProgressWG = r.ProgressWG

			remoteFS := lsync.NewRemoteFS(target.Connect, target.Pwd)
			leftToRight, rightToLeft, err := lsync.BuildBidirectionalPlans(localFS, remoteFS, sourceSpec.Path, target.Path[0])
			if err != nil {
				return err
			}

			if err := lsync.ApplyPlan(ctx, localFS, remoteFS, leftToRight, lsync.ApplyOptions{
				DryRun:      r.DryRun,
				Permission:  permission,
				ParallelNum: parallelNum,
				Output:      target.Output,
				SourceLabel: "local",
				TargetLabel: server,
			}); err != nil {
				return err
			}

			return lsync.ApplyPlan(ctx, remoteFS, localFS, rightToLeft, lsync.ApplyOptions{
				DryRun:      r.DryRun,
				Permission:  permission,
				ParallelNum: parallelNum,
				Output:      target.Output,
				SourceLabel: server,
				TargetLabel: "local",
			})
		}); err != nil {
			return err
		}
	}

	return nil
}

func (r *RunSftp) syncBidirectionalRemoteToLocal(sourceSpec lsync.PathSpec, targetSpec lsync.PathSpec, parallelNum int, permission bool, daemon bool, daemonInterval time.Duration) error {
	localFS, err := lsync.NewLocalFS()
	if err != nil {
		return err
	}

	sources := r.syncCreateTargetMap(sourceSpec)
	if len(sources) == 0 {
		return fmt.Errorf("source host not found")
	}

	for server, source := range sources {
		source := source
		if err := r.syncLoop(daemon, daemonInterval, func(ctx context.Context) error {
			source.Output.Create(server)
			source.Output.Progress = r.Progress
			source.Output.ProgressWG = r.ProgressWG

			destination := targetSpec.Path
			if len(sources) > 1 {
				destination = localFS.Join(destination, server)
			}

			remoteFS := lsync.NewRemoteFS(source.Connect, source.Pwd)
			leftToRight, rightToLeft, err := lsync.BuildBidirectionalPlans(remoteFS, localFS, source.Path[0], destination)
			if err != nil {
				return err
			}

			if err := lsync.ApplyPlan(ctx, remoteFS, localFS, leftToRight, lsync.ApplyOptions{
				DryRun:      r.DryRun,
				Permission:  permission,
				ParallelNum: parallelNum,
				Output:      source.Output,
				SourceLabel: server,
				TargetLabel: "local",
			}); err != nil {
				return err
			}

			return lsync.ApplyPlan(ctx, localFS, remoteFS, rightToLeft, lsync.ApplyOptions{
				DryRun:      r.DryRun,
				Permission:  permission,
				ParallelNum: parallelNum,
				Output:      source.Output,
				SourceLabel: "local",
				TargetLabel: server,
			})
		}); err != nil {
			return err
		}
	}

	return nil
}

func (r *RunSftp) syncBidirectionalRemoteToRemote(sourceSpec lsync.PathSpec, targetSpec lsync.PathSpec, parallelNum int, permission bool, daemon bool, daemonInterval time.Duration) error {
	sources := r.syncCreateTargetMap(sourceSpec)
	if len(sources) == 0 {
		return fmt.Errorf("source host not found")
	}
	if len(sources) > 1 {
		return fmt.Errorf("remote to remote sync requires source paths from a single host; use remote:@host:/path")
	}

	targets := r.syncCreateTargetMap(targetSpec)
	if len(targets) == 0 {
		return fmt.Errorf("target host not found")
	}

	var source *TargetConnectMap
	var sourceServer string
	for server, client := range sources {
		source = client
		sourceServer = server
		break
	}
	if source == nil {
		return fmt.Errorf("source host not found")
	}

	sourceFS := lsync.NewRemoteFS(source.Connect, source.Pwd)
	for server, target := range targets {
		target := target
		if err := r.syncBidirectionalLoop(daemon, daemonInterval, []string{server}, func(ctx context.Context, _ string) error {
			source.Output.Create(sourceServer)
			source.Output.Progress = r.Progress
			source.Output.ProgressWG = r.ProgressWG

			target.Output.Create(server)
			target.Output.Progress = r.Progress
			target.Output.ProgressWG = r.ProgressWG

			targetFS := lsync.NewRemoteFS(target.Connect, target.Pwd)
			leftToRight, rightToLeft, err := lsync.BuildBidirectionalPlans(sourceFS, targetFS, source.Path[0], target.Path[0])
			if err != nil {
				return err
			}

			if err := lsync.ApplyPlan(ctx, sourceFS, targetFS, leftToRight, lsync.ApplyOptions{
				DryRun:      r.DryRun,
				Permission:  permission,
				ParallelNum: parallelNum,
				Output:      target.Output,
				SourceLabel: sourceServer,
				TargetLabel: server,
			}); err != nil {
				return err
			}

			return lsync.ApplyPlan(ctx, targetFS, sourceFS, rightToLeft, lsync.ApplyOptions{
				DryRun:      r.DryRun,
				Permission:  permission,
				ParallelNum: parallelNum,
				Output:      source.Output,
				SourceLabel: server,
				TargetLabel: sourceServer,
			})
		}); err != nil {
			return err
		}
	}

	return nil
}

func (r *RunSftp) syncLoop(daemon bool, daemonInterval time.Duration, fn func(context.Context) error) error {
	run := func(ctx context.Context) error {
		if err := fn(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		}
		return nil
	}

	if daemon {
		return lsync.RunDaemonLoop(context.Background(), daemonInterval, run)
	}

	return run(context.Background())
}

func (r *RunSftp) syncBidirectionalLoop(daemon bool, daemonInterval time.Duration, servers []string, fn func(context.Context, string) error) error {
	runCycle := func(ctx context.Context) error {
		for _, server := range servers {
			if err := fn(ctx, server); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			}
		}
		return nil
	}

	if daemon {
		return lsync.RunDaemonLoop(context.Background(), daemonInterval, runCycle)
	}

	return runCycle(context.Background())
}

func (r *RunSftp) syncCreateTargetMap(spec lsync.PathSpec) map[string]*TargetConnectMap {
	return r.syncCreateTargetMapWithSeed(map[string]*TargetConnectMap{}, spec)
}

func (r *RunSftp) syncCreateTargetMapWithSeed(seed map[string]*TargetConnectMap, spec lsync.PathSpec) map[string]*TargetConnectMap {
	pathline := spec.Path
	if len(spec.Hosts) > 0 {
		pathline = strings.Join(spec.Hosts, ",") + ":" + spec.Path
	}
	return r.createTargetMap(seed, pathline)
}
