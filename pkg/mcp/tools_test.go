package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/0xmhha/token-monitor/pkg/discovery"
	"github.com/0xmhha/token-monitor/pkg/logger"
	"github.com/0xmhha/token-monitor/pkg/parser"
	"github.com/0xmhha/token-monitor/pkg/reader"
)

// --- Mock discoverer ---

type mockDiscoverer struct {
	sessions []discovery.SessionFile
}

func (m *mockDiscoverer) Discover() ([]discovery.SessionFile, error) {
	return m.sessions, nil
}

func (m *mockDiscoverer) DiscoverProject(path string) ([]discovery.SessionFile, error) {
	return m.sessions, nil
}

func (m *mockDiscoverer) FindCurrentSession() (*discovery.SessionFile, error) {
	if len(m.sessions) == 0 {
		return nil, discovery.ErrNoCurrentSession
	}
	return &m.sessions[0], nil
}

// --- ToolRegistry unit tests ---

func TestToolRegistry_Register(t *testing.T) {
	t.Parallel()

	registry := NewToolRegistry()
	def := ToolDefinition{
		Name:        "my_tool",
		Description: "a test tool",
		InputSchema: map[string]any{"type": "object"},
	}
	registry.Register(def, func(args json.RawMessage) (*ToolCallResult, error) {
		return nil, nil
	})

	tools := registry.List()
	require.Len(t, tools, 1, "expected exactly one registered tool")
	assert.Equal(t, "my_tool", tools[0].Name)
	assert.Equal(t, "a test tool", tools[0].Description)
}

func TestToolRegistry_Register_MultipleTools(t *testing.T) {
	t.Parallel()

	registry := NewToolRegistry()
	for i := 0; i < 3; i++ {
		registry.Register(
			ToolDefinition{Name: fmt.Sprintf("tool_%d", i), Description: "desc"},
			func(args json.RawMessage) (*ToolCallResult, error) { return nil, nil },
		)
	}

	assert.Len(t, registry.List(), 3, "all three tools must appear in List()")
}

func TestToolRegistry_List_ReturnsCopy(t *testing.T) {
	t.Parallel()

	registry := NewToolRegistry()
	registry.Register(
		ToolDefinition{Name: "immutable_check"},
		func(args json.RawMessage) (*ToolCallResult, error) { return nil, nil },
	)

	list1 := registry.List()
	list2 := registry.List()

	// Mutating one slice must not affect the other or the registry internals.
	list1[0].Name = "mutated"
	assert.Equal(t, "immutable_check", list2[0].Name,
		"List() must return an independent copy each time")
}

func TestToolRegistry_Call(t *testing.T) {
	t.Parallel()

	registry := NewToolRegistry()
	registry.Register(
		ToolDefinition{Name: "adder"},
		func(args json.RawMessage) (*ToolCallResult, error) {
			return &ToolCallResult{
				Content: []ToolContent{{Type: "text", Text: "42"}},
			}, nil
		},
	)

	result, err := registry.Call("adder", nil)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Content, 1)
	assert.Equal(t, "42", result.Content[0].Text)
}

func TestToolRegistry_CallNotFound(t *testing.T) {
	t.Parallel()

	registry := NewToolRegistry()

	_, err := registry.Call("does_not_exist", nil)
	require.Error(t, err, "calling an unregistered tool must return an error")
	assert.Contains(t, err.Error(), "does_not_exist")
}

func TestToolRegistry_Call_HandlerError(t *testing.T) {
	t.Parallel()

	registry := NewToolRegistry()
	registry.Register(
		ToolDefinition{Name: "failing_tool"},
		func(args json.RawMessage) (*ToolCallResult, error) {
			return nil, fmt.Errorf("handler exploded")
		},
	)

	_, err := registry.Call("failing_tool", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "handler exploded")
}

// --- Tool handler integration tests ---

