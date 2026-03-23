package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
)

// Logger defines the logging interface used by the MCP server.
type Logger interface {
	Debug(msg string, keysAndValues ...interface{})
	Info(msg string, keysAndValues ...interface{})
	Warn(msg string, keysAndValues ...interface{})
	Error(msg string, keysAndValues ...interface{})
}

// Server is a JSON-RPC 2.0 MCP server that reads from a reader and writes to a writer.
type Server struct {
	tools   *ToolRegistry
	version string
	reader  io.Reader
	writer  io.Writer
	logger  Logger
}

// NewServer creates a new MCP server.
func NewServer(reader io.Reader, writer io.Writer, registry *ToolRegistry, version string, log Logger) *Server {
	return &Server{
		tools:   registry,
		version: version,
		reader:  reader,
		writer:  writer,
		logger:  log,
	}
}

// Run starts the server loop, reading JSON-RPC requests until the reader is closed.
func (s *Server) Run() error {
	scanner := bufio.NewScanner(s.reader)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		req, parseErr := s.parseRequest(line)
		if parseErr != nil {
			s.writeError(nil, ErrCodeParseError, "parse error", nil)
			continue
		}

		s.logger.Debug("received request", "method", req.Method, "id", req.ID)

		// Notifications have no id and must not receive a response.
		if req.ID == nil && req.Method == "notifications/initialized" {
			continue
		}

		resp := s.dispatch(req)
		if resp == nil {
			continue
		}

		if err := s.writeResponse(resp); err != nil {
			s.logger.Error("failed to write response", "error", err)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanner error: %w", err)
	}

	return nil
}

// parseRequest decodes a JSON-RPC request from raw bytes.
func (s *Server) parseRequest(data []byte) (*Request, error) {
	var req Request
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, err
	}
	return &req, nil
}

// dispatch routes a request to the appropriate handler and returns a response.
func (s *Server) dispatch(req *Request) *Response {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "notifications/initialized":
		// Notification — no response.
		return nil
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolsCall(req)
	case "ping":
		return s.successResponse(req.ID, struct{}{})
	default:
		return s.errorResponse(req.ID, ErrCodeMethodNotFound, fmt.Sprintf("method not found: %s", req.Method), nil)
	}
}

// handleInitialize handles the initialize method.
func (s *Server) handleInitialize(req *Request) *Response {
	result := InitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities: Capabilities{
			Tools: &ToolsCapability{ListChanged: false},
		},
		ServerInfo: ServerInfo{
			Name:    "token-monitor",
			Version: s.version,
		},
	}
	return s.successResponse(req.ID, result)
}

// handleToolsList handles the tools/list method.
func (s *Server) handleToolsList(req *Request) *Response {
	result := ToolsListResult{
		Tools: s.tools.List(),
	}
	return s.successResponse(req.ID, result)
}

// handleToolsCall handles the tools/call method.
func (s *Server) handleToolsCall(req *Request) *Response {
	var params ToolCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return s.errorResponse(req.ID, ErrCodeInvalidParams, "invalid params", err.Error())
	}

	result, err := s.tools.Call(params.Name, params.Arguments)
	if err != nil {
		return s.errorResponse(req.ID, ErrCodeInternal, err.Error(), nil)
	}

	return s.successResponse(req.ID, result)
}

// successResponse builds a success JSON-RPC response.
func (s *Server) successResponse(id any, result any) *Response {
	return &Response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
}

// errorResponse builds an error JSON-RPC response.
func (s *Server) errorResponse(id any, code int, message string, data any) *Response {
	return &Response{
		JSONRPC: "2.0",
		ID:      id,
		Error: &Error{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
}

// writeError encodes and sends an error response directly.
func (s *Server) writeError(id any, code int, message string, data any) {
	resp := s.errorResponse(id, code, message, data)
	if err := s.writeResponse(resp); err != nil {
		s.logger.Error("failed to write error response", "error", err)
	}
}

// writeResponse encodes a response as a single JSON line to the writer.
func (s *Server) writeResponse(resp *Response) error {
	data, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}

	data = append(data, '\n')
	_, err = s.writer.Write(data)
	return err
}
