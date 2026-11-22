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

// HandleStore keeps handle values for Handles. Values arrive JSON encoded, and nobody modifies a
// value after passing it to Put or receiving it from Get.
//
//   - Put with a positive ttl stores a new entry. Handles never passes a ttl under a millisecond.
//   - Put with ttl 0 replaces the value of an existing entry, keeps its owner and deadline, and
//     ignores the owner argument. It returns ErrHandleNotFound when there is no such entry. In
//     Redis that is SET XX KEEPTTL on one key that holds both owner and value.
//   - Get and Delete return ErrHandleNotFound for a missing or expired entry. A store must return
//     ErrHandleNotFound itself, not its own not-found error, so that missing and foreign IDs look
//     the same. Handles passes every other error through unchanged.
type HandleStore interface {
	Put(ctx context.Context, id, owner string, value []byte, ttl time.Duration) error
	Get(ctx context.Context, id string) (owner string, value []byte, err error)
	Delete(ctx context.Context, id string) error
}

// maxSweep bounds the expired entries one Put removes, so a Put never stalls on a large backlog.
// The expiry timer removes the rest in batches of maxTimerSweep.
const (
	maxSweep      = 16
	maxTimerSweep = 1024
)

type memoryEntry struct {
	id, owner string
	value     []byte
	deadline  time.Time
	byOwner   *list.Element
}

// memoryStore keeps entries in insertion order. Every entry of one Handles has the same TTL, so
// that is also deadline order: the expired entries are at the front. Each owner also has its own
// list, so one owner at its limit evicts only its own oldest entry.
type memoryStore struct {
	mu         sync.Mutex
	entries    map[string]*list.Element
	order      *list.List
	owners     map[string]*list.List
	limit      int
	ownerLimit int
	now        func() time.Time
	onExpire   func(id string)
	timer      *time.Timer
	timerAt    time.Time
}

func newMemoryStore(limit, ownerLimit int, now func() time.Time, onExpire func(string)) *memoryStore {
	return &memoryStore{
		entries:    map[string]*list.Element{},
		order:      list.New(),
		owners:     map[string]*list.List{},
		limit:      limit,
		ownerLimit: ownerLimit,
		now:        now,
		onExpire:   onExpire,
	}
}

func (s *memoryStore) Put(_ context.Context, id, owner string, value []byte, ttl time.Duration) error {
	if ttl < 0 {
		return errors.New("mcp: negative handle TTL")
	}
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
		s.arm(now)
		s.mu.Unlock()
		s.fire(gone)
		if !ok {
			return ErrHandleNotFound
		}
		return nil
	}
	gone = s.sweep(now, maxSweep)
	if el, ok := s.entries[id]; ok {
		s.remove(el)
	}
	if mine := s.owners[owner]; s.ownerLimit > 0 && mine != nil && mine.Len() >= s.ownerLimit {
		gone = append(gone, s.remove(mine.Front().Value.(*list.Element)))
	}
	if s.limit > 0 && s.order.Len() >= s.limit {
		gone = append(gone, s.remove(s.order.Front()))
	}
	e := &memoryEntry{id: id, owner: owner, value: value, deadline: now.Add(ttl)}
	el := s.order.PushBack(e)
	mine := s.owners[owner]
	if mine == nil {
		mine = list.New()
		s.owners[owner] = mine
	}
	e.byOwner = mine.PushBack(el)
	s.entries[id] = el
	s.arm(now)
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
	if now := s.now(); s.expired(el, now) {
		gone := s.remove(el)
		s.arm(now)
		s.mu.Unlock()
		s.fire([]string{gone})
		return "", nil, ErrHandleNotFound
	}
	e := el.Value.(*memoryEntry)
	owner, value := e.owner, e.value
	s.mu.Unlock()
	return owner, value, nil
}

func (s *memoryStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	el, ok := s.entries[id]
	if !ok {
		s.mu.Unlock()
		return ErrHandleNotFound
	}
	now := s.now()
	expired := s.expired(el, now)
	s.remove(el)
	s.arm(now)
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

func (s *memoryStore) sweep(now time.Time, n int) []string {
	var gone []string
	for range n {
		front := s.order.Front()
		if front == nil || !s.expired(front, now) {
			break
		}
		gone = append(gone, s.remove(front))
	}
	return gone
}

func (s *memoryStore) remove(el *list.Element) string {
	e := el.Value.(*memoryEntry)
	s.order.Remove(el)
	delete(s.entries, e.id)
	if mine := s.owners[e.owner]; mine != nil {
		mine.Remove(e.byOwner)
		if mine.Len() == 0 {
			delete(s.owners, e.owner)
		}
	}
	return e.id
}

// arm sets the expiry timer for the oldest entry, so OnExpire runs on time even when nothing else
// touches the store. Without OnExpire, expired entries are left for the next Put or Get.
func (s *memoryStore) arm(now time.Time) {
	if s.onExpire == nil {
		return
	}
	front := s.order.Front()
	if front == nil {
		if s.timer != nil {
			s.timer.Stop()
			s.timerAt = time.Time{}
		}
		return
	}
	at := front.Value.(*memoryEntry).deadline
	if at.Equal(s.timerAt) {
		return
	}
	s.timerAt = at
	if s.timer == nil {
		s.timer = time.AfterFunc(at.Sub(now), s.expireDue)
		return
	}
	s.timer.Reset(at.Sub(now))
}

func (s *memoryStore) expireDue() {
	s.mu.Lock()
	now := s.now()
	gone := s.sweep(now, maxTimerSweep)
	s.timerAt = time.Time{}
	s.arm(now)
	s.mu.Unlock()
	s.fire(gone)
}

// fire runs OnExpire outside the lock, so the callback may use the store. Every callback runs even
// when an earlier one panics; the first panic is raised again afterwards.
func (s *memoryStore) fire(ids []string) {
	if s.onExpire == nil || len(ids) == 0 {
		return
	}
	var first any
	for _, id := range ids {
		func() {
			defer func() {
				if p := recover(); p != nil && first == nil {
					first = p
				}
			}()
			s.onExpire(id)
		}()
	}
	if first != nil {
		panic(first)
	}
}
