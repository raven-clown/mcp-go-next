package mcp

import (
	"context"
	"time"

	"github.com/raven-clown/mcp-go-next/protocol"
)

// Observer sees every message a Server handles, requests and notifications alike, for metrics
// and tracing. OnEnd gets what the client will get: the stamped result, or the error after panics
// and unexpected errors became internal errors. Both run on the request's goroutine, so keep them
// fast. A panic in either is logged and does not change the response.
//
// OnStart runs before middleware, so the principal is not set yet. OnEnd sees it when auth
// middleware called SetPrincipal on the same *Request, not on a copy.
type Observer interface {
	OnStart(ctx context.Context, req *Request)
	OnEnd(ctx context.Context, req *Request, res protocol.Result, err error, d time.Duration)
}

// WithObserver adds o. Observers run in the order they were added.
func WithObserver(o Observer) ServerOption {
	return func(c *serverConfig) { c.observers = append(c.observers, o) }
}
