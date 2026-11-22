package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/raven-clown/mcp-go-next/jsonschema"
	"github.com/raven-clown/mcp-go-next/protocol"
)

func newTestRequest(method string) *Request {
	return &Request{
		Method: method,
		Meta:   protocol.RequestMeta{ProtocolVersion: protocol.LatestVersion},
	}
}

func echoHandler(called *bool) Handler {
	return HandlerFunc(func(ctx context.Context, req *Request) (protocol.Result, error) {
		if called != nil {
			*called = true
		}
		return &protocol.EmptyResult{}, nil
	})
}

func asError(t *testing.T, err error) *Error {
	t.Helper()
	var e *Error
	if !errors.As(err, &e) {
		t.Fatalf("error %v (%T) is not *mcp.Error", err, err)
	}
	return e
}

func TestHandleVersion(t *testing.T) {
	tests := []struct {
		name     string
		version  string
		wantCode int
	}{
		{"missing", "", protocol.CodeUnsupportedProtocolVersion},
		{"older", "2025-11-25", protocol.CodeUnsupportedProtocolVersion},
		{"current", protocol.LatestVersion, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewServer(Info{Name: "t", Version: "1"})
			called := false
			s.HandleMethod("x.example/echo", echoHandler(&called))
			req := newTestRequest("x.example/echo")
			req.ID = idOf(1)
			req.Meta.ProtocolVersion = tt.version
			_, err := s.Handle(context.Background(), req)
			if tt.wantCode == 0 {
				if err != nil || !called {
					t.Fatalf("err = %v, called = %v", err, called)
				}
				return
			}
			e := asError(t, err)
			if e.Code != tt.wantCode || called {
				t.Fatalf("code = %d, called = %v", e.Code, called)
			}
			data, _ := json.Marshal(e.Data)
			want := `{"supported":["2026-07-28"],"requested":"` + tt.version + `"}`
			if string(data) != want {
				t.Fatalf("data = %s, want %s", data, want)
			}
		})
	}
}

func TestUnknownMethod(t *testing.T) {
	s := NewServer(Info{Name: "t", Version: "1"})
	req := newTestRequest("no/such")
	req.ID = idOf(1)
	_, err := s.Handle(context.Background(), req)
	if e := asError(t, err); e.Code != -32601 {
		t.Fatalf("code = %d", e.Code)
	}
}

func TestPanicRecovered(t *testing.T) {
	var logged []error
	var mu sync.Mutex
	s := NewServer(Info{Name: "t", Version: "1"}, WithErrorLog(func(err error) {
		mu.Lock()
		logged = append(logged, err)
		mu.Unlock()
	}))
	s.HandleMethod("x.example/panic", HandlerFunc(func(ctx context.Context, req *Request) (protocol.Result, error) {
		panic("boom")
	}))
	msg := requestMessage(t, 1, "x.example/panic", `{}`)
	resp := HandleMessage(context.Background(), s, msg, nil, nil)
	if resp == nil || resp.Error == nil {
		t.Fatalf("response = %+v", resp)
	}
	if resp.Error.Message != "internal error" || strings.Contains(string(mustEncode(t, resp)), "boom") {
		t.Fatalf("panic leaked to the client: %s", mustEncode(t, resp))
	}
	if len(logged) != 1 || !strings.Contains(logged[0].Error(), "boom") {
		t.Fatalf("error log = %v", logged)
	}
}

func TestPanicInMiddlewareRecovered(t *testing.T) {
	s := NewServer(Info{Name: "t", Version: "1"}, WithErrorLog(func(error) {}))
	s.HandleMethod("x.example/echo", echoHandler(nil))
	s.Use(func(next Handler) Handler {
		return HandlerFunc(func(ctx context.Context, req *Request) (protocol.Result, error) {
			panic("middleware")
		})
	})
	req := newTestRequest("x.example/echo")
	req.ID = idOf(1)
	if _, err := s.Handle(context.Background(), req); asError(t, err).Code != -32603 {
		t.Fatalf("err = %v", err)
	}
}

