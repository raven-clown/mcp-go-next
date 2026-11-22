package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"

	"github.com/raven-clown/mcp-go-next/jsonrpc"
	"github.com/raven-clown/mcp-go-next/protocol"
)

// Notifier sends notifications that belong to one request, ahead of its response.
type Notifier interface {
	Notify(ctx context.Context, msg *jsonrpc.Message) error
}

// Request is one message as a handler sees it. Transports build it through HandleMessage.
type Request struct {
	ID     jsonrpc.ID
	Method string
	// Params stays raw until a handler decodes it, so large arguments are parsed once.
	Params json.RawMessage
	Meta   protocol.RequestMeta
	// Header holds the HTTP request headers on HTTP transports and is nil on stdio.
	Header http.Header

	notifier Notifier
	mu       sync.Mutex
	done     bool
}

// DecodeParams decodes Params into v. Missing params decode as an empty object.
func (r *Request) DecodeParams(v any) error {
	if len(r.Params) == 0 {
		return nil
	}
	if err := json.Unmarshal(r.Params, v); err != nil {
		return ErrInvalidParams("invalid params: " + err.Error())
	}
	return nil
}

// markDone records that the response is being written. Notifications after this are refused.
func (r *Request) markDone() {
	r.mu.Lock()
	r.done = true
	r.mu.Unlock()
}

func (r *Request) isDone() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.done
}
