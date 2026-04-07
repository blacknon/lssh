// Copyright (c) 2026 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package mux

import (
	"fmt"
	"sort"
	"strings"
)

type transferJob struct {
	ID         int
	Mode       string
	Source     string
	Target     string
	Status     string
	DoneItems  int
	TotalItems int
	Err        string
}

func (j *transferJob) statusText() string {
	switch j.Status {
	case "done":
		return "done"
	case "error":
		if j.Err != "" {
			return "error: " + j.Err
		}
		return "error"
	default:
		return "running"
	}
}

func renderTransferProgress(done, total int) string {
	if total <= 0 {
		total = 1
	}
	if done < 0 {
		done = 0
	}
	if done > total {
		done = total
	}
	width := 12
	filled := done * width / total
	return "[" + strings.Repeat("=", filled) + strings.Repeat(" ", width-filled) + fmt.Sprintf("] %d/%d", done, total)
}

func (w *transferWizard) startTransfer() {
	if err := w.launchTransferJobs(); err != nil {
		w.manager.updateStatus(fmt.Sprintf("[red]transfer start failed[-]: %v", err))
		w.showResult(err)
		return
	}
	w.setMode(transferModeJobs)
	w.manager.updateStatus("[green]transfer started[-]")
}

func (m *Manager) newTransferJob(mode, source, target string, total int) *transferJob {
	m.transferMu.Lock()
	defer m.transferMu.Unlock()

	m.nextTransferID++
	if total <= 0 {
		total = 1
	}
	job := &transferJob{
		ID:         m.nextTransferID,
		Mode:       mode,
		Source:     source,
		Target:     target,
		Status:     "running",
		TotalItems: total,
	}
	m.transfers = append([]*transferJob{job}, m.transfers...)
	return job
}

func (m *Manager) updateTransferJob(job *transferJob, update func(*transferJob)) {
	if job == nil || update == nil {
		return
	}
	m.transferMu.Lock()
	update(job)
	m.transferMu.Unlock()
}

func (m *Manager) transferJobs() []*transferJob {
	m.transferMu.Lock()
	defer m.transferMu.Unlock()

	out := make([]*transferJob, 0, len(m.transfers))
	for _, job := range m.transfers {
		if job == nil {
			continue
		}
		copyJob := *job
		out = append(out, &copyJob)
	}
	return out
}

func (w *transferWizard) connectedTargetServers(includeCurrent bool) []string {
	seen := map[string]struct{}{}
	servers := []string{}
	for _, pg := range w.manager.sessionPages {
		for _, p := range pg.panes {
			if p == nil || p.session == nil || p.term == nil || p.failed || p.exited || p.transient {
				continue
			}
			if !includeCurrent && p == w.pane {
				continue
			}
			if _, ok := seen[p.server]; ok {
				continue
			}
			seen[p.server] = struct{}{}
			servers = append(servers, p.server)
		}
	}
	sort.Strings(servers)
	return servers
}

func (w *transferWizard) paneByServer(server string) *pane {
	for _, pg := range w.manager.sessionPages {
		for _, p := range pg.panes {
			if p == nil || p.server != server || p.session == nil || p.term == nil || p.failed || p.exited || p.transient {
				continue
			}
			return p
		}
	}
	return nil
}

func (w *transferWizard) launchTransferJobs() error {
	sources := w.currentSources()
	if len(sources) == 0 {
		return fmt.Errorf("source is not selected")
	}

	switch w.activeMode {
	case transferModeGet:
		for _, source := range sources {
			job := w.manager.newTransferJob("get", source, w.currentTargetLabel()+":"+w.getTargetPath, 1)
			go w.runGetJob(job, source)
		}
		return nil
	case transferModePut:
		for _, source := range sources {
			job := w.manager.newTransferJob("put", source, w.pane.server+":"+w.putTargetPath, 1)
			go w.runPutJob(job, source)
		}
		return nil
	case transferModeParallelPut:
		if len(w.copyTargets) == 0 {
			return fmt.Errorf("target panes are not selected")
		}
		for _, server := range w.copyTargets {
			for _, source := range sources {
				job := w.manager.newTransferJob("copy", source, server+":"+w.copyTargetPath, 1)
				go w.runCopyJob(job, source, server)
			}
		}
		return nil
	default:
		return fmt.Errorf("unknown transfer mode")
	}
}

func (w *transferWizard) runGetJob(job *transferJob, source string) {
	srcConn, err := w.openTransferSFTPForPane(w.pane)
	if err != nil {
		w.finishTransferJob(job, err)
		return
	}
	defer srcConn.Close()

	if w.getTargetServer == "" {
		err = copyRemotePathToLocal(srcConn.client, source, w.getTargetPath)
		w.finishTransferJob(job, err)
		return
	}

	targetPane := w.paneByServer(w.getTargetServer)
	if targetPane == nil {
		w.finishTransferJob(job, fmt.Errorf("target pane %s is not connected", w.getTargetServer))
		return
	}

	dstConn, err := w.openTransferSFTPForPane(targetPane)
	if err != nil {
		w.finishTransferJob(job, err)
		return
	}
	defer dstConn.Close()

	err = copyRemotePathToRemote(srcConn.client, dstConn.client, source, w.getTargetPath)
	w.finishTransferJob(job, err)
}

func (w *transferWizard) runPutJob(job *transferJob, source string) {
	dstConn, err := w.openTransferSFTPForPane(w.pane)
	if err != nil {
		w.finishTransferJob(job, err)
		return
	}
	defer dstConn.Close()

	err = copyLocalPathToRemote(dstConn.client, source, w.putTargetPath)
	w.finishTransferJob(job, err)
}

func (w *transferWizard) runCopyJob(job *transferJob, source, server string) {
	target := w.paneByServer(server)
	if target == nil {
		w.finishTransferJob(job, fmt.Errorf("target pane %s is not connected", server))
		return
	}

	srcConn, err := w.openTransferSFTPForPane(w.pane)
	if err != nil {
		w.finishTransferJob(job, err)
		return
	}
	defer srcConn.Close()

	dstConn, err := w.openTransferSFTPForPane(target)
	if err != nil {
		w.finishTransferJob(job, err)
		return
	}
	defer dstConn.Close()

	err = copyRemotePathToRemote(srcConn.client, dstConn.client, source, w.copyTargetPath)
	w.finishTransferJob(job, err)
}

func (w *transferWizard) finishTransferJob(job *transferJob, err error) {
	w.manager.updateTransferJob(job, func(j *transferJob) {
		j.DoneItems = j.TotalItems
		if err != nil {
			j.Status = "error"
			j.Err = err.Error()
			return
		}
		j.Status = "done"
	})
}
