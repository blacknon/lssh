package apputil

import "os"

func CurrentCLIArgs() []string {
	return append([]string(nil), os.Args[1:]...)
}

func FilterCLIArgs(args []string, bareFlags, valueFlags map[string]bool) []string {
	filtered := make([]string, 0, len(args))
	skipNext := false
	for _, arg := range args {
		if skipNext {
			skipNext = false
			continue
		}
		if bareFlags[arg] {
			continue
		}
		if valueFlags[arg] {
			skipNext = true
			continue
		}
		filtered = append(filtered, arg)
	}

	return filtered
}
