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
	"github.com/vbauerster/mpb"
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
		cli.IntFlag{Name: "parallel,P", Value: 1, Usage: "parallel file sync count per host"},
		cli.BoolFlag{Name: "permission,p", Usage: "copy file permission"},
		cli.BoolFlag{Name: "delete", Usage: "delete destination entries that do not exist in source"},
	}

	app.Action = func(c *cli.Context) error {
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
			spec, err := lsync.ParsePathSpec(raw)
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

		targetSpec, err := lsync.ParsePathSpec(parsed.Destination)
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

		r.ProgressWG = new(syncpkg.WaitGroup)
		r.Progress = mpb.New(mpb.WithWaitGroup(r.ProgressWG))

		switch {
		case !isSourceRemote && targetSpec.IsRemote:
			err = r.syncLocalToRemote(sourceSpecs, targetSpec, parallelNum, deleteExtra, permission)
		case isSourceRemote && !targetSpec.IsRemote:
			err = r.syncRemoteToLocal(sourceSpecs, targetSpec, parallelNum, deleteExtra, permission)
		case isSourceRemote && targetSpec.IsRemote:
			err = r.syncRemoteToRemote(sourceSpecs, targetSpec, parallelNum, deleteExtra, permission)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		}

		r.Progress.Wait()
		time.Sleep(300 * time.Millisecond)
		return nil
	}

	args = common.ParseArgs(app.Flags, args)
	app.Run(args)
}

func (r *RunSftp) syncLocalToRemote(sourceSpecs []lsync.PathSpec, targetSpec lsync.PathSpec, parallelNum int, deleteExtra, permission bool) error {
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
		target.Output.Create(server)
		target.Output.Progress = r.Progress
		target.Output.ProgressWG = r.ProgressWG

		plan, err := lsync.BuildPlan(localFS, lsync.NewRemoteFS(target.Connect, target.Pwd), sourcePaths, target.Path[0])
		if err != nil {
			return err
		}

		if err := lsync.ApplyPlan(context.Background(), localFS, lsync.NewRemoteFS(target.Connect, target.Pwd), plan, lsync.ApplyOptions{
			Delete:      deleteExtra,
			Permission:  permission,
			ParallelNum: parallelNum,
			Output:      target.Output,
		}); err != nil {
			return err
		}
	}

	return nil
}

func (r *RunSftp) syncRemoteToLocal(sourceSpecs []lsync.PathSpec, targetSpec lsync.PathSpec, parallelNum int, deleteExtra, permission bool) error {
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

		if err := lsync.ApplyPlan(context.Background(), remoteFS, localFS, plan, lsync.ApplyOptions{
			Delete:      deleteExtra,
			Permission:  permission,
			ParallelNum: parallelNum,
			Output:      source.Output,
		}); err != nil {
			return err
		}
	}

	return nil
}

func (r *RunSftp) syncRemoteToRemote(sourceSpecs []lsync.PathSpec, targetSpec lsync.PathSpec, parallelNum int, deleteExtra, permission bool) error {
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
		target.Output.Create(server)
		target.Output.Progress = r.Progress
		target.Output.ProgressWG = r.ProgressWG

		targetFS := lsync.NewRemoteFS(target.Connect, target.Pwd)
		plan, err := lsync.BuildPlan(sourceFS, targetFS, source.Path, target.Path[0])
		if err != nil {
			return err
		}

		if err := lsync.ApplyPlan(context.Background(), sourceFS, targetFS, plan, lsync.ApplyOptions{
			Delete:      deleteExtra,
			Permission:  permission,
			ParallelNum: parallelNum,
			Output:      target.Output,
		}); err != nil {
			return err
		}
	}

	return nil
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
