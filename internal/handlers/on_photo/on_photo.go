package on_photo

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/reijo1337/ToxicBot/internal/features/stats"
	"github.com/reijo1337/ToxicBot/internal/message"
	"gopkg.in/telebot.v3"
)

const describePrompt = "Опиши подробно что изображено на картинке: объекты, люди, их действия, обстановка, эмоции, детали одежды, текст если есть. 3-5 предложений."

type Handler struct {
	ctx              context.Context
	describer        imageDescriber
	generator        messageGenerator
	settingsProvider settingsProvider
	history          historyBuffer
	downloader       downloader
	fileReader       fileReader
	logger           logger
	statIncer        statIncer
	botID            int64
	r                *rand.Rand
	processedGroups  map[string]struct{}
	muGroups         sync.Mutex
}

func New(
	ctx context.Context,
	describer imageDescriber,
	generator messageGenerator,
	settingsProvider settingsProvider,
	history historyBuffer,
	downloader downloader,
	fileReader fileReader,
	logger logger,
	statIncer statIncer,
	botID int64,
) *Handler {
	return &Handler{
		ctx:              ctx,
		describer:        describer,
		generator:        generator,
		settingsProvider: settingsProvider,
		history:          history,
		downloader:       downloader,
		fileReader:       fileReader,
		logger:           logger,
		statIncer:        statIncer,
		botID:            botID,
		r:                rand.New(rand.NewSource(time.Now().UnixNano())),
		processedGroups:  make(map[string]struct{}),
	}
}

func (h *Handler) Slug() string {
	return "on_photo"
}

func (h *Handler) Handle(ctx telebot.Context) error {
	chat := ctx.Chat()
	sender := ctx.Sender()

	if chat == nil || sender == nil {
		return nil
	}

	msg := ctx.Message()
	if msg == nil || msg.Photo == nil {
		return nil
	}

	// Альбом — обрабатываем только первое фото
	if msg.AlbumID != "" {
		if !h.tryClaimAlbum(msg.AlbumID) {
			return nil
		}
	}

	isReply := h.isReplyToBot(ctx)

	if !isReply {
		settings, err := h.settingsProvider.GetForChat(h.ctx, chat.ID)
		if err != nil {
			return fmt.Errorf("can't get chat settings: %w", err)
		}

		if h.r.Float32() > settings.PhotoReactChance {
			return nil
		}
	}

	// Скачиваем файл
	file, err := h.downloader.FileByID(msg.Photo.FileID)
	if err != nil {
		h.logger.Warn(
			h.logger.WithError(h.ctx, err),
			"can't download photo",
		)
		return nil
	}

	reader, err := h.fileReader.ReadFile(&file)
	if err != nil {
		h.logger.Warn(
			h.logger.WithError(h.ctx, err),
			"can't get photo reader",
		)
		return nil
	}
	defer reader.Close() //nolint

	imageBytes, err := io.ReadAll(reader)
	if err != nil {
		h.logger.Warn(
			h.logger.WithError(h.ctx, err),
			"can't read photo bytes",
		)
		return nil
	}

	description, err := h.describer.GenerateContent(h.ctx, describePrompt, imageBytes)
	if err != nil {
		h.logger.Warn(
			h.logger.WithError(h.ctx, err),
			"can't describe image",
		)
		return nil
	}

	author := formatAuthor(sender)
	promptText := buildPrompt(msg.Caption, description)

	history := h.history.Get(chat.ID)
	result := h.generator.GetMessageTextWithHistory(
		history,
		message.HistoryMessage{Author: author, Text: promptText},
		1.0,
		true,
	)

	h.history.Add(chat.ID, author, promptText)

	go h.statIncer.Inc(
		h.ctx,
		chat.ID,
		sender.ID,
		stats.OnPhotoOperationType,
		stats.WithGenStrategy(result.Strategy),
	)

	if err := ctx.Notify(telebot.Typing); err != nil {
		return err
	}
	time.Sleep(time.Duration((float64(h.r.Intn(3)) + h.r.Float64()) * 1_000_000_000))

	return ctx.Reply(result.Message)
}

func (h *Handler) isReplyToBot(ctx telebot.Context) bool {
	replyTo := ctx.Message().ReplyTo
	if replyTo == nil || replyTo.Sender == nil {
		return false
	}
	return replyTo.Sender.ID == h.botID
}

func (h *Handler) tryClaimAlbum(albumID string) bool {
	h.muGroups.Lock()
	defer h.muGroups.Unlock()

	if _, ok := h.processedGroups[albumID]; ok {
		return false
	}

	h.processedGroups[albumID] = struct{}{}

	go func() {
		time.Sleep(time.Minute)
		h.muGroups.Lock()
		delete(h.processedGroups, albumID)
		h.muGroups.Unlock()
	}()

	return true
}

func formatAuthor(user *telebot.User) string {
	if user.Username != "" {
		return "@" + user.Username
	}
	return user.FirstName
}

func buildPrompt(caption, description string) string {
	var sb strings.Builder

	sb.WriteString("Пользователь")

	if caption != "" {
		sb.WriteString(" отправил фото с подписью: '")
		sb.WriteString(caption)
		sb.WriteString("'. На фото: ")
	} else {
		sb.WriteString(" отправил фото. На фото: ")
	}

	sb.WriteString(description)

	return sb.String()
}
