package jsonschema

import (
	"encoding/json"
	"strings"
	"testing"
)

// searchArgs looks like the input of a typical API-wrapping tool.
type searchArgs struct {
	Query     string   `json:"query" jsonschema:"minLength=1,maxLength=256"`
	Owner     string   `json:"owner,omitempty"`
	Repo      string   `json:"repo,omitempty"`
	State     string   `json:"state,omitempty" jsonschema:"enum=open|closed|all"`
	Labels    []string `json:"labels,omitempty"`
	Sort      string   `json:"sort,omitempty" jsonschema:"enum=created|updated|comments"`
	Direction string   `json:"direction,omitempty" jsonschema:"enum=asc|desc"`
	PerPage   int      `json:"perPage,omitempty" jsonschema:"minimum=1,maximum=100"`
	Page      int      `json:"page,omitempty" jsonschema:"minimum=1"`
	Since     *string  `json:"since,omitempty"`
}

var searchInstance = json.RawMessage(`{"query":"is:open label:bug","owner":"raven-clown","repo":"mcp-go-next","state":"open","labels":["bug","p1"],"sort":"updated","direction":"desc","perPage":50,"page":2}`)

func BenchmarkFor(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if _, err := For[searchArgs](); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCompile(b *testing.B) {
	schema, err := For[searchArgs]()
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := Basic().Compile(schema); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValidateToolArgs(b *testing.B) {
	schema, err := For[searchArgs]()
	if err != nil {
		b.Fatal(err)
	}
	s, err := Basic().Compile(schema)
	if err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(searchInstance)))
	b.ReportAllocs()
	for b.Loop() {
		if err := s.Validate(searchInstance); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValidateLargeArray(b *testing.B) {
	s, err := Basic().Compile(json.RawMessage(`{"type":"array","items":{"type":"object","properties":{"id":{"type":"integer"},"name":{"type":"string"}},"required":["id"]}}`))
	if err != nil {
		b.Fatal(err)
	}
	instance := json.RawMessage("[" + strings.TrimSuffix(strings.Repeat(`{"id":1,"name":"x"},`, 5000), ",") + "]")
	b.SetBytes(int64(len(instance)))
	b.ReportAllocs()
	for b.Loop() {
		if err := s.Validate(instance); err != nil {
			b.Fatal(err)
		}
	}
}
