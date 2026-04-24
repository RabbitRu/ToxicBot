package message

import (
	"context"
	"testing"
	"time"

	"github.com/reijo1337/ToxicBot/internal/features/chathistory"
	"github.com/reijo1337/ToxicBot/internal/infrastructure/ai/deepseek"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestGenerator_WithHistory_SendsChatCompletionsShape(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	aiMock := NewMockai(ctrl)
	rnd := NewMockrandomizer(ctrl)
	filter := NewMockmeaningfullFilter(ctrl)

	rnd.EXPECT().Float32().Return(float32(0.0))
	filter.EXPECT().IsMeaningfulPhrase("йо").Return(true)

	var captured []deepseek.ChatMessage
	aiMock.EXPECT().
		Chat(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, msgs ...deepseek.ChatMessage) (string, error) {
			captured = msgs
			return "ответ", nil
		})

	g := &Generator{
		r:                 rnd,
		ai:                aiMock,
		meaningfullFilter: filter,
		systemPrompt:      "SYS",
	}

	history := []chathistory.Entry{
		{
			ID:     1,
			Time:   time.Date(2026, 4, 24, 14, 0, 0, 0, time.UTC),
			Author: "@alice",
			Text:   "привет",
		},
		{
			ID:        2,
			Time:      time.Date(2026, 4, 24, 14, 0, 1, 0, time.UTC),
			Author:    "бот",
			Text:      "отвали",
			FromBot:   true,
			ReplyToID: 1,
		},
		{
			ID:        3,
			Time:      time.Date(2026, 4, 24, 14, 0, 2, 0, time.UTC),
			Author:    "@alice",
			Text:      "йо",
			ReplyToID: 2,
		},
	}

	res := g.GetMessageTextWithHistory(history, 1.0, false)

	assert.Equal(t, AiGenerationStrategy, res.Strategy)
	assert.Equal(t, "ответ", res.Message)
	require.Len(t, captured, 4)
	assert.Equal(t, deepseek.RoleSystem, captured[0].Role)
	assert.Equal(t, deepseek.RoleUser, captured[1].Role)
	assert.Equal(t, deepseek.RoleAssistant, captured[2].Role)
	assert.Equal(t, deepseek.RoleUser, captured[3].Role)
	assert.Equal(t, "[14:00 @alice → бот] йо", captured[3].Content)
}

func TestGenerator_WithHistory_FallbackOnAiChanceMiss(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	aiMock := NewMockai(ctrl)
	rnd := NewMockrandomizer(ctrl)
	filter := NewMockmeaningfullFilter(ctrl)

	rnd.EXPECT().Float32().Return(float32(0.9))
	rnd.EXPECT().Intn(1).Return(0)

	g := &Generator{
		r:                 rnd,
		ai:                aiMock,
		meaningfullFilter: filter,
		messages:          []string{"ха-ха"},
		systemPrompt:      "SYS",
	}

	history := []chathistory.Entry{{ID: 1, Author: "@alice", Text: "йо"}}
	res := g.GetMessageTextWithHistory(history, 0.5, false)

	assert.Equal(t, ByListGenerationStrategy, res.Strategy)
	assert.Equal(t, "ха-ха", res.Message)
}

func TestGenerator_WithHistory_ForceAI_BypassesFilterAndProbability(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	aiMock := NewMockai(ctrl)
	rnd := NewMockrandomizer(ctrl)
	filter := NewMockmeaningfullFilter(ctrl)
	aiMock.EXPECT().Chat(gomock.Any(), gomock.Any(), gomock.Any()).Return("ок", nil)

	g := &Generator{
		r:                 rnd,
		ai:                aiMock,
		meaningfullFilter: filter,
		systemPrompt:      "SYS",
	}

	history := []chathistory.Entry{{ID: 1, Author: "@alice", Text: "нечто"}}
	res := g.GetMessageTextWithHistory(history, 0.0, true)

	assert.Equal(t, AiGenerationStrategy, res.Strategy)
	assert.Equal(t, "ок", res.Message)
}

func TestGenerator_WithHistory_EmptyHistory_FallsBackToList(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	aiMock := NewMockai(ctrl)
	rnd := NewMockrandomizer(ctrl)
	filter := NewMockmeaningfullFilter(ctrl)

	rnd.EXPECT().Intn(1).Return(0)

	g := &Generator{
		r:                 rnd,
		ai:                aiMock,
		meaningfullFilter: filter,
		messages:          []string{"fallback"},
		systemPrompt:      "SYS",
	}

	res := g.GetMessageTextWithHistory(nil, 1.0, false)
	assert.Equal(t, ByListGenerationStrategy, res.Strategy)
	assert.Equal(t, "fallback", res.Message)
}
