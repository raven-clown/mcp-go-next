// Package jsonrpc holds JSON-RPC 2.0 messages as MCP uses them: one message per frame, no batches.
package jsonrpc

import (
	"bytes"
	"encoding/json"
	"errors"
	"strconv"
)

const Version = "2.0"

const (
	CodeParseError     = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternalError  = -32603
)

// ID keeps the exact bytes the peer sent, so 1.0 or 9007199254740993 comes back unchanged.
type ID struct{ raw json.RawMessage }

func StringID(s string) ID {
	b, _ := json.Marshal(s)
	return ID{raw: b}
}

func Int64ID(n int64) ID {
	return ID{raw: strconv.AppendInt(nil, n, 10)}
}

func (id ID) IsZero() bool { return len(id.raw) == 0 }

func (id ID) String() string { return string(id.raw) }

func (id ID) Raw() json.RawMessage { return bytes.Clone(id.raw) }

func (id ID) MarshalJSON() ([]byte, error) {
	if id.IsZero() {
		return []byte("null"), nil
	}
	return bytes.Clone(id.raw), nil
}

func (id *ID) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if !validID(b) || !json.Valid(b) {
		return errors.New("jsonrpc: id must be a string or a number")
	}
	id.raw = bytes.Clone(b)
	return nil
}

func validID(b []byte) bool {
	if len(b) == 0 {
		return false
	}
	c := b[0]
	return c == '"' || c == '-' || (c >= '0' && c <= '9')
}

type Error struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *Error) Error() string {
	return "jsonrpc: " + strconv.Itoa(e.Code) + ": " + e.Message
}

type Kind int

const (
	KindRequest Kind = iota + 1
	KindNotification
	KindResponse
)

func (k Kind) String() string {
	switch k {
	case KindRequest:
		return "request"
	case KindNotification:
		return "notification"
	case KindResponse:
		return "response"
	}
	return "Kind(" + strconv.Itoa(int(k)) + ")"
}

type Message struct {
	ID     ID
	Method string
	Params json.RawMessage
	Result json.RawMessage
	Error  *Error
}

func (m *Message) Kind() Kind {
	switch {
	case m.Method != "" && !m.ID.IsZero():
		return KindRequest
	case m.Method != "":
		return KindNotification
	default:
		return KindResponse
	}
}

// Encode writes m with "jsonrpc":"2.0" and no trailing newline. Raw fields are compacted.
func Encode(m *Message) ([]byte, error) {
	if m == nil {
		return nil, errors.New("jsonrpc: nil message")
	}
	hasResult, hasError := m.Result != nil, m.Error != nil
	switch {
	case m.Method != "" && (hasResult || hasError):
		return nil, errors.New("jsonrpc: a request cannot carry a result or an error")
	case m.Method == "" && hasResult == hasError:
		return nil, errors.New("jsonrpc: a response needs exactly one of result and error")
	case hasResult && m.ID.IsZero():
		return nil, errors.New("jsonrpc: a result needs an id")
	}

	var buf bytes.Buffer
	buf.Grow(64 + len(m.Params) + len(m.Result))
	buf.WriteString(`{"jsonrpc":"2.0"`)
	if !m.ID.IsZero() {
		buf.WriteString(`,"id":`)
		buf.Write(m.ID.raw)
	}
	if m.Method != "" {
		buf.WriteString(`,"method":`)
		name, err := json.Marshal(m.Method)
		if err != nil {
			return nil, err
		}
		buf.Write(name)
	}
	if m.Params != nil {
		buf.WriteString(`,"params":`)
		start := buf.Len()
		if err := json.Compact(&buf, m.Params); err != nil {
			return nil, errors.New("jsonrpc: params are not valid JSON")
		}
		if c := buf.Bytes()[start]; c != '{' && c != '[' {
			return nil, errors.New("jsonrpc: params must be an object or an array")
		}
	}
	if hasResult {
		buf.WriteString(`,"result":`)
		if err := json.Compact(&buf, m.Result); err != nil {
			return nil, errors.New("jsonrpc: result is not valid JSON")
		}
	}
	if hasError {
		buf.WriteString(`,"error":`)
		e, err := json.Marshal(m.Error)
		if err != nil {
			return nil, errors.New("jsonrpc: error data is not valid JSON")
		}
		buf.Write(e)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}