// makeSessionFile creates a temporary JSONL file with one valid usage entry
// and returns a SessionFile pointing to it.
func makeSessionFile(t *testing.T, sessionID string, inputTokens, outputTokens int) discovery.SessionFile {
	t.Helper()

	dir := t.TempDir()
	filePath := filepath.Join(dir, sessionID+".jsonl")

	entry := map[string]any{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"sessionId": sessionID,
		"version":   "1",
		"cwd":       dir,
		"message": map[string]any{
			"id":    "msg_test",
			"model": "claude-3-5-sonnet-20241022",
			"usage": map[string]any{
				"input_tokens":               inputTokens,
				"output_tokens":              outputTokens,
				"cache_creation_input_tokens": 0,
				"cache_read_input_tokens":     0,
			},
			"content": []any{},
		},
	}

	data, err := json.Marshal(entry)
	require.NoError(t, err)

	err = os.WriteFile(filePath, append(data, '\n'), 0600)
	require.NoError(t, err)

	info, err := os.Stat(filePath)
	require.NoError(t, err)

	return discovery.SessionFile{
		SessionID:   sessionID,
		FilePath:    filePath,
		ProjectPath: dir,
		Size:        info.Size(),
		ModTime:     info.ModTime().Unix(),
	}
}

// newTestReaderFactory returns a readerFactory backed by an in-memory
// position store and the real parser so tests don't depend on a database.
func newTestReaderFactory() func() (reader.Reader, error) {
	return func() (reader.Reader, error) {
		return reader.New(reader.Config{
			PositionStore: reader.NewMemoryPositionStore(),
			Parser:        parser.New(),
		}, logger.Noop())
	}
}

func TestGetTokenUsage_CurrentSession(t *testing.T) {
	t.Parallel()

	const sessionID = "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
	sf := makeSessionFile(t, sessionID, 100, 50)

	disc := &mockDiscoverer{sessions: []discovery.SessionFile{sf}}
	registry := NewToolRegistry()
	RegisterTokenTools(registry, disc, newTestReaderFactory(), &testLogger{})

	// Call with no arguments — should auto-detect the current session.
	result, err := registry.Call("get_token_usage", json.RawMessage(`{}`))
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Content, 1)

	var data map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &data))

	assert.Equal(t, sessionID, data["session_id"])
	// input(100) + output(50) = 150 total tokens.
	assert.EqualValues(t, 150, data["total_tokens"])
	assert.EqualValues(t, 100, data["input_tokens"])
	assert.EqualValues(t, 50, data["output_tokens"])
	assert.EqualValues(t, 1, data["count"])
}

func TestGetTokenUsage_ExplicitSessionID(t *testing.T) {
	t.Parallel()

	const sessionID = "b2c3d4e5-f6a7-8901-bcde-f12345678901"
	sf := makeSessionFile(t, sessionID, 200, 75)

	disc := &mockDiscoverer{sessions: []discovery.SessionFile{sf}}
	registry := NewToolRegistry()
	RegisterTokenTools(registry, disc, newTestReaderFactory(), &testLogger{})

	args := json.RawMessage(fmt.Sprintf(`{"session_id":%q}`, sessionID))
	result, err := registry.Call("get_token_usage", args)
	require.NoError(t, err)
	require.NotNil(t, result)

	var data map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &data))

	assert.Equal(t, sessionID, data["session_id"])
	assert.EqualValues(t, 275, data["total_tokens"])
}

func TestGetTokenUsage_NoCurrentSession(t *testing.T) {
	t.Parallel()

	disc := &mockDiscoverer{sessions: nil} // no sessions
	registry := NewToolRegistry()
	RegisterTokenTools(registry, disc, newTestReaderFactory(), &testLogger{})

	_, err := registry.Call("get_token_usage", json.RawMessage(`{}`))
	require.Error(t, err, "expected an error when no session is available")
}

func TestGetTokenUsage_SessionNotFound(t *testing.T) {
	t.Parallel()

	const existingID = "c3d4e5f6-a7b8-9012-cdef-123456789012"
	sf := makeSessionFile(t, existingID, 10, 10)
	disc := &mockDiscoverer{sessions: []discovery.SessionFile{sf}}

	registry := NewToolRegistry()
	RegisterTokenTools(registry, disc, newTestReaderFactory(), &testLogger{})

	args := json.RawMessage(`{"session_id":"00000000-0000-0000-0000-000000000000"}`)
	_, err := registry.Call("get_token_usage", args)
	require.Error(t, err, "expected an error for a session ID that does not exist")
}

