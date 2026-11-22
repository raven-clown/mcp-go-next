package protocol

// DiscoverParams is the params of server/discover, which carry only _meta.
type DiscoverParams struct {
	Meta RequestMeta `json:"_meta"`
}

type DiscoverResult struct {
	ResultBase
	CacheFields
	SupportedVersions List[string]       `json:"supportedVersions"`
	Capabilities      ServerCapabilities `json:"capabilities"`
	Instructions      string             `json:"instructions,omitempty"`
}
