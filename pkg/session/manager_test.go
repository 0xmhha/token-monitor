package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yourusername/token-monitor/pkg/logger"
)

func TestNew(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	mgr, err := New(Config{
		DBPath: dbPath,
	}, logger.Noop())

	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if mgr == nil {
		t.Error("New() returned nil manager")
	}

	if closeErr := mgr.Close(); closeErr != nil {
		t.Errorf("Close() error = %v", closeErr)
	}

	// Verify database file was created.
	if _, statErr := os.Stat(dbPath); statErr != nil {
		t.Errorf("Database file not created: %v", statErr)
	}
}

func TestNewWithHomeDir(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Use relative path to temp dir (not actual home).
	mgr, err := New(Config{
		DBPath: dbPath,
	}, logger.Noop())

	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if closeErr := mgr.Close(); closeErr != nil {
		t.Errorf("Close() error = %v", closeErr)
	}
}

func TestCreate(t *testing.T) {
	mgr := setupTestManager(t)

	metadata := &Metadata{
		UUID:        "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
		Name:        "test-session",
		ProjectPath: "/path/to/project",
		Tags:        []string{"dev", "backend"},
		Description: "Test session",
	}

	if err := mgr.Create(metadata); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Verify creation.
	retrieved, err := mgr.GetByUUID(metadata.UUID)
	if err != nil {
		t.Fatalf("GetByUUID() error = %v", err)
	}

	if retrieved.UUID != metadata.UUID {
		t.Errorf("UUID = %s, want %s", retrieved.UUID, metadata.UUID)
	}

	if retrieved.Name != metadata.Name {
		t.Errorf("Name = %s, want %s", retrieved.Name, metadata.Name)
	}

	if retrieved.CreatedAt.IsZero() {
		t.Error("CreatedAt is zero")
	}

	if retrieved.UpdatedAt.IsZero() {
		t.Error("UpdatedAt is zero")
	}
}

func TestCreateInvalidUUID(t *testing.T) {
	mgr := setupTestManager(t)

	metadata := &Metadata{
		UUID: "invalid-uuid",
		Name: "test",
	}

	err := mgr.Create(metadata)
	if err != ErrInvalidUUID {
		t.Errorf("Create() error = %v, want ErrInvalidUUID", err)
	}
}

func TestCreateEmptyName(t *testing.T) {
	mgr := setupTestManager(t)

	metadata := &Metadata{
		UUID: "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
		Name: "",
	}

	err := mgr.Create(metadata)
	if err != ErrEmptyName {
		t.Errorf("Create() error = %v, want ErrEmptyName", err)
	}
}

func TestCreateDuplicateName(t *testing.T) {
	mgr := setupTestManager(t)

	metadata1 := &Metadata{
		UUID: "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
		Name: "test-session",
	}

	metadata2 := &Metadata{
		UUID: "b2c3d4e5-f6a7-8901-bcde-f12345678901",
		Name: "test-session", // Same name.
	}

	if err := mgr.Create(metadata1); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	err := mgr.Create(metadata2)
	if err != ErrNameConflict {
		t.Errorf("Create() error = %v, want ErrNameConflict", err)
	}
}

func TestGetByUUID(t *testing.T) {
	mgr := setupTestManager(t)

	metadata := &Metadata{
		UUID: "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
		Name: "test-session",
	}

	if err := mgr.Create(metadata); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	retrieved, err := mgr.GetByUUID(metadata.UUID)
	if err != nil {
		t.Fatalf("GetByUUID() error = %v", err)
	}

	if retrieved.UUID != metadata.UUID {
		t.Errorf("UUID = %s, want %s", retrieved.UUID, metadata.UUID)
	}
}

func TestGetByUUIDNotFound(t *testing.T) {
	mgr := setupTestManager(t)

	_, err := mgr.GetByUUID("a1b2c3d4-e5f6-7890-abcd-ef1234567890")
	if err != ErrSessionNotFound {
		t.Errorf("GetByUUID() error = %v, want ErrSessionNotFound", err)
	}
}

func TestGetByName(t *testing.T) {
	mgr := setupTestManager(t)

	metadata := &Metadata{
		UUID: "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
		Name: "test-session",
	}

	if err := mgr.Create(metadata); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	retrieved, err := mgr.GetByName(metadata.Name)
	if err != nil {
		t.Fatalf("GetByName() error = %v", err)
	}

	if retrieved.Name != metadata.Name {
		t.Errorf("Name = %s, want %s", retrieved.Name, metadata.Name)
	}
}

func TestGetByNameNotFound(t *testing.T) {
	mgr := setupTestManager(t)

	_, err := mgr.GetByName("nonexistent")
	if err != ErrSessionNotFound {
		t.Errorf("GetByName() error = %v, want ErrSessionNotFound", err)
	}
}

