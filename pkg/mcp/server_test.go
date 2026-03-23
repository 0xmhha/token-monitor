package mcp

import (
	"bufio"
	"encoding/json"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testLogger is a no-op logger for use in tests.
type testLogger struct{}

func (l *testLogger) Debug(msg string, keysAndValues ...interface{}) {}
func (l *testLogger) Info(msg string, keysAndValues ...interface{})  {}
func (l *testLogger) Warn(msg string, keysAndValues ...interface{})  {}
func (l *testLogger) Error(msg string, keysAndValues ...interface{}) {}

// setupServer creates a server with piped I/O and starts it in a goroutine.
// Returns the writer to send requests, a scanner over the response stream, and
// a channel that receives the Run() return value when the server exits.
func setupServer(t *testing.T, registry *ToolRegistry) (io.WriteCloser, *bufio.Scanner, chan error) {
	t.Helper()

	reqReader, reqWriter := io.Pipe()
	respReader, respWriter := io.Pipe()

	log := &testLogger{}
	server := NewServer(reqReader, respWriter, registry, "test", log)

	done := make(chan error, 1)
	go func() {
		done <- server.Run()
	}()

	// Ensure pipes are closed when the test ends so the server goroutine exits.
	t.Cleanup(func() {
		reqWriter.Close()
		respWriter.Close()
		respReader.Close()
		// Drain the done channel if the server has not exited yet.
		select {
		case <-done:
		case <-time.After(2 * time.Second):
		}
	})

	return reqWriter, bufio.NewScanner(respReader), done
}

// sendRequest writes a JSON-RPC 2.0 request to w.
func sendRequest(t *testing.T, w io.Writer, method string, id any, params any) {
	t.Helper()

	req := map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
		"id":      id,
	}
	if params != nil {
		req["params"] = params
	}

	data, err := json.Marshal(req)
	require.NoError(t, err)

	_, err = w.Write(append(data, '\n'))
	require.NoError(t, err)
}

// readResponse reads a single line from scanner and unmarshals it into a Response.
func readResponse(t *testing.T, scanner *bufio.Scanner) Response {
	t.Helper()

	require.True(t, scanner.Scan(), "expected a response from the server")
	var resp Response
	require.NoError(t, json.Unmarshal(scanner.Bytes(), &resp))
	return resp
}

// --- Server tests ---

func TestServer_Initialize(t *testing.T) {
	t.Parallel()

	registry := NewToolRegistry()
	reqWriter, scanner, _ := setupServer(t, registry)

	sendRequest(t, reqWriter, "initialize", 1, nil)
	resp := readResponse(t, scanner)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.EqualValues(t, 1, resp.ID)
	assert.Nil(t, resp.Error, "expected no error in initialize response")

	// Re-marshal result for structured access.
	resultBytes, err := json.Marshal(resp.Result)
	require.NoError(t, err)

	var result InitializeResult
	require.NoError(t, json.Unmarshal(resultBytes, &result))

	assert.Equal(t, "2024-11-05", result.ProtocolVersion, "protocolVersion must be set")
	assert.NotNil(t, result.Capabilities.Tools, "capabilities.tools must be present")
	assert.Equal(t, "token-monitor", result.ServerInfo.Name, "serverInfo.name must match")
	assert.Equal(t, "test", result.ServerInfo.Version, "serverInfo.version must match the value passed to NewServer")
}

func TestServer_ToolsList(t *testing.T) {
	t.Parallel()

	registry := NewToolRegistry()
	registry.Register(
		ToolDefinition{Name: "alpha", Description: "alpha tool", InputSchema: map[string]any{}},
		func(args json.RawMessage) (*ToolCallResult, error) { return nil, nil },
	)
	registry.Register(
		ToolDefinition{Name: "beta", Description: "beta tool", InputSchema: map[string]any{}},
		func(args json.RawMessage) (*ToolCallResult, error) { return nil, nil },
	)

	reqWriter, scanner, _ := setupServer(t, registry)

	sendRequest(t, reqWriter, "tools/list", 2, nil)
	resp := readResponse(t, scanner)

	assert.Nil(t, resp.Error, "expected no error in tools/list response")
	assert.EqualValues(t, 2, resp.ID)

	resultBytes, err := json.Marshal(resp.Result)
	require.NoError(t, err)

	var result ToolsListResult
	require.NoError(t, json.Unmarshal(resultBytes, &result))

	require.Len(t, result.Tools, 2, "expected exactly 2 tools")

	names := []string{result.Tools[0].Name, result.Tools[1].Name}
	assert.Contains(t, names, "alpha")
	assert.Contains(t, names, "beta")
}

