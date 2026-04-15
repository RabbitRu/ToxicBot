package on_photo //nolint:testpackage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math/rand"
	"testing"
	"time"

	"github.com/reijo1337/ToxicBot/internal/chatsettings"
	"github.com/reijo1337/ToxicBot/internal/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gopkg.in/telebot.v3"
)

// fakeContext implements telebot.Context for testing without real HTTP calls.
type fakeContext struct {
	chat      *telebot.Chat
	sender    *telebot.User
	msg       *telebot.Message
	bot       *telebot.Bot
	replied   interface{}
	store     map[string]interface{}
	notifyErr error
}

func (f *fakeContext) Bot() *telebot.Bot { return f.bot }

func (f *fakeContext) Update() telebot.Update                      { return telebot.Update{Message: f.msg} }
func (f *fakeContext) Message() *telebot.Message                   { return f.msg }
func (f *fakeContext) Callback() *telebot.Callback                 { return nil }
func (f *fakeContext) Query() *telebot.Query                       { return nil }
func (f *fakeContext) InlineResult() *telebot.InlineResult         { return nil }
func (f *fakeContext) ShippingQuery() *telebot.ShippingQuery       { return nil }
func (f *fakeContext) PreCheckoutQuery() *telebot.PreCheckoutQuery { return nil }
func (f *fakeContext) Poll() *telebot.Poll                         { return nil }
func (f *fakeContext) PollAnswer() *telebot.PollAnswer             { return nil }
func (f *fakeContext) ChatMember() *telebot.ChatMemberUpdate       { return nil }
func (f *fakeContext) ChatJoinRequest() *telebot.ChatJoinRequest   { return nil }
func (f *fakeContext) Migration() (int64, int64)                   { return 0, 0 }
func (f *fakeContext) Topic() *telebot.Topic                       { return nil }
func (f *fakeContext) Sender() *telebot.User                       { return f.sender }
func (f *fakeContext) Chat() *telebot.Chat                         { return f.chat }
func (f *fakeContext) Recipient() telebot.Recipient                { return f.chat }
func (f *fakeContext) Text() string {
	if f.msg != nil {
		return f.msg.Text
	}
	return ""
}
func (f *fakeContext) Entities() telebot.Entities                           { return nil }
func (f *fakeContext) Data() string                                         { return "" }
func (f *fakeContext) Args() []string                                       { return nil }
func (f *fakeContext) Send(what interface{}, opts ...interface{}) error     { return nil }
func (f *fakeContext) SendAlbum(a telebot.Album, opts ...interface{}) error { return nil }
func (f *fakeContext) Reply(what interface{}, opts ...interface{}) error {
	f.replied = what
	return nil
}
func (f *fakeContext) Forward(msg telebot.Editable, opts ...interface{}) error   { return nil }
func (f *fakeContext) ForwardTo(to telebot.Recipient, opts ...interface{}) error { return nil }
func (f *fakeContext) Edit(what interface{}, opts ...interface{}) error          { return nil }
func (f *fakeContext) EditCaption(caption string, opts ...interface{}) error     { return nil }
func (f *fakeContext) EditOrSend(what interface{}, opts ...interface{}) error    { return nil }
func (f *fakeContext) EditOrReply(what interface{}, opts ...interface{}) error   { return nil }
func (f *fakeContext) Delete() error                                             { return nil }

func (f *fakeContext) DeleteAfter(
	d time.Duration,
) *time.Timer {
	return time.NewTimer(d)
}

func (f *fakeContext) Notify(
	action telebot.ChatAction,
) error {
	return f.notifyErr
}
func (f *fakeContext) Ship(what ...interface{}) error                  { return nil }
func (f *fakeContext) Accept(errorMessage ...string) error             { return nil }
func (f *fakeContext) Answer(resp *telebot.QueryResponse) error        { return nil }
func (f *fakeContext) Respond(resp ...*telebot.CallbackResponse) error { return nil }
func (f *fakeContext) Get(key string) interface{} {
	if f.store == nil {
		return nil
	}
	return f.store[key]
}

func (f *fakeContext) Set(key string, val interface{}) {
	if f.store == nil {
		f.store = make(map[string]interface{})
	}
	f.store[key] = val
}

