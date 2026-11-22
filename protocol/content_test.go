package protocol

import (
	"encoding/json"
	"testing"
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
