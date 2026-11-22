package protocol

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

type schemaDef struct {
	Properties map[string]json.RawMessage `json:"properties"`
	Required   []string                   `json:"required"`
}

func loadSchemaDefs(t *testing.T) map[string]schemaDef {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "2026-07-28", "schema.json"))
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	var s struct {
		Defs map[string]schemaDef `json:"$defs"`
	}
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	return s.Defs
}

type goField struct {
	name      string
	omitEmpty bool
}

// jsonFields lists the JSON names a struct encodes, following embedded structs like encoding/json does.
func jsonFields(t reflect.Type) []goField {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	var out []goField
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		tag := f.Tag.Get("json")
		if tag == "-" {
			continue
		}
		name, opts, _ := strings.Cut(tag, ",")
		if f.Anonymous && name == "" {
			out = append(out, jsonFields(f.Type)...)
			continue
		}
		if !f.IsExported() {
			continue
		}
		if name == "" {
			name = f.Name
		}
		out = append(out, goField{name: name, omitEmpty: strings.Contains(opts, "omitempty") || strings.Contains(opts, "omitzero")})
	}
	return out
}

// schemaFieldCases is extended by every task that adds protocol types.
// extra names fields a type writes in its own MarshalJSON, such as a content block's "type".
// optional names required fields a type may leave out, with the reason in a comment on the row.
var schemaFieldCases = []struct {
	value    any
	def      string
	extra    []string
	optional []string
}{
	{value: Implementation{}, def: "Implementation"},
	{value: Icon{}, def: "Icon"},
	{value: ClientCapabilities{}, def: "ClientCapabilities"},
	{value: ServerCapabilities{}, def: "ServerCapabilities"},
	{value: EmptyResult{}, def: "Result"},

	{value: Annotations{}, def: "Annotations"},
	{value: TextContent{}, def: "TextContent", extra: []string{"type"}},
	{value: ImageContent{}, def: "ImageContent", extra: []string{"type"}},
	{value: AudioContent{}, def: "AudioContent", extra: []string{"type"}},
	{value: ResourceLink{}, def: "ResourceLink", extra: []string{"type"}},
	{value: EmbeddedResource{}, def: "EmbeddedResource", extra: []string{"type"}},
	{value: ToolUseContent{}, def: "ToolUseContent", extra: []string{"type"}},
	{value: ToolResultContent{}, def: "ToolResultContent", extra: []string{"type"}},
	{value: TextResourceContents{}, def: "TextResourceContents"},
	{value: BlobResourceContents{}, def: "BlobResourceContents"},

	{value: Tool{}, def: "Tool"},
	{value: ToolAnnotations{}, def: "ToolAnnotations"},
	{value: CallToolParams{}, def: "CallToolRequestParams"},
	{value: CallToolResult{}, def: "CallToolResult"},
	{value: ListToolsResult{}, def: "ListToolsResult"},
	{value: PaginatedParams{}, def: "PaginatedRequestParams"},

	{value: Resource{}, def: "Resource"},
	{value: ResourceTemplate{}, def: "ResourceTemplate"},
	{value: ReadResourceParams{}, def: "ReadResourceRequestParams"},
	{value: ReadResourceResult{}, def: "ReadResourceResult"},
	{value: ListResourcesResult{}, def: "ListResourcesResult"},
	{value: ListResourceTemplatesResult{}, def: "ListResourceTemplatesResult"},

	{value: Prompt{}, def: "Prompt"},
	{value: PromptArgument{}, def: "PromptArgument"},
	{value: PromptMessage{}, def: "PromptMessage"},
	{value: GetPromptParams{}, def: "GetPromptRequestParams"},
	{value: GetPromptResult{}, def: "GetPromptResult"},
	{value: ListPromptsResult{}, def: "ListPromptsResult"},

	{value: CompleteParams{}, def: "CompleteRequestParams"},
	{value: CompleteResult{}, def: "CompleteResult"},

	{value: DiscoverParams{}, def: "RequestParams"},
	{value: DiscoverResult{}, def: "DiscoverResult"},

	{value: SubscriptionFilter{}, def: "SubscriptionFilter"},
	{value: ListenParams{}, def: "SubscriptionsListenRequestParams"},
	// The server always sets _meta with the subscription ID on a listen result.
	{value: ListenResult{}, def: "SubscriptionsListenResult", optional: []string{"_meta"}},
	{value: SubscriptionsAcknowledgedParams{}, def: "SubscriptionsAcknowledgedNotificationParams"},

	{value: InputRequiredResult{}, def: "InputRequiredResult"},

	{value: CreateMessageParams{}, def: "CreateMessageRequestParams"},
	{value: CreateMessageResult{}, def: "CreateMessageResult"},
	{value: SamplingMessage{}, def: "SamplingMessage"},
	{value: ModelPreferences{}, def: "ModelPreferences"},
	{value: ModelHint{}, def: "ModelHint"},
	{value: ToolChoice{}, def: "ToolChoice"},

	{value: ElicitResult{}, def: "ElicitResult"},
	{value: StringSchema{}, def: "StringSchema"},
	{value: NumberSchema{}, def: "NumberSchema"},
	{value: BooleanSchema{}, def: "BooleanSchema"},
	{value: UntitledSingleSelectEnumSchema{}, def: "UntitledSingleSelectEnumSchema"},
	{value: TitledSingleSelectEnumSchema{}, def: "TitledSingleSelectEnumSchema"},
	{value: UntitledMultiSelectEnumSchema{}, def: "UntitledMultiSelectEnumSchema"},
	{value: TitledMultiSelectEnumSchema{}, def: "TitledMultiSelectEnumSchema"},
	{value: LegacyTitledEnumSchema{}, def: "LegacyTitledEnumSchema"},

	{value: Root{}, def: "Root"},
	{value: ListRootsResult{}, def: "ListRootsResult"},

	{value: ProgressParams{}, def: "ProgressNotificationParams"},
	{value: LoggingMessageParams{}, def: "LoggingMessageNotificationParams"},
	{value: CancelledParams{}, def: "CancelledNotificationParams"},
	{value: ResourceUpdatedParams{}, def: "ResourceUpdatedNotificationParams"},
	{value: NotificationParams{}, def: "NotificationParams"},
}

