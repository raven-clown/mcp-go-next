package mcp

import (
	"context"

	"github.com/raven-clown/mcp-go-next/protocol"
)

// Handler answers one request. A Server is a Handler, and so is every middleware layer around it.
// Handle returns a fresh result on every call: the Server writes resultType, _meta and cache
// fields into it before encoding.
type Handler interface {
	Handle(ctx context.Context, req *Request) (protocol.Result, error)
}

type HandlerFunc func(ctx context.Context, req *Request) (protocol.Result, error)

func (f HandlerFunc) Handle(ctx context.Context, req *Request) (protocol.Result, error) {
	return f(ctx, req)
}

// Middleware wraps a Handler, the way net/http middleware wraps an http.Handler.
type Middleware func(Handler) Handler
