package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yourusername/token-monitor/pkg/logger"
	bolt "go.etcd.io/bbolt"
)

// Bucket names.
var (
	bucketSessions = []byte("sessions") // UUID -> Metadata
	bucketNames    = []byte("names")    // Name -> UUID (index)
)

// manager implements the Manager interface using BoltDB.
type manager struct {
	db     *bolt.DB
	logger logger.Logger
	config Config
}

// New creates a new session manager.
//
// Parameters:
//   - cfg: Manager configuration
//   - log: Logger instance
//
// Returns:
//   - Configured Manager
//   - Error if database cannot be opened
func New(cfg Config, log logger.Logger) (Manager, error) {
	// Set default timeout.
	if cfg.Timeout == 0 {
		cfg.Timeout = time.Second
	}

	// Expand home directory in path.
	dbPath := expandHome(cfg.DBPath)

	// Create directory if it doesn't exist.
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	// Open database.
	db, err := bolt.Open(dbPath, 0600, &bolt.Options{
		Timeout: cfg.Timeout,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Initialize buckets.
	if err := db.Update(func(tx *bolt.Tx) error {
		if _, createErr := tx.CreateBucketIfNotExists(bucketSessions); createErr != nil {
			return fmt.Errorf("failed to create sessions bucket: %w", createErr)
		}
		if _, createErr := tx.CreateBucketIfNotExists(bucketNames); createErr != nil {
			return fmt.Errorf("failed to create names bucket: %w", createErr)
		}
		return nil
	}); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			log.Error("failed to close database after initialization error",
				"error", closeErr)
		}
		return nil, err
	}

	log.Info("session manager initialized", "db_path", dbPath)

	return &manager{
		db:     db,
		logger: log,
		config: cfg,
	}, nil
}

// Create implements Manager.Create.
func (m *manager) Create(metadata *Metadata) error {
	if metadata == nil {
		return ErrInvalidMetadata
	}

	// Validate UUID.
	if !isValidUUID(metadata.UUID) {
		return ErrInvalidUUID
	}

	// Validate name.
	if metadata.Name == "" {
		return ErrEmptyName
	}

	// Set timestamps.
	now := time.Now()
	metadata.CreatedAt = now
	metadata.UpdatedAt = now

	return m.db.Update(func(tx *bolt.Tx) error {
		sessions := tx.Bucket(bucketSessions)
		names := tx.Bucket(bucketNames)

		// Check if UUID already exists.
		if sessions.Get([]byte(metadata.UUID)) != nil {
			return fmt.Errorf("session %s already exists", metadata.UUID)
		}

		// Check if name is already taken.
		if names.Get([]byte(metadata.Name)) != nil {
			return ErrNameConflict
		}

		// Marshal metadata.
		data, err := json.Marshal(metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}

		// Store in sessions bucket.
		if err := sessions.Put([]byte(metadata.UUID), data); err != nil {
			return fmt.Errorf("failed to store session: %w", err)
		}

		// Store in names index.
		if err := names.Put([]byte(metadata.Name), []byte(metadata.UUID)); err != nil {
			return fmt.Errorf("failed to store name index: %w", err)
		}

		m.logger.Info("session created",
			"uuid", metadata.UUID,
			"name", metadata.Name)

		return nil
	})
}

// GetByUUID implements Manager.GetByUUID.
func (m *manager) GetByUUID(uuid string) (*Metadata, error) {
	if !isValidUUID(uuid) {
		return nil, ErrInvalidUUID
	}

	var metadata *Metadata

	err := m.db.View(func(tx *bolt.Tx) error {
		sessions := tx.Bucket(bucketSessions)

		data := sessions.Get([]byte(uuid))
		if data == nil {
			return ErrSessionNotFound
		}

		var m Metadata
		if unmarshalErr := json.Unmarshal(data, &m); unmarshalErr != nil {
			return fmt.Errorf("failed to unmarshal metadata: %w", unmarshalErr)
		}

		metadata = &m
		return nil
	})

	if err != nil {
		return nil, err
	}

	return metadata, nil
}

// GetByName implements Manager.GetByName.
func (m *manager) GetByName(name string) (*Metadata, error) {
	if name == "" {
		return nil, ErrEmptyName
	}

	var uuid string

	// First, get UUID from name index.
	if err := m.db.View(func(tx *bolt.Tx) error {
		names := tx.Bucket(bucketNames)

		uuidBytes := names.Get([]byte(name))
		if uuidBytes == nil {
			return ErrSessionNotFound
		}

		uuid = string(uuidBytes)
		return nil
	}); err != nil {
		return nil, err
	}

	// Then, get metadata by UUID.
	return m.GetByUUID(uuid)
}

