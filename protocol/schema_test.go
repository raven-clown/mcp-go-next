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
var schemaFieldCases = []struct {
	value any
	def   string
}{
	{Implementation{}, "Implementation"},
	{Icon{}, "Icon"},
	{ClientCapabilities{}, "ClientCapabilities"},
	{ServerCapabilities{}, "ServerCapabilities"},
	{EmptyResult{}, "Result"},
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
			for name := range def.Properties {
				want = append(want, name)
			}
			sort.Strings(got)
			sort.Strings(want)
			if !reflect.DeepEqual(got, want) {
				t.Errorf("JSON fields\n got  %v\n want %v", got, want)
			}
			for _, name := range def.Required {
				if f, ok := byName[name]; ok && f.omitEmpty {
					t.Errorf("required field %q is omitempty", name)
				}
			}
		})
	}
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
