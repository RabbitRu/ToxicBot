package on_photo

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math/rand"
	"testing"
	"time"

	"github.com/reijo1337/ToxicBot/internal/features/chathistory"
	"github.com/reijo1337/ToxicBot/internal/features/chatsettings"
	"github.com/reijo1337/ToxicBot/internal/features/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gopkg.in/telebot.v3"
)

const (
	testBotID  = int64(42)
	testChatID = int64(100)
)

// fakeContext is a minimal telebot.Context stub. Unused methods panic via the
// embedded nil interface — tests must only touch methods the handler calls.
type fakeContext struct {
	telebot.Context

	chat   *telebot.Chat
	sender *telebot.User
	msg    *telebot.Message
}

func (c *fakeContext) Chat() *telebot.Chat       { return c.chat }
func (c *fakeContext) Sender() *telebot.User     { return c.sender }
func (c *fakeContext) Message() *telebot.Message { return c.msg }
func (*fakeContext) Notify(telebot.ChatAction) error {
	return nil
}

type testEnv struct {
	ctrl       *gomock.Controller
	describer  *MockimageDescriber
	generator  *MockmessageGenerator
	settings   *MocksettingsProvider
	history    *MockhistoryBuffer
	downloader *Mockdownloader
	fileReader *MockfileReader
	replier    *MockbotReplier
	logger     *Mocklogger
	statIncer  *MockstatIncer
	handler    *Handler
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()

	ctrl := gomock.NewController(t)
	env := &testEnv{
		ctrl:       ctrl,
		describer:  NewMockimageDescriber(ctrl),
		generator:  NewMockmessageGenerator(ctrl),
		settings:   NewMocksettingsProvider(ctrl),
		history:    NewMockhistoryBuffer(ctrl),
		downloader: NewMockdownloader(ctrl),
		fileReader: NewMockfileReader(ctrl),
		replier:    NewMockbotReplier(ctrl),
		logger:     NewMocklogger(ctrl),
		statIncer:  NewMockstatIncer(ctrl),
	}

	env.handler = New(
		context.Background(),
		env.describer,
		env.generator,
		env.settings,
		env.history,
		env.downloader,
		env.fileReader,
		env.replier,
		env.logger,
		env.statIncer,
		testBotID,
	)
	env.handler.r = rand.New(rand.NewSource(0))

	env.statIncer.EXPECT().
		Inc(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		AnyTimes()

	return env
}

func photoMessage(msgID int, caption string, replyTo *telebot.Message) *telebot.Message {
	return &telebot.Message{
		ID:       msgID,
		Unixtime: time.Date(2026, 4, 24, 14, 30, 0, 0, time.UTC).Unix(),
		Caption:  caption,
		Photo:    &telebot.Photo{File: telebot.File{FileID: "photo-id"}},
		ReplyTo:  replyTo,
	}
}

func replyToBotMessage() *telebot.Message {
	return &telebot.Message{ID: 999, Sender: &telebot.User{ID: testBotID}}
}

func newCtx(msg *telebot.Message, sender *telebot.User) *fakeContext {
	return &fakeContext{chat: &telebot.Chat{ID: testChatID}, sender: sender, msg: msg}
}

func goodSender() *telebot.User {
	return &telebot.User{ID: 7, FirstName: "Alice", Username: "alice"}
}

func (env *testEnv) setupPhotoPipeline(description string) {
	env.downloader.EXPECT().
		FileByID("photo-id").
		Return(telebot.File{FileID: "photo-id"}, nil)
	env.fileReader.EXPECT().
		ReadFile(gomock.Any()).
		Return(io.NopCloser(bytes.NewReader([]byte("img"))), nil)
	env.describer.EXPECT().
		GenerateContent(gomock.Any(), describePrompt, []byte("img")).
		Return(description, nil)
}

func TestHandle_HappyPath_WritesPairViaAddAll(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	env.setupPhotoPipeline("кот ест торт")

	msg := photoMessage(50, "смотри", replyToBotMessage())
	ctx := newCtx(msg, goodSender())

	var capturedHistory []chathistory.Entry
	var capturedPair []chathistory.Entry
	gomock.InOrder(
		env.history.EXPECT().Get(testChatID).Return([]chathistory.Entry{
			{ID: 1, Author: "@bob", Text: "past"},
		}),
		env.generator.EXPECT().
			GetMessageTextWithHistory(gomock.Any(), float32(1.0), true).
			DoAndReturn(func(h []chathistory.Entry, _ float32, _ bool) message.GenerationResult {
				capturedHistory = h
				return message.GenerationResult{
					Message:  "отвали",
					Strategy: message.AiGenerationStrategy,
				}
			}),
		env.replier.EXPECT().Reply(msg, "отвали").Return(&telebot.Message{ID: 51}, nil),
		env.history.EXPECT().
			AddAll(testChatID, gomock.Any(), gomock.Any()).
			Do(func(_ int64, entries ...chathistory.Entry) { capturedPair = entries }),
	)

	require.NoError(t, env.handler.Handle(ctx))

	// historyForLLM must equal past + trigger (2 items)
	require.Len(t, capturedHistory, 2)
	assert.Equal(t, "past", capturedHistory[0].Text)
	assert.Equal(t, "@alice", capturedHistory[1].Author)
	assert.Contains(t, capturedHistory[1].Text, "кот ест торт")

	// AddAll must receive the user entry and bot entry together
	require.Len(t, capturedPair, 2)
	assert.Equal(t, 50, capturedPair[0].ID)
	assert.Equal(t, "@alice", capturedPair[0].Author)
	assert.Equal(t, 999, capturedPair[0].ReplyToID)
	assert.False(t, capturedPair[0].FromBot)

	assert.Equal(t, 51, capturedPair[1].ID)
	assert.Equal(t, "бот", capturedPair[1].Author)
	assert.Equal(t, "отвали", capturedPair[1].Text)
	assert.Equal(t, 50, capturedPair[1].ReplyToID)
	assert.True(t, capturedPair[1].FromBot)
}

func TestHandle_NotReplyToBot_ChanceMiss_NoHistoryWrite(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)

	msg := photoMessage(50, "", nil)
	ctx := newCtx(msg, goodSender())

	env.settings.EXPECT().GetForChat(gomock.Any(), testChatID).
		Return(&chatsettings.Settings{PhotoReactChance: 0.0}, nil)

	// No Get / AddAll expected.
	require.NoError(t, env.handler.Handle(ctx))
}

