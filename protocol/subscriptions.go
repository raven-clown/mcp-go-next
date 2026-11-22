package protocol

type SubscriptionFilter struct {
	ToolsListChanged      bool     `json:"toolsListChanged,omitempty"`
	PromptsListChanged    bool     `json:"promptsListChanged,omitempty"`
	ResourcesListChanged  bool     `json:"resourcesListChanged,omitempty"`
	ResourceSubscriptions []string `json:"resourceSubscriptions,omitempty"`
}

type ListenParams struct {
	Meta          RequestMeta        `json:"_meta"`
	Notifications SubscriptionFilter `json:"notifications"`
}

// ListenResult ends a listen stream gracefully. Its _meta carries MetaSubscriptionID.
type ListenResult struct {
	ResultBase
}

type SubscriptionsAcknowledgedParams struct {
	Meta          Object             `json:"_meta,omitempty"`
	Notifications SubscriptionFilter `json:"notifications"`
}