type testEnv struct {
	ctrl       *gomock.Controller
	describer  *MockimageDescriber
	generator  *MockmessageGenerator
	settings   *MocksettingsProvider
	history    *MockhistoryBuffer
	downloader *Mockdownloader
	fileReader *MockfileReader
	logger     *Mocklogger
	statIncer  *MockstatIncer
	handler    *Handler
	ctx        context.Context
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
		logger:     NewMocklogger(ctrl),
		statIncer:  NewMockstatIncer(ctrl),
		ctx:        context.Background(),
	}

	env.handler = New(
		env.ctx,
		env.describer,
		env.generator,
		env.settings,
		env.history,
		env.downloader,
		env.fileReader,
		env.logger,
		env.statIncer,
		42,
	)
	// Use deterministic random for tests
	env.handler.r = rand.New(rand.NewSource(0))

	return env
}

func newFakeCtx(chat *telebot.Chat, sender *telebot.User, msg *telebot.Message) *fakeContext {
	return &fakeContext{
		chat:   chat,
		sender: sender,
		msg:    msg,
		bot: &telebot.Bot{
			Me: &telebot.User{ID: 42, Username: "testbot"},
		},
	}
}

func defaultChat() *telebot.Chat {
	return &telebot.Chat{ID: 100}
}

func defaultSender() *telebot.User {
	return &telebot.User{ID: 200, Username: "testuser", FirstName: "Test"}
}

func defaultPhoto() *telebot.Photo {
	return &telebot.Photo{
		File: telebot.File{FileID: "photo123"},
	}
}

func defaultMessage(
	chat *telebot.Chat,
	sender *telebot.User,
	photo *telebot.Photo,
) *telebot.Message {
	return &telebot.Message{
		Chat:   chat,
		Sender: sender,
		Photo:  photo,
	}
}

func TestHandle_NilChat(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)

	msg := &telebot.Message{
		Sender: defaultSender(),
		Photo:  defaultPhoto(),
	}

	ctx := newFakeCtx(nil, defaultSender(), msg)

	err := env.handler.Handle(ctx)
	require.NoError(t, err)
}

func TestHandle_NilSender(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)

	msg := &telebot.Message{
		Chat:  defaultChat(),
		Photo: defaultPhoto(),
	}

	ctx := newFakeCtx(defaultChat(), nil, msg)

	err := env.handler.Handle(ctx)
	require.NoError(t, err)
}

func TestHandle_NilPhoto(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)

	msg := &telebot.Message{
		Chat:   defaultChat(),
		Sender: defaultSender(),
	}

	ctx := newFakeCtx(defaultChat(), defaultSender(), msg)

	err := env.handler.Handle(ctx)
	require.NoError(t, err)
}

func TestHandle_ChanceNotPassed(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)

	chat := defaultChat()
	sender := defaultSender()
	photo := defaultPhoto()
	msg := defaultMessage(chat, sender, photo)

	ctx := newFakeCtx(chat, sender, msg)

	env.settings.EXPECT().
		GetForChat(gomock.Any(), chat.ID).
		Return(&chatsettings.Settings{PhotoReactChance: 0.0}, nil)

	err := env.handler.Handle(ctx)
	require.NoError(t, err)
}

func TestHandle_PhotoWithoutCaption(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)

	chat := defaultChat()
	sender := defaultSender()
	photo := defaultPhoto()
	msg := defaultMessage(chat, sender, photo)

	ctx := newFakeCtx(chat, sender, msg)

	imageData := []byte("fake-image-data")
	description := "A cat sitting on a chair"

	env.settings.EXPECT().
		GetForChat(gomock.Any(), chat.ID).
		Return(&chatsettings.Settings{PhotoReactChance: 1.0}, nil)

	env.downloader.EXPECT().
		FileByID(photo.FileID).
		Return(telebot.File{FileID: photo.FileID}, nil)

	env.fileReader.EXPECT().
		ReadFile(gomock.Any()).
		Return(io.NopCloser(bytes.NewReader(imageData)), nil)

	env.describer.EXPECT().
		GenerateContent(gomock.Any(), describePrompt, imageData).
		Return(description, nil)

	expectedPrompt := "Пользователь @testuser отправил фото. На фото: " + description

	env.history.EXPECT().
		Get(chat.ID).
		Return([]message.HistoryMessage{})

	env.generator.EXPECT().
		GetMessageTextWithHistory(
			[]message.HistoryMessage{},
			message.HistoryMessage{Author: "@testuser", Text: expectedPrompt},
			float32(1.0),
			true,
		).
		Return(message.GenerationResult{Message: "reply text", Strategy: message.AiGenerationStrategy})

	env.history.EXPECT().
		Add(chat.ID, "@testuser", expectedPrompt)

	env.statIncer.EXPECT().
		Inc(gomock.Any(), chat.ID, sender.ID, gomock.Any(), gomock.Any()).
		AnyTimes()

	err := env.handler.Handle(ctx)
	require.NoError(t, err)
	assert.Equal(t, "reply text", ctx.replied)
}