func TestUpdate(t *testing.T) {
	mgr := setupTestManager(t)

	metadata := &Metadata{
		UUID:        "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
		Name:        "test-session",
		Description: "Original",
	}

	if err := mgr.Create(metadata); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Wait to ensure UpdatedAt changes.
	time.Sleep(10 * time.Millisecond)

	// Update description.
	updated := &Metadata{
		Name:        "test-session",
		Description: "Updated",
	}

	if err := mgr.Update(metadata.UUID, updated); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	// Verify update.
	retrieved, err := mgr.GetByUUID(metadata.UUID)
	if err != nil {
		t.Fatalf("GetByUUID() error = %v", err)
	}

	if retrieved.Description != "Updated" {
		t.Errorf("Description = %s, want Updated", retrieved.Description)
	}

	if retrieved.UpdatedAt.Before(retrieved.CreatedAt) {
		t.Error("UpdatedAt should be after CreatedAt")
	}
}

func TestUpdateName(t *testing.T) {
	mgr := setupTestManager(t)

	metadata := &Metadata{
		UUID: "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
		Name: "old-name",
	}

	if err := mgr.Create(metadata); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Update name.
	updated := &Metadata{
		Name: "new-name",
	}

	if err := mgr.Update(metadata.UUID, updated); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	// Verify old name is gone.
	_, err := mgr.GetByName("old-name")
	if err != ErrSessionNotFound {
		t.Errorf("GetByName(old-name) error = %v, want ErrSessionNotFound", err)
	}

	// Verify new name works.
	retrieved, err := mgr.GetByName("new-name")
	if err != nil {
		t.Fatalf("GetByName(new-name) error = %v", err)
	}

	if retrieved.UUID != metadata.UUID {
		t.Errorf("UUID = %s, want %s", retrieved.UUID, metadata.UUID)
	}
}

func TestUpdateNameConflict(t *testing.T) {
	mgr := setupTestManager(t)

	metadata1 := &Metadata{
		UUID: "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
		Name: "session1",
	}

	metadata2 := &Metadata{
		UUID: "b2c3d4e5-f6a7-8901-bcde-f12345678901",
		Name: "session2",
	}

	if err := mgr.Create(metadata1); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if err := mgr.Create(metadata2); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Try to update session1 to session2's name.
	updated := &Metadata{
		Name: "session2",
	}

	err := mgr.Update(metadata1.UUID, updated)
	if err != ErrNameConflict {
		t.Errorf("Update() error = %v, want ErrNameConflict", err)
	}
}

func TestUpdateNotFound(t *testing.T) {
	mgr := setupTestManager(t)

	updated := &Metadata{
		Name: "test",
	}

	err := mgr.Update("a1b2c3d4-e5f6-7890-abcd-ef1234567890", updated)
	if err != ErrSessionNotFound {
		t.Errorf("Update() error = %v, want ErrSessionNotFound", err)
	}
}

