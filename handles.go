package mcp

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type handleConfig struct {
	ttl        time.Duration
	limit      int
	ownerLimit int
	store      HandleStore
	onExpire   func(id string)
	now        func() time.Time
}

type HandleOption func(*handleConfig)

// HandleTTL sets how long a handle lives after Create. Update does not extend it. The default is
// one hour, and NewHandles panics for a TTL under a millisecond.
func HandleTTL(d time.Duration) HandleOption { return func(c *handleConfig) { c.ttl = d } }

// HandleLimit bounds how many handles the memory store keeps in all; the oldest is evicted to make
// room. The default is 10,000, and n <= 0 means no bound. Other stores ignore it.
func HandleLimit(n int) HandleOption { return func(c *handleConfig) { c.limit = n } }

// HandleOwnerLimit bounds how many handles one owner keeps in the memory store; that owner's
// oldest is evicted to make room, so one principal cannot push out everyone else's handles. The
// default is 1,000, and n <= 0 means no bound. Other stores ignore it.
func HandleOwnerLimit(n int) HandleOption { return func(c *handleConfig) { c.ownerLimit = n } }

// WithHandleStore keeps values in s instead of in memory, for servers with more than one replica.
func WithHandleStore(s HandleStore) HandleOption { return func(c *handleConfig) { c.store = s } }

// OnExpire runs when the memory store drops a handle that expired or was evicted, so resources tied
// to it can be released. A timer makes it run soon after the deadline even on an idle server. It is
// not called for Delete, and other stores never call it.
func OnExpire(fn func(id string)) HandleOption { return func(c *handleConfig) { c.onExpire = fn } }

// HandleClock replaces time.Now in the memory store, for tests.
func HandleClock(now func() time.Time) HandleOption { return func(c *handleConfig) { c.now = now } }

// ErrNoSubject is returned when a request's principal has no Subject. Such principals cannot be
// told apart, so they cannot own handles.
var ErrNoSubject = errors.New("mcp: principal has no subject")

// Handles keeps values of type T under random IDs that a client passes back on later requests.
// Each value belongs to the principal that created it: any other principal gets
// ErrHandleNotFound, the same as for an ID that never existed. Requests without a principal share
// one anonymous owner, which suits a local stdio server; on HTTP, set a principal so that a leaked
// ID is useless to anyone else.
//
// There is no compare-and-swap: two concurrent read-modify-write cycles on one handle can lose a
// write.
type Handles[T any] struct {
	ttl   time.Duration
	store HandleStore
}

func NewHandles[T any](opts ...HandleOption) *Handles[T] {
	cfg := handleConfig{ttl: time.Hour, limit: 10_000, ownerLimit: 1_000, now: time.Now}
	for _, o := range opts {
		o(&cfg)
	}
	if cfg.ttl < time.Millisecond {
		panic(fmt.Sprintf("mcp: HandleTTL(%v): the TTL must be at least a millisecond", cfg.ttl))
	}
	store := cfg.store
	if store == nil {
		store = newMemoryStore(cfg.limit, cfg.ownerLimit, cfg.now, cfg.onExpire)
	}
	return &Handles[T]{ttl: cfg.ttl, store: store}
}

func handleOwner(req *Request) (string, error) {
	p, ok := req.Principal()
	if !ok {
		return "", nil
	}
	if p.Subject == "" {
		return "", ErrNoSubject
	}
	return p.Owner(), nil
}

// Create stores v for the request's principal and returns its new ID: 128 random bits, base64url
// encoded.
func (h *Handles[T]) Create(ctx context.Context, req *Request, v T) (string, error) {
	owner, err := handleOwner(req)
	if err != nil {
		return "", err
	}
	value, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("mcp: encoding handle value: %w", err)
	}
	var b [16]byte
	_, _ = rand.Read(b[:])
	id := base64.RawURLEncoding.EncodeToString(b[:])
	if err := h.store.Put(ctx, id, owner, value, h.ttl); err != nil {
		return "", err
	}
	return id, nil
}

func (h *Handles[T]) Get(ctx context.Context, req *Request, id string) (T, error) {
	var v T
	value, err := h.owned(ctx, req, id)
	if err != nil {
		return v, err
	}
	if err := json.Unmarshal(value, &v); err != nil {
		return v, fmt.Errorf("mcp: decoding handle value: %w", err)
	}
	return v, nil
}

// Update replaces the value and keeps the deadline Create set.
func (h *Handles[T]) Update(ctx context.Context, req *Request, id string, v T) error {
	if _, err := h.owned(ctx, req, id); err != nil {
		return err
	}
	value, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("mcp: encoding handle value: %w", err)
	}
	return h.store.Put(ctx, id, "", value, 0)
}

func (h *Handles[T]) Delete(ctx context.Context, req *Request, id string) error {
	if _, err := h.owned(ctx, req, id); err != nil {
		return err
	}
	return h.store.Delete(ctx, id)
}

func (h *Handles[T]) owned(ctx context.Context, req *Request, id string) ([]byte, error) {
	want, err := handleOwner(req)
	if err != nil {
		return nil, err
	}
	owner, value, err := h.store.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if owner != want {
		return nil, ErrHandleNotFound
	}
	return value, nil
}
