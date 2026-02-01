package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"sync"
)

// Transport handles stdio communication for MCP.
type Transport struct {
	reader *bufio.Reader
	writer io.Writer
	log    *slog.Logger
	mu     sync.Mutex
}

// NewTransport creates a new stdio transport.
func NewTransport(reader io.Reader, writer io.Writer, log *slog.Logger) *Transport {
	return &Transport{
		reader: bufio.NewReader(reader),
		writer: writer,
		log:    log,
	}
}

// ReadMessage reads a JSON-RPC message from stdin.
func (t *Transport) ReadMessage() (*Request, error) {
	line, err := t.reader.ReadBytes('\n')
	if err != nil {
		if err == io.EOF {
			return nil, err
		}
		return nil, fmt.Errorf("failed to read message: %w", err)
	}

	t.log.Debug("received message", "raw", string(line))

	var req Request
	if err := json.Unmarshal(line, &req); err != nil {
		return nil, fmt.Errorf("failed to parse message: %w", err)
	}

	return &req, nil
}

// WriteMessage writes a JSON-RPC response to stdout.
func (t *Transport) WriteMessage(resp *Response) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	data, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}

	t.log.Debug("sending message", "raw", string(data))

	if _, err := t.writer.Write(data); err != nil {
		return fmt.Errorf("failed to write response: %w", err)
	}
	if _, err := t.writer.Write([]byte("\n")); err != nil {
		return fmt.Errorf("failed to write newline: %w", err)
	}

	return nil
}

// SendResult sends a successful response.
func (t *Transport) SendResult(id interface{}, result interface{}) error {
	return t.WriteMessage(&Response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	})
}

// SendError sends an error response.
func (t *Transport) SendError(id interface{}, code int, message string, data interface{}) error {
	return t.WriteMessage(&Response{
		JSONRPC: "2.0",
		ID:      id,
		Error: &Error{
			Code:    code,
			Message: message,
			Data:    data,
		},
	})
}

// SendNotification sends a notification (no id, no response expected).
func (t *Transport) SendNotification(method string, params interface{}) error {
	data, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("failed to marshal params: %w", err)
	}

	req := Request{
		JSONRPC: "2.0",
		Method:  method,
		Params:  data,
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	reqData, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}

	if _, err := t.writer.Write(reqData); err != nil {
		return fmt.Errorf("failed to write notification: %w", err)
	}
	if _, err := t.writer.Write([]byte("\n")); err != nil {
		return fmt.Errorf("failed to write newline: %w", err)
	}

	return nil
}