func TestDelete(t *testing.T) {
	mgr := setupTestManager(t)

	metadata := &Metadata{
		UUID: "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
		Name: "test-session",
	}

	if err := mgr.Create(metadata); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Delete session.
	if err := mgr.Delete(metadata.UUID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Verify deletion.
	_, err := mgr.GetByUUID(metadata.UUID)
	if err != ErrSessionNotFound {
		t.Errorf("GetByUUID() error = %v, want ErrSessionNotFound", err)
	}

	// Verify name index is also removed.
	_, err = mgr.GetByName(metadata.Name)
	if err != ErrSessionNotFound {
		t.Errorf("GetByName() error = %v, want ErrSessionNotFound", err)
	}
}

func TestDeleteNotFound(t *testing.T) {
	mgr := setupTestManager(t)

	// Should not error when deleting nonexistent session.
	if err := mgr.Delete("a1b2c3d4-e5f6-7890-abcd-ef1234567890"); err != nil {
		t.Errorf("Delete() error = %v, want nil", err)
	}
}

func TestList(t *testing.T) {
	mgr := setupTestManager(t)

	// Create multiple sessions.
	sessions := []*Metadata{
		{
			UUID: "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
			Name: "session1",
		},
		{
			UUID: "b2c3d4e5-f6a7-8901-bcde-f12345678901",
			Name: "session2",
		},
		{
			UUID: "c3d4e5f6-a7b8-9012-cdef-123456789012",
			Name: "session3",
		},
	}

	for _, s := range sessions {
		if err := mgr.Create(s); err != nil {
			t.Fatalf("Create() error = %v", err)
		}
	}

	// List all sessions.
	list, err := mgr.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(list) != 3 {
		t.Errorf("List() returned %d sessions, want 3", len(list))
	}

	// Verify all sessions are present.
	found := make(map[string]bool)
	for _, s := range list {
		found[s.UUID] = true
	}

	for _, s := range sessions {
		if !found[s.UUID] {
			t.Errorf("Session %s not found in list", s.UUID)
		}
	}
}

func TestListEmpty(t *testing.T) {
	mgr := setupTestManager(t)

	list, err := mgr.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(list) != 0 {
		t.Errorf("List() returned %d sessions, want 0", len(list))
	}
}

func TestSetName(t *testing.T) {
	mgr := setupTestManager(t)

	metadata := &Metadata{
		UUID: "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
		Name: "old-name",
	}

	if err := mgr.Create(metadata); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Set new name.
	if err := mgr.SetName(metadata.UUID, "new-name"); err != nil {
		t.Fatalf("SetName() error = %v", err)
	}

	// Verify new name.
	retrieved, err := mgr.GetByName("new-name")
	if err != nil {
		t.Fatalf("GetByName() error = %v", err)
	}

	if retrieved.Name != "new-name" {
		t.Errorf("Name = %s, want new-name", retrieved.Name)
	}
}

func TestSetNameNotFound(t *testing.T) {
	mgr := setupTestManager(t)

	err := mgr.SetName("a1b2c3d4-e5f6-7890-abcd-ef1234567890", "test")
	if err != ErrSessionNotFound {
		t.Errorf("SetName() error = %v, want ErrSessionNotFound", err)
	}
}

func TestIsValidUUID(t *testing.T) {
	tests := []struct {
		name string
		uuid string
		want bool
	}{
		{
			name: "valid UUID v4",
			uuid: "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
			want: true,
		},
		{
			name: "valid UUID with uppercase",
			uuid: "A1B2C3D4-E5F6-7890-ABCD-EF1234567890",
			want: true,
		},
		{
			name: "too short",
			uuid: "a1b2c3d4-e5f6-7890-abcd-ef123456789",
			want: false,
		},
		{
			name: "too long",
			uuid: "a1b2c3d4-e5f6-7890-abcd-ef12345678901",
			want: false,
		},
		{
			name: "missing dashes",
			uuid: "a1b2c3d4e5f678 90abcdef1234567890",
			want: false,
		},
		{
			name: "non-hex characters",
			uuid: "g1b2c3d4-e5f6-7890-abcd-ef1234567890",
			want: false,
		},
		{
			name: "empty string",
			uuid: "",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidUUID(tt.uuid)
			if got != tt.want {
				t.Errorf("isValidUUID(%q) = %v, want %v", tt.uuid, got, tt.want)
			}
		})
	}
}

func TestConcurrentAccess(t *testing.T) {
	mgr := setupTestManager(t)

	// Create a session.
	metadata := &Metadata{
		UUID: "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
		Name: "test-session",
	}

	if err := mgr.Create(metadata); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Concurrent reads.
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			_, err := mgr.GetByUUID(metadata.UUID)
			if err != nil {
				t.Errorf("GetByUUID() error = %v", err)
			}
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestDataPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create manager and add session.
	mgr1, err := New(Config{DBPath: dbPath}, logger.Noop())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	metadata := &Metadata{
		UUID: "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
		Name: "test-session",
	}

	if createErr := mgr1.Create(metadata); createErr != nil {
		t.Fatalf("Create() error = %v", createErr)
	}

	if closeErr := mgr1.Close(); closeErr != nil {
		t.Fatalf("Close() error = %v", closeErr)
	}

	// Open manager again and verify persistence.
	mgr2, err := New(Config{DBPath: dbPath}, logger.Noop())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() {
		if closeErr := mgr2.Close(); closeErr != nil {
			t.Errorf("Close() error = %v", closeErr)
		}
	}()

	retrieved, getErr := mgr2.GetByUUID(metadata.UUID)
	if getErr != nil {
		t.Fatalf("GetByUUID() error = %v", getErr)
	}

	if retrieved.UUID != metadata.UUID {
		t.Errorf("UUID = %s, want %s", retrieved.UUID, metadata.UUID)
	}

	if retrieved.Name != metadata.Name {
		t.Errorf("Name = %s, want %s", retrieved.Name, metadata.Name)
	}
}

// setupTestManager creates a test manager with temp database.
func setupTestManager(t *testing.T) Manager {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	mgr, err := New(Config{
		DBPath: dbPath,
	}, logger.Noop())

	if err != nil {
		t.Fatalf("Failed to create test manager: %v", err)
	}

	// Register cleanup handler.
	t.Cleanup(func() {
		if closeErr := mgr.Close(); closeErr != nil {
			t.Errorf("Cleanup Close() error = %v", closeErr)
		}
	})

	return mgr
}
