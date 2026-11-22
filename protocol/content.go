package protocol

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

type Annotations struct {
	Audience     []Role   `json:"audience,omitempty"`
	Priority     *float64 `json:"priority,omitempty"`
	LastModified string   `json:"lastModified,omitempty"`
}

// Content is one content block. Unknown block types decode into RawContent and survive a round trip.
type Content interface {
	ContentType() string
}

type TextContent struct {
	Text        string       `json:"text"`
	Annotations *Annotations `json:"annotations,omitempty"`
	Meta        Object       `json:"_meta,omitempty"`
}

type ImageContent struct {
	Data        string       `json:"data"`
	MIMEType    string       `json:"mimeType"`
	Annotations *Annotations `json:"annotations,omitempty"`
	Meta        Object       `json:"_meta,omitempty"`
}

type AudioContent struct {
	Data        string       `json:"data"`
	MIMEType    string       `json:"mimeType"`
	Annotations *Annotations `json:"annotations,omitempty"`
	Meta        Object       `json:"_meta,omitempty"`
}

type ResourceLink struct {
	Resource
}

type EmbeddedResource struct {
	Resource    ResourceContents `json:"resource"`
	Annotations *Annotations     `json:"annotations,omitempty"`
	Meta        Object           `json:"_meta,omitempty"`
}

// ToolUseContent and ToolResultContent appear only in sampling messages.
type ToolUseContent struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
	Meta  Object          `json:"_meta,omitempty"`
}

type ToolResultContent struct {
	ToolUseID         string          `json:"toolUseId"`
	Content           ContentList     `json:"content"`
	StructuredContent json.RawMessage `json:"structuredContent,omitempty"`
	IsError           bool            `json:"isError,omitempty"`
	Meta              Object          `json:"_meta,omitempty"`
}

// RawContent holds a block of a type this package does not know, byte for byte.
type RawContent struct {
	Type string
	Raw  json.RawMessage
}

func (TextContent) ContentType() string       { return "text" }
func (ImageContent) ContentType() string      { return "image" }
func (AudioContent) ContentType() string      { return "audio" }
func (ResourceLink) ContentType() string      { return "resource_link" }
func (EmbeddedResource) ContentType() string  { return "resource" }
func (ToolUseContent) ContentType() string    { return "tool_use" }
func (ToolResultContent) ContentType() string { return "tool_result" }
func (c RawContent) ContentType() string      { return c.Type }

// withType writes "type" first, so a block is valid whatever the caller filled in.
func withType(kind string, v any) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	head := `{"type":` + quote(kind)
	if bytes.Equal(b, []byte("{}")) {
		return []byte(head + "}"), nil
	}
	return append([]byte(head+","), b[1:]...), nil
}

func quote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func (c TextContent) MarshalJSON() ([]byte, error) {
	type plain TextContent
	return withType(c.ContentType(), plain(c))
}

func (c ImageContent) MarshalJSON() ([]byte, error) {
	type plain ImageContent
	return withType(c.ContentType(), plain(c))
}

func (c AudioContent) MarshalJSON() ([]byte, error) {
	type plain AudioContent
	return withType(c.ContentType(), plain(c))
}

func (c ResourceLink) MarshalJSON() ([]byte, error) {
	type plain ResourceLink
	return withType(c.ContentType(), plain(c))
}

func (c EmbeddedResource) MarshalJSON() ([]byte, error) {
	type plain EmbeddedResource
	if c.Resource == nil {
		return nil, errors.New("protocol: an embedded resource needs its contents")
	}
	return withType(c.ContentType(), plain(c))
}

func (c ToolUseContent) MarshalJSON() ([]byte, error) {
	type plain ToolUseContent
	if c.Input == nil {
		c.Input = json.RawMessage(`{}`)
	}
	return withType(c.ContentType(), plain(c))
}

func (c ToolResultContent) MarshalJSON() ([]byte, error) {
	type plain ToolResultContent
	return withType(c.ContentType(), plain(c))
}

func (c RawContent) MarshalJSON() ([]byte, error) {
	if !json.Valid(c.Raw) {
		return nil, errors.New("protocol: raw content is not valid JSON")
	}
	return c.Raw, nil
}

func (e *EmbeddedResource) UnmarshalJSON(b []byte) error {
	var w struct {
		Resource    json.RawMessage `json:"resource"`
		Annotations *Annotations    `json:"annotations"`
		Meta        Object          `json:"_meta"`
	}
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	r, err := decodeResourceContents(w.Resource)
	if err != nil {
		return err
	}
	*e = EmbeddedResource{Resource: r, Annotations: w.Annotations, Meta: w.Meta}
	return nil
}

