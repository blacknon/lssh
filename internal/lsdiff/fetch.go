// Copyright (c) 2026 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package lsdiff

import (
	"fmt"
	"io"
	"strings"
	"sync"

	conf "github.com/blacknon/lssh/internal/config"
	sshl "github.com/blacknon/lssh/internal/ssh"
)

func FetchDocuments(config conf.Config, targets []Target, controlMasterOverride *bool) ([]Document, error) {
	run := &sshl.Run{
		ServerList: targetHosts(targets),
		Conf:       config,
		RunSessionConfig: sshl.RunSessionConfig{
			ControlMasterOverride: controlMasterOverride,
		},
	}
	run.CreateAuthMethodMap()

	type result struct {
		index int
		doc   Document
	}

	results := make(chan result, len(targets))
	var wg sync.WaitGroup

	for i, target := range targets {
		wg.Add(1)
		go func(index int, target Target) {
			defer wg.Done()

			connect, err := run.CreateSshConnectDirect(target.Host)
			if err != nil {
				results <- result{index: index, doc: errorDocument(target, err)}
				return
			}
			defer connect.Close()

			client, err := connect.OpenSFTP()
			if err != nil {
				results <- result{index: index, doc: errorDocument(target, err)}
				return
			}
			defer client.Close()

			file, err := client.Open(target.RemotePath)
			if err != nil {
				results <- result{index: index, doc: errorDocument(target, err)}
				return
			}
			defer file.Close()

			data, err := io.ReadAll(file)
			if err != nil {
				results <- result{index: index, doc: errorDocument(target, err)}
				return
			}

			results <- result{
				index: index,
				doc: Document{
					Target: target,
					Lines:  splitLines(string(data)),
				},
			}
		}(i, target)
	}

	wg.Wait()
	close(results)

	documents := make([]Document, len(targets))
	for result := range results {
		documents[result.index] = result.doc
	}

	return documents, nil
}

func errorDocument(target Target, err error) Document {
	message := fmt.Sprintf("failed to fetch %s\n\n%s", target.Title, err)
	return Document{
		Target: target,
		Error:  err.Error(),
		Lines:  splitLines(message),
	}
}

func splitLines(value string) []string {
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

func targetHosts(targets []Target) []string {
	seen := map[string]struct{}{}
	hosts := make([]string, 0, len(targets))
	for _, target := range targets {
		if _, ok := seen[target.Host]; ok {
			continue
		}
		seen[target.Host] = struct{}{}
		hosts = append(hosts, target.Host)
	}
	return hosts
}
