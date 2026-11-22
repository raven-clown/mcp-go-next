package mcp

import (
	"encoding/json"
	"strconv"

	"github.com/raven-clown/mcp-go-next/jsonrpc"
	"github.com/raven-clown/mcp-go-next/protocol"
)

// Error is a JSON-RPC error a handler returns to the client as is. Any other error a handler returns
// is logged and reaches the client as an internal error, so its text never leaks.
type Error struct {
	Code    int
	Message string
	Data    any
}

func (e *Error) Error() string {
	return "mcp: " + strconv.Itoa(e.Code) + ": " + e.Message
}

// wire converts e to its JSON-RPC form. Data that cannot be encoded is left out rather than failing.
func (e *Error) wire() *jsonrpc.Error {
	w := &jsonrpc.Error{Code: e.Code, Message: e.Message}
	if e.Data != nil {
		if b, err := json.Marshal(e.Data); err == nil {
			w.Data = b
		}
	}
	return w
}

func ErrInvalidParams(msg string) *Error {
	return &Error{Code: jsonrpc.CodeInvalidParams, Message: msg}
}

func ErrMethodNotFound(method string) *Error {
	return &Error{Code: jsonrpc.CodeMethodNotFound, Message: "method not found: " + method}
}

func ErrInternal() *Error {
	return &Error{Code: jsonrpc.CodeInternalError, Message: "internal error"}
}

// ErrResourceNotFound uses -32602, as 2026-07-28 does, with the URI in data.
func ErrResourceNotFound(uri string) *Error {
	return &Error{Code: jsonrpc.CodeInvalidParams, Message: "resource not found", Data: map[string]string{"uri": uri}}
}

func ErrHeaderMismatch(msg string) *Error {
	return &Error{Code: protocol.CodeHeaderMismatch, Message: msg}
}

func ErrMissingCapability(required protocol.ClientCapabilities) *Error {
	return &Error{
		Code:    protocol.CodeMissingRequiredClientCapability,
		Message: "missing required client capability",
		Data:    protocol.MissingCapabilityData{RequiredCapabilities: required},
	}
}

func ErrUnsupportedVersion(requested string, supported []string) *Error {
	return &Error{
		Code:    protocol.CodeUnsupportedProtocolVersion,
		Message: "unsupported protocol version",
		Data:    protocol.UnsupportedVersionData{Supported: supported, Requested: requested},
	}
}
