package protocol

import "encoding/json"

// Tool keeps both schemas raw, exactly as written or received.
type Tool struct {
	Name         string           `json:"name"`
	Title        string           `json:"title,omitempty"`
	Description  string           `json:"description,omitempty"`
	Icons        []Icon           `json:"icons,omitempty"`
	InputSchema  json.RawMessage  `json:"inputSchema"`
	OutputSchema json.RawMessage  `json:"outputSchema,omitempty"`
	Annotations  *ToolAnnotations `json:"annotations,omitempty"`
	Meta         Object           `json:"_meta,omitempty"`
}

// The hints are pointers because the spec gives some of them a default of true.
type ToolAnnotations struct {
	Title           string `json:"title,omitempty"`
	ReadOnlyHint    *bool  `json:"readOnlyHint,omitempty"`
	DestructiveHint *bool  `json:"destructiveHint,omitempty"`
	IdempotentHint  *bool  `json:"idempotentHint,omitempty"`
	OpenWorldHint   *bool  `json:"openWorldHint,omitempty"`
}

type CallToolParams struct {
	Meta           RequestMeta     `json:"_meta"`
	Name           string          `json:"name"`
	Arguments      json.RawMessage `json:"arguments,omitempty"`
	InputResponses InputResponses  `json:"inputResponses,omitempty"`
	RequestState   string          `json:"requestState,omitempty"`
}

type CallToolResult struct {
	ResultBase
	Content           ContentList     `json:"content"`
	StructuredContent json.RawMessage `json:"structuredContent,omitempty"`
	IsError           bool            `json:"isError,omitempty"`
}

type PaginatedParams struct {
	Meta   RequestMeta `json:"_meta"`
	Cursor string      `json:"cursor,omitempty"`
}

type ListToolsResult struct {
	ResultBase
	CacheFields
	Tools      []Tool `json:"tools"`
	NextCursor string `json:"nextCursor,omitempty"`
}
