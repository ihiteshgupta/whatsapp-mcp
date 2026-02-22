package bridge

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/ihiteshgupta/whatsapp-mcp/whatsapp-bridge-v2/internal/config"
	"github.com/ihiteshgupta/whatsapp-mcp/whatsapp-bridge-v2/internal/state"
	"github.com/ihiteshgupta/whatsapp-mcp/whatsapp-bridge-v2/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// FakeClient implements WhatsAppClient for testing.
type FakeClient struct {
	mu           sync.Mutex
	connected    bool
	loggedIn     bool
	sentMessages []FakeMessage
	qrChan       chan string
	eventHandler func(interface{})
}

type FakeMessage struct {
	JID     string
	Content string
}

func NewFakeClient() *FakeClient {
	return &FakeClient{
		qrChan: make(chan string, 1),
	}
}

func (f *FakeClient) Connect(_ context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.connected = true
	return nil
}

func (f *FakeClient) Disconnect() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.connected = false
}

func (f *FakeClient) IsConnected() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.connected
}

func (f *FakeClient) IsLoggedIn() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.loggedIn
}

func (f *FakeClient) SetLoggedIn(v bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.loggedIn = v
}

func (f *FakeClient) SendMessage(ctx context.Context, jid string, text string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sentMessages = append(f.sentMessages, FakeMessage{JID: jid, Content: text})
	return "msg-" + jid, nil
}

func (f *FakeClient) SendMedia(ctx context.Context, jid string, data []byte, mimeType string, filename string) (string, error) {
	return "media-" + jid, nil
}

func (f *FakeClient) GetQRChannel() <-chan string {
	return f.qrChan
}

func (f *FakeClient) AddEventHandler(handler func(interface{})) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.eventHandler = handler
}

func (f *FakeClient) ReplyToMessage(ctx context.Context, chatJID, messageID, text string) (string, error) {
	return "", nil
}

func (f *FakeClient) ForwardMessage(ctx context.Context, sourceChatJID, messageID, targetJID string) (string, error) {
	return "", nil
}

func (f *FakeClient) EditMessage(ctx context.Context, chatJID, messageID, newContent string) error {
	return nil
}

func (f *FakeClient) DeleteMessage(ctx context.Context, chatJID, messageID string, forEveryone bool) error {
	return nil
}

func (f *FakeClient) ReactToMessage(ctx context.Context, chatJID, messageID, emoji string) error {
	return nil
}

func (f *FakeClient) SendImage(ctx context.Context, jid, imagePath, caption string) (string, error) {
	return "", nil
}

func (f *FakeClient) SendVideo(ctx context.Context, jid, videoPath, caption string) (string, error) {
	return "", nil
}

func (f *FakeClient) SendAudio(ctx context.Context, jid, audioPath string, asVoice bool) (string, error) {
	return "", nil
}

func (f *FakeClient) SendDocument(ctx context.Context, jid, filePath, filename string) (string, error) {
	return "", nil
}

func (f *FakeClient) SendLocation(ctx context.Context, jid string, lat, lon float64, name, address string) (string, error) {
	return "", nil
}

func (f *FakeClient) SendContactCard(ctx context.Context, jid, contactJID string) (string, error) {
	return "", nil
}

func (f *FakeClient) DownloadMedia(ctx context.Context, chatJID, messageID, savePath string) (string, error) {
	return "", nil
}

func (f *FakeClient) ArchiveChat(ctx context.Context, jid string, archive bool) error {
	return nil
}

func (f *FakeClient) PinChat(ctx context.Context, jid string, pin bool) error {
	return nil
}

func (f *FakeClient) MuteChat(ctx context.Context, jid string, mute bool, duration string) error {
	return nil
}

func (f *FakeClient) MarkChatRead(ctx context.Context, jid string) error {
	return nil
}

func (f *FakeClient) DeleteChat(ctx context.Context, jid string) error {
	return nil
}

func (f *FakeClient) BlockContact(ctx context.Context, jid string, block bool) error {
	return nil
}

func (f *FakeClient) CheckPhoneRegistered(ctx context.Context, phone string) (bool, error) {
	return false, nil
}

func (f *FakeClient) CreateGroup(ctx context.Context, name string, participants []string) (string, error) {
	return "", nil
}

func (f *FakeClient) GetGroupInfo(ctx context.Context, jid string) (interface{}, error) {
	return nil, nil
}

func (f *FakeClient) LeaveGroup(ctx context.Context, jid string) error {
	return nil
}

func (f *FakeClient) AddGroupMembers(ctx context.Context, groupJID string, participants []string) error {
	return nil
}

func (f *FakeClient) RemoveGroupMembers(ctx context.Context, groupJID string, participants []string) error {
	return nil
}

func (f *FakeClient) PromoteAdmin(ctx context.Context, groupJID string, participants []string) error {
	return nil
}

func (f *FakeClient) DemoteAdmin(ctx context.Context, groupJID string, participants []string) error {
	return nil
}

func (f *FakeClient) SetGroupName(ctx context.Context, groupJID, name string) error {
	return nil
}

func (f *FakeClient) SetGroupTopic(ctx context.Context, groupJID, topic string) error {
	return nil
}

func (f *FakeClient) SetGroupPhoto(ctx context.Context, groupJID, imagePath string) error {
	return nil
}

func (f *FakeClient) GetInviteLink(ctx context.Context, groupJID string) (string, error) {
	return "", nil
}

func (f *FakeClient) RevokeInviteLink(ctx context.Context, groupJID string) (string, error) {
	return "", nil
}

func (f *FakeClient) JoinViaInvite(ctx context.Context, inviteLink string) (string, error) {
	return "", nil
}

