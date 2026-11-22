package protocol

import "encoding/json"

type Prompt struct {
	Name        string           `json:"name"`
	Title       string           `json:"title,omitempty"`
	Description string           `json:"description,omitempty"`
	Arguments   []PromptArgument `json:"arguments,omitempty"`
	Icons       []Icon           `json:"icons,omitempty"`
	Meta        Object           `json:"_meta,omitempty"`
}

type PromptArgument struct {
	Name        string `json:"name"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

type PromptMessage struct {
	Role    Role    `json:"role"`
	Content Content `json:"content"`
}

func (m *PromptMessage) UnmarshalJSON(b []byte) error {
	var w struct {
		Role    Role            `json:"role"`
		Content json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	c, err := decodeContent(w.Content)
	if err != nil {
		return err
	}
	*m = PromptMessage{Role: w.Role, Content: c}
	return nil
}

type GetPromptParams struct {
	Meta           RequestMeta       `json:"_meta"`
	Name           string            `json:"name"`
	Arguments      map[string]string `json:"arguments,omitempty"`
	InputResponses InputResponses    `json:"inputResponses,omitempty"`
	RequestState   string            `json:"requestState,omitempty"`
}

type GetPromptResult struct {
	ResultBase
	Description string          `json:"description,omitempty"`
	Messages    []PromptMessage `json:"messages"`
}

type ListPromptsResult struct {
	ResultBase
	CacheFields
	Prompts    []Prompt `json:"prompts"`
	NextCursor string   `json:"nextCursor,omitempty"`
}
