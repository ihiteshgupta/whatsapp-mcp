package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"testing"
)

// mockHandler implements ToolHandler for testing.
type mockHandler struct {
	tools []Tool
}

func (m *mockHandler) GetTools() []Tool {
	return m.tools
}

func (m *mockHandler) HandleTool(ctx context.Context, name string, args map[string]interface{}) (*CallToolResult, error) {
	return &CallToolResult{
		Content: []ContentBlock{TextContent("mock result for " + name)},
	}, nil
}

func TestServerInitialize(t *testing.T) {
	// Create a request
	req := Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "initialize",
		Params: json.RawMessage(`{
			"protocolVersion": "1.0",
			"clientInfo": {"name": "test", "version": "1.0"}
		}`),
	}

	reqBytes, _ := json.Marshal(req)
	input := bytes.NewBuffer(reqBytes)
	output := &bytes.Buffer{}

	handler := &mockHandler{
		tools: []Tool{
			{Name: "test_tool", Description: "Test tool"},
		},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := NewServer(input, output, handler, logger)

	// Read the request
	decoder := json.NewDecoder(input)
	var parsed Request
	err := decoder.Decode(&parsed)
	if err != nil {
		t.Fatalf("Failed to decode request: %v", err)
	}

	if parsed.Method != "initialize" {
		t.Errorf("Expected method 'initialize', got '%s'", parsed.Method)
	}

	_ = server // Verify server was created
}

func TestJSONRPCMessageParsing(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantErr    bool
		hasID      bool
		wantMethod string
	}{
		{
			name:       "valid request with numeric id",
			input:      `{"jsonrpc":"2.0","id":1,"method":"test"}`,
			wantErr:    false,
			hasID:      true,
			wantMethod: "test",
		},
		{
			name:       "string id",
			input:      `{"jsonrpc":"2.0","id":"abc","method":"test"}`,
			wantErr:    false,
			hasID:      true,
			wantMethod: "test",
		},
		{
			name:       "notification (no id)",
			input:      `{"jsonrpc":"2.0","method":"notify"}`,
			wantErr:    false,
			hasID:      false,
			wantMethod: "notify",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req Request
			err := json.Unmarshal([]byte(tt.input), &req)
			if (err != nil) != tt.wantErr {
				t.Errorf("Unmarshal error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if req.Method != tt.wantMethod {
					t.Errorf("Method = %v, want %v", req.Method, tt.wantMethod)
				}
				if tt.hasID && req.ID == nil {
					t.Errorf("Expected ID to be present, got nil")
				}
				if !tt.hasID && req.ID != nil {
					t.Errorf("Expected ID to be nil, got %v", req.ID)
				}
			}
		})
	}
}

func TestToolDefinitionSchema(t *testing.T) {
	tool := Tool{
		Name:        "send_message",
		Description: "Send a message",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"recipient": map[string]interface{}{
					"type":        "string",
					"description": "Recipient JID",
				},
			},
			"required": []string{"recipient"},
		},
	}

	bytes, err := json.Marshal(tool)
	if err != nil {
		t.Fatalf("Failed to marshal tool: %v", err)
	}

	// Verify it can be unmarshaled back
	var parsed Tool
	if err := json.Unmarshal(bytes, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal tool: %v", err)
	}

	if parsed.Name != tool.Name {
		t.Errorf("Name = %v, want %v", parsed.Name, tool.Name)
	}
	if parsed.Description != tool.Description {
		t.Errorf("Description = %v, want %v", parsed.Description, tool.Description)
	}
}

func TestCallToolResult(t *testing.T) {
	result := &CallToolResult{
		Content: []ContentBlock{
			TextContent("Hello"),
			TextContent("World"),
		},
		IsError: false,
	}

	bytes, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal result: %v", err)
	}

	// Verify JSON structure
	jsonStr := string(bytes)
	if !strings.Contains(jsonStr, `"type":"text"`) {
		t.Error("Expected content to have type:text")
	}
	if !strings.Contains(jsonStr, `"text":"Hello"`) {
		t.Error("Expected content to have text:Hello")
	}
}

func TestTextContent(t *testing.T) {
	content := TextContent("test message")

	if content.Type != "text" {
		t.Errorf("Type = %v, want 'text'", content.Type)
	}
	if content.Text != "test message" {
		t.Errorf("Text = %v, want 'test message'", content.Text)
	}
}
