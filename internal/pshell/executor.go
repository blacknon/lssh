// Copyright (c) 2024 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package pshell

import (
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"sync"
)

// PipeSet is pipe in/out set struct.
type PipeSet struct {
	in  *io.PipeReader
	out *io.PipeWriter
}

// Executor run ssh command in parallel-shell.
func (s *shell) Executor(command string) {
	// trim space
	command = strings.TrimSpace(command)
	if command == "" {
		return
	}

	// parse command
	pslice, _ := parsePipeLine(command)
	if len(pslice) == 0 {
		return
	}
	pslice = s.expandAliases(pslice)

	// set latest command
	s.latestCommand = command

	// regist history
	s.PutHistoryFile(command)

	// exec pipeline
	s.parseExecuter(pslice)

	return
}

// parseExecuter assemble and execute the parsed command line.
// TODO(blacknon): 現状はパイプにしか対応していないので、`&&`や`||`にも対応できるよう変更する(v0.6.1)
// TODO(blacknon): !commandで1プロセス、!!commandでssh接続ごとにプロセスを生成してローカルのコマンドを実行するように変更(v0.6.1)
func (s *shell) parseExecuter(pslice [][]pipeLine) {
	// Create History
	s.History[s.Count] = map[string]*shellHistory{}

	// for pslice
	for _, pline := range pslice {
		// join pipe set
		pline = joinPipeLine(pline)

		// printout run command
		fmt.Printf("[Command:%s ]\n", joinPipeLineSlice(pline))

		s.executeJoinedPipeLine(pline)
	}

	// add s.Count
	// (Does not count if only the built-in command is executed)
	isBuildInOnly := true
	for _, pline := range pslice {
		if len(pline) > 1 {
			isBuildInOnly = false
			break
		}

		if !checkBuildInCommand(pline[0].Args[0]) {
			isBuildInOnly = false
			break
		}
	}

	if !isBuildInOnly {
		s.Count++
	}
}

func (s *shell) executeJoinedPipeLine(pline []pipeLine) {
	if hasPerHostLocalCommand(pline) {
		s.executePerHostPipeLine(pline)
		return
	}

	// count pipe num
	pnum := countPipeSet(pline, "|")

	// create pipe set
	pipes := createPipeSet(pnum)

	// pipe counter
	var n int

	// create channel
	ch := make(chan bool)
	defer close(ch)

	kill := make(chan bool)
	defer close(kill)

	for i, p := range pline {
		// declare nextPipeLine
		var bp pipeLine

		// declare in,out
		var in *io.PipeReader
		var out *io.PipeWriter

		// get next pipe line
		if i > 0 {
			bp = pline[i-1]
		}

		// set stdin
		// If the before delimiter is a pipe, set the stdin before io.PipeReader.
		if bp.Oprator == "|" {
			in = pipes[n-1].in
		}

		// set stdout
		// If the delimiter is a pipe, set the stdout output a io.PipeWriter.
		if p.Oprator == "|" {
			out = pipes[n].out

			// add pipe num
			n++
		}

		// exec pipeline
		go s.run(p, in, out, ch, kill)
	}

	// get and send kill
	killExit := make(chan bool)
	defer close(killExit)
	go func(sig chan os.Signal) {
		select {
		case <-sig:
			for i := 0; i < len(pline); i++ {
				kill <- true
			}
		case <-killExit:
			return
		}
	}(s.Signal)

	// wait channel
	s.wait(len(pline), ch)
}

func (s *shell) executePerHostPipeLine(pline []pipeLine) {
	connects := s.pipelineScopedConnects(pline)
	if len(connects) == 0 {
		return
	}

	var wg sync.WaitGroup
	for _, conn := range connects {
		if conn == nil {
			continue
		}

		wg.Add(1)
		go func(conn *sConnect) {
			defer wg.Done()

			scoped := *s
			scoped.currentConns = []*sConnect{conn}
			scoped.executeJoinedPipeLine(normalizePerHostPipeLine(pline))
		}(conn)
	}

	wg.Wait()
}

