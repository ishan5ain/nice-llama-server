package logbuf

import (
	"sync"
	"time"

	"nice-llama-server/internal/config"
)

type Buffer struct {
	mu      sync.RWMutex
	max     int
	nextSeq int64
	items   []config.LogEntry
}

func New(max int) *Buffer {
	if max <= 0 {
		max = 2000
	}
	return &Buffer{
		max:   max,
		items: make([]config.LogEntry, 0, max),
	}
}

func (b *Buffer) Add(stream, line string) config.LogEntry {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.nextSeq++
	entry := config.LogEntry{
		Seq:    b.nextSeq,
		TS:     time.Now().UTC(),
		Stream: stream,
		Line:   line,
	}
	if len(b.items) == b.max {
		copy(b.items, b.items[1:])
		b.items[len(b.items)-1] = entry
		return entry
	}
	b.items = append(b.items, entry)
	return entry
}

func (b *Buffer) Since(seq int64) []config.LogEntry {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if seq <= 0 {
		return clone(b.items)
	}
	for i := range b.items {
		if b.items[i].Seq > seq {
			return clone(b.items[i:])
		}
	}
	return nil
}

func (b *Buffer) Tail(n int) []config.LogEntry {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if n <= 0 || n >= len(b.items) {
		return clone(b.items)
	}
	return clone(b.items[len(b.items)-n:])
}

func (b *Buffer) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.items = b.items[:0]
}

func clone(items []config.LogEntry) []config.LogEntry {
	if len(items) == 0 {
		return nil
	}
	out := make([]config.LogEntry, len(items))
	copy(out, items)
	return out
}
