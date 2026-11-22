package protocol

import "encoding/json"

// Sampling is deprecated in 2026-07-28 and still supported.

type SamplingMessage struct {
	Role    Role            `json:"role"`
	Content SamplingContent `json:"content"`
	Meta    Object          `json:"_meta,omitempty"`
}

type ModelHint struct {
	Name string `json:"name,omitempty"`
}

type ModelPreferences struct {
	Hints                []ModelHint `json:"hints,omitempty"`
	CostPriority         *float64    `json:"costPriority,omitempty"`
	SpeedPriority        *float64    `json:"speedPriority,omitempty"`
	IntelligencePriority *float64    `json:"intelligencePriority,omitempty"`
}

type ToolChoice struct {
	Mode string `json:"mode,omitempty"`
}

type CreateMessageParams struct {
	Messages         []SamplingMessage `json:"messages"`
	MaxTokens        int               `json:"maxTokens"`
	SystemPrompt     string            `json:"systemPrompt,omitempty"`
	IncludeContext   string            `json:"includeContext,omitempty"`
	Temperature      *float64          `json:"temperature,omitempty"`
	StopSequences    []string          `json:"stopSequences,omitempty"`
	Metadata         json.RawMessage   `json:"metadata,omitempty"`
	ModelPreferences *ModelPreferences `json:"modelPreferences,omitempty"`
	Tools            []Tool            `json:"tools,omitempty"`
	ToolChoice       *ToolChoice       `json:"toolChoice,omitempty"`
}

type CreateMessageResult struct {
	Role       Role            `json:"role"`
	Content    SamplingContent `json:"content"`
	Model      string          `json:"model"`
	StopReason string          `json:"stopReason,omitempty"`
	Meta       Object          `json:"_meta,omitempty"`
}
