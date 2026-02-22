package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
)

// ToolHandler is the interface for handling tool calls.
type ToolHandler interface {
	GetTools() []Tool
	HandleTool(ctx context.Context, name string, args map[string]interface{}) (*CallToolResult, error)
}

// Server is the MCP server that handles protocol messages.
type Server struct {
	transport   *Transport
	handler     ToolHandler
	log         *slog.Logger
	initialized bool

	serverInfo Implementation
}

// NewServer creates a new MCP server.
func NewServer(reader io.Reader, writer io.Writer, handler ToolHandler, log *slog.Logger) *Server {
	return &Server{
		transport: NewTransport(reader, writer, log),
		handler:   handler,
		log:       log,
		serverInfo: Implementation{
			Name:    "whatsapp-bridge-v2",
			Version: "2.0.0",
		},
	}
}

// Run starts the server message loop.
func (s *Server) Run(ctx context.Context) error {
	s.log.Info("MCP server starting")

	for {
		select {
		case <-ctx.Done():
			s.log.Info("MCP server shutting down")
			return ctx.Err()
		default:
		}

		req, err := s.transport.ReadMessage()
		if err != nil {
			if err == io.EOF {
				s.log.Info("Client disconnected")
				return nil
			}
			s.log.Error("Failed to read message", "error", err)
			continue
		}

		if err := s.handleRequest(ctx, req); err != nil {
			s.log.Error("Failed to handle request", "method", req.Method, "error", err)
		}
	}
}

func (s *Server) handleRequest(ctx context.Context, req *Request) error {
	s.log.Debug("handling request", "method", req.Method, "id", req.ID)

	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "initialized":
		// Notification, no response needed
		s.initialized = true
		s.log.Info("Client initialized")
		return nil
	case "ping":
		return s.transport.SendResult(req.ID, map[string]interface{}{})
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolsCall(ctx, req)
	case "resources/list":
		return s.handleResourcesList(req)
	case "resources/read":
		return s.handleResourcesRead(req)
	default:
		return s.transport.SendError(req.ID, MethodNotFound, fmt.Sprintf("Unknown method: %s", req.Method), nil)
	}
}

func (s *Server) handleInitialize(req *Request) error {
	var params InitializeParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return s.transport.SendError(req.ID, InvalidParams, "Invalid initialize params", nil)
		}
	}

	s.log.Info("Client initializing",
		"client", params.ClientInfo.Name,
		"version", params.ClientInfo.Version,
		"protocol", params.ProtocolVersion,
	)

	result := InitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities: ServerCapabilities{
			Tools: &ToolsCapability{
				ListChanged: false,
			},
			Resources: &ResourcesCapability{
				Subscribe:   false,
				ListChanged: false,
			},
		},
		ServerInfo: s.serverInfo,
	}

	return s.transport.SendResult(req.ID, result)
}

func (s *Server) handleToolsList(req *Request) error {
	tools := s.handler.GetTools()
	result := ListToolsResult{Tools: tools}
	return s.transport.SendResult(req.ID, result)
}

func (s *Server) handleToolsCall(ctx context.Context, req *Request) error {
	var params CallToolParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return s.transport.SendError(req.ID, InvalidParams, "Invalid tool call params", nil)
	}

	s.log.Info("Tool call", "name", params.Name)

	result, err := s.handler.HandleTool(ctx, params.Name, params.Arguments)
	if err != nil {
		s.log.Error("Tool call failed", "name", params.Name, "error", err)
		// Return error as tool result, not JSON-RPC error
		return s.transport.SendResult(req.ID, &CallToolResult{
			Content: []ContentBlock{TextContent(fmt.Sprintf("Error: %s", err.Error()))},
			IsError: true,
		})
	}

	return s.transport.SendResult(req.ID, result)
}

func (s *Server) handleResourcesList(req *Request) error {
	// Return empty list for now - can be expanded later
	result := ListResourcesResult{Resources: []Resource{}}
	return s.transport.SendResult(req.ID, result)
}

func (s *Server) handleResourcesRead(req *Request) error {
	var params ReadResourceParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return s.transport.SendError(req.ID, InvalidParams, "Invalid resource read params", nil)
	}

	// Return not found for now
	return s.transport.SendError(req.ID, -32002, fmt.Sprintf("Resource not found: %s", params.URI), nil)
}
