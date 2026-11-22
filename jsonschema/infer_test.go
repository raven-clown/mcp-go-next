package jsonschema

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

var update = flag.Bool("update", false, "rewrite golden files")

func golden(t *testing.T, name string, got []byte) {
	t.Helper()
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, got, "", "  "); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, got)
	}
	pretty.WriteByte('\n')
	path := filepath.Join("testdata", name+".golden.json")
	if *update {
		if err := os.WriteFile(path, pretty.Bytes(), 0o600); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden (run with -update to create it): %v", err)
	}
	if !bytes.Equal(bytes.ReplaceAll(want, []byte("\r\n"), []byte("\n")), pretty.Bytes()) {
		t.Fatalf("%s differs from golden\n got:\n%s\nwant:\n%s", name, pretty.Bytes(), want)
	}
}

type flat struct {
	Name   string  `json:"name"`
	Age    int     `json:"age,omitempty"`
	Score  float64 // no tag: the Go name is the key
	Active bool    `json:"active"`
	Count  uint8   `json:"count"`
	Hidden string  `json:"-"`
	secret string  //nolint:unused
	Zero   int64   `json:"zero,omitzero"`
}

type nested struct {
	Owner struct {
		ID string `json:"id"`
	} `json:"owner"`
	Tags []string `json:"tags,omitempty"`
}

type tagged struct {
	City  string  `json:"city" jsonschema:"title=City,description=City name\\, in English,minLength=1,maxLength=40,pattern=^[A-Z],format=uri"`
	Unit  string  `json:"unit" jsonschema:"enum=c|f,default=c"`
	Days  int     `json:"days" jsonschema:"minimum=1,maximum=14,default=3"`
	Ratio float64 `json:"ratio" jsonschema:"enum=0.5|1.5"`
	Flags []bool  `json:"flags" jsonschema:"minItems=1,maxItems=3"`
	Quiet bool    `json:"quiet" jsonschema:"default=true"`
}

type slicesMaps struct {
	Ints   []int              `json:"ints"`
	Fixed  [3]string          `json:"fixed"`
	Scores map[string]float64 `json:"scores"`
	ByID   map[int]string     `json:"byId"`
	Blob   []byte             `json:"blob"`
	Matrix [][]int            `json:"matrix"`
}

type inner struct {
	V string `json:"v"`
}

type pointers struct {
	S   *string `json:"s"`
	In  *inner  `json:"in"`
	PP  **int   `json:"pp"`
	Opt *int    `json:"opt,omitempty"`
}

type timeAndRaw struct {
	At    time.Time       `json:"at"`
	Maybe *time.Time      `json:"maybe,omitempty"`
	Raw   json.RawMessage `json:"raw"`
	Any   any             `json:"any"`
	Num   json.Number     `json:"num"`
	AsStr int             `json:"asStr,string"`
}

type Base struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
}

type Extra struct {
	Note string `json:"note"`
	Kind string `json:"kind"`
}

type Tagged struct {
	X int `json:"x"`
}

type unexportedEmbedded struct {
	Hidden string `json:"hidden"`
}

type embedded struct {
	Base
	*Extra
	Tagged `json:"tagged"`
	unexportedEmbedded
	Own string `json:"own"`
}

func TestFor(t *testing.T) {
	tests := []struct {
		name string
		fn   func() (json.RawMessage, error)
	}{
		{"flat", For[flat]},
		{"nested", For[nested]},
		{"tags", For[tagged]},
		{"slices_maps", For[slicesMaps]},
		{"pointers", For[pointers]},
		{"time", For[timeAndRaw]},
		{"embedded", For[embedded]},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.fn()
			if err != nil {
				t.Fatalf("For: %v", err)
			}
			golden(t, tt.name, got)
		})
	}
}

func TestForIsStable(t *testing.T) {
	a, err := For[tagged]()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		b, err := For[tagged]()
		if err != nil || !bytes.Equal(a, b) {
			t.Fatalf("output changed between calls:\n%s\n%s", a, b)
		}
	}
}

type Node struct {
	Name     string `json:"name"`
	Children []Node `json:"children"`
}

type Tree struct {
	Root *Node `json:"root"`
}

