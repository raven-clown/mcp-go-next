package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/raven-clown/mcp-go-next/jsonschema"
	"github.com/raven-clown/mcp-go-next/protocol"
)

// Info names a server or client implementation.
type Info = protocol.Implementation

type serverConfig struct {
	instructions string
	cacheTTL     time.Duration
	cacheScope   protocol.CacheScope
	errorLog     func(error)
	pageSize     int
	validator    jsonschema.Validator
	extensions   map[string]protocol.Object
}

type ServerOption func(*serverConfig)

// WithInstructions sets the guidance server/discover returns for the model.
func WithInstructions(s string) ServerOption { return func(c *serverConfig) { c.instructions = s } }

// WithCache sets ttlMs and cacheScope on list and read results that do not set their own.
// The default is 0 and "private".
func WithCache(ttl time.Duration, scope protocol.CacheScope) ServerOption {
	return func(c *serverConfig) { c.cacheTTL, c.cacheScope = ttl, scope }
}

// WithErrorLog receives panics, with their stack, and handler errors that are not *Error.
func WithErrorLog(fn func(error)) ServerOption { return func(c *serverConfig) { c.errorLog = fn } }

// WithPageSize sets how many items a list returns per page. The default is 100.
func WithPageSize(n int) ServerOption { return func(c *serverConfig) { c.pageSize = n } }

// WithValidator sets how tool arguments are validated. The default is jsonschema.Basic().
func WithValidator(v jsonschema.Validator) ServerOption {
	return func(c *serverConfig) { c.validator = v }
}

// WithExtension declares an extension in the server's capabilities. settings is encoded as JSON.
func WithExtension(id string, settings any) ServerOption {
	return func(c *serverConfig) {
		obj := protocol.Object{}
		if settings != nil {
			b, err := json.Marshal(settings)
			if err == nil {
				_ = json.Unmarshal(b, &obj)
			}
		}
		if c.extensions == nil {
			c.extensions = map[string]protocol.Object{}
		}
		c.extensions[id] = obj
	}
}

// Server dispatches requests to method handlers through middleware. It holds no per-connection state.
type Server struct {
	info       Info
	serverInfo json.RawMessage
	cfg        serverConfig

	mu         sync.Mutex
	middleware []Middleware
	// methods and chain are swapped whole on change, so requests read them without locks.
	methods atomic.Pointer[map[string]Handler]
	chain   atomic.Pointer[Handler]
}

func NewServer(info Info, opts ...ServerOption) *Server {
	cfg := serverConfig{cacheScope: protocol.CachePrivate, pageSize: 100}
	for _, o := range opts {
		o(&cfg)
	}
	if cfg.validator == nil {
		cfg.validator = jsonschema.Basic()
	}
	s := &Server{info: info, cfg: cfg}
	s.serverInfo, _ = json.Marshal(info)
	empty := map[string]Handler{}
	s.methods.Store(&empty)
	s.rebuild()
	return s
}

// Use adds middleware. The first one added runs first.
func (s *Server) Use(mw ...Middleware) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.middleware = append(s.middleware, mw...)
	s.rebuild()
}

// rebuild wraps the dispatcher in the middleware. Callers hold s.mu, or own s exclusively.
func (s *Server) rebuild() {
	var h Handler = HandlerFunc(s.dispatch)
	for i := len(s.middleware) - 1; i >= 0; i-- {
		h = s.middleware[i](h)
	}
	s.chain.Store(&h)
}

// reserved are method names the spec defines. HandleMethod refuses them so a custom handler cannot
// replace protocol behavior by accident.
var reserved = map[string]bool{
	"initialize": true, "ping": true, "logging/setLevel": true,
	protocol.MethodDiscover: true, protocol.MethodToolsList: true, protocol.MethodToolsCall: true,
	protocol.MethodResourcesList: true, protocol.MethodResourceTemplatesList: true, protocol.MethodResourcesRead: true,
	protocol.MethodPromptsList: true, protocol.MethodPromptsGet: true, protocol.MethodComplete: true,
	protocol.MethodSubscriptionsListen: true, protocol.MethodElicit: true, protocol.MethodSample: true,
	protocol.MethodRoots: true, protocol.NotifyCancelled: true, protocol.NotifyProgress: true,
	protocol.NotifyMessage: true, protocol.NotifyToolsListChanged: true, protocol.NotifyPromptsListChanged: true,
	protocol.NotifyResourcesListChanged: true, protocol.NotifyResourceUpdated: true,
	protocol.NotifySubscriptionsAcknowledged: true,
}

// HandleMethod registers a handler for a method the spec does not define, such as an extension
// method. It panics for a spec method, since that would replace protocol behavior.
func (s *Server) HandleMethod(name string, h Handler) {
	if reserved[name] {
		panic(fmt.Sprintf("mcp: HandleMethod(%q): the spec defines this method", name))
	}
	s.handle(name, h)
}

// handle registers a handler without the reserved-name check, for the server's own methods.
func (s *Server) handle(name string, h Handler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	old := *s.methods.Load()
	next := make(map[string]Handler, len(old)+1)
	for k, v := range old {
		next[k] = v
	}
	next[name] = h
	s.methods.Store(&next)
}

func (s *Server) dispatch(ctx context.Context, req *Request) (protocol.Result, error) {
	h, ok := (*s.methods.Load())[req.Method]
	if !ok {
		return nil, ErrMethodNotFound(req.Method)
	}
	return h.Handle(ctx, req)
}

// Handle checks the protocol version, runs the middleware and the method handler, and fills in
// resultType, server info and cache fields. Panics become internal errors and are logged.
func (s *Server) Handle(ctx context.Context, req *Request) (res protocol.Result, err error) {
	defer func() {
		if p := recover(); p != nil {
			s.logError(fmt.Errorf("mcp: panic serving %s: %v\n%s", req.Method, p, debug.Stack()))
			res, err = nil, ErrInternal()
		}
	}()
	// Notifications carry no protocol version, so only requests are checked.
	if !req.ID.IsZero() && req.Meta.ProtocolVersion != protocol.LatestVersion {
		return nil, ErrUnsupportedVersion(req.Meta.ProtocolVersion, []string{protocol.LatestVersion})
	}
	res, err = (*s.chain.Load()).Handle(ctx, req)
	if err != nil {
		var e *Error
		if !errors.As(err, &e) && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			s.logError(fmt.Errorf("mcp: %s: %w", req.Method, err))
			err = ErrInternal()
		}
		return nil, err
	}
	if res == nil {
		res = &protocol.EmptyResult{}
	}
	s.stamp(res)
	return res, nil
}

// stamp fills the fields every result carries. Handlers return fresh results, so this writes to
// values no other request holds.
func (s *Server) stamp(res protocol.Result) {
	base := res.Base()
	if base.ResultType == "" {
		base.ResultType = protocol.ResultComplete
	}
	if _, ok := base.Meta[protocol.MetaServerInfo]; !ok {
		meta := make(protocol.Object, len(base.Meta)+1)
		for k, v := range base.Meta {
			meta[k] = v
		}
		meta[protocol.MetaServerInfo] = s.serverInfo
		base.Meta = meta
	}
	if c, ok := res.(protocol.Cacheable); ok {
		if f := c.Cache(); f.CacheScope == "" {
			f.TTLMs = s.cfg.cacheTTL.Milliseconds()
			f.CacheScope = s.cfg.cacheScope
		}
	}
}

func (s *Server) logError(err error) {
	if s.cfg.errorLog != nil {
		s.cfg.errorLog(err)
	}
}