func TestGetBurnRate_ReturnsResult(t *testing.T) {
	t.Parallel()

	const sessionID = "d4e5f6a7-b8c9-0123-defa-234567890123"
	sf := makeSessionFile(t, sessionID, 300, 100)

	disc := &mockDiscoverer{sessions: []discovery.SessionFile{sf}}
	registry := NewToolRegistry()
	RegisterTokenTools(registry, disc, newTestReaderFactory(), &testLogger{})

	args := json.RawMessage(fmt.Sprintf(`{"session_id":%q,"window":"10m"}`, sessionID))
	result, err := registry.Call("get_burn_rate", args)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Content, 1)

	var data map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &data))

	assert.Equal(t, sessionID, data["session_id"])
	// All required keys must be present.
	assert.Contains(t, data, "tokens_per_min")
	assert.Contains(t, data, "tokens_per_hour")
	assert.Contains(t, data, "input_per_min")
	assert.Contains(t, data, "output_per_min")
	assert.Contains(t, data, "entry_count")
}

func TestGetBurnRate_InvalidWindow(t *testing.T) {
	t.Parallel()

	const sessionID = "e5f6a7b8-c9d0-1234-efab-345678901234"
	sf := makeSessionFile(t, sessionID, 10, 10)

	disc := &mockDiscoverer{sessions: []discovery.SessionFile{sf}}
	registry := NewToolRegistry()
	RegisterTokenTools(registry, disc, newTestReaderFactory(), &testLogger{})

	args := json.RawMessage(fmt.Sprintf(`{"session_id":%q,"window":"not-a-duration"}`, sessionID))
	_, err := registry.Call("get_burn_rate", args)
	require.Error(t, err, "expected error for invalid window duration")
}

func TestGetBillingBlock_ReturnsResult(t *testing.T) {
	t.Parallel()

	const sessionID = "f6a7b8c9-d0e1-2345-fabc-456789012345"
	sf := makeSessionFile(t, sessionID, 500, 200)

	disc := &mockDiscoverer{sessions: []discovery.SessionFile{sf}}
	registry := NewToolRegistry()
	RegisterTokenTools(registry, disc, newTestReaderFactory(), &testLogger{})

	result, err := registry.Call("get_billing_block", json.RawMessage(`{}`))
	require.NoError(t, err)
	require.NotNil(t, result)

	var data map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &data))

	assert.Contains(t, data, "start_time")
	assert.Contains(t, data, "end_time")
	assert.Contains(t, data, "time_remaining")
	assert.Contains(t, data, "is_active")
}

func TestListSessions_ReturnsSortedByModTime(t *testing.T) {
	t.Parallel()

	older := discovery.SessionFile{
		SessionID: "aaaaaaaa-bbbb-cccc-dddd-000000000001",
		FilePath:  "/tmp/old.jsonl",
		ModTime:   time.Now().Add(-2 * time.Hour).Unix(),
	}
	newer := discovery.SessionFile{
		SessionID: "aaaaaaaa-bbbb-cccc-dddd-000000000002",
		FilePath:  "/tmp/new.jsonl",
		ModTime:   time.Now().Unix(),
	}

	disc := &mockDiscoverer{sessions: []discovery.SessionFile{older, newer}}
	registry := NewToolRegistry()
	RegisterTokenTools(registry, disc, newTestReaderFactory(), &testLogger{})

	result, err := registry.Call("list_sessions", json.RawMessage(`{}`))
	require.NoError(t, err)
	require.NotNil(t, result)

	var items []map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &items))

	require.Len(t, items, 2)
	// Most recent session should appear first.
	assert.Equal(t, newer.SessionID, items[0]["session_id"])
	assert.Equal(t, older.SessionID, items[1]["session_id"])
}

func TestListSessions_LimitRespected(t *testing.T) {
	t.Parallel()

	sessions := make([]discovery.SessionFile, 5)
	for i := range sessions {
		sessions[i] = discovery.SessionFile{
			SessionID: fmt.Sprintf("aaaaaaaa-0000-0000-0000-00000000000%d", i),
			FilePath:  fmt.Sprintf("/tmp/session%d.jsonl", i),
			ModTime:   time.Now().Add(time.Duration(i) * time.Minute).Unix(),
		}
	}

	disc := &mockDiscoverer{sessions: sessions}
	registry := NewToolRegistry()
	RegisterTokenTools(registry, disc, newTestReaderFactory(), &testLogger{})

	result, err := registry.Call("list_sessions", json.RawMessage(`{"limit":3}`))
	require.NoError(t, err)

	var items []map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &items))

	assert.Len(t, items, 3, "limit:3 must cap the result to 3 sessions")
}

