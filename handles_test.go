package mcp

import (
	"context"
	"encoding/base64"
	"errors"
	"sync"
	"testing"
	"time"
)

type cart struct {
	Items []string `json:"items"`
}

func requestAs(issuer, subject string) *Request {
	req := newTestRequest("tools/call")
	req.SetPrincipal(Principal{Issuer: issuer, Subject: subject})
	return req
}

type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Add(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	c.mu.Unlock()
}

func TestHandleRoundTrip(t *testing.T) {
	ctx := context.Background()
	h := NewHandles[cart]()
	req := requestAs("https://idp.example", "alice")
	id, err := h.Create(ctx, req, cart{Items: []string{"tea"}})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := base64.RawURLEncoding.DecodeString(id)
	if err != nil || len(raw) != 16 {
		t.Fatalf("id %q is not 16 base64url bytes: %v", id, err)
	}
	got, err := h.Get(ctx, req, id)
	if err != nil || len(got.Items) != 1 || got.Items[0] != "tea" {
		t.Fatalf("Get = %+v, %v", got, err)
	}
	if err := h.Update(ctx, req, id, cart{Items: []string{"tea", "milk"}}); err != nil {
		t.Fatal(err)
	}
	if got, _ := h.Get(ctx, req, id); len(got.Items) != 2 {
		t.Fatalf("after Update, Get = %+v", got)
	}
	if err := h.Delete(ctx, req, id); err != nil {
		t.Fatal(err)
	}
	if _, err := h.Get(ctx, req, id); !errors.Is(err, ErrHandleNotFound) {
		t.Fatalf("after Delete, Get error = %v", err)
	}
	if err := h.Update(ctx, req, id, cart{}); !errors.Is(err, ErrHandleNotFound) {
		t.Fatalf("Update of a deleted handle = %v", err)
	}
	if err := h.Delete(ctx, req, id); !errors.Is(err, ErrHandleNotFound) {
		t.Fatalf("Delete of a deleted handle = %v", err)
	}
	other, _ := h.Create(ctx, req, cart{})
	if other == id {
		t.Fatal("two handles got the same id")
	}
}

