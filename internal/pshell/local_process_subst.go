// Copyright (c) 2024 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package pshell

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type localProcessSubstSpec struct {
	Raw     string
	Command string
	Split   bool
}

type localProcessSubstResult struct {
	Server string
	Output string
}

func (s *shell) expandLocalProcessSubstitutions(command string) (string, func(), error) {
	specs, err := parseLocalProcessSubstitutions(command)
	if err != nil {
		return "", func() {}, err
	}
	if len(specs) == 0 {
		return command, func() {}, nil
	}

	baseDir, err := os.MkdirTemp("", "lssh-psub-*")
	if err != nil {
		return "", func() {}, err
	}

	cleanup := func() {
		_ = os.RemoveAll(baseDir)
	}

	expanded := command
	for i, spec := range specs {
		paths, err := s.materializeLocalProcessSubstitution(baseDir, i, spec)
		if err != nil {
			cleanup()
			return "", func() {}, err
		}

		replacement := shellQuote(paths[0])
		if spec.Split {
			replacement = joinShellWords(paths)
		}

		expanded = strings.Replace(expanded, spec.Raw, replacement, 1)
	}

	return expanded, cleanup, nil
}

func parseLocalProcessSubstitutions(command string) ([]localProcessSubstSpec, error) {
	specs := []localProcessSubstSpec{}

	inSingle := false
	inDouble := false
	escaped := false

	for i := 0; i < len(command); i++ {
		ch := command[i]

		if escaped {
			escaped = false
			continue
		}

		switch {
		case inSingle:
			if ch == '\'' {
				inSingle = false
			}
			continue
		case inDouble:
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inDouble = false
			}
			continue
		}

		switch ch {
		case '\\':
			escaped = true
			continue
		case '\'':
			inSingle = true
			continue
		case '"':
			inDouble = true
			continue
		}

		spec, end, ok, err := readLocalProcessSubstitution(command, i)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}

		specs = append(specs, spec)
		i = end
	}

	return specs, nil
}

func readLocalProcessSubstitution(command string, start int) (localProcessSubstSpec, int, bool, error) {
	spec := localProcessSubstSpec{}

	split := false
	open := start

	switch {
	case strings.HasPrefix(command[start:], "+<("):
		split = true
		open = start + 2
	case strings.HasPrefix(command[start:], "<("):
		open = start + 1
	default:
		return spec, start, false, nil
	}

	body, end, err := readBalancedCommand(command, open+1)
	if err != nil {
		return spec, start, false, err
	}

	spec = localProcessSubstSpec{
		Raw:     command[start : end+1],
		Command: strings.TrimSpace(body),
		Split:   split,
	}

	return spec, end, true, nil
}

func readBalancedCommand(command string, start int) (string, int, error) {
	depth := 1
	inSingle := false
	inDouble := false
	escaped := false

	for i := start; i < len(command); i++ {
		ch := command[i]

		if escaped {
			escaped = false
			continue
		}

		switch {
		case inSingle:
			if ch == '\'' {
				inSingle = false
			}
			continue
		case inDouble:
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inDouble = false
			}
			continue
		}

		switch ch {
		case '\\':
			escaped = true
		case '\'':
			inSingle = true
		case '"':
			inDouble = true
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return command[start:i], i, nil
			}
		}
	}

	return "", 0, fmt.Errorf("unterminated process substitution")
}

func (s *shell) materializeLocalProcessSubstitution(baseDir string, index int, spec localProcessSubstSpec) ([]string, error) {
	results, err := s.runLocalProcessSubstitutionCommand(spec.Command)
	if err != nil {
		return nil, err
	}

	dir := filepath.Join(baseDir, fmt.Sprintf("subst-%02d", index))
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}

	if spec.Split {
		paths := make([]string, 0, len(results))
		for i, result := range results {
			path := filepath.Join(dir, fmt.Sprintf("%02d_%s.out", i, sanitizeLocalProcessSubstName(result.Server)))
			if err := os.WriteFile(path, []byte(result.Output), 0600); err != nil {
				return nil, err
			}
			paths = append(paths, path)
		}
		return paths, nil
	}

	path := filepath.Join(dir, "merged.out")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	for i, result := range results {
		if i > 0 {
			if _, err := file.WriteString("\n"); err != nil {
				return nil, err
			}
		}

		if _, err := fmt.Fprintf(file, "===== %s =====\n", result.Server); err != nil {
			return nil, err
		}
		if _, err := file.WriteString(result.Output); err != nil {
			return nil, err
		}
		if result.Output != "" && !strings.HasSuffix(result.Output, "\n") {
			if _, err := file.WriteString("\n"); err != nil {
				return nil, err
			}
		}
	}

	return []string{path}, nil
}

func (s *shell) runLocalProcessSubstitutionCommand(command string) ([]localProcessSubstResult, error) {
	pslice, err := parsePipeLine(command)
	if err != nil {
		return nil, err
	}
	if len(pslice) != 1 {
		return nil, fmt.Errorf("process substitution supports a single remote command")
	}

	joined := joinPipeLine(pslice[0])
	if len(joined) != 1 || len(joined[0].Args) == 0 {
		return nil, fmt.Errorf("process substitution supports a single remote command")
	}
	if checkLocalBuildInCommand(joined[0].Args[0]) {
		return nil, fmt.Errorf("process substitution does not support local commands")
	}

	connects, args, err := s.resolveTargetedConnects(joined[0].Args)
	if err != nil {
		return nil, err
	}

	remoteCommand := strings.Join(args, " ")
	results := make([]localProcessSubstResult, len(connects))
	errs := make(chan error, len(connects))
	var wg sync.WaitGroup

	for i, conn := range connects {
		wg.Add(1)
		go func(idx int, c *sConnect) {
			defer wg.Done()

			buf, runErr := runCompleteCommand(c, remoteCommand)
			if runErr != nil {
				errs <- fmt.Errorf("%s: %w", c.Name, runErr)
				return
			}

			results[idx] = localProcessSubstResult{
				Server: c.Name,
				Output: buf.String(),
			}
		}(i, conn)
	}

	wg.Wait()
	close(errs)

	for runErr := range errs {
		if runErr != nil {
			return nil, runErr
		}
	}

	return results, nil
}

func sanitizeLocalProcessSubstName(s string) string {
	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		":", "_",
		" ", "_",
		"\t", "_",
	)

	name := replacer.Replace(s)
	name = strings.Trim(name, "._")
	if name == "" {
		return "unknown"
	}

	return name
}

func joinShellWords(words []string) string {
	quoted := make([]string, 0, len(words))
	for _, word := range words {
		quoted = append(quoted, shellQuote(word))
	}

	return strings.Join(quoted, " ")
}

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}

	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}
