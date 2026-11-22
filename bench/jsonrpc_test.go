package bench

import (
	"encoding/json"
	"strings"
	"testing"

	m3 "github.com/mark3labs/mcp-go/mcp"
	sdkrpc "github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/raven-clown/mcp-go-next/jsonrpc"
)

const toolCallHead = `{"jsonrpc":"2.0","id":42,"method":"tools/call","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientCapabilities":{}},"name":"search","arguments":`

var (
	smallCall = []byte(toolCallHead + string(searchArgsJSON) + `}}`)
	largeCall = []byte(toolCallHead + `{"content":"` + strings.Repeat("x", 1<<20) + `"}}}`)
)

// Each library decodes a tools/call request from the bytes a transport reads. mark3labs has no
// exported decoder, so it is measured the way its clients and tests decode, into mcp.JSONRPCRequest.
var decoders = []struct {
	name   string
	decode func([]byte) error
}{
	{"lib=mcp-go-next", func(b []byte) error { _, err := jsonrpc.Decode(b); return err }},
	{"lib=go-sdk", func(b []byte) error { _, err := sdkrpc.DecodeMessage(b); return err }},
	{"lib=mark3labs", func(b []byte) error {
		var r m3.JSONRPCRequest
		return json.Unmarshal(b, &r)
	}},
}

func TestDecodersAccept(t *testing.T) {
	for _, d := range decoders {
		for _, msg := range [][]byte{smallCall, largeCall} {
			if err := d.decode(msg); err != nil {
				t.Errorf("%s: %v", d.name, err)
			}
		}
	}
}

func BenchmarkDecodeToolCall(b *testing.B) {
	for _, size := range []struct {
		name string
		msg  []byte
	}{{"size=small", smallCall}, {"size=1MiB", largeCall}} {
		for _, d := range decoders {
			b.Run(size.name+"/"+d.name, func(b *testing.B) {
				b.SetBytes(int64(len(size.msg)))
				b.ReportAllocs()
				for b.Loop() {
					if err := d.decode(size.msg); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}
