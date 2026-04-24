package message

import (
	"testing"
	"time"

	"github.com/reijo1337/ToxicBot/internal/features/chathistory"
	"github.com/reijo1337/ToxicBot/internal/infrastructure/ai/deepseek"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatUserContent_NoReplyTo(t *testing.T) {
	t.Parallel()

	ts := time.Date(2026, 4, 24, 14, 32, 0, 0, time.UTC)
	e := chathistory.Entry{ID: 1, Time: ts, Author: "@alice", Text: "привет"}
	got := formatUserContent(e, nil)
	assert.Equal(t, "[14:32 @alice] привет", got)
}

func TestFormatUserContent_ReplyToPresent(t *testing.T) {
	t.Parallel()

	ts := time.Date(2026, 4, 24, 14, 32, 0, 0, time.UTC)
	history := []chathistory.Entry{
		{ID: 9, Author: "@bob", Text: "hi"},
	}
	e := chathistory.Entry{ID: 10, Time: ts, Author: "@alice", Text: "йо", ReplyToID: 9}
	got := formatUserContent(e, history)
	assert.Equal(t, "[14:32 @alice → @bob] йо", got)
}

func TestFormatUserContent_ReplyToEvicted_NoArrow(t *testing.T) {
	t.Parallel()

	ts := time.Date(2026, 4, 24, 14, 32, 0, 0, time.UTC)
	e := chathistory.Entry{ID: 10, Time: ts, Author: "@alice", Text: "йо", ReplyToID: 999}
	got := formatUserContent(e, nil)
	assert.Equal(t, "[14:32 @alice] йо", got)
}

func TestFormatUserContent_ReplyToBot(t *testing.T) {
	t.Parallel()

	ts := time.Date(2026, 4, 24, 14, 32, 0, 0, time.UTC)
	history := []chathistory.Entry{
		{ID: 5, Author: "бот", Text: "отвали", FromBot: true},
	}
	e := chathistory.Entry{ID: 10, Time: ts, Author: "@alice", Text: "сам дурак", ReplyToID: 5}
	got := formatUserContent(e, history)
	assert.Equal(t, "[14:32 @alice → бот] сам дурак", got)
}

func TestBuildChatCompletions_UserEntryBecomesUserRole(t *testing.T) {
	t.Parallel()

	system := "SYS"
	history := []chathistory.Entry{
		{ID: 1, Time: time.Date(2026, 4, 24, 14, 0, 0, 0, time.UTC), Author: "@alice", Text: "привет"},
		{ID: 2, Time: time.Date(2026, 4, 24, 14, 1, 0, 0, time.UTC), Author: "@alice", Text: "ответь"},
	}

	msgs := buildChatCompletions(system, history)

	require.Len(t, msgs, 3)
	assert.Equal(t, deepseek.RoleSystem, msgs[0].Role)
	assert.Contains(t, msgs[0].Content, "SYS")
	assert.Contains(t, msgs[0].Content, "история чата")
	assert.Equal(t, deepseek.RoleUser, msgs[1].Role)
	assert.Equal(t, "[14:00 @alice] привет", msgs[1].Content)
	assert.Equal(t, deepseek.RoleUser, msgs[2].Role)
	assert.Equal(t, "[14:01 @alice] ответь", msgs[2].Content)
}

func TestBuildChatCompletions_BotEntryBecomesAssistantRole(t *testing.T) {
	t.Parallel()

	history := []chathistory.Entry{
		{ID: 1, Time: time.Date(2026, 4, 24, 14, 0, 0, 0, time.UTC), Author: "@alice", Text: "привет"},
		{ID: 2, Time: time.Date(2026, 4, 24, 14, 0, 1, 0, time.UTC), Author: "бот", Text: "отвали", FromBot: true, ReplyToID: 1},
	}
	trigger := chathistory.Entry{
		ID: 3, Time: time.Date(2026, 4, 24, 14, 0, 2, 0, time.UTC),
		Author: "@alice", Text: "сам такой", ReplyToID: 2,
	}

	history = append(history, trigger)
	msgs := buildChatCompletions("SYS", history)

	require.Len(t, msgs, 4)
	assert.Equal(t, deepseek.RoleUser, msgs[1].Role)
	assert.Equal(t, "[14:00 @alice] привет", msgs[1].Content)
	assert.Equal(t, deepseek.RoleAssistant, msgs[2].Role)
	assert.Equal(t, "отвали", msgs[2].Content)
	assert.Equal(t, deepseek.RoleUser, msgs[3].Role)
	assert.Equal(t, "[14:00 @alice → бот] сам такой", msgs[3].Content)
}

func TestBuildChatCompletions_EmptyHistory(t *testing.T) {
	t.Parallel()

	history := []chathistory.Entry{
		{ID: 1, Time: time.Date(2026, 4, 24, 14, 0, 0, 0, time.UTC), Author: "@alice", Text: "привет"},
	}

	msgs := buildChatCompletions("SYS", history)

	require.Len(t, msgs, 2)
	assert.Equal(t, deepseek.RoleSystem, msgs[0].Role)
	assert.Equal(t, deepseek.RoleUser, msgs[1].Role)
	assert.Equal(t, "[14:00 @alice] привет", msgs[1].Content)
}
