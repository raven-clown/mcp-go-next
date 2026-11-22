package protocol

type Resource struct {
	URI         string       `json:"uri"`
	Name        string       `json:"name"`
	Title       string       `json:"title,omitempty"`
	Description string       `json:"description,omitempty"`
	MIMEType    string       `json:"mimeType,omitempty"`
	Size        *int64       `json:"size,omitempty"`
	Icons       []Icon       `json:"icons,omitempty"`
	Annotations *Annotations `json:"annotations,omitempty"`
	Meta        Object       `json:"_meta,omitempty"`
}

type ResourceTemplate struct {
	URITemplate string       `json:"uriTemplate"`
	Name        string       `json:"name"`
	Title       string       `json:"title,omitempty"`
	Description string       `json:"description,omitempty"`
	MIMEType    string       `json:"mimeType,omitempty"`
	Icons       []Icon       `json:"icons,omitempty"`
	Annotations *Annotations `json:"annotations,omitempty"`
	Meta        Object       `json:"_meta,omitempty"`
}

type ReadResourceParams struct {
	Meta           RequestMeta    `json:"_meta"`
	URI            string         `json:"uri"`
	InputResponses InputResponses `json:"inputResponses,omitempty"`
	RequestState   string         `json:"requestState,omitempty"`
}

type ReadResourceResult struct {
	ResultBase
	CacheFields
	Contents ResourceContentsList `json:"contents"`
}

type ListResourcesResult struct {
	ResultBase
	CacheFields
	Resources  List[Resource] `json:"resources"`
	NextCursor string         `json:"nextCursor,omitempty"`
}

type ListResourceTemplatesResult struct {
	ResultBase
	CacheFields
	ResourceTemplates List[ResourceTemplate] `json:"resourceTemplates"`
	NextCursor        string                 `json:"nextCursor,omitempty"`
}
