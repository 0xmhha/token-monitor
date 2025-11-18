package discovery

import (
	"os"
	"path/filepath"
	"testing"
)

// mockLogger implements Logger interface for testing.
type mockLogger struct {
	debugCalls []string
	infoCalls  []string
	warnCalls  []string
	errorCalls []string
}

func (m *mockLogger) Debug(msg string, keysAndValues ...interface{}) {
	m.debugCalls = append(m.debugCalls, msg)
}

func (m *mockLogger) Info(msg string, keysAndValues ...interface{}) {
	m.infoCalls = append(m.infoCalls, msg)
}

func (m *mockLogger) Warn(msg string, keysAndValues ...interface{}) {
	m.warnCalls = append(m.warnCalls, msg)
}

func (m *mockLogger) Error(msg string, keysAndValues ...interface{}) {
	m.errorCalls = append(m.errorCalls, msg)
}

func TestNew(t *testing.T) {
	logger := &mockLogger{}
	dirs := []string{"/path1", "/path2"}

	d := New(dirs, logger)
	if d == nil {
		t.Error("New() returned nil")
	}
}

func TestDiscover(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test structure:
	// tmpDir/
	//   project1/
	//     session1.jsonl
	//     session2.jsonl
	//   project2/
	//     session3.jsonl
	//   not-a-project.txt (should be ignored)

	project1 := filepath.Join(tmpDir, "project1")
	project2 := filepath.Join(tmpDir, "project2")

	if err := os.MkdirAll(project1, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(project2, 0700); err != nil {
		t.Fatal(err)
	}

	// Create valid session files (UUID format)
	session1 := "a1b2c3d4-e5f6-7890-abcd-ef1234567890.jsonl"
	session2 := "b2c3d4e5-f6a7-8901-bcde-f12345678901.jsonl"
	session3 := "c3d4e5f6-a7b8-9012-cdef-123456789012.jsonl"

	createFile(t, filepath.Join(project1, session1), "test content")
	createFile(t, filepath.Join(project1, session2), "test content")
	createFile(t, filepath.Join(project2, session3), "test content")

	// Create a non-session file (should be ignored)
	createFile(t, filepath.Join(tmpDir, "not-a-project.txt"), "ignored")

	logger := &mockLogger{}
	d := New([]string{tmpDir}, logger)

	sessions, err := d.Discover()
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	if len(sessions) != 3 {
		t.Errorf("Discover() found %d sessions, want 3", len(sessions))
	}

	// Verify session details
	sessionIDs := make(map[string]bool)
	for _, s := range sessions {
		sessionIDs[s.SessionID] = true

		if s.FilePath == "" {
			t.Error("SessionFile has empty FilePath")
		}
		if s.ProjectPath == "" {
			t.Error("SessionFile has empty ProjectPath")
		}
		if s.Size == 0 {
			t.Error("SessionFile has zero Size")
		}
		if s.ModTime == 0 {
			t.Error("SessionFile has zero ModTime")
		}
	}

	// Check that all sessions were found
	expectedIDs := []string{
		"a1b2c3d4-e5f6-7890-abcd-ef1234567890",
		"b2c3d4e5-f6a7-8901-bcde-f12345678901",
		"c3d4e5f6-a7b8-9012-cdef-123456789012",
	}

	for _, id := range expectedIDs {
		if !sessionIDs[id] {
			t.Errorf("Session ID %s not found", id)
		}
	}
}

func TestDiscoverProject(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test project with sessions
	session1 := "a1b2c3d4-e5f6-7890-abcd-ef1234567890.jsonl"
	session2 := "b2c3d4e5-f6a7-8901-bcde-f12345678901.jsonl"

	createFile(t, filepath.Join(tmpDir, session1), "content")
	createFile(t, filepath.Join(tmpDir, session2), "content")

	logger := &mockLogger{}
	d := New([]string{}, logger)

	sessions, err := d.DiscoverProject(tmpDir)
	if err != nil {
		t.Fatalf("DiscoverProject() error = %v", err)
	}

	if len(sessions) != 2 {
		t.Errorf("DiscoverProject() found %d sessions, want 2", len(sessions))
	}
}

func TestDiscoverProjectNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	nonExistent := filepath.Join(tmpDir, "nonexistent")

	logger := &mockLogger{}
	d := New([]string{}, logger)

	_, err := d.DiscoverProject(nonExistent)
	if err == nil {
		t.Error("DiscoverProject() error = nil, want error for non-existent directory")
	}
}

func TestDiscoverNonJSONLFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create files that should be ignored
	createFile(t, filepath.Join(tmpDir, "readme.txt"), "content")
	createFile(t, filepath.Join(tmpDir, "config.yaml"), "content")
	createFile(t, filepath.Join(tmpDir, "data.json"), "content") // .json, not .jsonl

	logger := &mockLogger{}
	d := New([]string{tmpDir}, logger)

	sessions, err := d.Discover()
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	if len(sessions) != 0 {
		t.Errorf("Discover() found %d sessions, want 0 (all files should be ignored)", len(sessions))
	}
}

