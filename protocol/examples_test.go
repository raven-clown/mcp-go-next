package protocol

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// codec decodes an example into Go types and encodes it again.
type codec func(in []byte) ([]byte, error)

func plain[T any]() codec {
	return func(in []byte) ([]byte, error) {
		v := new(T)
		if err := json.Unmarshal(in, v); err != nil {
			return nil, err
		}
		return json.Marshal(v)
	}
}

// envelope is a JSON-RPC message as the examples write it. The inner part goes through a Go type.
type envelope struct {
	JSONRPC string          `json:"jsonrpc,omitempty"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   json.RawMessage `json:"error,omitempty"`
}

func wrapped(inner func(*envelope) (*json.RawMessage, codec)) codec {
	return func(in []byte) ([]byte, error) {
		var e envelope
		if err := json.Unmarshal(in, &e); err != nil {
			return nil, err
		}
		field, c := inner(&e)
		if len(*field) > 0 {
			out, err := c(*field)
			if err != nil {
				return nil, err
			}
			*field = out
		}
		return json.Marshal(e)
	}
}

func message[P any]() codec {
	return wrapped(func(e *envelope) (*json.RawMessage, codec) { return &e.Params, plain[P]() })
}

func response[R any]() codec {
	return wrapped(func(e *envelope) (*json.RawMessage, codec) { return &e.Result, plain[R]() })
}

// responseOrInput picks InputRequiredResult when the result asks for input.
func responseOrInput[R any]() codec {
	return wrapped(func(e *envelope) (*json.RawMessage, codec) {
		var probe struct {
			ResultType ResultType `json:"resultType"`
		}
		_ = json.Unmarshal(e.Result, &probe)
		if probe.ResultType == ResultInputRequired {
			return &e.Result, plain[InputRequiredResult]()
		}
		return &e.Result, plain[R]()
	})
}

type errorObject struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func errorWithData[D any]() codec {
	return func(in []byte) ([]byte, error) {
		var e errorObject
		if err := json.Unmarshal(in, &e); err != nil {
			return nil, err
		}
		if len(e.Data) > 0 {
			out, err := plain[D]()(e.Data)
			if err != nil {
				return nil, err
			}
			e.Data = out
		}
		return json.Marshal(e)
	}
}

func errorResponse(data codec) codec {
	return wrapped(func(e *envelope) (*json.RawMessage, codec) { return &e.Error, data })
}

// exampleCodecs maps a folder under testdata/2026-07-28/examples to how its files round-trip.
var exampleCodecs = map[string]codec{
	"AudioContent":                          plain[AudioContent](),
	"BlobResourceContents":                  plain[BlobResourceContents](),
	"BooleanSchema":                         plain[BooleanSchema](),
	"CallToolRequest":                       message[CallToolParams](),
	"CallToolRequestParams":                 plain[CallToolParams](),
	"CallToolResult":                        plain[CallToolResult](),
	"CallToolResultResponse":                responseOrInput[CallToolResult](),
	"CancelledNotification":                 message[CancelledParams](),
	"CancelledNotificationParams":           plain[CancelledParams](),
	"ClientCapabilities":                    plain[ClientCapabilities](),
	"CompleteRequest":                       message[CompleteParams](),
	"CompleteRequestParams":                 plain[CompleteParams](),
	"CompleteResult":                        plain[CompleteResult](),
	"CompleteResultResponse":                response[CompleteResult](),
	"CreateMessageRequest":                  message[CreateMessageParams](),
	"CreateMessageRequestParams":            plain[CreateMessageParams](),
	"CreateMessageResult":                   plain[CreateMessageResult](),
	"DiscoverRequest":                       message[DiscoverParams](),
	"DiscoverResult":                        plain[DiscoverResult](),
	"DiscoverResultResponse":                response[DiscoverResult](),
	"ElicitRequest":                         message[ElicitParams](),
	"ElicitRequestFormParams":               plain[ElicitParams](),
	"ElicitRequestURLParams":                plain[ElicitParams](),
	"ElicitResult":                          plain[ElicitResult](),
	"EmbeddedResource":                      plain[EmbeddedResource](),
	"GetPromptRequest":                      message[GetPromptParams](),
	"GetPromptRequestParams":                plain[GetPromptParams](),
	"GetPromptResult":                       plain[GetPromptResult](),
	"GetPromptResultResponse":               responseOrInput[GetPromptResult](),
	"HeaderMismatchError":                   errorResponse(errorWithData[json.RawMessage]()),
	"ImageContent":                          plain[ImageContent](),
	"InputRequests":                         plain[InputRequests](),
	"InputRequiredResult":                   plain[InputRequiredResult](),
	"InputResponses":                        plain[InputResponses](),
	"InternalError":                         errorWithData[json.RawMessage](),
	"InvalidParamsError":                    errorWithData[json.RawMessage](),
	"ListPromptsRequest":                    message[PaginatedParams](),
	"ListPromptsResult":                     plain[ListPromptsResult](),
	"ListPromptsResultResponse":             response[ListPromptsResult](),
	"ListResourceTemplatesRequest":          message[PaginatedParams](),
	"ListResourceTemplatesResult":           plain[ListResourceTemplatesResult](),
	"ListResourceTemplatesResultResponse":   response[ListResourceTemplatesResult](),
	"ListResourcesRequest":                  message[PaginatedParams](),
	"ListResourcesResult":                   plain[ListResourcesResult](),
	"ListResourcesResultResponse":           response[ListResourcesResult](),
	"ListRootsRequest":                      message[ListRootsParams](),
	"ListRootsResult":                       plain[ListRootsResult](),
	"ListToolsRequest":                      message[PaginatedParams](),
	"ListToolsResult":                       plain[ListToolsResult](),
	"ListToolsResultResponse":               response[ListToolsResult](),
	"LoggingMessageNotification":            message[LoggingMessageParams](),
	"LoggingMessageNotificationParams":      plain[LoggingMessageParams](),
	"MethodNotFoundError":                   errorWithData[json.RawMessage](),
	"MissingRequiredClientCapabilityError":  errorResponse(errorWithData[MissingCapabilityData]()),
	"ModelPreferences":                      plain[ModelPreferences](),
	"NumberSchema":                          plain[NumberSchema](),
	"PaginatedRequestParams":                plain[PaginatedParams](),
	"ParseError":                            errorWithData[json.RawMessage](),
	"ProgressNotification":                  message[ProgressParams](),
	"ProgressNotificationParams":            plain[ProgressParams](),
	"PromptListChangedNotification":         message[NotificationParams](),
	"ReadResourceRequest":                   message[ReadResourceParams](),
	"ReadResourceResult":                    plain[ReadResourceResult](),
	"ReadResourceResultResponse":            responseOrInput[ReadResourceResult](),
	"Resource":                              plain[Resource](),
	"ResourceLink":                          plain[ResourceLink](),
	"ResourceListChangedNotification":       message[NotificationParams](),
	"ResourceUpdatedNotification":           message[ResourceUpdatedParams](),
	"ResourceUpdatedNotificationParams":     plain[ResourceUpdatedParams](),
	"Root":                                  plain[Root](),
	"SamplingMessage":                       plain[SamplingMessage](),
	"ServerCapabilities":                    plain[ServerCapabilities](),
	"StringSchema":                          plain[StringSchema](),
	"SubscriptionsAcknowledgedNotification": message[SubscriptionsAcknowledgedParams](),
	"SubscriptionsListenRequest":            message[ListenParams](),
	"SubscriptionsListenResult":             plain[ListenResult](),
	"SubscriptionsListenResultResponse":     response[ListenResult](),
	"TextContent":                           plain[TextContent](),
	"TextResourceContents":                  plain[TextResourceContents](),
	"TitledMultiSelectEnumSchema":           plain[TitledMultiSelectEnumSchema](),
	"TitledSingleSelectEnumSchema":          plain[TitledSingleSelectEnumSchema](),
	"Tool":                                  plain[Tool](),
	"ToolListChangedNotification":           message[NotificationParams](),
	"ToolResultContent":                     plain[ToolResultContent](),
	"ToolUseContent":                        plain[ToolUseContent](),
	"UnsupportedProtocolVersionError":       errorResponse(errorWithData[UnsupportedVersionData]()),
	"UntitledMultiSelectEnumSchema":         plain[UntitledMultiSelectEnumSchema](),
	"UntitledSingleSelectEnumSchema":        plain[UntitledSingleSelectEnumSchema](),
}

// exampleSkipped lists folders that are not mapped yet, with the reason. Extension and auth types arrive in later phases.
var exampleSkipped = map[string]string{}

// falseByDefault are keys the spec defines as false when absent, so an explicit false and no key are the same message.
var falseByDefault = map[string]bool{"isError": true, "hasMore": true}

// exampleDeviations are example files that break the schema. Our output follows the schema, so the
// listed keys are dropped from our output before comparing, only when they hold their zero value.
var exampleDeviations = map[string]struct {
	reason string
	keys   []string
}{
	"ReadResourceResultResponse/read-resource-result-response.json": {
		reason: "the example omits ttlMs and cacheScope, which ReadResourceResult requires",
		keys:   []string{"ttlMs", "cacheScope"},
	},
}

func dropZeroKeys(v any, keys []string) any {
	switch x := v.(type) {
	case map[string]any:
		for _, k := range keys {
			if val, ok := x[k]; ok && (val == float64(0) || val == "") {
				delete(x, k)
			}
		}
		for k, val := range x {
			x[k] = dropZeroKeys(val, keys)
		}
	case []any:
		for i, val := range x {
			x[i] = dropZeroKeys(val, keys)
		}
	}
	return v
}

func normalizeExample(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, val := range x {
			if b, ok := val.(bool); ok && !b && falseByDefault[k] {
				continue
			}
			out[k] = normalizeExample(val)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, val := range x {
			out[i] = normalizeExample(val)
		}
		return out
	}
	return v
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
		c, ok := exampleCodecs[name]
		if !ok {
			if _, skipped := exampleSkipped[name]; !skipped {
				t.Errorf("example folder %q has no codec and no skip reason", name)
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
				out, err := c(in)
				if err != nil {
					t.Fatalf("round trip: %v", err)
				}
				var want, got any
				if err := json.Unmarshal(in, &want); err != nil {
					t.Fatalf("example is not JSON: %v", err)
				}
				if err := json.Unmarshal(out, &got); err != nil {
					t.Fatalf("output is not JSON: %v\n%s", err, out)
				}
				if dev, ok := exampleDeviations[name+"/"+filepath.Base(file)]; ok {
					t.Logf("known deviation: %s", dev.reason)
					got = dropZeroKeys(got, dev.keys)
				}
				if !reflect.DeepEqual(normalizeExample(want), normalizeExample(got)) {
					t.Fatalf("round trip changed the example\n got  %s\n want %s", out, in)
				}
			})
		}
	}
	for name := range exampleCodecs {
		if _, ok := exampleSkipped[name]; ok {
			t.Errorf("folder %q is both mapped and skipped", name)
		}
	}
}
