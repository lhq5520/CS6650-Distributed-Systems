package main

import "sync"

// KVEntry represents a versioned key-value pair.
// Version is a logical clock used to determine which value is newer
// when reading from multiple nodes.
type KVEntry struct {
	Value   string `json:"value"`
	Version int    `json:"version"`
}

// KVStore is a thread-safe in-memory key-value store.
// It uses a RWMutex so multiple reads can happen concurrently,
// but writes require exclusive access.
type KVStore struct {
	mu   sync.RWMutex
	data map[string]KVEntry
}

func NewKVStore() *KVStore {
	return &KVStore{
		data: make(map[string]KVEntry),
	}
}

// Set stores a key-value pair with an explicit version number.
// Only updates if the new version is greater than the existing one,
// preventing stale data from overwriting newer data.
// Used by Follower nodes when receiving replicated data from the Leader.
func (s *KVStore) Set(key, value string, version int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.data[key]; !ok || version > existing.Version {
		s.data[key] = KVEntry{Value: value, Version: version}
	}
}

// Get retrieves a key-value entry.
// Returns the entry and whether the key exists.
func (s *KVStore) Get(key string) (KVEntry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.data[key]
	return entry, ok
}

// SetAndIncrement atomically increments the version and stores the value.
// Used by the Leader or Write Coordinator to assign new version numbers.
// Returns the new version number.
func (s *KVStore) SetAndIncrement(key, value string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	newVersion := 1
	if existing, ok := s.data[key]; ok {
		newVersion = existing.Version + 1
	}
	s.data[key] = KVEntry{Value: value, Version: newVersion}
	return newVersion
}
