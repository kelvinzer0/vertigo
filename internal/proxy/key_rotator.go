package proxy

import (
	"sync"
)

// KeyRotator manages a list of API keys and rotates them in a thread-safe manner.
type KeyRotator struct {
	keys    []string
	index   int
	mutex   sync.Mutex
}

// NewKeyRotator creates a new KeyRotator with the given API keys.
func NewKeyRotator(keys []string) *KeyRotator {
	return &KeyRotator{
		keys: keys,
	}
}

// GetNextKey returns the next API key in the list in a round-robin fashion.
func (kr *KeyRotator) GetNextKey() string {
	kr.mutex.Lock()
	defer kr.mutex.Unlock()

	if len(kr.keys) == 0 {
		return ""
	}

	key := kr.keys[kr.index]
	kr.index = (kr.index + 1) % len(kr.keys)
	return key
}
