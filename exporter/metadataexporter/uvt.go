package metadataexporter

import (
	"sync"
	"time"
)

// ValueEntry represents a value and its expiration information.
type ValueEntry struct {
	Value    string
	ExpireAt time.Time
	AddedAt  time.Time
}

// KeyValueSet stores unique values for a specific key.
type KeyValueSet struct {
	values map[string]*ValueEntry
}

// UniqueValueTracker tracks unique values per key with a maximum limit and TTL.
type UniqueValueTracker struct {
	maxUniqueValues int
	ttl             time.Duration
	data            map[string]*KeyValueSet
	mu              sync.Mutex
}

// NewUniqueValueTracker initializes a new UniqueValueTracker.
func NewUniqueValueTracker(maxUniqueValues int, ttl time.Duration) *UniqueValueTracker {
	return &UniqueValueTracker{
		maxUniqueValues: maxUniqueValues,
		ttl:             ttl,
		data:            make(map[string]*KeyValueSet),
	}
}

// AddValue adds a value to a key, respecting the maximum unique values and TTL.
func (uvt *UniqueValueTracker) AddValue(key string, value string) {
	uvt.mu.Lock()
	defer uvt.mu.Unlock()

	now := time.Now()

	// Get or create the KeyValueSet for the key.
	kvSet, exists := uvt.data[key]
	if !exists {
		kvSet = &KeyValueSet{
			values: make(map[string]*ValueEntry),
		}
		uvt.data[key] = kvSet
	}

	// Remove expired values.
	for val, entry := range kvSet.values {
		if now.After(entry.ExpireAt) {
			delete(kvSet.values, val)
		}
	}

	// If the value already exists, update its expiration.
	if entry, exists := kvSet.values[value]; exists {
		entry.ExpireAt = now.Add(uvt.ttl)
		entry.AddedAt = now
		return
	}

	// Enforce the maximum unique values constraint.
	if len(kvSet.values) >= uvt.maxUniqueValues {
		// Remove the oldest value.
		var oldestVal string
		var oldestTime time.Time
		for val, entry := range kvSet.values {
			if oldestTime.IsZero() || entry.AddedAt.Before(oldestTime) {
				oldestTime = entry.AddedAt
				oldestVal = val
			}
		}
		delete(kvSet.values, oldestVal)
	}

	// Add the new value.
	kvSet.values[value] = &ValueEntry{
		Value:    value,
		ExpireAt: now.Add(uvt.ttl),
		AddedAt:  now,
	}
}

// GetUniqueValueCount returns the count of unique values for a key.
func (uvt *UniqueValueTracker) GetUniqueValueCount(key string) int {
	uvt.mu.Lock()
	defer uvt.mu.Unlock()

	now := time.Now()

	kvSet, exists := uvt.data[key]
	if !exists {
		return 0
	}

	// Remove expired values.
	for val, entry := range kvSet.values {
		if now.After(entry.ExpireAt) {
			delete(kvSet.values, val)
		}
	}

	return len(kvSet.values)
}

// GetUniqueValues returns the list of unique values for a key.
func (uvt *UniqueValueTracker) GetUniqueValues(key string) []string {
	uvt.mu.Lock()
	defer uvt.mu.Unlock()

	now := time.Now()

	kvSet, exists := uvt.data[key]
	if !exists {
		return []string{}
	}

	// Remove expired values.
	for val, entry := range kvSet.values {
		if now.After(entry.ExpireAt) {
			delete(kvSet.values, val)
		}
	}

	uniqueValues := make([]string, 0, len(kvSet.values))
	for val := range kvSet.values {
		uniqueValues = append(uniqueValues, val)
	}

	return uniqueValues
}
