package mcp

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/raven-clown/mcp-go-next/protocol"
)

type observed struct {
	method string
	res    protocol.Result
	err    error
	d      time.Duration
}

type recordingObserver struct {
	mu     sync.Mutex
	starts []string
	ends   []observed
}

func (o *recordingObserver) OnStart(_ context.Context, req *Request) {
	o.mu.Lock()
	o.starts = append(o.starts, req.Method)
	o.mu.Unlock()
}

func (o *recordingObserver) OnEnd(_ context.Context, req *Request, res protocol.Result, err error, d time.Duration) {
	o.mu.Lock()
	o.ends = append(o.ends, observed{req.Method, res, err, d})
	o.mu.Unlock()
}

func TestObserver(t *testing.T) {
	o := &recordingObserver{}
	s := NewServer(Info{Name: "t", Version: "1"}, WithObserver(o))
	s.HandleMethod("x.example/echo", echoHandler(nil))
	s.HandleMethod("x.example/boom", HandlerFunc(func(ctx context.Context, req *Request) (protocol.Result, error) {
		panic("boom")
	}))
	ctx := context.Background()
	HandleMessage(ctx, s, requestMessage(t, 1, "x.example/echo", `{}`), nil, nil)
	HandleMessage(ctx, s, requestMessage(t, 2, "x.example/missing", `{}`), nil, nil)
	HandleMessage(ctx, s, requestMessage(t, 3, "x.example/boom", `{}`), nil, nil)

	want := []string{"x.example/echo", "x.example/missing", "x.example/boom"}
	if len(o.starts) != 3 || len(o.ends) != 3 {
		t.Fatalf("starts = %v, ends = %d", o.starts, len(o.ends))
	}
	for i, m := range want {
		if o.starts[i] != m || o.ends[i].method != m {
			t.Fatalf("call %d: start %q, end %q, want %q", i, o.starts[i], o.ends[i].method, m)
		}
	}
	if o.ends[0].err != nil || o.ends[0].res == nil || o.ends[0].res.Base().ResultType != protocol.ResultComplete {
		t.Fatalf("echo end = %+v; the observer sees the stamped result", o.ends[0])
	}
	if e := asError(t, o.ends[1].err); e.Code != -32601 {
		t.Fatalf("missing method end error = %v", e)
	}
	if e := asError(t, o.ends[2].err); e.Code != -32603 {
		t.Fatalf("panic end error = %v", e)
	}
}

type panickingObserver struct{}

func (panickingObserver) OnStart(context.Context, *Request) { panic("start") }
func (panickingObserver) OnEnd(context.Context, *Request, protocol.Result, error, time.Duration) {
	panic("end")
}

func TestObserverPanicIsLogged(t *testing.T) {
	var logged []error
	o := &recordingObserver{}
	s := NewServer(Info{Name: "t", Version: "1"},
		WithErrorLog(func(err error) { logged = append(logged, err) }),
		WithObserver(panickingObserver{}), WithObserver(o))
	called := false
	s.HandleMethod("x.example/echo", echoHandler(&called))
	resp := HandleMessage(context.Background(), s, requestMessage(t, 1, "x.example/echo", `{}`), nil, nil)
	if resp == nil || resp.Error != nil || !called {
		t.Fatalf("response = %+v, handler called = %v", resp, called)
	}
	if len(o.starts) != 1 || len(o.ends) != 1 {
		t.Fatalf("the second observer saw starts %v, ends %d", o.starts, len(o.ends))
	}
	if len(logged) != 2 {
		t.Fatalf("logged %d errors, want 2", len(logged))
	}
}

func TestObserverSeesVersionErrors(t *testing.T) {
	o := &recordingObserver{}
	s := NewServer(Info{Name: "t", Version: "1"}, WithObserver(o), WithObserver(o))
	req := newTestRequest("x.example/echo")
	req.ID = idOf(1)
	req.Meta.ProtocolVersion = "2025-11-25"
	_, err := s.Handle(context.Background(), req)
	if e := asError(t, err); e.Code != -32022 {
		t.Fatalf("error = %v", e)
	}
	if len(o.starts) != 2 || len(o.ends) != 2 || o.ends[0].err != err {
		t.Fatalf("two observers should each see the request once: starts %v, ends %+v", o.starts, o.ends)
	}
}
