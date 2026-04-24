package message

import (
	"fmt"

	"github.com/reijo1337/ToxicBot/internal/features/chathistory"
	"github.com/reijo1337/ToxicBot/internal/infrastructure/ai/deepseek"
)

const historyPromptSuffix = "\n\nНиже история чата: роль user — чужие реплики (имя и время в префиксе), " +
	"роль assistant — твои собственные прошлые ответы. " +
	"Когда пользователь отвечает на чьё-то сообщение, префикс будет " +
	"\"[HH:MM @alice → @bob] текст\"."

func formatUserContent(e chathistory.Entry, history []chathistory.Entry) string {
	stamp := e.Time.Format("15:04")
	if e.ReplyToID != 0 {
		for _, h := range history {
			if h.ID == e.ReplyToID {
				return fmt.Sprintf("[%s %s → %s] %s", stamp, e.Author, h.Author, e.Text)
			}
		}
	}
	return fmt.Sprintf("[%s %s] %s", stamp, e.Author, e.Text)
}

// buildChatCompletions assembles messages for DeepSeek: one system prompt
// followed by each entry from history in chronological order. Bot entries
// become role=assistant; user entries become role=user with a formatted
// "[HH:MM @alice → @bob] текст" content. The trigger message is expected to
// already be the last element of history (handlers add it before calling).
func buildChatCompletions(
	system string,
	history []chathistory.Entry,
) []deepseek.ChatMessage {
	msgs := make([]deepseek.ChatMessage, 0, len(history)+1)
	msgs = append(msgs, deepseek.ChatMessage{
		Role:    deepseek.RoleSystem,
		Content: system + historyPromptSuffix,
	})

	for _, e := range history {
		if e.FromBot {
			msgs = append(msgs, deepseek.ChatMessage{
				Role:    deepseek.RoleAssistant,
				Content: e.Text,
			})
			continue
		}
		msgs = append(msgs, deepseek.ChatMessage{
			Role:    deepseek.RoleUser,
			Content: formatUserContent(e, history),
		})
	}

	return msgs
}
