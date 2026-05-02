package monitor

import (
	"sync"

	"github.com/blacknon/tvxterm"
)

type redrawNotifyingBackend struct {
	backend     tvxterm.Backend
	onRead      func()
	lastCols    int
	lastRows    int
	hasLastSize bool
	sync.Mutex
}

func newRedrawNotifyingBackend(backend tvxterm.Backend, onRead func()) tvxterm.Backend {
	if backend == nil {
		return nil
	}

	return &redrawNotifyingBackend{
		backend: backend,
		onRead:  onRead,
	}
}

func (b *redrawNotifyingBackend) Read(p []byte) (int, error) {
	n, err := b.backend.Read(p)
	if n > 0 && b.onRead != nil {
		b.onRead()
	}
	return n, err
}

func (b *redrawNotifyingBackend) Write(p []byte) (int, error) {
	return b.backend.Write(p)
}

func (b *redrawNotifyingBackend) Resize(cols, rows int) error {
	b.Lock()
	defer b.Unlock()

	if b.hasLastSize && b.lastCols == cols && b.lastRows == rows {
		return nil
	}

	b.lastCols = cols
	b.lastRows = rows
	b.hasLastSize = true

	return b.backend.Resize(cols, rows)
}

func (b *redrawNotifyingBackend) Close() error {
	return b.backend.Close()
}
