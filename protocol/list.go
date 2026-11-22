package protocol

import (
	"bytes"
	"encoding/json"
	"errors"
)

// List is a slice the schema requires, so a nil List is written as [] and never as null.
type List[T any] []T

func (l List[T]) MarshalJSON() ([]byte, error) {
	if l == nil {
		return []byte("[]"), nil
	}
	return json.Marshal([]T(l))
}

// ProgressToken is a JSON string or number, kept exactly as sent.
type ProgressToken json.RawMessage

func (t ProgressToken) MarshalJSON() ([]byte, error) { return marshalScalar(t, "progress token") }

func (t *ProgressToken) UnmarshalJSON(b []byte) error {
	v, err := unmarshalScalar(b, "progress token")
	*t = v
	return err
}

// RequestID is a JSON-RPC request ID inside a payload, such as the request a cancellation refers to.
type RequestID json.RawMessage

func (id RequestID) MarshalJSON() ([]byte, error) { return marshalScalar(id, "request id") }

func (id *RequestID) UnmarshalJSON(b []byte) error {
	v, err := unmarshalScalar(b, "request id")
	*id = v
	return err
}

// requireKeys reports an error naming the first key that b lacks. UnmarshalJSON is never called for an
// absent field, so types with required raw fields check presence here.
func requireKeys(b []byte, keys ...string) error {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}
	for _, k := range keys {
		if _, ok := m[k]; !ok {
			return errors.New(`protocol: "` + k + `" is required`)
		}
	}
	return nil
}

func validScalar(b []byte) bool {
	if len(b) == 0 {
		return false
	}
	c := b[0]
	return (c == '"' || c == '-' || (c >= '0' && c <= '9')) && json.Valid(b)
}

func marshalScalar(b []byte, what string) ([]byte, error) {
	if !validScalar(b) {
		return nil, errors.New("protocol: a " + what + " must be a JSON string or number")
	}
	return bytes.Clone(b), nil
}

func unmarshalScalar(b []byte, what string) ([]byte, error) {
	b = bytes.TrimSpace(b)
	if !validScalar(b) {
		return nil, errors.New("protocol: a " + what + " must be a JSON string or number")
	}
	return bytes.Clone(b), nil
}
