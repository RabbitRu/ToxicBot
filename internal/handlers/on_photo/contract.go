//go:generate go tool go.uber.org/mock/mockgen -source $GOFILE -destination mocks_test.go -package ${GOPACKAGE}
package on_photo

import (
	"context"
	"io"

	"github.com/reijo1337/ToxicBot/internal/chatsettings"
	"github.com/reijo1337/ToxicBot/internal/features/stats"
	"github.com/reijo1337/ToxicBot/internal/message"
	"gopkg.in/telebot.v3"
)

type imageDescriber interface {
	GenerateContent(ctx context.Context, prompt string, imageBytes []byte) (string, error)
}

type messageGenerator interface {
	GetMessageTextWithHistory(
		history []message.HistoryMessage,
		replyTo message.HistoryMessage,
		aiChance float32,
		forceAI bool,
	) message.GenerationResult
}

type settingsProvider interface {
	GetForChat(ctx context.Context, chatID int64) (*chatsettings.Settings, error)
}

type historyBuffer interface {
	Add(chatID int64, author, text string)
	Get(chatID int64) []message.HistoryMessage
}

type downloader interface {
	FileByID(fileID string) (telebot.File, error)
}

type fileReader interface {
	ReadFile(file *telebot.File) (io.ReadCloser, error)
}

type logger interface {
	WithError(context.Context, error) context.Context
	Warn(context.Context, string)
}

type statIncer interface {
	Inc(ctx context.Context, chatID, userID int64, op stats.OperationType, opts ...stats.Option)
}
