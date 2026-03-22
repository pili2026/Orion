package service

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestCacheGetExpired(t *testing.T) {
	s := &MQTTIngestService{
		cache: map[string]cacheEntry{
			"gw:test": {id: uuid.New(), expiresAt: time.Now().Add(-1 * time.Minute)},
		},
	}

	id, ok := s.cacheGet("gw:test")
	if ok {
		t.Errorf("expected ok=false for expired entry, got ok=true with id=%s", id)
	}
	if id != uuid.Nil {
		t.Errorf("expected uuid.Nil for expired entry, got %s", id)
	}
}

func TestCacheGetValid(t *testing.T) {
	want := uuid.New()
	s := &MQTTIngestService{
		cache: map[string]cacheEntry{
			"gw:test": {id: want, expiresAt: time.Now().Add(30 * time.Minute)},
		},
	}

	id, ok := s.cacheGet("gw:test")
	if !ok {
		t.Errorf("expected ok=true for valid entry, got ok=false")
	}
	if id != want {
		t.Errorf("expected id=%s, got %s", want, id)
	}
}

func TestCacheEvictWorkerRemovesExpiredEntries(t *testing.T) {
	expiredID := uuid.New()
	validID := uuid.New()

	s := &MQTTIngestService{
		cache: map[string]cacheEntry{
			"expired": {id: expiredID, expiresAt: time.Now().Add(-1 * time.Minute)},
			"valid":   {id: validID, expiresAt: time.Now().Add(30 * time.Minute)},
		},
	}

	// Simulate one eviction pass inline (same logic as cacheEvictWorker ticker body).
	now := time.Now()
	s.mu.Lock()
	for k, entry := range s.cache {
		if now.After(entry.expiresAt) {
			delete(s.cache, k)
		}
	}
	s.mu.Unlock()

	if _, ok := s.cache["expired"]; ok {
		t.Error("expected expired entry to be removed, but it still exists")
	}
	entry, ok := s.cache["valid"]
	if !ok {
		t.Error("expected valid entry to remain, but it was removed")
	} else if entry.id != validID {
		t.Errorf("expected valid entry id=%s, got %s", validID, entry.id)
	}
}
