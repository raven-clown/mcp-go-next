package mcp

import (
	"container/list"
	"context"
	"errors"
	"sync"
	"time"
)

// ErrHandleNotFound is returned for a handle that is missing, expired, or owned by another
// principal. The three look the same, so IDs cannot be probed.
var ErrHandleNotFound = errors.New("mcp: handle not found")

// HandleStore keeps handle values for Handles. Values arrive JSON encoded.
//
// Put with a positive ttl stores a new entry. Put with ttl 0 replaces the value of an existing
// entry and keeps its owner and deadline, as SET XX KEEPTTL does in Redis, and returns
// ErrHandleNotFound when there is no such entry. Get and Delete return ErrHandleNotFound for a
// missing or expired entry.
type HandleStore interface {
	Put(ctx context.Context, id, owner string, value []byte, ttl time.Duration) error
	Get(ctx context.Context, id string) (owner string, value []byte, err error)
	Delete(ctx context.Context, id string) error
}

// maxSweep bounds the expired entries one Put removes, so a Put never stalls on a large backlog.
const maxSweep = 16

type memoryEntry struct {
	id, owner string
	value     []byte
	deadline  time.Time
}

// memoryStore keeps entries in insertion order. Every entry of one Handles has the same TTL, so
// that is also deadline order: the expired entries are at the front.
type memoryStore struct {
	mu       sync.Mutex
	entries  map[string]*list.Element
	order    *list.List
	limit    int
	now      func() time.Time
	onExpire func(id string)
}

func newMemoryStore(limit int, now func() time.Time, onExpire func(string)) *memoryStore {
	return &memoryStore{entries: map[string]*list.Element{}, order: list.New(), limit: limit, now: now, onExpire: onExpire}
}

func (s *memoryStore) Put(_ context.Context, id, owner string, value []byte, ttl time.Duration) error {
	s.mu.Lock()
	now := s.now()
	var gone []string
	if ttl == 0 {
		el, ok := s.entries[id]
		if ok && s.expired(el, now) {
			gone = append(gone, s.remove(el))
			ok = false
		}
		if ok {
			el.Value.(*memoryEntry).value = value
		}
		s.mu.Unlock()
		s.fire(gone)
		if !ok {
			return ErrHandleNotFound
		}
		return nil
	}
	for range maxSweep {
		front := s.order.Front()
		if front == nil || !s.expired(front, now) {
			break
		}
		gone = append(gone, s.remove(front))
	}
	if el, ok := s.entries[id]; ok {
		s.remove(el)
	}
	if s.limit > 0 && s.order.Len() >= s.limit {
		gone = append(gone, s.remove(s.order.Front()))
	}
	s.entries[id] = s.order.PushBack(&memoryEntry{id: id, owner: owner, value: value, deadline: now.Add(ttl)})
	s.mu.Unlock()
	s.fire(gone)
	return nil
}

func (s *memoryStore) Get(_ context.Context, id string) (string, []byte, error) {
	s.mu.Lock()
	el, ok := s.entries[id]
	if !ok {
		s.mu.Unlock()
		return "", nil, ErrHandleNotFound
	}
	if s.expired(el, s.now()) {
		gone := s.remove(el)
		s.mu.Unlock()
		s.fire([]string{gone})
		return "", nil, ErrHandleNotFound
	}
	e := el.Value.(*memoryEntry)
	s.mu.Unlock()
	return e.owner, e.value, nil
}

func (s *memoryStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	el, ok := s.entries[id]
	if !ok {
		s.mu.Unlock()
		return ErrHandleNotFound
	}
	expired := s.expired(el, s.now())
	s.remove(el)
	s.mu.Unlock()
	if expired {
		s.fire([]string{id})
		return ErrHandleNotFound
	}
	return nil
}

func (s *memoryStore) expired(el *list.Element, now time.Time) bool {
	return !now.Before(el.Value.(*memoryEntry).deadline)
}

func (s *memoryStore) remove(el *list.Element) string {
	id := el.Value.(*memoryEntry).id
	s.order.Remove(el)
	delete(s.entries, id)
	return id
}

// fire runs OnExpire outside the lock, so the callback may use the store.
func (s *memoryStore) fire(ids []string) {
	if s.onExpire == nil {
		return
	}
	for _, id := range ids {
		s.onExpire(id)
	}
}