func TestOtherErrorsBecomeInternal(t *testing.T) {
	var logged error
	s := NewServer(Info{Name: "t", Version: "1"}, WithErrorLog(func(err error) { logged = err }))
	s.HandleMethod("x.example/fail", HandlerFunc(func(ctx context.Context, req *Request) (protocol.Result, error) {
		return nil, errors.New("database password is hunter2")
	}))
	resp := HandleMessage(context.Background(), s, requestMessage(t, 1, "x.example/fail", `{}`), nil, nil)
	if resp.Error == nil || resp.Error.Code != -32603 || strings.Contains(string(mustEncode(t, resp)), "hunter2") {
		t.Fatalf("response = %s", mustEncode(t, resp))
	}
	if logged == nil || !strings.Contains(logged.Error(), "hunter2") {
		t.Fatalf("error log = %v", logged)
	}
}

func TestMiddlewareOrder(t *testing.T) {
	var order []string
	mark := func(name string) Middleware {
		return func(next Handler) Handler {
			return HandlerFunc(func(ctx context.Context, req *Request) (protocol.Result, error) {
				order = append(order, name)
				return next.Handle(ctx, req)
			})
		}
	}
	s := NewServer(Info{Name: "t", Version: "1"})
	s.HandleMethod("x.example/echo", HandlerFunc(func(ctx context.Context, req *Request) (protocol.Result, error) {
		order = append(order, "handler")
		return &protocol.EmptyResult{}, nil
	}))
	s.Use(mark("a"), mark("b"))
	s.Use(mark("c"))
	req := newTestRequest("x.example/echo")
	req.ID = idOf(1)
	if _, err := s.Handle(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(order, ","); got != "a,b,c,handler" {
		t.Fatalf("order = %s", got)
	}
}

func TestResultStamping(t *testing.T) {
	s := NewServer(Info{Name: "srv", Version: "2"})
	s.HandleMethod("x.example/list", HandlerFunc(func(ctx context.Context, req *Request) (protocol.Result, error) {
		return &protocol.ListToolsResult{}, nil
	}))
	s.HandleMethod("x.example/nil", HandlerFunc(func(ctx context.Context, req *Request) (protocol.Result, error) {
		return nil, nil
	}))
	s.HandleMethod("x.example/own", HandlerFunc(func(ctx context.Context, req *Request) (protocol.Result, error) {
		return &protocol.ListToolsResult{CacheFields: protocol.CacheFields{TTLMs: 5000, CacheScope: protocol.CachePublic}}, nil
	}))
	tests := []struct {
		method string
		want   string
	}{
		{"x.example/list", `{"_meta":{"io.modelcontextprotocol/serverInfo":{"name":"srv","version":"2"}},"resultType":"complete","ttlMs":0,"cacheScope":"private","tools":[]}`},
		{"x.example/nil", `{"_meta":{"io.modelcontextprotocol/serverInfo":{"name":"srv","version":"2"}},"resultType":"complete"}`},
		{"x.example/own", `{"_meta":{"io.modelcontextprotocol/serverInfo":{"name":"srv","version":"2"}},"resultType":"complete","ttlMs":5000,"cacheScope":"public","tools":[]}`},
	}
	for _, tt := range tests {
		resp := HandleMessage(context.Background(), s, requestMessage(t, 1, tt.method, `{}`), nil, nil)
		if resp.Error != nil {
			t.Fatalf("%s: %v", tt.method, resp.Error)
		}
		assertJSON(t, tt.want, resp.Result)
	}
}

func TestWithCache(t *testing.T) {
	s := NewServer(Info{Name: "s", Version: "1"}, WithCache(90*time.Second, protocol.CachePublic))
	s.HandleMethod("x.example/list", HandlerFunc(func(ctx context.Context, req *Request) (protocol.Result, error) {
		return &protocol.ListToolsResult{}, nil
	}))
	resp := HandleMessage(context.Background(), s, requestMessage(t, 1, "x.example/list", `{}`), nil, nil)
	var r protocol.ListToolsResult
	if err := json.Unmarshal(resp.Result, &r); err != nil {
		t.Fatal(err)
	}
	if r.TTLMs != 90_000 || r.CacheScope != protocol.CachePublic {
		t.Fatalf("cache = %+v", r.CacheFields)
	}
}

func TestServerOptions(t *testing.T) {
	v := jsonschema.Basic()
	s := NewServer(Info{Name: "s", Version: "1"},
		WithInstructions("use the search tool first"),
		WithPageSize(25),
		WithValidator(v),
		WithExtension("x.example/tasks", map[string]any{"poll": true}),
		WithExtension("x.example/bare", nil),
	)
	if s.cfg.instructions != "use the search tool first" || s.cfg.pageSize != 25 || s.cfg.validator != v {
		t.Fatalf("cfg = %+v", s.cfg)
	}
	if got := s.cfg.extensions["x.example/tasks"]; len(got) != 1 || string(got["poll"]) != "true" {
		t.Fatalf("tasks extension = %v", got)
	}
	if got, ok := s.cfg.extensions["x.example/bare"]; !ok || len(got) != 0 {
		t.Fatalf("bare extension = %v, %v", got, ok)
	}
}

func TestHandleMethodCustom(t *testing.T) {
	s := NewServer(Info{Name: "t", Version: "1"})
	var seen []string
	s.Use(func(next Handler) Handler {
		return HandlerFunc(func(ctx context.Context, req *Request) (protocol.Result, error) {
			seen = append(seen, req.Method)
			return next.Handle(ctx, req)
		})
	})
	called := false
	s.HandleMethod("x.example/echo", echoHandler(&called))
	resp := HandleMessage(context.Background(), s, requestMessage(t, 1, "x.example/echo", `{}`), nil, nil)
	if resp.Error != nil || !called || len(seen) != 1 {
		t.Fatalf("resp = %+v, called = %v, middleware saw %v", resp.Error, called, seen)
	}
	for _, reserved := range []string{protocol.MethodToolsList, protocol.MethodToolsCall, protocol.MethodDiscover, protocol.NotifyCancelled, "initialize"} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("HandleMethod(%q) did not panic", reserved)
				}
			}()
			s.HandleMethod(reserved, echoHandler(nil))
		}()
	}
}

