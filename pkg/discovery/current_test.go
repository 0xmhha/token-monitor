package discovery

import (
	"os"
	"path/filepath"
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

// sessionIDs contains valid UUID v4 session IDs for reuse across tests.
var sessionIDs = [5]string{
	"a1b2c3d4-e5f6-7890-abcd-ef1234567890",
	"b2c3d4e5-f6a7-8901-bcde-f12345678901",
	"c3d4e5f6-a7b8-9012-cdef-123456789012",
	"d4e5f6a7-b8c9-0123-defa-234567890123",
	"e5f6a7b8-c9d0-1234-efab-345678901234",
}

// makeProjectDir creates a project subdirectory inside baseDir and returns its path.
func makeProjectDir(t *testing.T, baseDir, name string) string {
	t.Helper()
	dir := filepath.Join(baseDir, name)
	require.NoError(t, os.MkdirAll(dir, 0700))
	return dir
}

// createSessionFile creates a UUID.jsonl file in the given directory.
func createSessionFile(t *testing.T, dir, sessionID string) string {
	t.Helper()
	path := filepath.Join(dir, sessionID+".jsonl")
	require.NoError(t, os.WriteFile(path, []byte(`{"test":true}`), 0600))
	return path
}

// setEnv sets an environment variable and registers cleanup to unset it.
func setEnv(t *testing.T, key, value string) {
	t.Helper()
	t.Cleanup(func() { os.Unsetenv(key) })
	require.NoError(t, os.Setenv(key, value))
}

// TestFindCurrentSession_EnvSessionID verifies that when CLAUDE_SESSION_ID is set
// to a session that exists on disk, FindCurrentSession returns that exact session.
func TestFindCurrentSession_EnvSessionID(t *testing.T) {
	baseDir := t.TempDir()
	projectDir := makeProjectDir(t, baseDir, "my-project")

	targetID := sessionIDs[0]
	createSessionFile(t, projectDir, targetID)
	// Create another session to make sure the right one is picked.
	createSessionFile(t, projectDir, sessionIDs[1])

	setEnv(t, "CLAUDE_SESSION_ID", targetID)

	d := New([]string{baseDir}, &testLogger{})
	session, err := d.FindCurrentSession()

	require.NoError(t, err)
	require.NotNil(t, session)
	assert.Equal(t, targetID, session.SessionID)
}

// TestFindCurrentSession_EnvProjectDir verifies that when CLAUDE_PROJECT_DIR is set,
// FindCurrentSession returns the most recently modified .jsonl file in that directory.
func TestFindCurrentSession_EnvProjectDir(t *testing.T) {
	baseDir := t.TempDir()
	// CLAUDE_PROJECT_DIR points directly to a project directory, not a baseDir.
	projectDir := makeProjectDir(t, baseDir, "target-project")

	olderID := sessionIDs[0]
	newerID := sessionIDs[1]

	createSessionFile(t, projectDir, olderID)
	createSessionFile(t, projectDir, newerID)

	// Assign unambiguous timestamps: older file gets a past time, newer gets a future time.
	past := time.Now().Add(-time.Hour)
	future := time.Now().Add(time.Hour)
	require.NoError(t, os.Chtimes(filepath.Join(projectDir, olderID+".jsonl"), past, past))
	require.NoError(t, os.Chtimes(filepath.Join(projectDir, newerID+".jsonl"), future, future))

	setEnv(t, "CLAUDE_PROJECT_DIR", projectDir)

	d := New([]string{baseDir}, &testLogger{})
	session, err := d.FindCurrentSession()

	require.NoError(t, err)
	require.NotNil(t, session)
	assert.Equal(t, newerID, session.SessionID)
}

// TestFindCurrentSession_FallbackMostRecent verifies that with no env vars set,
// FindCurrentSession returns the session with the highest ModTime across all dirs.
func TestFindCurrentSession_FallbackMostRecent(t *testing.T) {
	baseDir := t.TempDir()
	projectA := makeProjectDir(t, baseDir, "project-a")
	projectB := makeProjectDir(t, baseDir, "project-b")

	oldID := sessionIDs[0]
	newID := sessionIDs[1]

	createSessionFile(t, projectA, oldID)
	// Ensure a measurable gap so ModTime differs reliably.
	time.Sleep(10 * time.Millisecond)
	createSessionFile(t, projectB, newID)

	// Explicitly set the newer file's timestamp to be in the future.
	future := time.Now().Add(time.Hour)
	require.NoError(t, os.Chtimes(
		filepath.Join(projectB, newID+".jsonl"),
		future, future,
	))

	d := New([]string{baseDir}, &testLogger{})
	session, err := d.FindCurrentSession()

	require.NoError(t, err)
	require.NotNil(t, session)
	assert.Equal(t, newID, session.SessionID)
}

// TestFindCurrentSession_Caching verifies that two consecutive calls within the
// cache TTL window return the same *SessionFile pointer (cached result).
func TestFindCurrentSession_Caching(t *testing.T) {
	baseDir := t.TempDir()
	projectDir := makeProjectDir(t, baseDir, "project")
	createSessionFile(t, projectDir, sessionIDs[0])

	d := New([]string{baseDir}, &testLogger{})

	first, err := d.FindCurrentSession()
	require.NoError(t, err)
	require.NotNil(t, first)

	// Second call must happen within the 1-second cache TTL.
	second, err := d.FindCurrentSession()
	require.NoError(t, err)
	require.NotNil(t, second)

	// Both calls should return the same pointer — proof the cache was used.
	assert.Same(t, first, second, "second call should return cached pointer")
}

// TestFindCurrentSession_NoSessions verifies that ErrNoCurrentSession is returned
// when the configured directories contain no valid session files.
func TestFindCurrentSession_NoSessions(t *testing.T) {
	baseDir := t.TempDir()
	// Create a project dir but leave it empty.
	makeProjectDir(t, baseDir, "empty-project")

	d := New([]string{baseDir}, &testLogger{})
	session, err := d.FindCurrentSession()

	assert.Nil(t, session)
	assert.ErrorIs(t, err, ErrNoCurrentSession)
}

// TestFindCurrentSession_EnvSessionIDNotFound verifies that when CLAUDE_SESSION_ID
// is set to a non-existent ID, the implementation falls through to the next
// detection method (most-recent fallback) rather than returning an error.
func TestFindCurrentSession_EnvSessionIDNotFound(t *testing.T) {
	baseDir := t.TempDir()
	projectDir := makeProjectDir(t, baseDir, "project")

	existingID := sessionIDs[0]
	createSessionFile(t, projectDir, existingID)

	// Point CLAUDE_SESSION_ID at an ID that does NOT exist on disk.
	setEnv(t, "CLAUDE_SESSION_ID", sessionIDs[4])

	d := New([]string{baseDir}, &testLogger{})
	session, err := d.FindCurrentSession()

	// Fallthrough should succeed and return the only existing session.
	require.NoError(t, err)
	require.NotNil(t, session)
	assert.Equal(t, existingID, session.SessionID)
}

// TestFindCurrentSession_EnvProjectDir_MultipleFiles verifies that when
// CLAUDE_PROJECT_DIR contains several files, the most recently modified one wins.
func TestFindCurrentSession_EnvProjectDir_MultipleFiles(t *testing.T) {
	projectDir := t.TempDir()

	for _, id := range sessionIDs[:3] {
		createSessionFile(t, projectDir, id)
		time.Sleep(5 * time.Millisecond)
	}

	// Explicitly make the last file the newest.
	latestID := sessionIDs[2]
	future := time.Now().Add(time.Hour)
	require.NoError(t, os.Chtimes(
		filepath.Join(projectDir, latestID+".jsonl"),
		future, future,
	))

	setEnv(t, "CLAUDE_PROJECT_DIR", projectDir)

	d := New([]string{}, &testLogger{})
	session, err := d.FindCurrentSession()

	require.NoError(t, err)
	require.NotNil(t, session)
	assert.Equal(t, latestID, session.SessionID)
}

// TestFindCurrentSession_EnvProjectDir_Empty verifies that when CLAUDE_PROJECT_DIR
// exists but contains no .jsonl files, FindCurrentSession falls through to baseDirs
// rather than failing. This handles the case where Claude Code sets CLAUDE_PROJECT_DIR
// to the source code directory (not the session storage directory).
func TestFindCurrentSession_EnvProjectDir_Empty(t *testing.T) {
	baseDir := t.TempDir()
	projectDir := t.TempDir() // separate empty dir simulating a source code directory

	existingProject := makeProjectDir(t, baseDir, "project")
	existingID := sessionIDs[0]
	createSessionFile(t, existingProject, existingID)

	setEnv(t, "CLAUDE_PROJECT_DIR", projectDir)

	d := New([]string{baseDir}, &testLogger{})
	session, err := d.FindCurrentSession()

	require.NoError(t, err)
	require.NotNil(t, session)
	assert.Equal(t, existingID, session.SessionID)
}

// TestFindCurrentSession_EnvProjectDir_EmptyNoFallback verifies that ErrNoCurrentSession
// is returned when CLAUDE_PROJECT_DIR has no sessions AND baseDirs also have none.
func TestFindCurrentSession_EnvProjectDir_EmptyNoFallback(t *testing.T) {
	projectDir := t.TempDir()
	setEnv(t, "CLAUDE_PROJECT_DIR", projectDir)

	d := New([]string{}, &testLogger{})
	session, err := d.FindCurrentSession()

	assert.Nil(t, session)
	assert.ErrorIs(t, err, ErrNoCurrentSession)
}
