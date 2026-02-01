package bridge

import (
	"context"
)

// WhatsAppClient defines the interface for WhatsApp operations.
// This allows for easy mocking in tests.
type WhatsAppClient interface {
	Connect() error
	Disconnect()
	IsConnected() bool
	IsLoggedIn() bool

	SendMessage(ctx context.Context, jid string, text string) (string, error)
	SendMedia(ctx context.Context, jid string, data []byte, mimeType string, filename string) (string, error)

	GetQRChannel() (<-chan string, error)

	// Event handling
	AddEventHandler(handler func(interface{}))
}

// SendMessageResult contains the result of sending a message.
type SendMessageResult struct {
	ID        string
	Timestamp int64
}
