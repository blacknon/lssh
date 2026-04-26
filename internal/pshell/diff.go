package pshell

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"sync"

	diffapp "github.com/blacknon/lssh/internal/lsdiff"
)

func (s *shell) buildin_diff(args []string, out *io.PipeWriter, ch chan<- bool) {
	stdout := setOutput(out)

	connects, args, err := s.resolveTargetedConnects(args)
	if err != nil {
		fmt.Fprintf(stdout, "Error: %s\n", err)
		if out != nil {
			out.CloseWithError(io.ErrClosedPipe)
		}
		ch <- true
		return
	}

	if len(args) < 2 {
		_, _ = io.WriteString(stdout, "%diff remote_path | @host:/path...\n")
		if out != nil {
			out.CloseWithError(io.ErrClosedPipe)
		}
		ch <- true
		return
	}

	targets, err := resolveShellDiffTargets(connects, args[1:])
	if err != nil {
		fmt.Fprintf(stdout, "Error: %s\n", err)
		if out != nil {
			out.CloseWithError(io.ErrClosedPipe)
		}
		ch <- true
		return
	}

	if out != nil {
		out.CloseWithError(io.ErrClosedPipe)
		stdout = os.Stderr
	}

	documents, err := s.fetchDiffDocuments(connects, targets)
	if err != nil {
		fmt.Fprintf(stdout, "Error: %s\n", err)
		ch <- true
		return
	}
	if len(documents) < 2 {
		fmt.Fprintf(stdout, "Error: %%diff requires at least two files to compare\n")
		ch <- true
		return
	}

	viewer := diffapp.NewViewer(diffapp.AlignDocuments(documents))
	if err := viewer.Run(); err != nil {
		fmt.Fprintf(stdout, "Error: %s\n", err)
	}

	ch <- true
}

func resolveShellDiffTargets(connects []*sConnect, args []string) ([]diffapp.Target, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("%%diff requires at least one remote path")
	}

	knownHosts := knownHostsFromConnects(connects)
	explicitTargets := make([]diffapp.Target, 0, len(args))
	for _, arg := range args {
		target, err := diffapp.ParseTargetSpecWithHosts(arg, knownHosts)
		if err != nil {
			return nil, err
		}
		explicitTargets = append(explicitTargets, target)
	}

	allExplicit := true
	for _, target := range explicitTargets {
		if target.Host == "" {
			allExplicit = false
			break
		}
	}

	if allExplicit {
		if len(explicitTargets) < 2 {
			return nil, fmt.Errorf("%%diff requires at least two targets to compare")
		}

		available := make(map[string]struct{}, len(knownHosts))
		for _, host := range knownHosts {
			available[host] = struct{}{}
		}
		for _, target := range explicitTargets {
			if _, ok := available[target.Host]; !ok {
				return nil, fmt.Errorf("target host not available in current shell: %s", target.Host)
			}
		}

		return explicitTargets, nil
	}

	if len(explicitTargets) != 1 || explicitTargets[0].Host != "" {
		return nil, fmt.Errorf("use a single common remote path or explicit @host:/path targets")
	}

	liveConnects := make([]*sConnect, 0, len(connects))
	for _, conn := range connects {
		if conn == nil {
			continue
		}
		liveConnects = append(liveConnects, conn)
	}
	if len(liveConnects) < 2 {
		return nil, fmt.Errorf("select at least two hosts")
	}

	sort.SliceStable(liveConnects, func(i, j int) bool {
		return liveConnects[i].Name < liveConnects[j].Name
	})

	targets := make([]diffapp.Target, 0, len(liveConnects))
	for _, conn := range liveConnects {
		targets = append(targets, diffapp.Target{
			Host:       conn.Name,
			RemotePath: explicitTargets[0].RemotePath,
			Title:      conn.Name + ":" + explicitTargets[0].RemotePath,
		})
	}

	return targets, nil
}

func (s *shell) fetchDiffDocuments(connects []*sConnect, targets []diffapp.Target) ([]diffapp.Document, error) {
	connectMap := make(map[string]*sConnect, len(connects))
	for _, conn := range connects {
		if conn == nil {
			continue
		}
		connectMap[conn.Name] = conn
	}

	type result struct {
		index int
		doc   diffapp.Document
	}

	results := make(chan result, len(targets))
	var wg sync.WaitGroup

	for i, target := range targets {
		wg.Add(1)
		go func(index int, target diffapp.Target) {
			defer wg.Done()

			conn, ok := connectMap[target.Host]
			if !ok {
				results <- result{index: index, doc: shellDiffErrorDocument(target, fmt.Errorf("host %s is not available in current shell", target.Host))}
				return
			}

			client, closeClient, err := s.openSFTPClient(conn)
			if err != nil {
				results <- result{index: index, doc: shellDiffErrorDocument(target, err)}
				return
			}
			defer closeClient()

			file, err := client.Open(target.RemotePath)
			if err != nil {
				results <- result{index: index, doc: shellDiffErrorDocument(target, err)}
				return
			}
			defer file.Close()

			data, err := io.ReadAll(file)
			if err != nil {
				results <- result{index: index, doc: shellDiffErrorDocument(target, err)}
				return
			}

			results <- result{
				index: index,
				doc: diffapp.Document{
					Target: target,
					Lines:  splitDiffLines(string(data)),
				},
			}
		}(i, target)
	}

	wg.Wait()
	close(results)

	documents := make([]diffapp.Document, len(targets))
	for result := range results {
		documents[result.index] = result.doc
	}

	return documents, nil
}

func shellDiffErrorDocument(target diffapp.Target, err error) diffapp.Document {
	message := fmt.Sprintf("failed to fetch %s\n\n%s", target.Title, err)
	return diffapp.Document{
		Target: target,
		Error:  err.Error(),
		Lines:  splitDiffLines(message),
	}
}

func splitDiffLines(value string) []string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")

	lines := strings.Split(value, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 {
		return []string{""}
	}

	return lines
}