func TestHandle_PhotoWithCaption(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)

	chat := defaultChat()
	sender := defaultSender()
	photo := defaultPhoto()
	msg := defaultMessage(chat, sender, photo)
	msg.Caption = "Look at this!"

	ctx := newFakeCtx(chat, sender, msg)

	imageData := []byte("fake-image-data")
	description := "A dog playing fetch"

	env.settings.EXPECT().
		GetForChat(gomock.Any(), chat.ID).
		Return(&chatsettings.Settings{PhotoReactChance: 1.0}, nil)

	env.downloader.EXPECT().
		FileByID(photo.FileID).
		Return(telebot.File{FileID: photo.FileID}, nil)

	env.fileReader.EXPECT().
		ReadFile(gomock.Any()).
		Return(io.NopCloser(bytes.NewReader(imageData)), nil)

	env.describer.EXPECT().
		GenerateContent(gomock.Any(), describePrompt, imageData).
		Return(description, nil)

	expectedPrompt := "Пользователь @testuser отправил фото с подписью: 'Look at this!'. На фото: " + description

	env.history.EXPECT().
		Get(chat.ID).
		Return([]message.HistoryMessage{})

	env.generator.EXPECT().
		GetMessageTextWithHistory(
			[]message.HistoryMessage{},
			message.HistoryMessage{Author: "@testuser", Text: expectedPrompt},
			float32(1.0),
			true,
		).
		Return(message.GenerationResult{Message: "nice dog!", Strategy: message.AiGenerationStrategy})

	env.history.EXPECT().
		Add(chat.ID, "@testuser", expectedPrompt)

	env.statIncer.EXPECT().
		Inc(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		AnyTimes()

	err := env.handler.Handle(ctx)
	require.NoError(t, err)
	assert.Equal(t, "nice dog!", ctx.replied)
}

func TestHandle_ReplyToBot(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)

	chat := defaultChat()
	sender := defaultSender()
	photo := defaultPhoto()
	msg := defaultMessage(chat, sender, photo)
	msg.ReplyTo = &telebot.Message{
		Sender: &telebot.User{ID: 42},
	}

	ctx := newFakeCtx(chat, sender, msg)

	imageData := []byte("fake-image-data")
	description := "A sunset over the sea"

	// No settings call expected because reply to bot skips chance check
	env.downloader.EXPECT().
		FileByID(photo.FileID).
		Return(telebot.File{FileID: photo.FileID}, nil)

	env.fileReader.EXPECT().
		ReadFile(gomock.Any()).
		Return(io.NopCloser(bytes.NewReader(imageData)), nil)

	env.describer.EXPECT().
		GenerateContent(gomock.Any(), describePrompt, imageData).
		Return(description, nil)

	env.history.EXPECT().
		Get(chat.ID).
		Return([]message.HistoryMessage{})

	env.generator.EXPECT().
		GetMessageTextWithHistory(gomock.Any(), gomock.Any(), float32(1.0), true).
		Return(message.GenerationResult{Message: "beautiful!", Strategy: message.AiGenerationStrategy})

	env.history.EXPECT().
		Add(gomock.Any(), gomock.Any(), gomock.Any())

	env.statIncer.EXPECT().
		Inc(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		AnyTimes()

	err := env.handler.Handle(ctx)
	require.NoError(t, err)
	assert.Equal(t, "beautiful!", ctx.replied)
}

