// Package bench compares mcp-go-next with mark3labs/mcp-go and modelcontextprotocol/go-sdk on the
// same inputs. It is a separate module so the core module keeps no dependencies.
//
// Both other libraries build schemas with github.com/google/jsonschema-go. To validate tool
// arguments, go-sdk uses jsonschema-go and mark3labs uses github.com/santhosh-tekuri/jsonschema/v6.
// Each benchmark starts from the raw bytes a server receives and measures only the library's
// validator core. Their servers do more per call (mark3labs hashes the schema with SHA-256 to find
// its cache entry), so these numbers favor them.
package bench

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	gojs "github.com/google/jsonschema-go/jsonschema"
	"github.com/raven-clown/mcp-go-next/jsonschema"
	santhosh "github.com/santhosh-tekuri/jsonschema/v6"
)

// searchArgs looks like the input of a typical tool that wraps an API.
type searchArgs struct {
	Query     string   `json:"query"`
	Owner     string   `json:"owner,omitempty"`
	Repo      string   `json:"repo,omitempty"`
	State     string   `json:"state,omitempty"`
	Labels    []string `json:"labels,omitempty"`
	Sort      string   `json:"sort,omitempty"`
	Direction string   `json:"direction,omitempty"`
	PerPage   int      `json:"perPage,omitempty"`
	Page      int      `json:"page,omitempty"`
	Since     *string  `json:"since,omitempty"`
}

// searchSchema is written by hand so all three validators check exactly the same keywords.
var searchSchema = []byte(`{
	"type": "object",
	"properties": {
		"query": {"type": "string", "minLength": 1, "maxLength": 256},
		"owner": {"type": "string"},
		"repo": {"type": "string"},
		"state": {"type": "string", "enum": ["open", "closed", "all"]},
		"labels": {"type": "array", "items": {"type": "string"}},
		"sort": {"type": "string", "enum": ["created", "updated", "comments"]},
		"direction": {"type": "string", "enum": ["asc", "desc"]},
		"perPage": {"type": "integer", "minimum": 1, "maximum": 100},
		"page": {"type": "integer", "minimum": 1},
		"since": {"type": ["string", "null"]}
	},
	"required": ["query"],
	"additionalProperties": false
}`)

var searchArgsJSON = []byte(`{"query":"is:open label:bug","owner":"raven-clown","repo":"mcp-go-next","state":"open","labels":["bug","p1"],"sort":"updated","direction":"desc","perPage":50,"page":2}`)

var arraySchema = []byte(`{"type":"array","items":{"type":"object","properties":{"id":{"type":"integer"},"name":{"type":"string"}},"required":["id"]}}`)

var arrayJSON = []byte("[" + strings.TrimSuffix(strings.Repeat(`{"id":1,"name":"x"},`, 5000), ",") + "]")

// A validator turns raw argument bytes into a verdict, the way a server sees them.
type validator func(instance []byte) error

func ours(tb testing.TB, schema []byte) validator {
	tb.Helper()
	s, err := jsonschema.Basic().Compile(schema)
	if err != nil {
		tb.Fatal(err)
	}
	return func(instance []byte) error { return s.Validate(instance) }
}

// goSDK mirrors go-sdk: jsonschema-go resolves the schema once, and each call unmarshals then validates.
func goSDK(tb testing.TB, schema []byte) validator {
	tb.Helper()
	var s gojs.Schema
	if err := json.Unmarshal(schema, &s); err != nil {
		tb.Fatal(err)
	}
	resolved, err := s.Resolve(nil)
	if err != nil {
		tb.Fatal(err)
	}
	return func(instance []byte) error {
		var v any
		if err := json.Unmarshal(instance, &v); err != nil {
			return err
		}
		return resolved.Validate(v)
	}
}

// mark3labs mirrors mark3labs/mcp-go: santhosh-tekuri compiles the schema once, and each call
// decodes with its UnmarshalJSON then validates.
func mark3labs(tb testing.TB, schema []byte) validator {
	tb.Helper()
	doc, err := santhosh.UnmarshalJSON(bytes.NewReader(schema))
	if err != nil {
		tb.Fatal(err)
	}
	c := santhosh.NewCompiler()
	if err := c.AddResource("schema.json", doc); err != nil {
		tb.Fatal(err)
	}
	sch, err := c.Compile("schema.json")
	if err != nil {
		tb.Fatal(err)
	}
	return func(instance []byte) error {
		v, err := santhosh.UnmarshalJSON(bytes.NewReader(instance))
		if err != nil {
			return err
		}
		return sch.Validate(v)
	}
}

var libraries = []struct {
	name string
	make func(testing.TB, []byte) validator
}{
	{"lib=mcp-go-next", ours},
	{"lib=go-sdk", goSDK},
	{"lib=mark3labs", mark3labs},
}

// TestSameVerdicts makes sure no library looks fast by checking less: all three must agree.
func TestSameVerdicts(t *testing.T) {
	cases := []struct {
		schema   []byte
		instance string
		valid    bool
	}{
		{searchSchema, string(searchArgsJSON), true},
		{searchSchema, `{"query":""}`, false},
		{searchSchema, `{"query":"x","state":"merged"}`, false},
		{searchSchema, `{"query":"x","perPage":101}`, false},
		{searchSchema, `{"query":"x","perPage":1.5}`, false},
		{searchSchema, `{"query":"x","extra":1}`, false},
		{searchSchema, `{"owner":"x"}`, false},
		{searchSchema, `{"query":"x","since":null}`, true},
		{arraySchema, string(arrayJSON), true},
		{arraySchema, `[{"id":1},{"name":"x"}]`, false},
	}
	for _, lib := range libraries {
		for _, schema := range [][]byte{searchSchema, arraySchema} {
			v := lib.make(t, schema)
			for _, c := range cases {
				if !bytes.Equal(c.schema, schema) {
					continue
				}
				if err := v([]byte(c.instance)); (err == nil) != c.valid {
					t.Errorf("%s: %.60s: err = %v, want valid=%v", lib.name, c.instance, err, c.valid)
				}
			}
		}
	}
}

func BenchmarkValidateToolArgs(b *testing.B) {
	for _, lib := range libraries {
		b.Run(lib.name, func(b *testing.B) {
			v := lib.make(b, searchSchema)
			b.SetBytes(int64(len(searchArgsJSON)))
			b.ReportAllocs()
			for b.Loop() {
				if err := v(searchArgsJSON); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkValidateArray5000(b *testing.B) {
	for _, lib := range libraries {
		b.Run(lib.name, func(b *testing.B) {
			v := lib.make(b, arraySchema)
			b.SetBytes(int64(len(arrayJSON)))
			b.ReportAllocs()
			for b.Loop() {
				if err := v(arrayJSON); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkCompileSchema(b *testing.B) {
	for _, lib := range libraries {
		b.Run(lib.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				lib.make(b, searchSchema)
			}
		})
	}
}

// BenchmarkInferSchema builds the input schema of searchArgs as JSON. Both other libraries use
// jsonschema-go for this, so it appears once.
func BenchmarkInferSchema(b *testing.B) {
	b.Run("lib=mcp-go-next", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if _, err := jsonschema.For[searchArgs](); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("lib=go-sdk+mark3labs", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			s, err := gojs.For[searchArgs](nil)
			if err != nil {
				b.Fatal(err)
			}
			if _, err := json.Marshal(s); err != nil {
				b.Fatal(err)
			}
		}
	})
}
