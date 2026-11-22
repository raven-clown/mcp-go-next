package protocol

import (
	"encoding/json"
	"errors"
)

const (
	RefPrompt   = "ref/prompt"
	RefResource = "ref/resource"
)

// CompleteReference is a prompt reference (Type RefPrompt, Name) or a resource template reference
// (Type RefResource, URI). One type for both keeps CompleteParams simple; MarshalJSON enforces each shape.
type CompleteReference struct {
	Type  string `json:"type"`
	Name  string `json:"name,omitempty"`
	Title string `json:"title,omitempty"`
	URI   string `json:"uri,omitempty"`
}

func (r CompleteReference) MarshalJSON() ([]byte, error) {
	type plain CompleteReference
	switch {
	case r.Type == RefPrompt && r.Name == "":
		return nil, errors.New("protocol: a prompt reference needs a name")
	case r.Type == RefResource && r.URI == "":
		return nil, errors.New("protocol: a resource reference needs a uri")
	case r.Type != RefPrompt && r.Type != RefResource:
		return nil, errors.New(`protocol: a completion reference type must be "ref/prompt" or "ref/resource"`)
	}
	return json.Marshal(plain(r))
}

type CompleteArgument struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type CompleteContext struct {
	Arguments map[string]string `json:"arguments,omitempty"`
}

type CompleteParams struct {
	Meta     RequestMeta       `json:"_meta"`
	Ref      CompleteReference `json:"ref"`
	Argument CompleteArgument  `json:"argument"`
	Context  *CompleteContext  `json:"context,omitempty"`
}

type Completion struct {
	Values  List[string] `json:"values"`
	Total   *int         `json:"total,omitempty"`
	HasMore bool         `json:"hasMore,omitempty"`
}

type CompleteResult struct {
	ResultBase
	Completion Completion `json:"completion"`
}