// decodeContent reads one block. Tool blocks are allowed only in sampling messages. A tool result's own
// content is a ContentList, which allows no tool blocks, so blocks never nest more than one level.
func decodeContent(raw json.RawMessage, allowToolBlocks bool) (Content, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || raw[0] != '{' {
		return nil, errors.New("protocol: a content block must be an object")
	}
	var probe struct {
		Type *string `json:"type"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return nil, err
	}
	if probe.Type == nil {
		return nil, errors.New(`protocol: a content block needs a "type"`)
	}
	if !allowToolBlocks && (*probe.Type == "tool_use" || *probe.Type == "tool_result") {
		return nil, fmt.Errorf("protocol: %s blocks are only allowed in sampling messages", *probe.Type)
	}
	var (
		c   Content
		err error
	)
	switch *probe.Type {
	case "text":
		var v TextContent
		err = json.Unmarshal(raw, &v)
		c = v
	case "image":
		var v ImageContent
		err = json.Unmarshal(raw, &v)
		c = v
	case "audio":
		var v AudioContent
		err = json.Unmarshal(raw, &v)
		c = v
	case "resource_link":
		var v ResourceLink
		err = json.Unmarshal(raw, &v)
		c = v
	case "resource":
		var v EmbeddedResource
		err = json.Unmarshal(raw, &v)
		c = v
	case "tool_use":
		var v ToolUseContent
		err = json.Unmarshal(raw, &v)
		c = v
	case "tool_result":
		var v ToolResultContent
		err = json.Unmarshal(raw, &v)
		c = v
	default:
		c = RawContent{Type: *probe.Type, Raw: bytes.Clone(raw)}
	}
	if err != nil {
		return nil, fmt.Errorf("protocol: %s content: %w", *probe.Type, err)
	}
	return c, nil
}

// ContentList is a list of content blocks. A nil list is written as [].
type ContentList []Content

func (l ContentList) MarshalJSON() ([]byte, error) {
	if l == nil {
		return []byte("[]"), nil
	}
	return json.Marshal([]Content(l))
}

func (l *ContentList) UnmarshalJSON(b []byte) error {
	var raws []json.RawMessage
	if err := json.Unmarshal(b, &raws); err != nil {
		return err
	}
	out := make(ContentList, 0, len(raws))
	for _, raw := range raws {
		c, err := decodeContent(raw, false)
		if err != nil {
			return err
		}
		out = append(out, c)
	}
	*l = out
	return nil
}

// SamplingContent is a single block or an array of blocks. AsArray keeps a one-block array an array.
type SamplingContent struct {
	Blocks  []Content
	AsArray bool
}

func (s SamplingContent) MarshalJSON() ([]byte, error) {
	if len(s.Blocks) == 1 && !s.AsArray {
		return json.Marshal(s.Blocks[0])
	}
	return ContentList(s.Blocks).MarshalJSON()
}

func (s *SamplingContent) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) > 0 && b[0] == '[' {
		var raws []json.RawMessage
		if err := json.Unmarshal(b, &raws); err != nil {
			return err
		}
		blocks := make([]Content, 0, len(raws))
		for _, raw := range raws {
			c, err := decodeContent(raw, true)
			if err != nil {
				return err
			}
			blocks = append(blocks, c)
		}
		*s = SamplingContent{Blocks: blocks, AsArray: true}
		return nil
	}
	c, err := decodeContent(b, true)
	if err != nil {
		return err
	}
	*s = SamplingContent{Blocks: []Content{c}}
	return nil
}

// decodeSamplingContent reads a required content field, which UnmarshalJSON never sees when the key is absent.
func decodeSamplingContent(raw json.RawMessage) (SamplingContent, error) {
	var s SamplingContent
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return s, errors.New(`protocol: "content" is required`)
	}
	err := s.UnmarshalJSON(raw)
	return s, err
}

// ResourceContents is the text or blob body of a resource.
type ResourceContents interface {
	ResourceURI() string
}

type TextResourceContents struct {
	URI      string `json:"uri"`
	MIMEType string `json:"mimeType,omitempty"`
	Text     string `json:"text"`
	Meta     Object `json:"_meta,omitempty"`
}

type BlobResourceContents struct {
	URI      string `json:"uri"`
	MIMEType string `json:"mimeType,omitempty"`
	Blob     string `json:"blob"`
	Meta     Object `json:"_meta,omitempty"`
}

func (c TextResourceContents) ResourceURI() string { return c.URI }
func (c BlobResourceContents) ResourceURI() string { return c.URI }

func decodeResourceContents(raw json.RawMessage) (ResourceContents, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || raw[0] != '{' {
		return nil, errors.New("protocol: resource contents must be an object")
	}
	var probe struct {
		Text *string `json:"text"`
		Blob *string `json:"blob"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return nil, fmt.Errorf("protocol: resource contents: %w", err)
	}
	switch {
	case probe.Blob != nil:
		var v BlobResourceContents
		err := json.Unmarshal(raw, &v)
		return v, err
	case probe.Text != nil:
		var v TextResourceContents
		err := json.Unmarshal(raw, &v)
		return v, err
	}
	return nil, errors.New(`protocol: resource contents need "text" or "blob"`)
}

// ResourceContentsList is a list of resource bodies. A nil list is written as [].
type ResourceContentsList []ResourceContents

func (l ResourceContentsList) MarshalJSON() ([]byte, error) {
	if l == nil {
		return []byte("[]"), nil
	}
	return json.Marshal([]ResourceContents(l))
}

func (l *ResourceContentsList) UnmarshalJSON(b []byte) error {
	var raws []json.RawMessage
	if err := json.Unmarshal(b, &raws); err != nil {
		return err
	}
	out := make(ResourceContentsList, 0, len(raws))
	for _, raw := range raws {
		c, err := decodeResourceContents(raw)
		if err != nil {
			return err
		}
		out = append(out, c)
	}
	*l = out
	return nil
}
