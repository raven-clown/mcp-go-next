package jsonrpc

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

// decodeReference is the earlier Decode, built on json.Unmarshal into a map. The scanner-based
// Decode must give the same result for every input.
func decodeReference(b []byte) (*Message, error) {
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
	var raw bytes.Buffer
	enc := json.NewEncoder(&raw)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(fields); err != nil {
		return nil, parseError()
	}
	return Decode(raw.Bytes())
}

func sameDecode(t *testing.T, b []byte) {
	t.Helper()
	got, gotErr := Decode(b)
	want, wantErr := decodeReference(b)
	if (gotErr == nil) != (wantErr == nil) {
		t.Fatalf("%q: Decode err = %v, reference err = %v", b, gotErr, wantErr)
	}
	if gotErr != nil {
		if gotErr.(*Error).Code != wantErr.(*Error).Code {
			t.Fatalf("%q: Decode code %d, reference code %d", b, gotErr.(*Error).Code, wantErr.(*Error).Code)
		}
		return
	}
	if !reflect.DeepEqual(normalize(t, got), normalize(t, want)) {
		t.Fatalf("%q:\n Decode    %+v\n reference %+v", b, got, want)
	}
}

func TestScanMatchesReference(t *testing.T) {
	for _, in := range []string{
		`{"jsonrpc":"2.0","id":1,"method":"a","params":{"x":[1,2,{"y":"\u00e9\n"}]}}`,
		`{"jsonrpc":"2.0","method":"a","method":"b"}`,
		`{"jsonrpc":"2.0","\u006dethod":"a"}`,
		"{\"jsonrpc\":\"2.0\",\"m\xffx\":1,\"method\":\"a\"}",
		`{"jsonrpc":"2.0","id":1,"result":{}} `,
		`{"jsonrpc":"2.0","id":1,"result":{}}x`,
		`  [ {"jsonrpc":"2.0"} ] `, `[1,`, `null`, `"s"`, `{}`, `{`, `{"a"}`, `{"a":}`, `{"a":1,}`,
		`{"jsonrpc":"2.0","id":01,"method":"a"}`, `{"jsonrpc":"2.0","id":1.,"method":"a"}`,
		`{"jsonrpc":"2.0","id":"\x","method":"a"}`, "{\"jsonrpc\":\"2.0\",\"id\":\"\x01\",\"method\":\"a\"}",
		`{"jsonrpc":"2.0","id":1,"error":{"code":1,"message":"x","data":[true,false,null]}}`,
		strings.Repeat("[", 10001) + strings.Repeat("]", 10001),
		`{"jsonrpc":"2.0","method":"a","params":` + strings.Repeat("[", 9998) + strings.Repeat("]", 9998) + `}`,
	} {
		sameDecode(t, []byte(in))
	}
}

func TestDecodeDoesNotAliasInput(t *testing.T) {
	b := []byte(`{"jsonrpc":"2.0","id":7,"method":"a","params":{"k":"v"}}`)
	m, err := Decode(b)
	if err != nil {
		t.Fatal(err)
	}
	for i := range b {
		b[i] = 'X'
	}
	if string(m.Params) != `{"k":"v"}` || m.ID.String() != "7" {
		t.Fatalf("message changed with its input: %s %s", m.Params, m.ID)
	}
}

func FuzzScanMatchesReference(f *testing.F) {
	for _, seed := range []string{
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"x"}}`,
		`{"jsonrpc":"2.0","id":"a","result":{"resultType":"complete"}}`,
		`{"jsonrpc":"2.0","error":{"code":-32700,"message":"parse error"}}`,
		`[{}]`, `{"a":1}{}`, `{"jsonrpc":"2.0","\u006dethod":"a"}`,
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		sameDecode(t, b)
	})
}
