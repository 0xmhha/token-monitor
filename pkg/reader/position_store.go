package reader

import (
	"encoding/json"
	"fmt"
	"sync"

	bolt "go.etcd.io/bbolt"
)

var (
	bucketPositions = []byte("file_positions") // Path -> Offset
)

// boltPositionStore implements PositionStore using BoltDB.
type boltPositionStore struct {
	db *bolt.DB
	mu sync.RWMutex
}

// NewBoltPositionStore creates a BoltDB-based position store.
//
// Parameters:
//   - db: BoltDB database instance
//
// Returns:
//   - Configured PositionStore
//   - Error if initialization fails
func NewBoltPositionStore(db *bolt.DB) (PositionStore, error) {
	// Initialize bucket.
	if err := db.Update(func(tx *bolt.Tx) error {
		_, createErr := tx.CreateBucketIfNotExists(bucketPositions)
		return createErr
	}); err != nil {
		return nil, fmt.Errorf("failed to create positions bucket: %w", err)
	}

	return &boltPositionStore{
		db: db,
	}, nil
}

// GetPosition implements PositionStore.GetPosition.
func (s *boltPositionStore) GetPosition(path string) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var offset int64

	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketPositions)
		data := b.Get([]byte(path))

		if data == nil {
			// No position stored, start from beginning.
			offset = 0
			return nil
		}

		if unmarshalErr := json.Unmarshal(data, &offset); unmarshalErr != nil {
			return fmt.Errorf("failed to unmarshal offset: %w", unmarshalErr)
		}

		return nil
	})

	if err != nil {
		return 0, err
	}

	return offset, nil
}

// SetPosition implements PositionStore.SetPosition.
func (s *boltPositionStore) SetPosition(path string, offset int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketPositions)

		data, err := json.Marshal(offset)
		if err != nil {
			return fmt.Errorf("failed to marshal offset: %w", err)
		}

		if putErr := b.Put([]byte(path), data); putErr != nil {
			return fmt.Errorf("failed to store position: %w", putErr)
		}

		return nil
	})
}

// memoryPositionStore implements PositionStore using in-memory map.
// Useful for testing.
type memoryPositionStore struct {
	positions map[string]int64
	mu        sync.RWMutex
}

// NewMemoryPositionStore creates an in-memory position store.
//
// Returns a configured PositionStore.
// Useful for testing or when persistence is not needed.
func NewMemoryPositionStore() PositionStore {
	return &memoryPositionStore{
		positions: make(map[string]int64),
	}
}

// GetPosition implements PositionStore.GetPosition.
func (s *memoryPositionStore) GetPosition(path string) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	offset, exists := s.positions[path]
	if !exists {
		return 0, nil
	}

	return offset, nil
}

// SetPosition implements PositionStore.SetPosition.
func (s *memoryPositionStore) SetPosition(path string, offset int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.positions[path] = offset
	return nil
}
