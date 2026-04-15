package chathistory

import (
	"sync"

	"github.com/reijo1337/ToxicBot/internal/message"
)

type Buffer struct {
	mu      sync.RWMutex
	data    map[int64][]entry
	maxSize int
}

type entry struct {
	Author string
	Text   string
}

func NewBuffer(maxSize int) *Buffer {
	if maxSize <= 0 {
		panic("chathistory: maxSize must be positive")
	}

	return &Buffer{
		data:    make(map[int64][]entry),
		maxSize: maxSize,
	}
}

func (b *Buffer) Add(chatID int64, author, text string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	buf := b.data[chatID]
	buf = append(buf, entry{Author: author, Text: text})
	if len(buf) > b.maxSize {
		buf = buf[len(buf)-b.maxSize:]
	}
	b.data[chatID] = buf
}

func (b *Buffer) Get(chatID int64) []message.HistoryMessage {
	b.mu.RLock()
	defer b.mu.RUnlock()

	buf := b.data[chatID]
	out := make([]message.HistoryMessage, len(buf))
	for i, e := range buf {
		out[i] = message.HistoryMessage{Author: e.Author, Text: e.Text}
	}
	return out
}
