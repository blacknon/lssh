// Copyright (c) 2026 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package lspipe

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type FIFOWorker struct {
	SessionName string
	FIFOName    string
	Endpoints   []FIFOEndpoint
}

func (w *FIFOWorker) Run() error {
	baseDir, err := fifoBaseDir(w.SessionName, w.FIFOName)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return err
	}

	for _, endpoint := range w.Endpoints {
		for _, path := range []string{endpoint.CmdPath, endpoint.StdinPath, endpoint.OutPath} {
			if err := ensureFIFO(path); err != nil {
				return err
			}
		}
	}

	var wg sync.WaitGroup
	for _, endpoint := range w.Endpoints {
		endpoint := endpoint
		wg.Add(1)
		go func() {
			defer wg.Done()
			w.serveEndpoint(endpoint)
		}()
	}

	wg.Wait()
	return nil
}

func (w *FIFOWorker) serveEndpoint(endpoint FIFOEndpoint) {
	var mu sync.Mutex
	var pendingStdin []byte

	go func() {
		for {
			data, err := readFIFOOnce(endpoint.StdinPath)
			if err != nil {
				return
			}
			mu.Lock()
			pendingStdin = data
			mu.Unlock()
		}
	}()

	for {
		data, err := readFIFOOnce(endpoint.CmdPath)
		if err != nil {
			return
		}

		command := strings.TrimSpace(string(data))
		if command == "" {
			continue
		}

		mu.Lock()
		stdin := append([]byte(nil), pendingStdin...)
		pendingStdin = nil
		mu.Unlock()

		reader, writer, err := os.Pipe()
		if err != nil {
			continue
		}

		runErrCh := make(chan error, 1)
		go func() {
			runErrCh <- Execute(ExecOptions{
				Name:    w.SessionName,
				Command: command,
				Hosts:   endpoint.Hosts,
				Raw:     len(endpoint.Hosts) == 1,
				Stdin:   stdin,
				Stdout:  writer,
				Stderr:  writer,
			})
			_ = writer.Close()
		}()

		if err := writeFIFOOnce(endpoint.OutPath, reader); err != nil {
			_ = reader.Close()
			<-runErrCh
			continue
		}
		_ = reader.Close()
		<-runErrCh
	}
}

func ensureFIFO(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	return createNamedPipe(path)
}

func readFIFOOnce(path string) ([]byte, error) {
	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

func writeFIFOOnce(path string, src io.Reader) error {
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, src)
	return err
}
