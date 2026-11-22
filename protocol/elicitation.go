package protocol

import "encoding/json"

const (
	ElicitForm = "form"
	ElicitURL  = "url"
)

// ElicitParams covers both modes: form (Message, RequestedSchema) and url (Mode ElicitURL, Message, URL).
// An empty Mode means form.
type ElicitParams struct {
	Mode            string          `json:"mode,omitempty"`
	Message         string          `json:"message"`
	RequestedSchema json.RawMessage `json:"requestedSchema,omitempty"`
	URL             string          `json:"url,omitempty"`
}

const (
	ElicitAccept  = "accept"
	ElicitDecline = "decline"
	ElicitCancel  = "cancel"
)

type ElicitResult struct {
	Action  string `json:"action"`
	Content Object `json:"content,omitempty"`
}

// The schemas below are the only property shapes a form may ask for.

type StringSchema struct {
	Type        string  `json:"type"`
	Title       string  `json:"title,omitempty"`
	Description string  `json:"description,omitempty"`
	MinLength   *int    `json:"minLength,omitempty"`
	MaxLength   *int    `json:"maxLength,omitempty"`
	Format      string  `json:"format,omitempty"`
	Default     *string `json:"default,omitempty"`
}

type NumberSchema struct {
	Type        string   `json:"type"`
	Title       string   `json:"title,omitempty"`
	Description string   `json:"description,omitempty"`
	Minimum     *float64 `json:"minimum,omitempty"`
	Maximum     *float64 `json:"maximum,omitempty"`
	Default     *float64 `json:"default,omitempty"`
}

type BooleanSchema struct {
	Type        string `json:"type"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	Default     *bool  `json:"default,omitempty"`
}

type EnumOption struct {
	Const string `json:"const"`
	Title string `json:"title"`
}

type UntitledSingleSelectEnumSchema struct {
	Type        string   `json:"type"`
	Title       string   `json:"title,omitempty"`
	Description string   `json:"description,omitempty"`
	Enum        []string `json:"enum"`
	Default     *string  `json:"default,omitempty"`
}

type TitledSingleSelectEnumSchema struct {
	Type        string       `json:"type"`
	Title       string       `json:"title,omitempty"`
	Description string       `json:"description,omitempty"`
	OneOf       []EnumOption `json:"oneOf"`
	Default     *string      `json:"default,omitempty"`
}

type UntitledEnumItems struct {
	Type string   `json:"type"`
	Enum []string `json:"enum"`
}

type UntitledMultiSelectEnumSchema struct {
	Type        string            `json:"type"`
	Title       string            `json:"title,omitempty"`
	Description string            `json:"description,omitempty"`
	MinItems    *int              `json:"minItems,omitempty"`
	MaxItems    *int              `json:"maxItems,omitempty"`
	Items       UntitledEnumItems `json:"items"`
	Default     []string          `json:"default,omitempty"`
}

type TitledEnumItems struct {
	AnyOf []EnumOption `json:"anyOf"`
}

type TitledMultiSelectEnumSchema struct {
	Type        string          `json:"type"`
	Title       string          `json:"title,omitempty"`
	Description string          `json:"description,omitempty"`
	MinItems    *int            `json:"minItems,omitempty"`
	MaxItems    *int            `json:"maxItems,omitempty"`
	Items       TitledEnumItems `json:"items"`
	Default     []string        `json:"default,omitempty"`
}

// LegacyTitledEnumSchema uses enumNames, which newer schemas replace with oneOf.
type LegacyTitledEnumSchema struct {
	Type        string   `json:"type"`
	Title       string   `json:"title,omitempty"`
	Description string   `json:"description,omitempty"`
	Enum        []string `json:"enum"`
	EnumNames   []string `json:"enumNames,omitempty"`
	Default     *string  `json:"default,omitempty"`
}