// Union types cover several schema definitions at once, so they are checked against the union of their fields.
var schemaUnionCases = []struct {
	value any
	defs  []string
}{
	{ElicitParams{}, []string{"ElicitRequestFormParams", "ElicitRequestURLParams"}},
	{CompleteReference{}, []string{"PromptReference", "ResourceTemplateReference"}},
}

func TestSchemaFields(t *testing.T) {
	defs := loadSchemaDefs(t)
	for _, tc := range schemaFieldCases {
		t.Run(tc.def, func(t *testing.T) {
			def, ok := defs[tc.def]
			if !ok {
				t.Fatalf("schema has no definition %q", tc.def)
			}
			fields := jsonFields(reflect.TypeOf(tc.value))
			var got, want []string
			byName := map[string]goField{}
			for _, f := range fields {
				got = append(got, f.name)
				byName[f.name] = f
			}
			got = append(got, tc.extra...)
			for name := range def.Properties {
				want = append(want, name)
			}
			sort.Strings(got)
			sort.Strings(want)
			if !reflect.DeepEqual(got, want) {
				t.Errorf("JSON fields\n got  %v\n want %v", got, want)
			}
			for _, name := range def.Required {
				if f, ok := byName[name]; ok && f.omitEmpty && !contains(tc.optional, name) {
					t.Errorf("required field %q is omitempty", name)
				}
			}
		})
	}
}

