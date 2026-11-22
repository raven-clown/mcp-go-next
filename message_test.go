package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"strconv"
	"testing"

	"github.com/raven-clown/mcp-go-next/jsonrpc"
	"github.com/raven-clown/mcp-go-next/protocol"
)

const testMeta = `{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientCapabilities":{}}`

func idOf(n int64) jsonrpc.ID { return jsonrpc.Int64ID(n) }

// requestMessage builds a request whose params carry the standard _meta plus the given fields.
func requestMessage(t *testing.T, id int64, method, params string) *jsonrpc.Message {
	t.Helper()
	merged := `{"_meta":` + testMeta
	if inner := bytes.TrimSpace([]byte(params)); len(inner) > 2 {
		merged += "," + string(inner[1:len(inner)-1])
	}
	merged += "}"
	return &jsonrpc.Message{ID: idOf(id), Method: method, Params: json.RawMessage(merged)}
}

func mustEncode(t *testing.T, m *jsonrpc.Message) []byte {
	t.Helper()
	b, err := jsonrpc.Encode(m)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func assertJSON(t *testing.T, want string, got []byte) {
	t.Helper()
	var w, g any
	if err := json.Unmarshal([]byte(want), &w); err != nil {
		t.Fatalf("want is not JSON: %v", err)
	}
	if err := json.Unmarshal(got, &g); err != nil {
		t.Fatalf("got is not JSON: %v\n%s", err, got)
	}
	if !reflect.DeepEqual(w, g) {
		t.Fatalf("JSON differs\n got  %s\n want %s", got, want)
	}
}

func TestHandleMessageEchoesID(t *testing.T) {
	s := NewServer(Info{Name: "t", Version: "1"})
	s.HandleMethod("x.example/echo", echoHandler(nil))
	for _, raw := range []string{`1.0`, `9007199254740993`, `"abc"`} {
		decoded, err := jsonrpc.Decode([]byte(`{"jsonrpc":"2.0","id":` + raw + `,"method":"x.example/echo","params":{"_meta":` + testMeta + `}}`))
		if err != nil {
			t.Fatal(err)
		}
		resp := HandleMessage(context.Background(), s, decoded, nil, nil)
		if resp.ID.String() != raw {
			t.Fatalf("id = %s, want %s", resp.ID, raw)
		}
	}
}

func TestHandleMessageNotification(t *testing.T) {
	s := NewServer(Info{Name: "t", Version: "1"})
	called := false
	s.HandleMethod("x.example/note", echoHandler(&called))
	msg := &jsonrpc.Message{Method: "x.example/note", Params: json.RawMessage(`{}`)}
	if resp := HandleMessage(context.Background(), s, msg, nil, nil); resp != nil {
		t.Fatalf("a notification got a response: %+v", resp)
	}
	if !called {
		t.Fatal("notification handler was not called; notifications carry no protocol version")
	}
	unknown := &jsonrpc.Message{Method: "x.example/unknown"}
	if resp := HandleMessage(context.Background(), s, unknown, nil, nil); resp != nil {
		t.Fatalf("an unknown notification got a response: %+v", resp)
	}
}

func TestHandleMessageIgnoresResponses(t *testing.T) {
	s := NewServer(Info{Name: "t", Version: "1"})
	msg := &jsonrpc.Message{ID: idOf(1), Result: json.RawMessage(`{}`)}
	if resp := HandleMessage(context.Background(), s, msg, nil, nil); resp != nil {
		t.Fatalf("a response got a response: %+v", resp)
	}
}

func TestCancelledRequestNoResponse(t *testing.T) {
	s := NewServer(Info{Name: "t", Version: "1"})
	started := make(chan struct{})
	s.HandleMethod("x.example/wait", HandlerFunc(func(ctx context.Context, req *Request) (protocol.Result, error) {
		close(started)
		<-ctx.Done()
		return nil, ctx.Err()
	}))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan *jsonrpc.Message, 1)
	go func() { done <- HandleMessage(ctx, s, requestMessage(t, 1, "x.example/wait", `{}`), nil, nil) }()
	<-started
	cancel()
	if resp := <-done; resp != nil {
		t.Fatalf("a cancelled request got a response: %+v", resp)
	}
}

func TestHandleMessageBadMeta(t *testing.T) {
	s := NewServer(Info{Name: "t", Version: "1"})
	s.HandleMethod("x.example/echo", echoHandler(nil))
	msg := &jsonrpc.Message{ID: idOf(1), Method: "x.example/echo", Params: json.RawMessage(`{"_meta":{"progressToken":{"a":1}}}`)}
	resp := HandleMessage(context.Background(), s, msg, nil, nil)
	if resp == nil || resp.Error == nil || resp.Error.Code != -32602 {
		t.Fatalf("response = %+v", resp)
	}
}