func (f *FakeClient) SubscribePresence(ctx context.Context, jid string) error {
	return nil
}

func (f *FakeClient) SendTyping(ctx context.Context, jid string) error {
	return nil
}

func (f *FakeClient) SendRecording(ctx context.Context, jid string) error {
	return nil
}

func (f *FakeClient) SetOnline(ctx context.Context) error {
	return nil
}

func (f *FakeClient) SetOffline(ctx context.Context) error {
	return nil
}

func (f *FakeClient) PostTextStatus(ctx context.Context, text, backgroundColor string) error {
	return nil
}

func (f *FakeClient) PostImageStatus(ctx context.Context, imagePath, caption string) error {
	return nil
}

func (f *FakeClient) DeleteStatus(ctx context.Context, statusID string) error {
	return nil
}

func (f *FakeClient) SimulateEvent(evt interface{}) {
	f.mu.Lock()
	handler := f.eventHandler
	f.mu.Unlock()
	if handler != nil {
		handler(evt)
	}
}

func (f *FakeClient) GetSentMessages() []FakeMessage {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := make([]FakeMessage, len(f.sentMessages))
	copy(result, f.sentMessages)
	return result
}

func setupTestBridge(t *testing.T) (*Bridge, *FakeClient, *store.SQLiteStore) {
	storeDB, err := store.NewSQLiteStore(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { storeDB.Close() })

	cfg := config.DefaultConfig()
	fakeClient := NewFakeClient()

	bridge := NewBridge(cfg, storeDB, fakeClient)
	t.Cleanup(func() { bridge.Stop() })

	return bridge, fakeClient, storeDB
}

func TestNewBridge(t *testing.T) {
	bridge, _, _ := setupTestBridge(t)

	assert.NotNil(t, bridge)
	assert.Equal(t, state.StateDisconnected, bridge.CurrentState())
}

func TestBridge_Connect(t *testing.T) {
	bridge, client, _ := setupTestBridge(t)

	client.SetLoggedIn(true) // Simulate existing session

	err := bridge.Connect(context.Background())
	require.NoError(t, err)

	// Should transition to connecting then authenticating
	// Give it a moment to process
	time.Sleep(50 * time.Millisecond)

	// Client should be connected
	assert.True(t, client.IsConnected())
}

func TestBridge_SendMessage(t *testing.T) {
	bridge, client, _ := setupTestBridge(t)
	ctx := context.Background()

	// Must be in Ready state to send
	client.SetLoggedIn(true)
	err := bridge.Connect(ctx)
	require.NoError(t, err)

	// Manually transition to ready for test
	bridge.stateMachine.Fire(ctx, state.TriggerAuthenticated)
	bridge.stateMachine.Fire(ctx, state.TriggerSyncComplete)

	assert.Equal(t, state.StateReady, bridge.CurrentState())

	// Send message
	msgID, err := bridge.SendMessage(ctx, "123@s.whatsapp.net", "Hello")
	require.NoError(t, err)
	assert.NotEmpty(t, msgID)

	// Verify message was sent
	sent := client.GetSentMessages()
	assert.Len(t, sent, 1)
	assert.Equal(t, "123@s.whatsapp.net", sent[0].JID)
	assert.Equal(t, "Hello", sent[0].Content)
}

func TestBridge_SendMessage_NotReady(t *testing.T) {
	bridge, _, _ := setupTestBridge(t)
	ctx := context.Background()

	// Try to send while disconnected
	_, err := bridge.SendMessage(ctx, "123@s.whatsapp.net", "Hello")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not ready")
}

func TestBridge_Disconnect(t *testing.T) {
	bridge, client, _ := setupTestBridge(t)
	ctx := context.Background()

	client.SetLoggedIn(true)
	err := bridge.Connect(ctx)
	require.NoError(t, err)

	bridge.Disconnect()

	assert.False(t, client.IsConnected())
	assert.Equal(t, state.StateDisconnected, bridge.CurrentState())
}

func TestBridge_EventProcessing(t *testing.T) {
	bridge, _, _ := setupTestBridge(t)

	events := make([]Event, 0)
	var mu sync.Mutex

	bridge.OnEvent(func(evt Event) {
		mu.Lock()
		events = append(events, evt)
		mu.Unlock()
	})

	// Emit an event
	bridge.EmitEvent(NewEvent(EventMessage, MessagePayload{
		ID:      "test",
		ChatJID: "123@s.whatsapp.net",
		Content: "Hello",
	}))

	// Wait for processing
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	assert.Len(t, events, 1)
	assert.Equal(t, EventMessage, events[0].Type)
	mu.Unlock()
}

func TestBridge_StateTransitionCallback(t *testing.T) {
	bridge, client, _ := setupTestBridge(t)
	ctx := context.Background()

	var transitions []struct {
		from, to state.State
	}
	var mu sync.Mutex

	bridge.OnStateChange(func(from, to state.State) {
		mu.Lock()
		transitions = append(transitions, struct{ from, to state.State }{from, to})
		mu.Unlock()
	})

	client.SetLoggedIn(true)
	_ = bridge.Connect(ctx)

	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	assert.GreaterOrEqual(t, len(transitions), 1)
	mu.Unlock()
}

func TestBridge_IsReady(t *testing.T) {
	bridge, client, _ := setupTestBridge(t)
	ctx := context.Background()

	assert.False(t, bridge.IsReady())

	client.SetLoggedIn(true)
	_ = bridge.Connect(ctx)
	bridge.stateMachine.Fire(ctx, state.TriggerAuthenticated)
	bridge.stateMachine.Fire(ctx, state.TriggerSyncComplete)

	assert.True(t, bridge.IsReady())
}