func TestHandle_Album_OnlyFirstProcessed(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)

	chat := defaultChat()
	sender := defaultSender()
	photo := defaultPhoto()

	albumID := "album_123"

	msg1 := defaultMessage(chat, sender, photo)
	msg1.AlbumID = albumID

	msg2 := defaultMessage(chat, sender, photo)
	msg2.AlbumID = albumID

	imageData := []byte("fake-image-data")
	description := "Album photo"

	// First photo should be processed
	env.settings.EXPECT().
		GetForChat(gomock.Any(), chat.ID).
		Return(&chatsettings.Settings{PhotoReactChance: 1.0}, nil)

	env.downloader.EXPECT().
		FileByID(photo.FileID).
		Return(telebot.File{FileID: photo.FileID}, nil)

	env.fileReader.EXPECT().
		ReadFile(gomock.Any()).
		Return(io.NopCloser(bytes.NewReader(imageData)), nil)

	env.describer.EXPECT().
		GenerateContent(gomock.Any(), describePrompt, imageData).
		Return(description, nil)

	env.history.EXPECT().
		Get(chat.ID).
		Return([]message.HistoryMessage{})

	env.generator.EXPECT().
		GetMessageTextWithHistory(gomock.Any(), gomock.Any(), float32(1.0), true).
		Return(message.GenerationResult{Message: "album reply", Strategy: message.AiGenerationStrategy})

	env.history.EXPECT().
		Add(gomock.Any(), gomock.Any(), gomock.Any())

	env.statIncer.EXPECT().
		Inc(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		AnyTimes()

	ctx1 := newFakeCtx(chat, sender, msg1)
	err1 := env.handler.Handle(ctx1)
	require.NoError(t, err1)

	// Second photo in the same album should be skipped
	ctx2 := newFakeCtx(chat, sender, msg2)
	err2 := env.handler.Handle(ctx2)
	require.NoError(t, err2)
}

func TestHandle_GeminiError(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)

	chat := defaultChat()
	sender := defaultSender()
	photo := defaultPhoto()
	msg := defaultMessage(chat, sender, photo)

	ctx := newFakeCtx(chat, sender, msg)

	imageData := []byte("fake-image-data")
	geminiErr := errors.New("gemini api error")

	env.settings.EXPECT().
		GetForChat(gomock.Any(), chat.ID).
		Return(&chatsettings.Settings{PhotoReactChance: 1.0}, nil)

	env.downloader.EXPECT().
		FileByID(photo.FileID).
		Return(telebot.File{FileID: photo.FileID}, nil)

	env.fileReader.EXPECT().
		ReadFile(gomock.Any()).
		Return(io.NopCloser(bytes.NewReader(imageData)), nil)

	env.describer.EXPECT().
		GenerateContent(gomock.Any(), describePrompt, imageData).
		Return("", geminiErr)

	env.logger.EXPECT().WithError(gomock.Any(), geminiErr).Return(context.Background())
	env.logger.EXPECT().Warn(gomock.Any(), "can't describe image")

	err := env.handler.Handle(ctx)
	require.NoError(t, err)
}

func TestHandle_DownloadError(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)

	chat := defaultChat()
	sender := defaultSender()
	photo := defaultPhoto()
	msg := defaultMessage(chat, sender, photo)

	ctx := newFakeCtx(chat, sender, msg)

	downloadErr := errors.New("download failed")

	env.settings.EXPECT().
		GetForChat(gomock.Any(), chat.ID).
		Return(&chatsettings.Settings{PhotoReactChance: 1.0}, nil)

	env.downloader.EXPECT().
		FileByID(photo.FileID).
		Return(telebot.File{}, downloadErr)

	env.logger.EXPECT().WithError(gomock.Any(), downloadErr).Return(context.Background())
	env.logger.EXPECT().Warn(gomock.Any(), "can't download photo")

	err := env.handler.Handle(ctx)
	require.NoError(t, err)
}