func TestDiscoverInvalidSessionIDs(t *testing.T) {
	tmpDir := t.TempDir()
	project := filepath.Join(tmpDir, "project")

	if err := os.MkdirAll(project, 0700); err != nil {
		t.Fatal(err)
	}

	// Create files with invalid session IDs
	invalidFiles := []string{
		"not-a-uuid.jsonl",
		"too-short.jsonl",
		"12345678-1234-1234-1234-12345678901.jsonl",  // wrong length
		"xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx.jsonl", // non-hex chars
	}

	for _, file := range invalidFiles {
		createFile(t, filepath.Join(project, file), "content")
	}

	logger := &mockLogger{}
	d := New([]string{tmpDir}, logger)

	sessions, err := d.Discover()
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	if len(sessions) != 0 {
		t.Errorf("Discover() found %d sessions, want 0 (all IDs invalid)", len(sessions))
	}
}

func TestIsValidSessionID(t *testing.T) {
	tests := []struct {
		name string
		id   string
		want bool
	}{
		{
			name: "valid UUID v4",
			id:   "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
			want: true,
		},
		{
			name: "valid UUID with uppercase",
			id:   "A1B2C3D4-E5F6-7890-ABCD-EF1234567890",
			want: true,
		},
		{
			name: "valid UUID mixed case",
			id:   "a1B2c3D4-e5F6-7890-aBcD-eF1234567890",
			want: true,
		},
		{
			name: "too short",
			id:   "a1b2c3d4-e5f6-7890-abcd-ef123456789",
			want: false,
		},
		{
			name: "too long",
			id:   "a1b2c3d4-e5f6-7890-abcd-ef12345678901",
			want: false,
		},
		{
			name: "missing dashes",
			id:   "a1b2c3d4e5f678 90abcdef1234567890",
			want: false,
		},
		{
			name: "dashes in wrong positions",
			id:   "a1b2c3d-4e5f6-7890-abcd-ef1234567890",
			want: false,
		},
		{
			name: "non-hex characters",
			id:   "g1b2c3d4-e5f6-7890-abcd-ef1234567890",
			want: false,
		},
		{
			name: "empty string",
			id:   "",
			want: false,
		},
		{
			name: "special characters",
			id:   "a1b2c3d4-e5f6-7890-abcd-ef123456789!",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidSessionID(tt.id)
			if got != tt.want {
				t.Errorf("isValidSessionID(%q) = %v, want %v", tt.id, got, tt.want)
			}
		})
	}
}

func TestIsHexDigit(t *testing.T) {
	tests := []struct {
		name string
		char rune
		want bool
	}{
		{"digit 0", '0', true},
		{"digit 5", '5', true},
		{"digit 9", '9', true},
		{"lowercase a", 'a', true},
		{"lowercase f", 'f', true},
		{"uppercase A", 'A', true},
		{"uppercase F", 'F', true},
		{"lowercase g", 'g', false},
		{"uppercase G", 'G', false},
		{"special char", '-', false},
		{"space", ' ', false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isHexDigit(tt.char)
			if got != tt.want {
				t.Errorf("isHexDigit(%c) = %v, want %v", tt.char, got, tt.want)
			}
		})
	}
}

func TestExpandHome(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string // empty means check it's not the same as input
	}{
		{
			name: "tilde only",
			path: "~",
			want: "", // Should expand to home dir
		},
		{
			name: "tilde with path",
			path: "~/.config/claude",
			want: "", // Should expand to home dir + path
		},
		{
			name: "absolute path",
			path: "/absolute/path",
			want: "/absolute/path", // Should not change
		},
		{
			name: "relative path",
			path: "relative/path",
			want: "relative/path", // Should not change
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := expandHome(tt.path)

			if tt.want != "" {
				// Exact match expected
				if got != tt.want {
					t.Errorf("expandHome(%q) = %q, want %q", tt.path, got, tt.want)
				}
			} else {
				// Should be different from input (expanded)
				if got == tt.path {
					t.Errorf("expandHome(%q) = %q, expected expansion", tt.path, got)
				}
			}
		})
	}
}

// Helper function to create test files.
func createFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("Failed to create test file %s: %v", path, err)
	}
}

// Benchmark discovery performance.
func BenchmarkDiscover(b *testing.B) {
	tmpDir := b.TempDir()

	// Create 100 projects with 10 sessions each
	for i := 0; i < 100; i++ {
		projectDir := filepath.Join(tmpDir, filepath.Base(tmpDir)+"-project-"+itoa(i))
		if err := os.MkdirAll(projectDir, 0700); err != nil {
			b.Fatal(err)
		}

		for j := 0; j < 10; j++ {
			sessionFile := filepath.Join(projectDir,
				"a1b2c3d4-e5f6-7890-abcd-"+padHex(j, 12)+".jsonl")
			if err := os.WriteFile(sessionFile, []byte("test"), 0600); err != nil {
				b.Fatal(err)
			}
		}
	}

	logger := &mockLogger{}
	d := New([]string{tmpDir}, logger)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := d.Discover()
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Helper to convert int to string.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}

	negative := i < 0
	if negative {
		i = -i
	}

	var buf [20]byte
	pos := len(buf)

	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}

	if negative {
		pos--
		buf[pos] = '-'
	}

	return string(buf[pos:])
}

// Helper to pad hex string.
func padHex(n, width int) string {
	hex := "0123456789abcdef"
	result := make([]byte, width)

	for i := width - 1; i >= 0; i-- {
		result[i] = hex[n%16]
		n /= 16
	}

	return string(result)
}