func hasPerHostLocalCommand(pline []pipeLine) bool {
	for _, p := range pline {
		if len(p.Args) == 0 {
			continue
		}
		if strings.HasPrefix(p.Args[0], "++") {
			return true
		}
	}

	return false
}

func normalizePerHostPipeLine(pline []pipeLine) []pipeLine {
	result := make([]pipeLine, 0, len(pline))
	for _, p := range pline {
		cloned := pipeLine{
			Args:    append([]string{}, p.Args...),
			Oprator: p.Oprator,
		}
		if len(cloned.Args) > 0 && strings.HasPrefix(cloned.Args[0], "++") {
			cloned.Args[0] = "+" + strings.TrimPrefix(cloned.Args[0], "++")
		}
		result = append(result, cloned)
	}

	return result
}

func (s *shell) activeConnects() []*sConnect {
	if len(s.currentConns) > 0 {
		return s.currentConns
	}

	return s.Connects
}

func (s *shell) pipelineScopedConnects(pline []pipeLine) []*sConnect {
	connects := append([]*sConnect{}, s.activeConnects()...)
	if len(connects) == 0 {
		return nil
	}

	targeted := false
	for _, p := range pline {
		if len(p.Args) == 0 || checkLocalBuildInCommand(p.Args[0]) {
			continue
		}
		if !isTargetedRemoteCommand(p.Args[0]) {
			continue
		}

		targeted = true
		targetNames, err := parseTargetedNames(p.Args[0], knownHostsFromConnects(connects))
		if err != nil {
			return nil
		}

		filtered := make([]*sConnect, 0, len(connects))
		for _, c := range connects {
			if c == nil {
				continue
			}
			if slices.Contains(targetNames, c.Name) {
				filtered = append(filtered, c)
			}
		}
		connects = filtered
		if len(connects) == 0 {
			return nil
		}
	}

	if targeted {
		return connects
	}

	return s.activeConnects()
}

func parseTargetedNames(command string, knownHosts []string) ([]string, error) {
	targets, _, err := parseTargetedCommand(command, knownHosts)
	if err != nil {
		return nil, err
	}

	return targets, nil
}

func (s *shell) resolveTargetedConnects(args []string) ([]*sConnect, []string, error) {
	baseConnects := s.activeConnects()

	if len(args) == 0 {
		return baseConnects, args, nil
	}

	command := args[0]
	if !isTargetedRemoteCommand(command) {
		return baseConnects, args, nil
	}

	targets, commandHead, err := parseTargetedCommand(command, knownHostsFromConnects(baseConnects))
	if err != nil {
		return nil, nil, err
	}

	targetSet := make(map[string]struct{}, len(targets))
	for _, target := range targets {
		targetSet[target] = struct{}{}
	}

	connects := make([]*sConnect, 0, len(targetSet))
	found := make([]string, 0, len(targetSet))
	for _, c := range baseConnects {
		if c == nil {
			continue
		}
		if _, ok := targetSet[c.Name]; ok {
			connects = append(connects, c)
			found = append(found, c.Name)
		}
	}

	if len(connects) == 0 {
		return nil, nil, fmt.Errorf("target server not found: %s", strings.Join(targets, ","))
	}

	if len(found) != len(targetSet) {
		missing := make([]string, 0, len(targetSet)-len(found))
		for name := range targetSet {
			if !slices.Contains(found, name) {
				missing = append(missing, name)
			}
		}
		return nil, nil, fmt.Errorf("target server not found: %s", strings.Join(missing, ","))
	}

	updatedArgs := append([]string{}, args...)
	updatedArgs[0] = commandHead

	return connects, updatedArgs, nil
}

// countPipeSet count delimiter in pslice.
func countPipeSet(pline []pipeLine, del string) (count int) {
	for _, p := range pline {
		if p.Oprator == del {
			count++
		}
	}

	return count
}

// createPipeSet return Returns []*PipeSet used by the process.
func createPipeSet(count int) (pipes []*PipeSet) {
	for i := 0; i < count; i++ {
		in, out := io.Pipe()
		pipe := &PipeSet{
			in:  in,
			out: out,
		}

		pipes = append(pipes, pipe)
	}

	return
}