func TestHandle_PromptContainsUsername(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)

	chat := defaultChat()
	sender := &telebot.User{ID: 200, Username: "myuser", FirstName: "John"}
	photo := defaultPhoto()
	msg := defaultMessage(chat, sender, photo)

	ctx := newFakeCtx(chat, sender, msg)

	imageData := []byte("fake-image-data")
	description := "some description"

	env.settings.EXPECT().
		GetForChat(gomock.Any(), chat.ID).
		Return(&chatsettings.Settings{PhotoReactChance: 1.0}, nil)

	env.downloader.EXPECT().
		FileByID(photo.FileID).
		Return(telebot.File{FileID: photo.FileID}, nil)

	env.fileReader.EXPECT().
		ReadFile(gomock.Any()).
		Return(io.NopCloser(bytes.NewReader(imageData)), nil)

	env.describer.EXPECT().
		GenerateContent(gomock.Any(), describePrompt, imageData).
		Return(description, nil)

	var capturedPrompt message.HistoryMessage
	env.history.EXPECT().
		Get(chat.ID).
		Return([]message.HistoryMessage{})

	env.generator.EXPECT().
		GetMessageTextWithHistory(gomock.Any(), gomock.Any(), float32(1.0), true).
		DoAndReturn(func(history []message.HistoryMessage, replyTo message.HistoryMessage, aiChance float32) message.GenerationResult {
			capturedPrompt = replyTo
			return message.GenerationResult{
				Message:  "reply",
				Strategy: message.AiGenerationStrategy,
			}
		})

	env.history.EXPECT().
		Add(gomock.Any(), gomock.Any(), gomock.Any())

	env.statIncer.EXPECT().
		Inc(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		AnyTimes()

	err := env.handler.Handle(ctx)
	require.NoError(t, err)

	assert.Contains(t, capturedPrompt.Text, "@myuser")
	assert.Equal(t, "@myuser", capturedPrompt.Author)
}

func TestHandle_HistorySaved(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)

	chat := defaultChat()
	sender := defaultSender()
	photo := defaultPhoto()
	msg := defaultMessage(chat, sender, photo)

	ctx := newFakeCtx(chat, sender, msg)

	imageData := []byte("fake-image-data")
	description := "test description"

	env.settings.EXPECT().
		GetForChat(gomock.Any(), chat.ID).
		Return(&chatsettings.Settings{PhotoReactChance: 1.0}, nil)

	env.downloader.EXPECT().
		FileByID(photo.FileID).
		Return(telebot.File{FileID: photo.FileID}, nil)

	env.fileReader.EXPECT().
		ReadFile(gomock.Any()).
		Return(io.NopCloser(bytes.NewReader(imageData)), nil)

	env.describer.EXPECT().
		GenerateContent(gomock.Any(), describePrompt, imageData).
		Return(description, nil)

	expectedPrompt := "Пользователь @testuser отправил фото. На фото: " + description

	env.history.EXPECT().
		Get(chat.ID).
		Return([]message.HistoryMessage{})

	env.generator.EXPECT().
		GetMessageTextWithHistory(gomock.Any(), gomock.Any(), float32(1.0), true).
		Return(message.GenerationResult{Message: "response", Strategy: message.AiGenerationStrategy})

	// Verify history.Add is called with the correct prompt
	env.history.EXPECT().
		Add(chat.ID, "@testuser", expectedPrompt)

	env.statIncer.EXPECT().
		Inc(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		AnyTimes()

	err := env.handler.Handle(ctx)
	require.NoError(t, err)
}

func TestHandle_FileReaderError(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)

	chat := defaultChat()
	sender := defaultSender()
	photo := defaultPhoto()
	msg := defaultMessage(chat, sender, photo)

	ctx := newFakeCtx(chat, sender, msg)

	readerErr := errors.New("reader error")

	env.settings.EXPECT().
		GetForChat(gomock.Any(), chat.ID).
		Return(&chatsettings.Settings{PhotoReactChance: 1.0}, nil)

	env.downloader.EXPECT().
		FileByID(photo.FileID).
		Return(telebot.File{FileID: photo.FileID}, nil)

	env.fileReader.EXPECT().
		ReadFile(gomock.Any()).
		Return(nil, readerErr)

	env.logger.EXPECT().WithError(gomock.Any(), readerErr).Return(context.Background())
	env.logger.EXPECT().Warn(gomock.Any(), "can't get photo reader")

	err := env.handler.Handle(ctx)
	require.NoError(t, err)
}

