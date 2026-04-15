package chathistory_test

import (
	"sync"
	"testing"

	"github.com/reijo1337/ToxicBot/internal/chathistory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuffer_AddAndGet(t *testing.T) {
	t.Parallel()

	buf := chathistory.NewBuffer(50)
	buf.Add(1, "alice", "hello")
	buf.Add(1, "bob", "world")

	history := buf.Get(1)
	require.Len(t, history, 2)
	assert.Equal(t, "alice", history[0].Author)
	assert.Equal(t, "hello", history[0].Text)
	assert.Equal(t, "bob", history[1].Author)
	assert.Equal(t, "world", history[1].Text)
}

func TestBuffer_GetEmptyChat(t *testing.T) {
	t.Parallel()

	buf := chathistory.NewBuffer(50)
	history := buf.Get(999)
	assert.Empty(t, history)
}

func TestBuffer_IndependentChats(t *testing.T) {
	t.Parallel()

	buf := chathistory.NewBuffer(50)
	buf.Add(1, "alice", "chat1")
	buf.Add(2, "bob", "chat2")

	h1 := buf.Get(1)
	h2 := buf.Get(2)
	require.Len(t, h1, 1)
	require.Len(t, h2, 1)
	assert.Equal(t, "chat1", h1[0].Text)
	assert.Equal(t, "chat2", h2[0].Text)
}

func TestBuffer_Overflow(t *testing.T) {
	t.Parallel()

	buf := chathistory.NewBuffer(3)
	buf.Add(1, "a", "msg1")
	buf.Add(1, "b", "msg2")
	buf.Add(1, "c", "msg3")
	buf.Add(1, "d", "msg4")

	history := buf.Get(1)
	require.Len(t, history, 3)
	assert.Equal(t, "msg2", history[0].Text)
	assert.Equal(t, "msg4", history[2].Text)
}

func TestNewBuffer_PanicsOnZeroMaxSize(t *testing.T) {
	t.Parallel()

	assert.Panics(t, func() { chathistory.NewBuffer(0) })
}

func TestNewBuffer_PanicsOnNegativeMaxSize(t *testing.T) {
	t.Parallel()

	assert.Panics(t, func() { chathistory.NewBuffer(-1) })
}

func TestBuffer_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	buf := chathistory.NewBuffer(50)
	var wg sync.WaitGroup

	for range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			buf.Add(1, "user", "msg")
			buf.Get(1)
		}()
	}

	wg.Wait()
	history := buf.Get(1)
	assert.LessOrEqual(t, len(history), 50)
}