func TestForRecursive(t *testing.T) {
	got, err := For[Node]()
	if err != nil {
		t.Fatalf("For: %v", err)
	}
	golden(t, "recursive", got)
	var s map[string]any
	if err := json.Unmarshal(got, &s); err != nil {
		t.Fatal(err)
	}
	if s["type"] != "object" {
		t.Errorf("root is not inline: %s", got)
	}
	if _, ok := s["$defs"].(map[string]any)["Node"]; !ok {
		t.Errorf("no $defs.Node: %s", got)
	}
	if !strings.Contains(string(got), `"$ref":"#/$defs/Node"`) {
		t.Errorf("no $ref: %s", got)
	}

	got, err = For[Tree]()
	if err != nil {
		t.Fatalf("For[Tree]: %v", err)
	}
	golden(t, "recursive_nested", got)
}

type provider struct{}

func (provider) MCPSchema() json.RawMessage { return json.RawMessage(`{"type":"object","x":1}`) }

type ptrProvider struct{}

func (*ptrProvider) MCPSchema() json.RawMessage { return json.RawMessage(`{"type":"string","y":2}`) }

type usesProviders struct {
	P provider    `json:"p"`
	Q ptrProvider `json:"q"`
}

func TestSchemaProvider(t *testing.T) {
	got, err := For[provider]()
	if err != nil || string(got) != `{"type":"object","x":1}` {
		t.Errorf("value receiver: %s, %v", got, err)
	}
	got, err = For[ptrProvider]()
	if err != nil || string(got) != `{"type":"string","y":2}` {
		t.Errorf("pointer receiver: %s, %v", got, err)
	}
	got, err = ForType(reflect.TypeOf(&ptrProvider{}))
	if err != nil || string(got) != `{"type":"string","y":2}` {
		t.Errorf("pointer type: %s, %v", got, err)
	}
	got, err = For[usesProviders]()
	if err != nil {
		t.Fatalf("For: %v", err)
	}
	golden(t, "providers", got)
}

type badProvider struct{}

func (badProvider) MCPSchema() json.RawMessage { return json.RawMessage(`{not json`) }

func TestForErrors(t *testing.T) {
	tests := []struct {
		name string
		fn   func() (json.RawMessage, error)
		want string
	}{
		{"chan", func() (json.RawMessage, error) { return For[struct{ C chan int }]() }, "chan"},
		{"func", func() (json.RawMessage, error) { return For[func()]() }, "func"},
		{"complex", func() (json.RawMessage, error) { return For[complex128]() }, "complex128"},
		{"struct map key", func() (json.RawMessage, error) { return For[map[inner]int]() }, "map key"},
		{"unknown tag key", func() (json.RawMessage, error) {
			return For[struct {
				A string `json:"a" jsonschema:"summary=x"`
			}]()
		}, "summary"},
		{"bad pattern", func() (json.RawMessage, error) {
			return For[struct {
				A string `json:"a" jsonschema:"pattern=("`
			}]()
		}, "pattern"},
		{"bad minimum", func() (json.RawMessage, error) {
			return For[struct {
				A int `json:"a" jsonschema:"minimum=abc"`
			}]()
		}, "minimum"},
		{"bad bool enum", func() (json.RawMessage, error) {
			return For[struct {
				A bool `json:"a" jsonschema:"enum=yes"`
			}]()
		}, "enum"},
		{"bad provider schema", For[badProvider], "MCPSchema"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.fn()
			if err == nil {
				t.Fatalf("For = %s, want an error", got)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error %q does not mention %q", err, tt.want)
			}
		})
	}
}

// What encoding/json writes for a zero value must pass the schema inferred for the same type.
func TestZeroValueMatchesOwnSchema(t *testing.T) {
	tests := []struct {
		name  string
		value any
	}{
		{"flat", flat{}},
		{"nested", nested{}},
		{"slices_maps", slicesMaps{}},
		{"pointers", pointers{}},
		{"time", timeAndRaw{Raw: json.RawMessage(`{}`)}},
		{"embedded", embedded{}},
		{"tree", Tree{}},
		{"node", Node{Name: "a", Children: []Node{{Name: "b"}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema, err := ForType(reflect.TypeOf(tt.value))
			if err != nil {
				t.Fatal(err)
			}
			s, err := Basic().Compile(schema)
			if err != nil {
				t.Fatalf("Compile: %v", err)
			}
			instance, err := json.Marshal(tt.value)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Validate(instance); err != nil {
				t.Fatalf("%s fails its own schema: %v\nschema: %s", instance, err, schema)
			}
		})
	}
}

func TestForOutputCompiles(t *testing.T) {
	for _, fn := range []func() (json.RawMessage, error){For[flat], For[tagged], For[embedded], For[Node], For[Tree], For[timeAndRaw]} {
		s, err := fn()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Basic().Compile(s); err != nil {
			t.Errorf("inferred schema does not compile: %v\n%s", err, s)
		}
	}
}
