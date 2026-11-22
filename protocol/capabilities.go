package protocol

type Icon struct {
	Src      string   `json:"src"`
	MIMEType string   `json:"mimeType,omitempty"`
	Sizes    []string `json:"sizes,omitempty"`
	Theme    string   `json:"theme,omitempty"`
}

type Implementation struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	WebsiteURL  string `json:"websiteUrl,omitempty"`
	Icons       []Icon `json:"icons,omitempty"`
}

// A capability set to a non-nil pointer is advertised, even when it is an empty object.
type ClientCapabilities struct {
	Experimental map[string]Object      `json:"experimental,omitempty"`
	Roots        *Object                `json:"roots,omitempty"`
	Sampling     *SamplingCapability    `json:"sampling,omitempty"`
	Elicitation  *ElicitationCapability `json:"elicitation,omitempty"`
	Extensions   map[string]Object      `json:"extensions,omitempty"`
}

type SamplingCapability struct {
	Context *Object `json:"context,omitempty"`
	Tools   *Object `json:"tools,omitempty"`
}

// An empty ElicitationCapability means form mode only.
type ElicitationCapability struct {
	Form *Object `json:"form,omitempty"`
	URL  *Object `json:"url,omitempty"`
}

type ServerCapabilities struct {
	Experimental map[string]Object      `json:"experimental,omitempty"`
	Logging      *Object                `json:"logging,omitempty"`
	Completions  *Object                `json:"completions,omitempty"`
	Prompts      *ListChangedCapability `json:"prompts,omitempty"`
	Resources    *ResourcesCapability   `json:"resources,omitempty"`
	Tools        *ListChangedCapability `json:"tools,omitempty"`
	Extensions   map[string]Object      `json:"extensions,omitempty"`
}

type ListChangedCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

type ResourcesCapability struct {
	Subscribe   bool `json:"subscribe,omitempty"`
	ListChanged bool `json:"listChanged,omitempty"`
}
