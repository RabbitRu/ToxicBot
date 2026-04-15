package stat

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/reijo1337/ToxicBot/internal/features/stats"
	"github.com/reijo1337/ToxicBot/internal/message"
	"gopkg.in/telebot.v3"
)

var (
	opTypeEmoji = []struct {
		opType stats.OperationType
		emoji  string
	}{
		{stats.OnTextOperationType, "📝"},
		{stats.OnStickerOperationType, "🖼"},
		{stats.OnVoiceOperationType, "🔊"},
		{stats.OnUserJoinOperationType, "🚶‍➡️"},
		{stats.OnUserLeftOperationType, "🧎‍♂️"},
		{stats.PersonalOperationType, "🤱"},
		{stats.TaggerOperationType, "🌀"},
		{stats.OnPhotoOperationType, "📷"},
	}

	getTypeEmoji = []struct {
		genType message.GenerationStrategy
		empji   string
	}{
		{message.ByListGenerationStrategy, "📖"},
		{message.AiGenerationStrategy, "🤖"},
	}
)

type Handler struct {
	ctx     context.Context
	storage storage
}

func New(ctx context.Context, storage storage) *Handler {
	return &Handler{
		ctx:     ctx,
		storage: storage,
	}
}

func (h *Handler) Handle(ctx telebot.Context) error {
	if args := ctx.Args(); len(args) > 0 {
		return h.handleDetailedStat(ctx, args[0])
	}

	return h.handleTotalStat(ctx)
}

func (h *Handler) handleDetailedStat(ctx telebot.Context, date string) error {
	datetime, err := time.Parse(time.DateOnly, date)
	if err != nil {
		return ctx.Reply("Ты что, еблан? Формат даты должен быть YYYY-MM-DD")
	}

	datetime = time.Date(datetime.Year(), datetime.Month(), datetime.Day(), 0, 0, 0, 0, time.UTC)

	stats, err := h.storage.GetDetailedStat(h.ctx, datetime)
	if err != nil {
		return fmt.Errorf("can't get detailed stat for date '%s': %w", date, err)
	}

	if len(stats) == 0 {
		return ctx.Reply("Нихуя не найдено, иди нахуй")
	}

	result := "Статистика за " + date

	entities := telebot.Entities{telebot.MessageEntity{
		Type:   telebot.EntityBold,
		Offset: 0,
		Length: len([]rune(result)),
	}}

	for _, ds := range stats {
		result += "\n"

		start := len([]rune(result))
		result += "Чат " + strconv.FormatUint(ds.ChatNumber, 10) + "\n"
		end := len([]rune(result))

		entities = append(
			entities,
			telebot.MessageEntity{
				Type:   telebot.EntityBold,
				Offset: start,
				Length: end - start,
			},
			telebot.MessageEntity{
				Type:   telebot.EntityItalic,
				Offset: start,
				Length: end - start,
			},
		)

		result += "Забулено юзеров: " + strconv.FormatUint(ds.BulledUsers, 10) + "\n"

		result += "Типы взаимодействия:\n"
		otes := make([]string, 0, len(opTypeEmoji))
		for _, ote := range opTypeEmoji {
			otes = append(otes, fmt.Sprintf("%d %s", ds.ByOpTypeStat[ote.opType], ote.emoji))
		}
		result += strings.Join(otes, " / ") + "\n"

		result += "Тип генерации текста:\n"
		gtes := make([]string, 0, len(getTypeEmoji))
		for _, ote := range getTypeEmoji {
			gtes = append(gtes, fmt.Sprintf("%d %s", ds.ByGenTypeStat[ote.genType], ote.empji))
		}
		result += strings.Join(gtes, " / ")
	}

	return ctx.Reply(result, entities)
}

func (h *Handler) handleTotalStat(ctx telebot.Context) error {
	total, err := h.storage.GetTotalStat(h.ctx)
	if err != nil {
		return fmt.Errorf("can't get total stat: %w", err)
	}

	if total == nil {
		return ctx.Reply("Нихуя не найдено, иди нахуй")
	}

	result := "Полная статистика:\n"
	entities := telebot.Entities{telebot.MessageEntity{
		Type:   telebot.EntityBold,
		Offset: 0,
		Length: len([]rune(result)),
	}}

	result += "Забулено чатов: " + strconv.FormatUint(total.BulledChats, 10) + "\n"

	result += "Забулено юзеров: " + strconv.FormatUint(total.BulledUsers, 10) + "\n"

	result += "Самый ранний день со статистикой: " + total.OldestDate.Format(time.DateOnly) + "\n"

	result += "Типы взаимодействия:\n"

	otes := make([]string, 0, len(opTypeEmoji))
	for _, ote := range opTypeEmoji {
		otes = append(otes, fmt.Sprintf("%d %s", total.ByOpTypeStat[ote.opType], ote.emoji))
	}
	result += strings.Join(otes, " / ") + "\n"

	result += "Тип генерации текста:\n"

	gtes := make([]string, 0, len(getTypeEmoji))
	for _, ote := range getTypeEmoji {
		gtes = append(gtes, fmt.Sprintf("%d %s", total.ByGenTypeStat[ote.genType], ote.empji))
	}
	result += strings.Join(gtes, " / ")

	return ctx.Reply(result, entities)
}
