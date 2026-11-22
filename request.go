package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"sync/atomic"

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
	// principal is a pointer so servers without auth do not pay for it on every request.
	principal *Principal
	// state is a pointer so middleware can copy a Request.
	state *requestState
}

type requestState struct {
	done atomic.Bool
}

// DecodeParams decodes Params into v with encoding/json, so keys match struct fields without
// regard to case and the last of a repeated key wins. Req.Meta is read the same way. Middleware
// that inspects params should decode them here too, rather than with a case-sensitive reader.
// Missing params decode as an empty object.
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
	if r.state != nil {
		r.state.done.Store(true)
	}
}

func (r *Request) isDone() bool {
	return r.state != nil && r.state.done.Load()
}
