package proxy

import (
	"sync"
	"time"
)

// KeyStatus represents the status of an API key.
type KeyStatus struct {
	IsBad     bool
	BadUntil  time.Time
}

// KeyManager manages a list of API keys and their statuses.
type KeyManager struct {
	keys      []string
	keyStatus map[string]*KeyStatus // Map key to its status
	mutex     sync.Mutex
}

// NewKeyManager creates a new KeyManager with the given API keys.
func NewKeyManager(keys []string) *KeyManager {
	km := &KeyManager{
		keys:      keys,
		keyStatus: make(map[string]*KeyStatus),
	}
	for _, key := range keys {
		km.keyStatus[key] = &KeyStatus{IsBad: false}
	}
	return km
}

// GetNextAvailableKey returns the next available API key. It prioritizes keys that are not marked as bad.
// If all keys are bad, it will return an empty string.
func (km *KeyManager) GetNextAvailableKey() string {
	km.mutex.Lock()
	defer km.mutex.Unlock()

	for _, key := range km.keys {
		status := km.keyStatus[key]
		if !status.IsBad || time.Now().After(status.BadUntil) {
			// If the key is not bad, or if it was bad but the badUntil time has passed, mark it as good and return it.
			status.IsBad = false
			return key
		}
	}
	return "" // No available key
}

// MarkKeyAsBad marks a key as bad for a certain duration.
func (km *KeyManager) MarkKeyAsBad(key string, duration time.Duration) {
	km.mutex.Lock()
	defer km.mutex.Unlock()

	if status, ok := km.keyStatus[key]; ok {
		status.IsBad = true
		status.BadUntil = time.Now().Add(duration)
	}
}