func TestHandle_ReadAllError(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)

	chat := defaultChat()
	sender := defaultSender()
	photo := defaultPhoto()
	msg := defaultMessage(chat, sender, photo)

	ctx := newFakeCtx(chat, sender, msg)

	readErr := errors.New("read error")

	env.settings.EXPECT().
		GetForChat(gomock.Any(), chat.ID).
		Return(&chatsettings.Settings{PhotoReactChance: 1.0}, nil)

	env.downloader.EXPECT().
		FileByID(photo.FileID).
		Return(telebot.File{FileID: photo.FileID}, nil)

	env.fileReader.EXPECT().
		ReadFile(gomock.Any()).
		Return(io.NopCloser(&errReader{err: readErr}), nil)

	env.logger.EXPECT().WithError(gomock.Any(), readErr).Return(context.Background())
	env.logger.EXPECT().Warn(gomock.Any(), "can't read photo bytes")

	err := env.handler.Handle(ctx)
	require.NoError(t, err)
}

func TestHandle_NilMessage(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)

	ctx := newFakeCtx(defaultChat(), defaultSender(), nil)

	err := env.handler.Handle(ctx)
	require.NoError(t, err)
}

func TestSlug(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	require.Equal(t, "on_photo", env.handler.Slug())
}

func TestFormatAuthor_WithUsername(t *testing.T) {
	t.Parallel()

	user := &telebot.User{Username: "alice", FirstName: "Alice"}
	assert.Equal(t, "@alice", formatAuthor(user))
}

func TestFormatAuthor_WithoutUsername(t *testing.T) {
	t.Parallel()

	user := &telebot.User{FirstName: "Bob"}
	assert.Equal(t, "Bob", formatAuthor(user))
}

func TestBuildPrompt_WithCaption(t *testing.T) {
	t.Parallel()

	result := buildPrompt("my caption", "a photo of a cat")
	assert.Equal(
		t,
		"Пользователь отправил фото с подписью: 'my caption'. На фото: a photo of a cat",
		result,
	)
}

func TestBuildPrompt_WithoutCaption(t *testing.T) {
	t.Parallel()

	result := buildPrompt("", "a photo of a cat")
	assert.Equal(t, "Пользователь отправил фото. На фото: a photo of a cat", result)
}

func TestHandle_NotifyError(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)

	chat := defaultChat()
	sender := defaultSender()
	photo := defaultPhoto()
	msg := defaultMessage(chat, sender, photo)

	notifyErr := errors.New("notify failed")
	ctx := &fakeContext{
		chat:   chat,
		sender: sender,
		msg:    msg,
		bot: &telebot.Bot{
			Me: &telebot.User{ID: 42, Username: "testbot"},
		},
		notifyErr: notifyErr,
	}

	imageData := []byte("fake-image-data")
	description := "test"

	env.settings.EXPECT().
		GetForChat(gomock.Any(), chat.ID).
		Return(&chatsettings.Settings{PhotoReactChance: 1.0}, nil)

	env.downloader.EXPECT().
		FileByID(photo.FileID).
		Return(telebot.File{FileID: photo.FileID}, nil)

	env.fileReader.EXPECT().
		ReadFile(gomock.Any()).
		Return(io.NopCloser(bytes.NewReader(imageData)), nil)

	env.describer.EXPECT().
		GenerateContent(gomock.Any(), describePrompt, imageData).
		Return(description, nil)

	env.history.EXPECT().Get(chat.ID).Return([]message.HistoryMessage{})
	env.generator.EXPECT().
		GetMessageTextWithHistory(gomock.Any(), gomock.Any(), float32(1.0), true).
		Return(message.GenerationResult{Message: "r", Strategy: message.AiGenerationStrategy})
	env.history.EXPECT().Add(gomock.Any(), gomock.Any(), gomock.Any())
	env.statIncer.EXPECT().
		Inc(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		AnyTimes()

	err := env.handler.Handle(ctx)
	assert.ErrorIs(t, err, notifyErr)
}

func TestHandle_SettingsError(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)

	chat := defaultChat()
	sender := defaultSender()
	photo := defaultPhoto()
	msg := defaultMessage(chat, sender, photo)

	ctx := newFakeCtx(chat, sender, msg)

	settingsErr := errors.New("settings error")
	env.settings.EXPECT().
		GetForChat(gomock.Any(), chat.ID).
		Return(nil, settingsErr)

	err := env.handler.Handle(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "can't get chat settings")
}

// errReader is a reader that always returns an error
type errReader struct {
	err error
}

func (r *errReader) Read(p []byte) (int, error) {
	return 0, r.err
}
