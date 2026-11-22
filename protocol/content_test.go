package protocol

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestContentListUnknownType(t *testing.T) {
	in := `[{"type":"x-future","a":1},{"type":"text","text":"hi"}]`
	var l ContentList
	if err := json.Unmarshal([]byte(in), &l); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if raw, ok := l[0].(RawContent); !ok || raw.Type != "x-future" {
		t.Fatalf("first block = %#v, want RawContent", l[0])
	}
	if text, ok := l[1].(TextContent); !ok || text.Text != "hi" {
		t.Fatalf("second block = %#v, want TextContent", l[1])
	}
	out, err := json.Marshal(l)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	assertSameJSON(t, []byte(in), out)
}

func TestContentTypeAlwaysWritten(t *testing.T) {
	tests := []struct {
		block Content
		want  string
	}{
		{TextContent{Text: "a"}, `{"type":"text","text":"a"}`},
		{ImageContent{Data: "AA==", MIMEType: "image/png"}, `{"type":"image","data":"AA==","mimeType":"image/png"}`},
		{AudioContent{Data: "AA==", MIMEType: "audio/wav"}, `{"type":"audio","data":"AA==","mimeType":"audio/wav"}`},
		{ResourceLink{Resource: Resource{URI: "file:///a", Name: "a"}}, `{"type":"resource_link","uri":"file:///a","name":"a"}`},
		{EmbeddedResource{Resource: TextResourceContents{URI: "file:///a", Text: ""}}, `{"type":"resource","resource":{"uri":"file:///a","text":""}}`},
		{ToolUseContent{ID: "1", Name: "f", Input: json.RawMessage(`{}`)}, `{"type":"tool_use","id":"1","name":"f","input":{}}`},
		{ToolResultContent{ToolUseID: "1"}, `{"type":"tool_result","toolUseId":"1","content":[]}`},
		{ToolUseContent{ID: "2", Name: "g"}, `{"type":"tool_use","id":"2","name":"g","input":{}}`},
	}
	for _, tt := range tests {
		t.Run(tt.block.ContentType(), func(t *testing.T) {
			out, err := json.Marshal(tt.block)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			assertSameJSON(t, []byte(tt.want), out)
		})
	}
}

func TestContentListNilIsEmptyArray(t *testing.T) {
	out, err := json.Marshal(CallToolResult{ResultBase: ResultBase{ResultType: ResultComplete}})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	assertSameJSON(t, []byte(`{"resultType":"complete","content":[]}`), out)
}

func TestContentListRejectsBadBlock(t *testing.T) {
	var l ContentList
	for _, in := range []string{`[{"text":"no type"}]`, `[1]`, `{"type":"text"}`} {
		if err := json.Unmarshal([]byte(in), &l); err == nil {
			t.Errorf("Unmarshal(%s) succeeded", in)
		}
	}
}

func TestSamplingContentShape(t *testing.T) {
	for _, in := range []string{
		`{"type":"text","text":"one"}`,
		`[{"type":"text","text":"one"}]`,
		`[{"type":"text","text":"one"},{"type":"tool_use","id":"1","name":"f","input":{}}]`,
	} {
		var c SamplingContent
		if err := json.Unmarshal([]byte(in), &c); err != nil {
			t.Fatalf("Unmarshal(%s): %v", in, err)
		}
		out, err := json.Marshal(c)
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		assertSameJSON(t, []byte(in), out)
	}
	out, err := json.Marshal(SamplingContent{Blocks: []Content{TextContent{Text: "x"}}})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	assertSameJSON(t, []byte(`{"type":"text","text":"x"}`), out)
}

func TestResourceContentsKinds(t *testing.T) {
	in := `[{"uri":"a","text":"t"},{"uri":"b","blob":"AA==","mimeType":"image/png"}]`
	var l ResourceContentsList
	if err := json.Unmarshal([]byte(in), &l); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if _, ok := l[0].(TextResourceContents); !ok {
		t.Errorf("first = %T", l[0])
	}
	if b, ok := l[1].(BlobResourceContents); !ok || b.ResourceURI() != "b" {
		t.Errorf("second = %#v", l[1])
	}
	out, err := json.Marshal(l)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	assertSameJSON(t, []byte(in), out)
}

