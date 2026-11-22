package jsonrpc

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestDecodeKinds(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want Kind
	}{
		{"request", `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`, KindRequest},
		{"request without params", `{"jsonrpc":"2.0","id":"a","method":"ping"}`, KindRequest},
		{"notification", `{"jsonrpc":"2.0","method":"notifications/progress","params":{"progress":1}}`, KindNotification},
		{"result response", `{"jsonrpc":"2.0","id":1,"result":{"resultType":"complete"}}`, KindResponse},
		{"error response", `{"jsonrpc":"2.0","id":1,"error":{"code":-32601,"message":"method not found"}}`, KindResponse},
		{"error response with null id", `{"jsonrpc":"2.0","id":null,"error":{"code":-32700,"message":"parse error"}}`, KindResponse},
		{"error response without id", `{"jsonrpc":"2.0","error":{"code":-32700,"message":"parse error"}}`, KindResponse},
		{"leading whitespace", " \n\t{\"jsonrpc\":\"2.0\",\"method\":\"a\"}\n", KindNotification},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := Decode([]byte(tt.in))
			if err != nil {
				t.Fatalf("Decode: %v", err)
			}
			if got := m.Kind(); got != tt.want {
				t.Fatalf("Kind() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDecodeFields(t *testing.T) {
	m, err := Decode([]byte(`{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"x"}}`))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if m.Method != "tools/call" {
		t.Errorf("Method = %q", m.Method)
	}
	if string(m.Params) != `{"name":"x"}` {
		t.Errorf("Params = %s", m.Params)
	}
	if m.ID.String() != "7" {
		t.Errorf("ID = %s", m.ID)
	}

	m, err = Decode([]byte(`{"jsonrpc":"2.0","id":7,"error":{"code":-32602,"message":"bad","data":{"uri":"x"}}}`))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if m.Error == nil || m.Error.Code != CodeInvalidParams || m.Error.Message != "bad" || string(m.Error.Data) != `{"uri":"x"}` {
		t.Errorf("Error = %+v", m.Error)
	}
}

func TestDecodeRejects(t *testing.T) {
	tests := []struct {
		name string
		in   string
		code int
	}{
		{"batch", `[{}]`, CodeInvalidRequest},
		{"batch of requests", `[{"jsonrpc":"2.0","id":1,"method":"a"}]`, CodeInvalidRequest},
		{"trailing data", `{"jsonrpc":"2.0","method":"a"}{}`, CodeParseError},
		{"bad json", `{"jsonrpc":`, CodeParseError},
		{"empty", ``, CodeParseError},
		{"not an object", `42`, CodeInvalidRequest},
		{"missing jsonrpc", `{"id":1,"method":"a"}`, CodeInvalidRequest},
		{"jsonrpc 1.0", `{"jsonrpc":"1.0","id":1,"method":"a"}`, CodeInvalidRequest},
		{"jsonrpc not a string", `{"jsonrpc":2,"id":1,"method":"a"}`, CodeInvalidRequest},
		{"result and error", `{"jsonrpc":"2.0","id":1,"result":{},"error":{"code":1,"message":"x"}}`, CodeInvalidRequest},
		{"neither method nor result nor error", `{"jsonrpc":"2.0","id":1}`, CodeInvalidRequest},
		{"method and result", `{"jsonrpc":"2.0","id":1,"method":"a","result":{}}`, CodeInvalidRequest},
		{"object id", `{"jsonrpc":"2.0","id":{},"method":"a"}`, CodeInvalidRequest},
		{"null id on request", `{"jsonrpc":"2.0","id":null,"method":"a"}`, CodeInvalidRequest},
		{"bool id", `{"jsonrpc":"2.0","id":true,"method":"a"}`, CodeInvalidRequest},
		{"null id on result", `{"jsonrpc":"2.0","id":null,"result":{}}`, CodeInvalidRequest},
		{"missing id on result", `{"jsonrpc":"2.0","result":{}}`, CodeInvalidRequest},
		{"empty method", `{"jsonrpc":"2.0","id":1,"method":""}`, CodeInvalidRequest},
		{"method not a string", `{"jsonrpc":"2.0","id":1,"method":5}`, CodeInvalidRequest},
		{"params not structured", `{"jsonrpc":"2.0","id":1,"method":"a","params":"x"}`, CodeInvalidRequest},
		{"error not an object", `{"jsonrpc":"2.0","id":1,"error":"boom"}`, CodeInvalidRequest},
		{"error code not an integer", `{"jsonrpc":"2.0","id":1,"error":{"code":1.5,"message":"x"}}`, CodeInvalidRequest},
		{"error without message", `{"jsonrpc":"2.0","id":1,"error":{"code":1}}`, CodeInvalidRequest},
		{"key in the wrong case", `{"jsonrpc":"2.0","id":1,"Method":"a"}`, CodeInvalidRequest},
		{"error keys in the wrong case", `{"jsonrpc":"2.0","id":1,"error":{"Code":1,"MESSAGE":"x"}}`, CodeInvalidRequest},
		{"error code in the wrong case", `{"jsonrpc":"2.0","id":1,"error":{"CODE":1,"message":"x"}}`, CodeInvalidRequest},
		{"error message not a string", `{"jsonrpc":"2.0","id":1,"error":{"code":1,"message":5}}`, CodeInvalidRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := Decode([]byte(tt.in))
			if err == nil {
				t.Fatalf("Decode(%s) = %+v, want error %d", tt.in, m, tt.code)
			}
			var rpcErr *Error
			if !errors.As(err, &rpcErr) {
				t.Fatalf("error %T is not *Error", err)
			}
			if rpcErr.Code != tt.code {
				t.Fatalf("code = %d, want %d (%v)", rpcErr.Code, tt.code, rpcErr)
			}
		})
	}
}

func TestIDPreservesBytes(t *testing.T) {
	for _, id := range []string{`1`, `1.0`, `9007199254740993`, `-3`, `1e3`, `"a"`, `"ü"`, `"ü"`} {
		t.Run(id, func(t *testing.T) {
			req, err := Decode([]byte(`{"jsonrpc":"2.0","id":` + id + `,"method":"a"}`))
			if err != nil {
				t.Fatalf("Decode: %v", err)
			}
			out, err := Encode(&Message{ID: req.ID, Result: json.RawMessage(`{}`)})
			if err != nil {
				t.Fatalf("Encode: %v", err)
			}
			want := `"id":` + id + `,`
			if !strings.Contains(string(out), want) {
				t.Fatalf("Encode = %s, want it to contain %s", out, want)
			}
		})
	}
}

func TestIDConstructors(t *testing.T) {
	if got := Int64ID(42).String(); got != "42" {
		t.Errorf("Int64ID = %s", got)
	}
	if got := StringID(`a"b`).String(); got != `"a\"b"` {
		t.Errorf("StringID = %s", got)
	}
	var zero ID
	if !zero.IsZero() || Int64ID(0).IsZero() {
		t.Errorf("IsZero wrong")
	}
	b, err := json.Marshal(zero)
	if err != nil || string(b) != "null" {
		t.Errorf("Marshal(zero) = %s, %v", b, err)
	}
	var id ID
	if err := json.Unmarshal([]byte(`{}`), &id); err == nil {
		t.Errorf("UnmarshalJSON accepted an object")
	}
	if err := json.Unmarshal([]byte(`"x"`), &id); err != nil || id.String() != `"x"` {
		t.Errorf("UnmarshalJSON = %s, %v", id, err)
	}
	raw := id.Raw()
	raw[1] = 'y'
	if id.String() != `"x"` {
		t.Errorf("Raw shares memory with the ID")
	}
}

func TestEncode(t *testing.T) {
	tests := []struct {
		name string
		in   *Message
		want string
	}{
		{"request", &Message{ID: Int64ID(1), Method: "tools/list", Params: json.RawMessage(`{ "a" : 1 }`)}, `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"a":1}}`},
		{"notification", &Message{Method: "notifications/cancelled", Params: json.RawMessage(`{}`)}, `{"jsonrpc":"2.0","method":"notifications/cancelled","params":{}}`},
		{"result", &Message{ID: StringID("x"), Result: json.RawMessage(`{"resultType":"complete"}`)}, `{"jsonrpc":"2.0","id":"x","result":{"resultType":"complete"}}`},
		{"error", &Message{ID: Int64ID(2), Error: &Error{Code: CodeMethodNotFound, Message: "method not found"}}, `{"jsonrpc":"2.0","id":2,"error":{"code":-32601,"message":"method not found"}}`},
		{"error without id", &Message{Error: &Error{Code: CodeParseError, Message: "parse error"}}, `{"jsonrpc":"2.0","error":{"code":-32700,"message":"parse error"}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Encode(tt.in)
			if err != nil {
				t.Fatalf("Encode: %v", err)
			}
			if string(got) != tt.want {
				t.Fatalf("Encode =\n%s\nwant\n%s", got, tt.want)
			}
		})
	}
}

func TestEncodeRejects(t *testing.T) {
	tests := []struct {
		name string
		in   *Message
	}{
		{"nil", nil},
		{"empty", &Message{}},
		{"result without id", &Message{Result: json.RawMessage(`{}`)}},
		{"result and error", &Message{ID: Int64ID(1), Result: json.RawMessage(`{}`), Error: &Error{Code: 1, Message: "x"}}},
		{"method and result", &Message{ID: Int64ID(1), Method: "a", Result: json.RawMessage(`{}`)}},
		{"invalid params", &Message{Method: "a", Params: json.RawMessage(`{`)}},
		{"null params", &Message{Method: "a", Params: json.RawMessage(`null`)}},
		{"scalar params", &Message{Method: "a", Params: json.RawMessage(` 5 `)}},
		{"string params", &Message{ID: Int64ID(1), Method: "a", Params: json.RawMessage(`"x"`)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, err := Encode(tt.in); err == nil {
				t.Fatalf("Encode = %s, want error", got)
			}
		})
	}
}

// Anything Encode writes, Decode must accept.
func TestEncodeOutputDecodes(t *testing.T) {
	for _, m := range []*Message{
		{ID: Int64ID(1), Method: "a", Params: json.RawMessage(`{}`)},
		{ID: Int64ID(1), Method: "a", Params: json.RawMessage(`[1]`)},
		{Method: "a"},
		{ID: StringID("x"), Result: json.RawMessage(`null`)},
		{ID: Int64ID(2), Error: &Error{Code: 1, Message: "x", Data: json.RawMessage(`{"a":1}`)}},
		{Error: &Error{Code: CodeParseError, Message: "parse error"}},
	} {
		out, err := Encode(m)
		if err != nil {
			t.Fatalf("Encode(%+v): %v", m, err)
		}
		if _, err := Decode(out); err != nil {
			t.Fatalf("Decode(Encode(%+v)) = %v\n%s", m, err, out)
		}
	}
}

func TestEncodeNoNewline(t *testing.T) {
	out, err := Encode(&Message{Method: "a"})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if bytes.HasSuffix(out, []byte("\n")) {
		t.Fatalf("Encode output ends with a newline: %q", out)
	}
}

func TestErrorString(t *testing.T) {
	e := &Error{Code: CodeInvalidParams, Message: "bad cursor"}
	if got := e.Error(); got != "jsonrpc: -32602: bad cursor" {
		t.Fatalf("Error() = %q", got)
	}
}
