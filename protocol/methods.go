package protocol

const (
	MethodDiscover              = "server/discover"
	MethodToolsList             = "tools/list"
	MethodToolsCall             = "tools/call"
	MethodResourcesList         = "resources/list"
	MethodResourceTemplatesList = "resources/templates/list"
	MethodResourcesRead         = "resources/read"
	MethodPromptsList           = "prompts/list"
	MethodPromptsGet            = "prompts/get"
	MethodComplete              = "completion/complete"
	MethodSubscriptionsListen   = "subscriptions/listen"

	// Input requests, sent inside an InputRequiredResult rather than as requests.
	MethodElicit = "elicitation/create"
	MethodSample = "sampling/createMessage"
	MethodRoots  = "roots/list"

	NotifyCancelled                 = "notifications/cancelled"
	NotifyProgress                  = "notifications/progress"
	NotifyMessage                   = "notifications/message"
	NotifyToolsListChanged          = "notifications/tools/list_changed"
	NotifyPromptsListChanged        = "notifications/prompts/list_changed"
	NotifyResourcesListChanged      = "notifications/resources/list_changed"
	NotifyResourceUpdated           = "notifications/resources/updated"
	NotifySubscriptionsAcknowledged = "notifications/subscriptions/acknowledged"
)
