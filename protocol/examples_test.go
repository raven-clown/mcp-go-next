package protocol

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// exampleFactories maps a folder under testdata/2026-07-28/examples to the Go type its files decode into.
var exampleFactories = map[string]func() any{
	"ClientCapabilities": func() any { return new(ClientCapabilities) },
	"ServerCapabilities": func() any { return new(ServerCapabilities) },
}

// exampleSkipped lists folders whose Go types do not exist yet, with the reason.
var exampleSkipped = map[string]string{
	"AudioContent":                          "Task 4",
	"BlobResourceContents":                  "Task 4",
	"BooleanSchema":                         "Task 4",
	"CallToolRequest":                       "Task 4",
	"CallToolRequestParams":                 "Task 4",
	"CallToolResult":                        "Task 4",
	"CallToolResultResponse":                "Task 4",
	"CancelledNotification":                 "Task 4",
	"CancelledNotificationParams":           "Task 4",
	"CompleteRequest":                       "Task 4",
	"CompleteRequestParams":                 "Task 4",
	"CompleteResult":                        "Task 4",
	"CompleteResultResponse":                "Task 4",
	"CreateMessageRequest":                  "Task 4",
	"CreateMessageRequestParams":            "Task 4",
	"CreateMessageResult":                   "Task 4",
	"DiscoverRequest":                       "Task 4",
	"DiscoverResult":                        "Task 4",
	"DiscoverResultResponse":                "Task 4",
	"ElicitRequest":                         "Task 4",
	"ElicitRequestFormParams":               "Task 4",
	"ElicitRequestURLParams":                "Task 4",
	"ElicitResult":                          "Task 4",
	"EmbeddedResource":                      "Task 4",
	"GetPromptRequest":                      "Task 4",
	"GetPromptRequestParams":                "Task 4",
	"GetPromptResult":                       "Task 4",
	"GetPromptResultResponse":               "Task 4",
	"HeaderMismatchError":                   "Task 4",
	"ImageContent":                          "Task 4",
	"InputRequests":                         "Task 4",
	"InputRequiredResult":                   "Task 4",
	"InputResponses":                        "Task 4",
	"InternalError":                         "Task 4",
	"InvalidParamsError":                    "Task 4",
	"ListPromptsRequest":                    "Task 4",
	"ListPromptsResult":                     "Task 4",
	"ListPromptsResultResponse":             "Task 4",
	"ListResourceTemplatesRequest":          "Task 4",
	"ListResourceTemplatesResult":           "Task 4",
	"ListResourceTemplatesResultResponse":   "Task 4",
	"ListResourcesRequest":                  "Task 4",
	"ListResourcesResult":                   "Task 4",
	"ListResourcesResultResponse":           "Task 4",
	"ListRootsRequest":                      "Task 4",
	"ListRootsResult":                       "Task 4",
	"ListToolsRequest":                      "Task 4",
	"ListToolsResult":                       "Task 4",
	"ListToolsResultResponse":               "Task 4",
	"LoggingMessageNotification":            "Task 4",
	"LoggingMessageNotificationParams":      "Task 4",
	"MethodNotFoundError":                   "Task 4",
	"MissingRequiredClientCapabilityError":  "Task 4",
	"ModelPreferences":                      "Task 4",
	"NumberSchema":                          "Task 4",
	"PaginatedRequestParams":                "Task 4",
	"ParseError":                            "Task 4",
	"ProgressNotification":                  "Task 4",
	"ProgressNotificationParams":            "Task 4",
	"PromptListChangedNotification":         "Task 4",
	"ReadResourceRequest":                   "Task 4",
	"ReadResourceResult":                    "Task 4",
	"ReadResourceResultResponse":            "Task 4",
	"Resource":                              "Task 4",
	"ResourceLink":                          "Task 4",
	"ResourceListChangedNotification":       "Task 4",
	"ResourceUpdatedNotification":           "Task 4",
	"ResourceUpdatedNotificationParams":     "Task 4",
	"Root":                                  "Task 4",
	"SamplingMessage":                       "Task 4",
	"StringSchema":                          "Task 4",
	"SubscriptionsAcknowledgedNotification": "Task 4",
	"SubscriptionsListenRequest":            "Task 4",
	"SubscriptionsListenResult":             "Task 4",
	"SubscriptionsListenResultResponse":     "Task 4",
	"TextContent":                           "Task 4",
	"TextResourceContents":                  "Task 4",
	"TitledMultiSelectEnumSchema":           "Task 4",
	"TitledSingleSelectEnumSchema":          "Task 4",
	"Tool":                                  "Task 4",
	"ToolListChangedNotification":           "Task 4",
	"ToolResultContent":                     "Task 4",
	"ToolUseContent":                        "Task 4",
	"UnsupportedProtocolVersionError":       "Task 4",
	"UntitledMultiSelectEnumSchema":         "Task 4",
	"UntitledSingleSelectEnumSchema":        "Task 4",
}

func TestExamplesRoundTrip(t *testing.T) {
	root := filepath.Join("testdata", "2026-07-28", "examples")
	dirs, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read examples: %v", err)
	}
	if len(dirs) == 0 {
		t.Fatal("no example folders")
	}
	for _, dir := range dirs {
		if !dir.IsDir() {
			continue
		}
		name := dir.Name()
		factory, ok := exampleFactories[name]
		if !ok {
			if _, skipped := exampleSkipped[name]; !skipped {
				t.Errorf("example folder %q has no Go type and no skip reason", name)
			}
			continue
		}
		files, err := filepath.Glob(filepath.Join(root, name, "*.json"))
		if err != nil || len(files) == 0 {
			t.Errorf("folder %q has no JSON files", name)
			continue
		}
		for _, file := range files {
			t.Run(name+"/"+filepath.Base(file), func(t *testing.T) {
				in, err := os.ReadFile(file)
				if err != nil {
					t.Fatalf("read: %v", err)
				}
				v := factory()
				if err := json.Unmarshal(in, v); err != nil {
					t.Fatalf("decode into %T: %v", v, err)
				}
				out, err := json.Marshal(v)
				if err != nil {
					t.Fatalf("encode: %v", err)
				}
				assertSameJSON(t, in, out)
			})
		}
	}
	for name := range exampleFactories {
		if _, ok := exampleSkipped[name]; ok {
			t.Errorf("folder %q is both mapped and skipped", name)
		}
	}
}
