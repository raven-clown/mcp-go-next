package protocol

const (
	RefPrompt   = "ref/prompt"
	RefResource = "ref/resource"
)

// CompleteReference is a prompt reference (Type RefPrompt, Name) or a resource template reference (Type RefResource, URI).
type CompleteReference struct {
	Type  string `json:"type"`
	Name  string `json:"name,omitempty"`
	Title string `json:"title,omitempty"`
	URI   string `json:"uri,omitempty"`
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
	Values  []string `json:"values"`
	Total   *int     `json:"total,omitempty"`
	HasMore bool     `json:"hasMore,omitempty"`
}

type CompleteResult struct {
	ResultBase
	Completion Completion `json:"completion"`
}