// Update implements Manager.Update.
func (m *manager) Update(uuid string, metadata *Metadata) error {
	if metadata == nil {
		return ErrInvalidMetadata
	}

	if !isValidUUID(uuid) {
		return ErrInvalidUUID
	}

	if metadata.Name == "" {
		return ErrEmptyName
	}

	return m.db.Update(func(tx *bolt.Tx) error {
		sessions := tx.Bucket(bucketSessions)
		names := tx.Bucket(bucketNames)

		// Check if session exists.
		existingData := sessions.Get([]byte(uuid))
		if existingData == nil {
			return ErrSessionNotFound
		}

		// Unmarshal existing metadata.
		var existing Metadata
		if err := json.Unmarshal(existingData, &existing); err != nil {
			return fmt.Errorf("failed to unmarshal existing metadata: %w", err)
		}

		// Check if name changed and if new name is available.
		if existing.Name != metadata.Name {
			// Check if new name is already taken.
			if existingUUID := names.Get([]byte(metadata.Name)); existingUUID != nil {
				return ErrNameConflict
			}

			// Remove old name from index.
			if err := names.Delete([]byte(existing.Name)); err != nil {
				return fmt.Errorf("failed to delete old name index: %w", err)
			}

			// Add new name to index.
			if err := names.Put([]byte(metadata.Name), []byte(uuid)); err != nil {
				return fmt.Errorf("failed to store new name index: %w", err)
			}
		}

		// Preserve creation time, update modification time.
		metadata.UUID = uuid
		metadata.CreatedAt = existing.CreatedAt
		metadata.UpdatedAt = time.Now()

		// Marshal updated metadata.
		data, err := json.Marshal(metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}

		// Store updated metadata.
		if err := sessions.Put([]byte(uuid), data); err != nil {
			return fmt.Errorf("failed to update session: %w", err)
		}

		m.logger.Info("session updated",
			"uuid", uuid,
			"name", metadata.Name)

		return nil
	})
}

// Delete implements Manager.Delete.
func (m *manager) Delete(uuid string) error {
	if !isValidUUID(uuid) {
		return ErrInvalidUUID
	}

	return m.db.Update(func(tx *bolt.Tx) error {
		sessions := tx.Bucket(bucketSessions)
		names := tx.Bucket(bucketNames)

		// Get existing metadata to find name.
		data := sessions.Get([]byte(uuid))
		if data == nil {
			// Session doesn't exist, no error.
			return nil
		}

		// Unmarshal to get name.
		var metadata Metadata
		if err := json.Unmarshal(data, &metadata); err != nil {
			return fmt.Errorf("failed to unmarshal metadata: %w", err)
		}

		// Delete from sessions bucket.
		if err := sessions.Delete([]byte(uuid)); err != nil {
			return fmt.Errorf("failed to delete session: %w", err)
		}

		// Delete from names index.
		if err := names.Delete([]byte(metadata.Name)); err != nil {
			return fmt.Errorf("failed to delete name index: %w", err)
		}

		m.logger.Info("session deleted",
			"uuid", uuid,
			"name", metadata.Name)

		return nil
	})
}

// List implements Manager.List.
func (m *manager) List() ([]*Metadata, error) {
	sessions := make([]*Metadata, 0, 10)

	err := m.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketSessions)

		return b.ForEach(func(k, v []byte) error {
			var metadata Metadata
			if unmarshalErr := json.Unmarshal(v, &metadata); unmarshalErr != nil {
				m.logger.Warn("failed to unmarshal session",
					"uuid", string(k),
					"error", unmarshalErr)
				return nil // Skip invalid entries.
			}

			sessions = append(sessions, &metadata)
			return nil
		})
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}

	return sessions, nil
}

// SetName implements Manager.SetName.
func (m *manager) SetName(uuid, name string) error {
	if !isValidUUID(uuid) {
		return ErrInvalidUUID
	}

	if name == "" {
		return ErrEmptyName
	}

	// Get existing metadata.
	existing, err := m.GetByUUID(uuid)
	if err != nil {
		return err
	}

	// Update name.
	existing.Name = name

	// Use Update to handle name change.
	return m.Update(uuid, existing)
}

// Close implements Manager.Close.
func (m *manager) Close() error {
	if err := m.db.Close(); err != nil {
		return fmt.Errorf("failed to close database: %w", err)
	}

	m.logger.Info("session manager closed")
	return nil
}

// isValidUUID performs basic validation on UUID format.
//
// Expected format: UUID v4 (8-4-4-4-12 hex digits with dashes)
// Example: a1b2c3d4-e5f6-7890-abcd-ef1234567890.
func isValidUUID(id string) bool {
	// Basic length check (UUID v4 is 36 characters).
	if len(id) != 36 {
		return false
	}

	// Check for dashes at correct positions.
	if id[8] != '-' || id[13] != '-' || id[18] != '-' || id[23] != '-' {
		return false
	}

	// Check that other characters are hex digits.
	for i, c := range id {
		// Skip dash positions.
		if i == 8 || i == 13 || i == 18 || i == 23 {
			continue
		}

		// Check if character is hex digit.
		if !isHexDigit(c) {
			return false
		}
	}

	return true
}

// isHexDigit checks if a rune is a hexadecimal digit.
func isHexDigit(r rune) bool {
	return (r >= '0' && r <= '9') ||
		(r >= 'a' && r <= 'f') ||
		(r >= 'A' && r <= 'F')
}

// expandHome expands ~ in file paths to the user's home directory.
func expandHome(path string) string {
	if !strings.HasPrefix(path, "~") {
		return path
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return path
	}

	if path == "~" {
		return homeDir
	}

	return filepath.Join(homeDir, path[2:])
}