func TestHandleWithoutPrincipal(t *testing.T) {
	ctx := context.Background()
	h := NewHandles[cart]()
	id, err := h.Create(ctx, newTestRequest("tools/call"), cart{Items: []string{"a"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.Get(ctx, newTestRequest("tools/call"), id); err != nil {
		t.Fatalf("another request without a principal cannot read it: %v", err)
	}
	if _, err := h.Get(ctx, requestAs("https://idp.example", "alice"), id); !errors.Is(err, ErrHandleNotFound) {
		t.Fatalf("a signed-in principal read an anonymous handle: %v", err)
	}
}

func TestHandleForeignOwner(t *testing.T) {
	ctx := context.Background()
	h := NewHandles[cart]()
	a := requestAs("https://idp.example", "alice")
	b := requestAs("https://idp.example", "bob")
	id, err := h.Create(ctx, a, cart{Items: []string{"tea"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.Get(ctx, b, id); !errors.Is(err, ErrHandleNotFound) {
		t.Fatalf("foreign Get = %v", err)
	}
	if err := h.Update(ctx, b, id, cart{}); !errors.Is(err, ErrHandleNotFound) {
		t.Fatalf("foreign Update = %v", err)
	}
	if err := h.Delete(ctx, b, id); !errors.Is(err, ErrHandleNotFound) {
		t.Fatalf("foreign Delete = %v", err)
	}
	got, err := h.Get(ctx, a, id)
	if err != nil || len(got.Items) != 1 {
		t.Fatalf("owner lost the handle: %+v, %v", got, err)
	}
}

func TestHandleExpiry(t *testing.T) {
	ctx := context.Background()
	clock := &fakeClock{now: time.Unix(1_800_000_000, 0)}
	var expired []string
	h := NewHandles[cart](HandleTTL(time.Minute), HandleClock(clock.Now), OnExpire(func(id string) { expired = append(expired, id) }))
	req := requestAs("https://idp.example", "alice")
	id, _ := h.Create(ctx, req, cart{})

	clock.Add(40 * time.Second)
	if err := h.Update(ctx, req, id, cart{Items: []string{"late"}}); err != nil {
		t.Fatal(err)
	}
	clock.Add(30 * time.Second)
	if _, err := h.Get(ctx, req, id); !errors.Is(err, ErrHandleNotFound) {
		t.Fatalf("Get past the TTL = %v; Update must keep the original deadline", err)
	}
	if _, err := h.Get(ctx, req, id); !errors.Is(err, ErrHandleNotFound) {
		t.Fatalf("second Get = %v", err)
	}
	if len(expired) != 1 || expired[0] != id {
		t.Fatalf("OnExpire calls = %v, want [%s]", expired, id)
	}
}

func TestHandleSweepOnPut(t *testing.T) {
	ctx := context.Background()
	clock := &fakeClock{now: time.Unix(1_800_000_000, 0)}
	var mu sync.Mutex
	expired := 0
	h := NewHandles[cart](HandleTTL(time.Minute), HandleClock(clock.Now), OnExpire(func(string) { mu.Lock(); expired++; mu.Unlock() }))
	req := requestAs("https://idp.example", "alice")
	for range 20 {
		if _, err := h.Create(ctx, req, cart{}); err != nil {
			t.Fatal(err)
		}
	}
	clock.Add(2 * time.Minute)
	if _, err := h.Create(ctx, req, cart{}); err != nil {
		t.Fatal(err)
	}
	if expired != 16 {
		t.Fatalf("one Put swept %d expired handles, want 16", expired)
	}
}

func TestMemoryStoreExpiredEntries(t *testing.T) {
	ctx := context.Background()
	clock := &fakeClock{now: time.Unix(1_800_000_000, 0)}
	var expired []string
	s := newMemoryStore(0, clock.Now, func(id string) { expired = append(expired, id) })
	for _, id := range []string{"a", "b"} {
		if err := s.Put(ctx, id, "o", []byte(`1`), time.Minute); err != nil {
			t.Fatal(err)
		}
	}
	clock.Add(time.Minute)
	if err := s.Put(ctx, "a", "o", []byte(`2`), 0); !errors.Is(err, ErrHandleNotFound) {
		t.Fatalf("Put with ttl 0 on an expired entry = %v", err)
	}
	if err := s.Delete(ctx, "b"); !errors.Is(err, ErrHandleNotFound) {
		t.Fatalf("Delete of an expired entry = %v", err)
	}
	if err := s.Delete(ctx, "b"); !errors.Is(err, ErrHandleNotFound) {
		t.Fatalf("Delete of a removed entry = %v", err)
	}
	if err := s.Put(ctx, "c", "o", []byte(`1`), 0); !errors.Is(err, ErrHandleNotFound) {
		t.Fatalf("Put with ttl 0 on a missing entry = %v", err)
	}
	if len(expired) != 2 || expired[0] != "a" || expired[1] != "b" {
		t.Fatalf("OnExpire calls = %v", expired)
	}
}

func TestHandleTTLMustBePositive(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("NewHandles accepted a zero TTL")
		}
	}()
	NewHandles[cart](HandleTTL(0))
}

func TestHandleLimitEvictsOldest(t *testing.T) {
	ctx := context.Background()
	var evicted []string
	h := NewHandles[cart](HandleLimit(3), OnExpire(func(id string) { evicted = append(evicted, id) }))
	req := requestAs("https://idp.example", "alice")
	var ids []string
	for range 4 {
		id, err := h.Create(ctx, req, cart{})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	if _, err := h.Get(ctx, req, ids[0]); !errors.Is(err, ErrHandleNotFound) {
		t.Fatalf("oldest handle still there: %v", err)
	}
	for _, id := range ids[1:] {
		if _, err := h.Get(ctx, req, id); err != nil {
			t.Fatalf("handle %s lost: %v", id, err)
		}
	}
	if len(evicted) != 1 || evicted[0] != ids[0] {
		t.Fatalf("OnExpire calls = %v, want [%s]", evicted, ids[0])
	}
}

func TestHandleConcurrent(t *testing.T) {
	ctx := context.Background()
	h := NewHandles[cart](HandleLimit(1000))
	var wg sync.WaitGroup
	errs := make(chan error, 50)
	for i := range 50 {
		wg.Go(func() {
			req := requestAs("https://idp.example", string(rune('a'+i%26)))
			for range 20 {
				id, err := h.Create(ctx, req, cart{Items: []string{"x"}})
				if err != nil {
					errs <- err
					return
				}
				if _, err := h.Get(ctx, req, id); err != nil {
					errs <- err
					return
				}
				if err := h.Update(ctx, req, id, cart{}); err != nil {
					errs <- err
					return
				}
			}
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
}

type recordingStore struct {
	mu      sync.Mutex
	puts    []storePut
	entries map[string]storePut
}

type storePut struct {
	id, owner string
	value     []byte
	ttl       time.Duration
}

func (s *recordingStore) Put(_ context.Context, id, owner string, value []byte, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.puts = append(s.puts, storePut{id, owner, value, ttl})
	if ttl == 0 {
		e, ok := s.entries[id]
		if !ok {
			return ErrHandleNotFound
		}
		e.value = value
		s.entries[id] = e
		return nil
	}
	s.entries[id] = storePut{id, owner, value, ttl}
	return nil
}

func (s *recordingStore) Get(_ context.Context, id string) (string, []byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.entries[id]
	if !ok {
		return "", nil, ErrHandleNotFound
	}
	return e.owner, e.value, nil
}

func (s *recordingStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.entries[id]; !ok {
		return ErrHandleNotFound
	}
	delete(s.entries, id)
	return nil
}

func TestHandleCustomStore(t *testing.T) {
	ctx := context.Background()
	store := &recordingStore{entries: map[string]storePut{}}
	h := NewHandles[cart](WithHandleStore(store), HandleTTL(30*time.Minute))
	req := requestAs("https://idp.example", "alice")
	id, err := h.Create(ctx, req, cart{Items: []string{"tea"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Update(ctx, req, id, cart{Items: []string{"milk"}}); err != nil {
		t.Fatal(err)
	}
	if len(store.puts) != 2 {
		t.Fatalf("puts = %+v", store.puts)
	}
	first, second := store.puts[0], store.puts[1]
	if first.id != id || first.owner != "19:https://idp.example alice" || string(first.value) != `{"items":["tea"]}` || first.ttl != 30*time.Minute {
		t.Fatalf("Create put = %+v (%s)", first, first.value)
	}
	if second.id != id || string(second.value) != `{"items":["milk"]}` || second.ttl != 0 {
		t.Fatalf("Update put = %+v (%s); it must pass ttl 0 to keep the deadline", second, second.value)
	}
	got, err := h.Get(ctx, req, id)
	if err != nil || got.Items[0] != "milk" {
		t.Fatalf("Get = %+v, %v", got, err)
	}
}

type failingStore struct{ err error }

func (s failingStore) Put(context.Context, string, string, []byte, time.Duration) error { return s.err }
func (s failingStore) Get(context.Context, string) (string, []byte, error)              { return "", nil, s.err }
func (s failingStore) Delete(context.Context, string) error                             { return s.err }

func TestHandleStoreErrors(t *testing.T) {
	ctx := context.Background()
	down := errors.New("store is down")
	h := NewHandles[cart](WithHandleStore(failingStore{down}))
	req := requestAs("https://idp.example", "alice")
	if _, err := h.Create(ctx, req, cart{}); !errors.Is(err, down) {
		t.Fatalf("Create = %v", err)
	}
	if _, err := h.Get(ctx, req, "x"); !errors.Is(err, down) {
		t.Fatalf("Get = %v", err)
	}
	if err := h.Update(ctx, req, "x", cart{}); !errors.Is(err, down) {
		t.Fatalf("Update = %v", err)
	}
	if err := h.Delete(ctx, req, "x"); !errors.Is(err, down) {
		t.Fatalf("Delete = %v", err)
	}
}

func TestHandleBadStoredValue(t *testing.T) {
	ctx := context.Background()
	store := &recordingStore{entries: map[string]storePut{}}
	req := requestAs("https://idp.example", "alice")
	store.entries["bad"] = storePut{id: "bad", owner: req.owner(), value: []byte(`{"items":5}`), ttl: time.Hour}
	h := NewHandles[cart](WithHandleStore(store))
	if _, err := h.Get(ctx, req, "bad"); err == nil || errors.Is(err, ErrHandleNotFound) {
		t.Fatalf("Get of an undecodable value = %v", err)
	}
}

func TestHandleUnencodableValue(t *testing.T) {
	h := NewHandles[func()]()
	if _, err := h.Create(context.Background(), newTestRequest("tools/call"), func() {}); err == nil {
		t.Fatal("Create of a func value succeeded")
	}
}

func BenchmarkHandles(b *testing.B) {
	ctx := context.Background()
	h := NewHandles[cart](HandleLimit(1 << 20))
	req := requestAs("https://idp.example", "alice")
	v := cart{Items: []string{"tea", "milk"}}
	b.ReportAllocs()
	for b.Loop() {
		id, err := h.Create(ctx, req, v)
		if err != nil {
			b.Fatal(err)
		}
		if _, err := h.Get(ctx, req, id); err != nil {
			b.Fatal(err)
		}
		if err := h.Delete(ctx, req, id); err != nil {
			b.Fatal(err)
		}
	}
}
