package protocol

import (
	"encoding/json"
	"errors"
)

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
	c, err := decodeContent(w.Content, false)
	if err != nil {
		return err
	}
	*m = PromptMessage{Role: w.Role, Content: c}
	return nil
}

func (m PromptMessage) MarshalJSON() ([]byte, error) {
	type plain PromptMessage
	if m.Content == nil {
		return nil, errors.New("protocol: a prompt message needs content")
	}
	return json.Marshal(plain(m))
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
	Description string              `json:"description,omitempty"`
	Messages    List[PromptMessage] `json:"messages"`
}

type ListPromptsResult struct {
	ResultBase
	CacheFields
	Prompts    List[Prompt] `json:"prompts"`
	NextCursor string       `json:"nextCursor,omitempty"`
}