func TestSchemaUnionFields(t *testing.T) {
	defs := loadSchemaDefs(t)
	for _, tc := range schemaUnionCases {
		t.Run(strings.Join(tc.defs, "|"), func(t *testing.T) {
			wantSet := map[string]bool{}
			for _, name := range tc.defs {
				def, ok := defs[name]
				if !ok {
					t.Fatalf("schema has no definition %q", name)
				}
				for p := range def.Properties {
					wantSet[p] = true
				}
			}
			var got, want []string
			for _, f := range jsonFields(reflect.TypeOf(tc.value)) {
				got = append(got, f.name)
			}
			for p := range wantSet {
				want = append(want, p)
			}
			sort.Strings(got)
			sort.Strings(want)
			if !reflect.DeepEqual(got, want) {
				t.Errorf("JSON fields\n got  %v\n want %v", got, want)
			}
		})
	}
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func TestRequestMetaRoundTrip(t *testing.T) {
	in := `{
		"progressToken": "p-1",
		"io.modelcontextprotocol/protocolVersion": "2026-07-28",
		"io.modelcontextprotocol/clientInfo": {"name": "agent", "version": "1.0.0"},
		"io.modelcontextprotocol/clientCapabilities": {"elicitation": {"form": {}}},
		"io.modelcontextprotocol/logLevel": "warning",
		"x.example/k": 1
	}`
	var m RequestMeta
	if err := json.Unmarshal([]byte(in), &m); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if m.ProtocolVersion != LatestVersion || m.LogLevel != "warning" || string(m.ProgressToken) != `"p-1"` {
		t.Errorf("decoded %+v", m)
	}
	if m.ClientInfo == nil || m.ClientInfo.Name != "agent" {
		t.Errorf("ClientInfo = %+v", m.ClientInfo)
	}
	if m.ClientCapabilities.Elicitation == nil || m.ClientCapabilities.Elicitation.Form == nil {
		t.Errorf("ClientCapabilities = %+v", m.ClientCapabilities)
	}
	if string(m.Extra["x.example/k"]) != "1" {
		t.Errorf("Extra = %v", m.Extra)
	}
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	assertSameJSON(t, []byte(in), out)
}

func TestRequestMetaRequiredKeys(t *testing.T) {
	out, err := json.Marshal(RequestMeta{ProtocolVersion: LatestVersion})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	assertSameJSON(t, []byte(`{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientCapabilities":{}}`), out)

	var m RequestMeta
	if err := json.Unmarshal([]byte(`{"io.modelcontextprotocol/protocolVersion": 5}`), &m); err == nil {
		t.Errorf("Unmarshal accepted a numeric protocol version")
	}
}

func TestLoggingRank(t *testing.T) {
	tests := []struct {
		level LoggingLevel
		want  int
	}{
		{LogDebug, 0}, {LogInfo, 1}, {LogNotice, 2}, {LogWarning, 3},
		{LogError, 4}, {LogCritical, 5}, {LogAlert, 6}, {LogEmergency, 7},
		{"", -1}, {"verbose", -1},
	}
	for _, tt := range tests {
		if got := tt.level.Rank(); got != tt.want {
			t.Errorf("%q.Rank() = %d, want %d", tt.level, got, tt.want)
		}
	}
}

func TestResultInterfaces(t *testing.T) {
	var r Result = &EmptyResult{}
	r.Base().ResultType = ResultComplete
	out, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if string(out) != `{"resultType":"complete"}` {
		t.Errorf("EmptyResult = %s", out)
	}
	var c Cacheable = &CacheFields{}
	c.Cache().CacheScope = CachePrivate
	if c.Cache().CacheScope != CachePrivate {
		t.Errorf("Cache() does not point at the fields")
	}
	out, err = json.Marshal(CacheFields{})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if string(out) != `{"ttlMs":0,"cacheScope":""}` {
		t.Errorf("CacheFields = %s, ttlMs must always be present", out)
	}
}

func assertSameJSON(t *testing.T, want, got []byte) {
	t.Helper()
	var w, g any
	if err := json.Unmarshal(want, &w); err != nil {
		t.Fatalf("want is not JSON: %v", err)
	}
	if err := json.Unmarshal(got, &g); err != nil {
		t.Fatalf("got is not JSON: %v\n%s", err, got)
	}
	if !reflect.DeepEqual(w, g) {
		t.Fatalf("JSON differs\n got  %s\n want %s", got, want)
	}
}