func TestGetSessionDetail_RequiresSessionID(t *testing.T) {
	t.Parallel()

	disc := &mockDiscoverer{}
	registry := NewToolRegistry()
	RegisterTokenTools(registry, disc, newTestReaderFactory(), &testLogger{})

	_, err := registry.Call("get_session_detail", json.RawMessage(`{}`))
	require.Error(t, err, "get_session_detail without session_id must fail")
}

func TestGetSessionDetail_ReturnsAllSections(t *testing.T) {
	t.Parallel()

	const sessionID = "a1a1a1a1-b2b2-c3c3-d4d4-e5e5e5e5e5e5"
	sf := makeSessionFile(t, sessionID, 150, 60)
	disc := &mockDiscoverer{sessions: []discovery.SessionFile{sf}}

	registry := NewToolRegistry()
	RegisterTokenTools(registry, disc, newTestReaderFactory(), &testLogger{})

	args := json.RawMessage(fmt.Sprintf(`{"session_id":%q}`, sessionID))
	result, err := registry.Call("get_session_detail", args)
	require.NoError(t, err)

	var data map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &data))

	assert.Equal(t, sessionID, data["session_id"])
	assert.Contains(t, data, "stats", "response must include stats section")
	assert.Contains(t, data, "burn_rate", "response must include burn_rate section")
	assert.Contains(t, data, "billing_block", "response must include billing_block section")
}

func TestCompareSessions_ReturnsDiff(t *testing.T) {
	t.Parallel()

	const idA = "11111111-2222-3333-4444-555555555555"
	const idB = "66666666-7777-8888-9999-aaaaaaaaaaaa"

	sfA := makeSessionFile(t, idA, 100, 50)
	sfB := makeSessionFile(t, idB, 200, 80)

	disc := &mockDiscoverer{sessions: []discovery.SessionFile{sfA, sfB}}
	registry := NewToolRegistry()
	RegisterTokenTools(registry, disc, newTestReaderFactory(), &testLogger{})

	args := json.RawMessage(fmt.Sprintf(`{"session_a":%q,"session_b":%q}`, idA, idB))
	result, err := registry.Call("compare_sessions", args)
	require.NoError(t, err)

	var data map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &data))

	assert.Contains(t, data, "session_a")
	assert.Contains(t, data, "session_b")
	assert.Contains(t, data, "diff")

	diff, ok := data["diff"].(map[string]any)
	require.True(t, ok, "diff must be an object")
	// B.total(280) - A.total(150) = 130
	assert.EqualValues(t, 130, diff["total_tokens"])
}

func TestCompareSessions_RequiresBothSessionIDs(t *testing.T) {
	t.Parallel()

	disc := &mockDiscoverer{}
	registry := NewToolRegistry()
	RegisterTokenTools(registry, disc, newTestReaderFactory(), &testLogger{})

	_, err := registry.Call("compare_sessions", json.RawMessage(`{"session_a":"id-a"}`))
	require.Error(t, err, "compare_sessions must fail when session_b is missing")
}

func TestTextResult_EncodesAsJSON(t *testing.T) {
	t.Parallel()

	input := map[string]any{"key": "value", "count": 42}
	result, err := textResult(input)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Content, 1)
	assert.Equal(t, "text", result.Content[0].Type)

	// The text must be valid JSON.
	var decoded map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &decoded))
	assert.Equal(t, "value", decoded["key"])
	assert.EqualValues(t, 42, decoded["count"])
}

func TestRegisterTokenTools_RegistersAllTools(t *testing.T) {
	t.Parallel()

	disc := &mockDiscoverer{}
	registry := NewToolRegistry()
	RegisterTokenTools(registry, disc, newTestReaderFactory(), &testLogger{})

	tools := registry.List()
	assert.Len(t, tools, 9, "RegisterTokenTools must register exactly 9 tools (6 v0.1 + 3 v0.2)")

	names := make([]string, len(tools))
	for i, tool := range tools {
		names[i] = tool.Name
	}
	// v0.1 single-session tools.
	assert.Contains(t, names, "get_token_usage")
	assert.Contains(t, names, "get_burn_rate")
	assert.Contains(t, names, "get_billing_block")
	assert.Contains(t, names, "list_sessions")
	assert.Contains(t, names, "get_session_detail")
	assert.Contains(t, names, "compare_sessions")
	// v0.2 cross-session breakdown tools.
	assert.Contains(t, names, "get_session_breakdown")
	assert.Contains(t, names, "get_today_usage")
	assert.Contains(t, names, "get_usage_by_window")
}