func TestConcurrentSetupAndServe(t *testing.T) {
	s := NewServer(Info{Name: "t", Version: "1"})
	s.HandleMethod("x.example/echo", echoHandler(nil))
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			s.Use(func(next Handler) Handler { return next })
		}()
		go func() {
			defer wg.Done()
			req := newTestRequest("x.example/echo")
			req.ID = idOf(1)
			if _, err := s.Handle(context.Background(), req); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
}

func TestErrorConstructors(t *testing.T) {
	tests := []struct {
		err  *Error
		want string
	}{
		{ErrInvalidParams("bad cursor"), `{"code":-32602,"message":"bad cursor"}`},
		{ErrMethodNotFound("x/y"), `{"code":-32601,"message":"method not found: x/y"}`},
		{ErrInternal(), `{"code":-32603,"message":"internal error"}`},
		{ErrResourceNotFound("file:///a"), `{"code":-32602,"message":"resource not found","data":{"uri":"file:///a"}}`},
		{ErrHeaderMismatch("Mcp-Name does not match"), `{"code":-32020,"message":"Mcp-Name does not match"}`},
		{ErrMissingCapability(protocol.ClientCapabilities{Elicitation: &protocol.ElicitationCapability{}}), `{"code":-32021,"message":"missing required client capability","data":{"requiredCapabilities":{"elicitation":{}}}}`},
		{ErrUnsupportedVersion("1999-01-01", []string{"2026-07-28"}), `{"code":-32022,"message":"unsupported protocol version","data":{"supported":["2026-07-28"],"requested":"1999-01-01"}}`},
	}
	for _, tt := range tests {
		got, err := json.Marshal(tt.err.wire())
		if err != nil {
			t.Fatal(err)
		}
		assertJSON(t, tt.want, got)
	}
	if got := ErrInvalidParams("x").Error(); got != "mcp: -32602: x" {
		t.Errorf("Error() = %q", got)
	}
}

func TestDecodeParams(t *testing.T) {
	req := &Request{Params: json.RawMessage(`{"name":"x","n":1}`)}
	var p struct {
		Name string `json:"name"`
		N    int    `json:"n"`
	}
	if err := req.DecodeParams(&p); err != nil || p.Name != "x" || p.N != 1 {
		t.Fatalf("p = %+v, err = %v", p, err)
	}
	req.Params = json.RawMessage(`{"n":"one"}`)
	if e := asError(t, req.DecodeParams(&p)); e.Code != -32602 {
		t.Fatalf("code = %d", e.Code)
	}
	req.Params = nil
	if err := req.DecodeParams(&p); err != nil {
		t.Fatalf("missing params: %v", err)
	}
}
