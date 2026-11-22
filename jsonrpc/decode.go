package jsonrpc

import (
	"bytes"
	"encoding/json"
	"errors"
)

var null = []byte("null")

func parseError() *Error { return &Error{Code: CodeParseError, Message: "parse error"} }

func invalid(reason string) *Error {
	return &Error{Code: CodeInvalidRequest, Message: "invalid request: " + reason}
}

// Decode reads one message in a single pass. The error is always a *Error, ready to send back.
// Keys are matched exactly, unlike encoding/json struct fields, so "Method" is not "method".
func Decode(b []byte) (*Message, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(b, &fields); err != nil {
		var typeErr *json.UnmarshalTypeError
		if errors.As(err, &typeErr) {
			if t := bytes.TrimSpace(b); len(t) > 0 && t[0] == '[' {
				return nil, invalid("batches are not supported")
			}
			return nil, invalid("a message must be a JSON object")
		}
		return nil, parseError()
	}
	if fields == nil {
		return nil, invalid("a message must be a JSON object")
	}

	var version string
	if v, ok := fields["jsonrpc"]; !ok || json.Unmarshal(v, &version) != nil || version != Version {
		return nil, invalid(`"jsonrpc" must be "2.0"`)
	}

	m := &Message{}
	rawID, hasID := fields["id"]
	idIsNull := hasID && bytes.Equal(rawID, null)
	if hasID && !idIsNull {
		if !validID(rawID) {
			return nil, invalid("id must be a string or a number")
		}
		m.ID = ID{raw: rawID}
	}

	rawMethod, hasMethod := fields["method"]
	result, hasResult := fields["result"]
	rawError, hasError := fields["error"]

	if hasMethod {
		if hasResult || hasError {
			return nil, invalid("a request cannot carry a result or an error")
		}
		if json.Unmarshal(rawMethod, &m.Method) != nil || m.Method == "" {
			return nil, invalid("method must be a non-empty string")
		}
		if idIsNull {
			return nil, invalid("a request id cannot be null")
		}
		if params, ok := fields["params"]; ok {
			if len(params) == 0 || (params[0] != '{' && params[0] != '[') {
				return nil, invalid("params must be an object or an array")
			}
			m.Params = params
		}
		return m, nil
	}

	switch {
	case hasResult && hasError:
		return nil, invalid("a response cannot carry both a result and an error")
	case hasResult:
		if m.ID.IsZero() {
			return nil, invalid("a result needs an id")
		}
		m.Result = result
	case hasError:
		e, ok := decodeError(rawError)
		if !ok {
			return nil, invalid("error must be an object with an integer code and a message")
		}
		m.Error = e
	default:
		return nil, invalid("a message needs a method, a result or an error")
	}
	return m, nil
}

// decodeError matches the error object's keys exactly, like the envelope.
func decodeError(raw json.RawMessage) (*Error, bool) {
	var fields map[string]json.RawMessage
	if len(raw) == 0 || raw[0] != '{' || json.Unmarshal(raw, &fields) != nil {
		return nil, false
	}
	rawCode, okCode := fields["code"]
	rawMessage, okMessage := fields["message"]
	if !okCode || !okMessage {
		return nil, false
	}
	e := &Error{Data: fields["data"]}
	if json.Unmarshal(rawCode, &e.Code) != nil || json.Unmarshal(rawMessage, &e.Message) != nil {
		return nil, false
	}
	if bytes.Equal(rawMessage, null) || bytes.Equal(rawCode, null) {
		return nil, false
	}
	return e, true
}
