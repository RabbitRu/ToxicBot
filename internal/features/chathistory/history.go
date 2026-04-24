package chathistory

import "sync"

type Buffer struct {
	mu      sync.RWMutex
	data    map[int64][]Entry
	maxSize int
}

func NewBuffer(maxSize int) *Buffer {
	if maxSize <= 0 {
		panic("chathistory: maxSize must be positive")
	}

	return &Buffer{
		data:    make(map[int64][]Entry),
		maxSize: maxSize,
	}
}

func (b *Buffer) Add(chatID int64, e Entry) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.appendLocked(chatID, e)
}

// AddAll appends multiple entries under a single lock, so no concurrent Add
// from another goroutine can interleave between them. Use for atomic
// user→bot pairs.
func (b *Buffer) AddAll(chatID int64, entries ...Entry) {
	if len(entries) == 0 {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, e := range entries {
		b.appendLocked(chatID, e)
	}
}

func (b *Buffer) appendLocked(chatID int64, e Entry) {
	buf := b.data[chatID]
	buf = append(buf, e)
	if len(buf) > b.maxSize {
		buf = buf[len(buf)-b.maxSize:]
	}
	b.data[chatID] = buf
}

func (b *Buffer) Get(chatID int64) []Entry {
	b.mu.RLock()
	defer b.mu.RUnlock()

	src := b.data[chatID]
	out := make([]Entry, len(src))
	copy(out, src)
	return out
}
