package mcp

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"
)

type handleConfig struct {
	ttl      time.Duration
	limit    int
	store    HandleStore
	onExpire func(id string)
	now      func() time.Time
}

type HandleOption func(*handleConfig)

// HandleTTL sets how long a handle lives after Create. Update does not extend it. The default is
// one hour, and NewHandles panics unless d is positive.
func HandleTTL(d time.Duration) HandleOption { return func(c *handleConfig) { c.ttl = d } }

// HandleLimit bounds how many handles the memory store keeps; the oldest is evicted to make room.
// The default is 10,000, and n <= 0 means no bound. Other stores ignore it.
func HandleLimit(n int) HandleOption { return func(c *handleConfig) { c.limit = n } }

// WithHandleStore keeps values in s instead of in memory, for servers with more than one replica.
func WithHandleStore(s HandleStore) HandleOption { return func(c *handleConfig) { c.store = s } }

// OnExpire runs when the memory store drops a handle that expired or was evicted, so resources tied
// to it can be released. It is not called for Delete, and other stores never call it.
func OnExpire(fn func(id string)) HandleOption { return func(c *handleConfig) { c.onExpire = fn } }

// HandleClock replaces time.Now in the memory store, for tests.
func HandleClock(now func() time.Time) HandleOption { return func(c *handleConfig) { c.now = now } }

// Handles keeps values of type T under random IDs that a client passes back on later requests.
// Each value belongs to the principal that created it: any other principal gets
// ErrHandleNotFound, the same as for an ID that never existed.
type Handles[T any] struct {
	ttl   time.Duration
	store HandleStore
}

func NewHandles[T any](opts ...HandleOption) *Handles[T] {
	cfg := handleConfig{ttl: time.Hour, limit: 10_000, now: time.Now}
	for _, o := range opts {
		o(&cfg)
	}
	if cfg.ttl <= 0 {
		panic(fmt.Sprintf("mcp: HandleTTL(%v): the TTL must be positive", cfg.ttl))
	}
	store := cfg.store
	if store == nil {
		store = newMemoryStore(cfg.limit, cfg.now, cfg.onExpire)
	}
	return &Handles[T]{ttl: cfg.ttl, store: store}
}

// Create stores v for the request's principal and returns its new ID: 128 random bits, base64url
// encoded.
func (h *Handles[T]) Create(ctx context.Context, req *Request, v T) (string, error) {
	value, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("mcp: encoding handle value: %w", err)
	}
	var b [16]byte
	_, _ = rand.Read(b[:])
	id := base64.RawURLEncoding.EncodeToString(b[:])
	if err := h.store.Put(ctx, id, req.owner(), value, h.ttl); err != nil {
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
	return h.store.Put(ctx, id, req.owner(), value, 0)
}

func (h *Handles[T]) Delete(ctx context.Context, req *Request, id string) error {
	if _, err := h.owned(ctx, req, id); err != nil {
		return err
	}
	return h.store.Delete(ctx, id)
}

func (h *Handles[T]) owned(ctx context.Context, req *Request, id string) ([]byte, error) {
	owner, value, err := h.store.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if owner != req.owner() {
		return nil, ErrHandleNotFound
	}
	return value, nil
}