func TestNestedToolResultsRejectedFast(t *testing.T) {
	var b strings.Builder
	b.WriteString(`{"resultType":"complete","content":[`)
	const depth = 4000
	for i := 0; i < depth; i++ {
		b.WriteString(`{"type":"tool_result","toolUseId":"","content":[`)
	}
	for i := 0; i < depth; i++ {
		b.WriteString(`]}`)
	}
	b.WriteString(`]}`)
	start := time.Now()
	var r CallToolResult
	if err := json.Unmarshal([]byte(b.String()), &r); err == nil {
		t.Fatal("a tool_result block inside a tool result was accepted")
	}
	if d := time.Since(start); d > time.Second {
		t.Fatalf("rejecting nested blocks took %v", d)
	}
}

func TestToolBlocksOnlyInSampling(t *testing.T) {
	toolUse := `{"type":"tool_use","id":"1","name":"f","input":{}}`
	var l ContentList
	if err := json.Unmarshal([]byte(`[`+toolUse+`]`), &l); err == nil {
		t.Error("ContentList accepted a tool_use block")
	}
	var m PromptMessage
	if err := json.Unmarshal([]byte(`{"role":"user","content":`+toolUse+`}`), &m); err == nil {
		t.Error("PromptMessage accepted a tool_use block")
	}
	var s SamplingContent
	if err := json.Unmarshal([]byte(toolUse), &s); err != nil {
		t.Errorf("SamplingContent rejected tool_use: %v", err)
	}
	nested := `{"type":"tool_result","toolUseId":"1","content":[{"type":"tool_result","toolUseId":"2","content":[]}]}`
	if err := json.Unmarshal([]byte(nested), &s); err == nil {
		t.Error("SamplingContent accepted a tool_result inside a tool_result")
	}
}

func TestMissingRequiredContentRejected(t *testing.T) {
	tests := []struct {
		name string
		in   string
		v    any
	}{
		{"sampling message without content", `{"role":"user"}`, new(SamplingMessage)},
		{"sampling message with null content", `{"role":"user","content":null}`, new(SamplingMessage)},
		{"create message result without content", `{"role":"assistant","model":"m"}`, new(CreateMessageResult)},
		{"embedded resource without resource", `{"type":"resource"}`, new(EmbeddedResource)},
		{"embedded resource with null resource", `{"type":"resource","resource":null}`, new(EmbeddedResource)},
		{"resource contents without text or blob", `{"type":"resource","resource":{"uri":"a"}}`, new(EmbeddedResource)},
		{"read result contents without text or blob", `{"resultType":"complete","ttlMs":0,"cacheScope":"public","contents":[{"uri":"a"}]}`, new(ReadResourceResult)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := json.Unmarshal([]byte(tt.in), tt.v); err == nil {
				t.Fatalf("Unmarshal(%s) succeeded", tt.in)
			}
		})
	}
}

func FuzzContent(f *testing.F) {
	for _, seed := range []string{
		`[{"type":"text","text":"a"}]`,
		`[{"type":"resource","resource":{"uri":"a","blob":"AA=="}}]`,
		`{"type":"tool_result","toolUseId":"1","content":[{"type":"text","text":"x"}]}`,
		`[{"type":"tool_use","id":"1","name":"f","input":{}}]`,
		`[{"type":"x-future"}]`,
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		var l ContentList
		if json.Unmarshal(b, &l) == nil {
			if _, err := json.Marshal(l); err != nil {
				t.Fatalf("ContentList decoded but does not encode: %v", err)
			}
		}
		var s SamplingContent
		if json.Unmarshal(b, &s) == nil {
			if _, err := json.Marshal(s); err != nil {
				t.Fatalf("SamplingContent decoded but does not encode: %v", err)
			}
		}
	})
}

func TestPromptMessageContent(t *testing.T) {
	in := `{"role":"user","content":{"type":"text","text":"hello"}}`
	var m PromptMessage
	if err := json.Unmarshal([]byte(in), &m); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if text, ok := m.Content.(TextContent); !ok || text.Text != "hello" {
		t.Fatalf("Content = %#v", m.Content)
	}
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	assertSameJSON(t, []byte(in), out)
}