func TestServer_ToolsCall(t *testing.T) {
	t.Parallel()

	registry := NewToolRegistry()
	registry.Register(
		ToolDefinition{Name: "echo_tool", Description: "returns fixed text", InputSchema: map[string]any{}},
		func(args json.RawMessage) (*ToolCallResult, error) {
			return &ToolCallResult{
				Content: []ToolContent{{Type: "text", Text: "hello from echo_tool"}},
			}, nil
		},
	)

	reqWriter, scanner, _ := setupServer(t, registry)

	sendRequest(t, reqWriter, "tools/call", 3, map[string]any{"name": "echo_tool"})
	resp := readResponse(t, scanner)

	assert.Nil(t, resp.Error, "expected no error when calling a registered tool")
	assert.EqualValues(t, 3, resp.ID)

	resultBytes, err := json.Marshal(resp.Result)
	require.NoError(t, err)

	var result ToolCallResult
	require.NoError(t, json.Unmarshal(resultBytes, &result))

	require.Len(t, result.Content, 1, "expected one content item in the result")
	assert.Equal(t, "text", result.Content[0].Type)
	assert.Equal(t, "hello from echo_tool", result.Content[0].Text)
}

func TestServer_Ping(t *testing.T) {
	t.Parallel()

	registry := NewToolRegistry()
	reqWriter, scanner, _ := setupServer(t, registry)

	sendRequest(t, reqWriter, "ping", 4, nil)
	resp := readResponse(t, scanner)

	assert.Nil(t, resp.Error, "expected no error for ping")
	assert.EqualValues(t, 4, resp.ID)

	// The result must be an empty object {}.
	resultBytes, err := json.Marshal(resp.Result)
	require.NoError(t, err)
	assert.Equal(t, "{}", string(resultBytes), "ping result must be an empty object")
}

func TestServer_MethodNotFound(t *testing.T) {
	t.Parallel()

	registry := NewToolRegistry()
	reqWriter, scanner, _ := setupServer(t, registry)

	sendRequest(t, reqWriter, "does/not/exist", 5, nil)
	resp := readResponse(t, scanner)

	require.NotNil(t, resp.Error, "expected error for unknown method")
	assert.Equal(t, ErrCodeMethodNotFound, resp.Error.Code,
		"error code must be -32601 for method not found")
}

func TestServer_MalformedJSON(t *testing.T) {
	t.Parallel()

	registry := NewToolRegistry()
	reqWriter, scanner, _ := setupServer(t, registry)

	// Write raw invalid JSON directly.
	_, err := io.WriteString(reqWriter, "this is not json\n")
	require.NoError(t, err)

	resp := readResponse(t, scanner)

	require.NotNil(t, resp.Error, "expected error for malformed JSON")
	assert.Equal(t, ErrCodeParseError, resp.Error.Code,
		"error code must be -32700 for parse error")
}

func TestServer_Notification(t *testing.T) {
	t.Parallel()

	registry := NewToolRegistry()
	reqWriter, scanner, _ := setupServer(t, registry)

	// Send a notification (no "id" field) — the server must not respond.
	notif := map[string]any{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	}
	data, err := json.Marshal(notif)
	require.NoError(t, err)
	_, err = reqWriter.Write(append(data, '\n'))
	require.NoError(t, err)

	// Send a follow-up ping so we know the server has processed the notification.
	sendRequest(t, reqWriter, "ping", 99, nil)
	resp := readResponse(t, scanner)

	// The only response we receive must be for the ping, not the notification.
	assert.EqualValues(t, 99, resp.ID,
		"the first (and only) response should be for ping, not the notification")
	assert.Nil(t, resp.Error)
}

func TestServer_ToolsCall_UnknownTool(t *testing.T) {
	t.Parallel()

	registry := NewToolRegistry()
	reqWriter, scanner, _ := setupServer(t, registry)

	sendRequest(t, reqWriter, "tools/call", 6, map[string]any{"name": "no_such_tool"})
	resp := readResponse(t, scanner)

	require.NotNil(t, resp.Error, "expected error for calling an unknown tool")
	assert.Equal(t, ErrCodeInternal, resp.Error.Code)
}

func TestServer_ToolsCall_InvalidParams(t *testing.T) {
	t.Parallel()

	registry := NewToolRegistry()
	reqWriter, scanner, _ := setupServer(t, registry)

	// Send tools/call with a non-object params value to trigger invalid params.
	req := map[string]any{
		"jsonrpc": "2.0",
		"method":  "tools/call",
		"id":      7,
		"params":  "this is a string not an object",
	}
	data, err := json.Marshal(req)
	require.NoError(t, err)
	_, err = reqWriter.Write(append(data, '\n'))
	require.NoError(t, err)

	resp := readResponse(t, scanner)

	require.NotNil(t, resp.Error, "expected error for invalid params")
	assert.Equal(t, ErrCodeInvalidParams, resp.Error.Code)
}

func TestServer_MultipleRequests_Ordered(t *testing.T) {
	t.Parallel()

	registry := NewToolRegistry()
	reqWriter, scanner, _ := setupServer(t, registry)

	const count = 5

	// Send all requests from a goroutine so we can concurrently read responses
	// without deadlocking the pipe.
	go func() {
		for i := 1; i <= count; i++ {
			sendRequest(t, reqWriter, "ping", i, nil)
		}
	}()

	for i := 1; i <= count; i++ {
		resp := readResponse(t, scanner)
		assert.EqualValues(t, i, resp.ID, "responses must arrive in request order")
		assert.Nil(t, resp.Error)
	}
}
