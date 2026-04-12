// Copyright (c) 2026 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package common

// NormalizeTrailingBoolFlags moves known trailing bool flags ahead of the first
// positional argument so ParseArgs can still interpret them as options.
// valueFlags lists options that consume the following argument as their value.
func NormalizeTrailingBoolFlags(args []string, boolFlags, valueFlags map[string]bool) []string {
	if len(args) <= 2 {
		return args
	}

	firstPositional := -1
	for i := 1; i < len(args); i++ {
		arg := args[i]
		if valueFlags[arg] {
			i++
			continue
		}
		if len(arg) > 0 && arg[0] == '-' {
			continue
		}
		firstPositional = i
		break
	}

	if firstPositional == -1 {
		return args
	}

	prefix := append([]string{}, args[:firstPositional]...)
	positionals := make([]string, 0, len(args)-firstPositional)
	trailingFlags := make([]string, 0, len(args)-firstPositional)

	for _, arg := range args[firstPositional:] {
		if boolFlags[arg] {
			trailingFlags = append(trailingFlags, arg)
			continue
		}
		positionals = append(positionals, arg)
	}

	result := make([]string, 0, len(args))
	result = append(result, prefix...)
	result = append(result, trailingFlags...)
	result = append(result, positionals...)
	return result
}
