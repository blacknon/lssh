package sync

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/blacknon/lssh/internal/common"
)

type CommandArgs struct {
	Delete         bool
	DryRun         bool
	Permission     bool
	Daemon         bool
	DaemonInterval time.Duration
	Bidirectional  bool
	ParallelNum    int
	Sources        []string
	Destination    string
}

type PathSpec struct {
	IsRemote bool
	Hosts    []string
	Path     string
	Raw      string
}

func ParseCommandArgs(args []string) (CommandArgs, error) {
	result := CommandArgs{
		ParallelNum:    1,
		DaemonInterval: 5 * time.Second,
	}

	if len(args) == 0 {
		return result, fmt.Errorf("missing command")
	}

	rest := []string{}
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--delete":
			result.Delete = true
		case "--dry-run":
			result.DryRun = true
		case "--daemon", "-D":
			result.Daemon = true
		case "--bidirectional", "-B":
			result.Bidirectional = true
		case "--permission", "-p":
			result.Permission = true
		case "--daemon-interval":
			if i+1 >= len(args) {
				return result, fmt.Errorf("missing value for %s", args[i])
			}
			i++
			v, err := time.ParseDuration(args[i])
			if err != nil {
				return result, fmt.Errorf("invalid daemon interval value: %s", args[i])
			}
			if v <= 0 {
				v = 5 * time.Second
			}
			result.DaemonInterval = v
		case "--parallel", "-P":
			if i+1 >= len(args) {
				return result, fmt.Errorf("missing value for %s", args[i])
			}
			i++
			v, err := strconv.Atoi(args[i])
			if err != nil {
				return result, fmt.Errorf("invalid parallel value: %s", args[i])
			}
			if v < 1 {
				v = 1
			}
			result.ParallelNum = v
		default:
			rest = append(rest, args[i])
		}
	}

	if len(rest) < 2 {
		return result, fmt.Errorf("requires source and destination paths")
	}

	result.Sources = append(result.Sources, rest[:len(rest)-1]...)
	result.Destination = rest[len(rest)-1]
	return result, nil
}

func ParsePathSpec(arg string) (PathSpec, error) {
	spec := PathSpec{Raw: arg}
	parts := strings.SplitN(arg, ":", 2)
	if len(parts) < 2 {
		spec.Path = arg
		return spec, nil
	}

	switch strings.ToLower(parts[0]) {
	case "local", "l":
		spec.Path = parts[1]
		return spec, nil
	case "remote", "r":
		spec.IsRemote = true
		spec.Path = parts[1]
	default:
		return spec, fmt.Errorf("invalid path prefix: %s", parts[0])
	}

	if strings.HasPrefix(spec.Path, "@") {
		hosts, path := common.ParseHostPath(strings.TrimPrefix(spec.Path, "@"))
		if len(hosts) == 0 || path == "" {
			return spec, fmt.Errorf("remote path must be remote:@host:/path or remote:/path")
		}
		spec.Hosts = hosts
		spec.Path = path
	}

	return spec, nil
}
