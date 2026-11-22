package jsonrpc

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"
)

func FuzzDecode(f *testing.F) {
	for _, seed := range []string{
		`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`,
		`{"jsonrpc":"2.0","method":"notifications/progress","params":{"progress":1}}`,
		`{"jsonrpc":"2.0","id":"a","result":{"resultType":"complete"}}`,
		`{"jsonrpc":"2.0","id":1,"error":{"code":-32601,"message":"x","data":[1]}}`,
		`{"jsonrpc":"2.0","error":{"code":-32700,"message":"parse error"}}`,
		`{"jsonrpc":"2.0","id":9007199254740993,"method":"a"}`,
		`{"jsonrpc":"2.0","id":1.0,"method":"a","params":[ 1, 2 ]}`,
		`[{}]`,
		`{"jsonrpc":"2.0","method":"a"}{}`,
		`{"jsonrpc":`,
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		m, err := Decode(b)
		if err != nil {
			if _, ok := err.(*Error); !ok {
				t.Fatalf("error %T is not *Error", err)
			}
			return
		}
		out, err := Encode(m)
		if err != nil {
			t.Fatalf("Encode of a decoded message failed: %v", err)
		}
		again, err := Decode(out)
		if err != nil {
			t.Fatalf("Decode(Encode(m)) failed: %v\n%s", err, out)
		}
		if !reflect.DeepEqual(normalize(t, m), normalize(t, again)) {
			t.Fatalf("round trip changed the message\nfirst:  %+v\nsecond: %+v", m, again)
		}
	})
}

func normalize(t *testing.T, m *Message) Message {
	t.Helper()
	c := *m
	c.Params = compact(t, c.Params)
	c.Result = compact(t, c.Result)
	if c.Error != nil {
		e := *c.Error
		e.Data = compact(t, e.Data)
		c.Error = &e
	}
	return c
}

func compact(t *testing.T, b json.RawMessage) json.RawMessage {
	t.Helper()
	if b == nil {
		return nil
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, b); err != nil {
		t.Fatalf("compact: %v", err)
	}
	return buf.Bytes()
}