type recordingNotifier struct{ msgs []*jsonrpc.Message }

func (r *recordingNotifier) Notify(_ context.Context, m *jsonrpc.Message) error {
	r.msgs = append(r.msgs, m)
	return nil
}

func TestHandleMessageFillsRequest(t *testing.T) {
	s := NewServer(Info{Name: "t", Version: "1"})
	var got *Request
	s.HandleMethod("x.example/echo", HandlerFunc(func(ctx context.Context, req *Request) (protocol.Result, error) {
		got = req
		return nil, nil
	}))
	n := &recordingNotifier{}
	h := http.Header{"X-Org": []string{"7"}}
	msg := requestMessage(t, 3, "x.example/echo", `{"a":1}`)
	msg.Params = json.RawMessage(`{"a":1,"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientCapabilities":{"roots":{}},"progressToken":"p"}}`)
	if resp := HandleMessage(context.Background(), s, msg, n, h); resp.Error != nil {
		t.Fatal(resp.Error)
	}
	if got.Method != "x.example/echo" || got.ID.String() != "3" || got.Header.Get("X-Org") != "7" || got.notifier != n {
		t.Fatalf("request = %+v", got)
	}
	if got.Meta.ClientCapabilities.Roots == nil || string(got.Meta.ProgressToken) != `"p"` {
		t.Fatalf("meta = %+v", got.Meta)
	}
	if !got.isDone() {
		t.Fatal("request not marked done after the response")
	}
}

func TestTopLevelMember(t *testing.T) {
	tests := []struct {
		name, obj, want string
		found           bool
	}{
		{"first", `{"_meta":{"a":1},"x":2}`, `{"a":1}`, true},
		{"last of several", `{"x":"_meta","_meta":1,"y":[{"_meta":3}],"_meta":2}`, `2`, true},
		{"nested only", `{"x":{"_meta":1}}`, ``, false},
		{"in a string", `{"x":"\"_meta\":1"}`, ``, false},
		{"escaped key", `{"_meta":5}`, `5`, true},
		{"spaces", " { \"_meta\" : [1, \"]\"] } ", `[1, "]"]`, true},
		{"not an object", `[{"_meta":1}]`, ``, false},
		{"empty", ``, ``, false},
		{"empty object", `{}`, ``, false},
		{"truncated", `{"_meta":{"a":`, ``, false},
		{"escaped quote in value", `{"k":"a\\\"b","_meta":true}`, `true`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, found := topLevelMember([]byte(tt.obj), "_meta")
			if found != tt.found || string(got) != tt.want {
				t.Fatalf("topLevelMember = %q, %v; want %q, %v", got, found, tt.want, tt.found)
			}
		})
	}
}

// FuzzTopLevelMember checks the fast scan against encoding/json for every valid object.
func FuzzTopLevelMember(f *testing.F) {
	for _, seed := range []string{`{"_meta":{"a":1},"x":2}`, `{"x":"_meta","_meta":1,"_meta":2}`, `{"_meta":5}`, `{"k":"a\\\"b","_meta":true}`} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		got, found := topLevelMember(b, "_meta")
		var m map[string]json.RawMessage
		if json.Unmarshal(b, &m) != nil || m == nil {
			return
		}
		want, ok := m["_meta"]
		if ok != found {
			t.Fatalf("%q: found = %v, encoding/json found = %v", b, found, ok)
		}
		if ok && !bytes.Equal(bytes.TrimSpace(got), bytes.TrimSpace(want)) {
			t.Fatalf("%q: got %q, encoding/json %q", b, got, want)
		}
	})
}

func BenchmarkHandleMessage(b *testing.B) {
	s := NewServer(Info{Name: "bench", Version: "1"})
	s.HandleMethod("x.example/echo", echoHandler(nil))
	for _, size := range []int{0, 1 << 20} {
		params := `{"_meta":` + testMeta + `,"blob":"` + string(bytes.Repeat([]byte("x"), size)) + `"}`
		raw := []byte(`{"jsonrpc":"2.0","id":1,"method":"x.example/echo","params":` + params + `}`)
		b.Run("params="+strconv.Itoa(size), func(b *testing.B) {
			b.SetBytes(int64(len(raw)))
			b.ReportAllocs()
			for b.Loop() {
				msg, err := jsonrpc.Decode(raw)
				if err != nil {
					b.Fatal(err)
				}
				resp := HandleMessage(context.Background(), s, msg, nil, nil)
				if _, err := jsonrpc.Encode(resp); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
