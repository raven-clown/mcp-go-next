package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/raven-clown/mcp-go-next/jsonrpc"
)

// HandleMessage runs one decoded message through h and returns the response to send, or nil for a
// notification, a response, or a request whose ctx ended before h returned. Transports call it for
// every message they read. n receives the request's notifications; header is nil off HTTP.
func HandleMessage(ctx context.Context, h Handler, msg *jsonrpc.Message, n Notifier, header http.Header) (resp *jsonrpc.Message) {
	if msg.Kind() == jsonrpc.KindResponse {
		return nil
	}
	// A Server recovers handler panics itself; this catches the rest, such as a result whose
	// MarshalJSON panics or a Handler that is not a Server.
	defer func() {
		if p := recover(); p != nil {
			logTo(h, fmt.Errorf("mcp: panic serving %s: %v\n%s", msg.Method, p, debug.Stack()))
			resp = nil
			if !msg.ID.IsZero() && ctx.Err() == nil {
				resp = errorResponse(msg.ID, ErrInternal())
			}
		}
	}()
	box := &struct {
		req   Request
		state requestState
	}{}
	req := &box.req
	*req = Request{
		ID:       msg.ID,
		Method:   msg.Method,
		Params:   msg.Params,
		Header:   header,
		notifier: n,
		state:    &box.state,
	}
	if raw, ok := topLevelMember(msg.Params, "_meta"); ok {
		if err := json.Unmarshal(raw, &req.Meta); err != nil {
			if msg.ID.IsZero() {
				return nil
			}
			return errorResponse(msg.ID, ErrInvalidParams("invalid _meta: "+err.Error()))
		}
	}

	res, err := h.Handle(ctx, req)
	req.markDone()
	if msg.ID.IsZero() || ctx.Err() != nil {
		return nil
	}
	if err != nil {
		return errorResponse(msg.ID, asWireError(err))
	}
	body, err := json.Marshal(res)
	if err != nil {
		logTo(h, fmt.Errorf("mcp: %s: encoding result: %w", msg.Method, err))
		return errorResponse(msg.ID, ErrInternal())
	}
	return &jsonrpc.Message{ID: msg.ID, Result: body}
}

// asWireError returns the *Error in err, or an internal error when there is none. A typed nil
// *Error counts as none.
func asWireError(err error) *Error {
	var e *Error
	if errors.As(err, &e) && e != nil {
		return e
	}
	return ErrInternal()
}

type errorLogger interface {
	logError(error)
}

func logTo(h Handler, err error) {
	if l, ok := h.(errorLogger); ok {
		l.logError(err)
	}
}

func errorResponse(id jsonrpc.ID, e *Error) *jsonrpc.Message {
	return &jsonrpc.Message{ID: id, Error: e.wire()}
}

// topLevelMember returns the value of key in the JSON object b, without decoding anything else. It
// runs on every request to find _meta, so it skips values by scanning for their end instead of
// parsing them. Keys match the way encoding/json matches struct fields, without regard to ASCII
// case and with the last match winning, so it finds the same value DecodeParams would put in a
// field tagged with key. It does not check the grammar; a malformed object reports the key as
// missing.
func topLevelMember(b []byte, key string) ([]byte, bool) {
	i := skipSpace(b, 0)
	if i >= len(b) || b[i] != '{' {
		return nil, false
	}
	i = skipSpace(b, i+1)
	if i < len(b) && b[i] == '}' {
		return nil, false
	}
	var value []byte
	found := false
	for {
		if i >= len(b) || b[i] != '"' {
			return nil, false
		}
		keyEnd, ok := skipString(b, i)
		if !ok {
			return nil, false
		}
		match := keyEquals(b[i:keyEnd], key)
		i = skipSpace(b, keyEnd)
		if i >= len(b) || b[i] != ':' {
			return nil, false
		}
		start := skipSpace(b, i+1)
		end, ok := skipValue(b, start)
		if !ok {
			return nil, false
		}
		if match {
			value, found = b[start:end], true
		}
		i = skipSpace(b, end)
		if i >= len(b) {
			return nil, false
		}
		switch b[i] {
		case ',':
			i = skipSpace(b, i+1)
		case '}':
			return value, found
		default:
			return nil, false
		}
	}
}

// keyEquals compares a quoted JSON key with key, ignoring ASCII case, and decodes escapes only when
// the key has any. key must be ASCII with no letter whose Unicode case folding goes beyond ASCII,
// such as k or s, so that this agrees with encoding/json.
func keyEquals(quoted []byte, key string) bool {
	inner := quoted[1 : len(quoted)-1]
	if bytes.IndexByte(inner, '\\') < 0 {
		return asciiEqualFold(inner, key)
	}
	var s string
	return json.Unmarshal(quoted, &s) == nil && asciiEqualFold([]byte(s), key)
}

func asciiEqualFold(b []byte, s string) bool {
	if len(b) != len(s) {
		return false
	}
	for i := range len(b) {
		x, y := b[i], s[i]
		if 'A' <= x && x <= 'Z' {
			x += 'a' - 'A'
		}
		if 'A' <= y && y <= 'Z' {
			y += 'a' - 'A'
		}
		if x != y {
			return false
		}
	}
	return true
}

func skipSpace(b []byte, i int) int {
	for i < len(b) && (b[i] == ' ' || b[i] == '\t' || b[i] == '\n' || b[i] == '\r') {
		i++
	}
	return i
}

// skipString returns the index after the closing quote. It jumps between quotes with
// bytes.IndexByte, which is vectorized, and treats a quote as escaped when an odd number of
// backslashes come right before it.
func skipString(b []byte, i int) (int, bool) {
	j := i + 1
	for j < len(b) {
		k := bytes.IndexByte(b[j:], '"')
		if k < 0 {
			return 0, false
		}
		j += k
		backslashes := 0
		for p := j - 1; p > i && b[p] == '\\'; p-- {
			backslashes++
		}
		if backslashes%2 == 0 {
			return j + 1, true
		}
		j++
	}
	return 0, false
}

// skipValue returns the end of the value at i: strings are skipped as above, containers by
// counting brackets outside strings, and scalars up to the next delimiter.
func skipValue(b []byte, i int) (int, bool) {
	if i >= len(b) {
		return 0, false
	}
	switch b[i] {
	case '"':
		return skipString(b, i)
	case '{', '[':
		depth := 0
		for j := i; j < len(b); j++ {
			switch b[j] {
			case '"':
				end, ok := skipString(b, j)
				if !ok {
					return 0, false
				}
				j = end - 1
			case '{', '[':
				depth++
			case '}', ']':
				depth--
				if depth == 0 {
					return j + 1, true
				}
			}
		}
		return 0, false
	}
	j := i
	for j < len(b) {
		switch b[j] {
		case ',', '}', ']', ' ', '\t', '\n', '\r':
			return j, j > i
		}
		j++
	}
	return j, j > i
}