func TestHandle_NilChat_ReturnsNil(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)

	ctx := &fakeContext{chat: nil, sender: goodSender(), msg: photoMessage(1, "", nil)}
	require.NoError(t, env.handler.Handle(ctx))
}

func TestHandle_NilSender_ReturnsNil(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)

	ctx := &fakeContext{
		chat:   &telebot.Chat{ID: testChatID},
		sender: nil,
		msg:    photoMessage(1, "", nil),
	}
	require.NoError(t, env.handler.Handle(ctx))
}

func TestHandle_MessageWithoutPhoto_ReturnsNil(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)

	msg := &telebot.Message{ID: 1}
	ctx := newCtx(msg, goodSender())
	require.NoError(t, env.handler.Handle(ctx))
}

func TestHandle_DescriberError_NoHistoryWrite(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)

	msg := photoMessage(50, "", replyToBotMessage())
	ctx := newCtx(msg, goodSender())

	env.downloader.EXPECT().FileByID("photo-id").
		Return(telebot.File{FileID: "photo-id"}, nil)
	env.fileReader.EXPECT().ReadFile(gomock.Any()).
		Return(io.NopCloser(bytes.NewReader([]byte("img"))), nil)
	env.describer.EXPECT().GenerateContent(gomock.Any(), describePrompt, []byte("img")).
		Return("", errors.New("gigachat down"))
	env.logger.EXPECT().WithError(gomock.Any(), gomock.Any()).Return(context.Background())
	env.logger.EXPECT().Warn(gomock.Any(), "can't describe image")

	// No AddAll, no Get expected.
	require.NoError(t, env.handler.Handle(ctx))
}

func TestHandle_ReplierError_NoAddAll(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	env.setupPhotoPipeline("описание")

	msg := photoMessage(50, "", replyToBotMessage())
	ctx := newCtx(msg, goodSender())

	gomock.InOrder(
		env.history.EXPECT().Get(testChatID).Return([]chathistory.Entry{}),
		env.generator.EXPECT().
			GetMessageTextWithHistory(gomock.Any(), float32(1.0), true).
			Return(message.GenerationResult{Message: "бля", Strategy: message.AiGenerationStrategy}),
		env.replier.EXPECT().Reply(msg, "бля").Return(nil, errors.New("telegram down")),
	)
	// No AddAll — test fails if it happens.

	err := env.handler.Handle(ctx)
	require.Error(t, err)
}

func TestHandle_AlbumDedup_SkipsSecondPhotoInSameAlbum(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	env.setupPhotoPipeline("кот")

	first := photoMessage(50, "", replyToBotMessage())
	first.AlbumID = "album-1"
	second := photoMessage(51, "", replyToBotMessage())
	second.AlbumID = "album-1"

	gomock.InOrder(
		env.history.EXPECT().Get(testChatID).Return([]chathistory.Entry{}),
		env.generator.EXPECT().
			GetMessageTextWithHistory(gomock.Any(), float32(1.0), true).
			Return(message.GenerationResult{Message: "ok", Strategy: message.AiGenerationStrategy}),
		env.replier.EXPECT().Reply(first, "ok").Return(&telebot.Message{ID: 60}, nil),
		env.history.EXPECT().AddAll(testChatID, gomock.Any(), gomock.Any()),
	)

	require.NoError(t, env.handler.Handle(newCtx(first, goodSender())))
	require.NoError(t, env.handler.Handle(newCtx(second, goodSender())))
}

func TestHandle_UsesFirstNameWhenUsernameEmpty(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	env.setupPhotoPipeline("что-то")

	sender := &telebot.User{ID: 7, FirstName: "Боб"}
	msg := photoMessage(50, "", replyToBotMessage())
	ctx := newCtx(msg, sender)

	var capturedPair []chathistory.Entry
	gomock.InOrder(
		env.history.EXPECT().Get(testChatID).Return([]chathistory.Entry{}),
		env.generator.EXPECT().
			GetMessageTextWithHistory(gomock.Any(), float32(1.0), true).
			Return(message.GenerationResult{Message: "ok", Strategy: message.AiGenerationStrategy}),
		env.replier.EXPECT().Reply(msg, "ok").Return(&telebot.Message{ID: 51}, nil),
		env.history.EXPECT().
			AddAll(testChatID, gomock.Any(), gomock.Any()).
			Do(func(_ int64, entries ...chathistory.Entry) { capturedPair = entries }),
	)

	require.NoError(t, env.handler.Handle(ctx))
	require.Len(t, capturedPair, 2)
	assert.Equal(t, "Боб", capturedPair[0].Author)
}
