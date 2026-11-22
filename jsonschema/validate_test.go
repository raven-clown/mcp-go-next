package jsonschema

import (
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"testing"
)

// errorPaths returns the sorted paths of every ValidationError inside err.
func errorPaths(t *testing.T, err error) []string {
	t.Helper()
	var paths []string
	var walk func(error)
	walk = func(e error) {
		if e == nil {
			return
		}
		if joined, ok := e.(interface{ Unwrap() []error }); ok {
			for _, inner := range joined.Unwrap() {
				walk(inner)
			}
			return
		}
		var ve *ValidationError
		if !errors.As(e, &ve) {
			t.Fatalf("error %T is not a *ValidationError: %v", e, e)
		}
		paths = append(paths, ve.Path)
	}
	walk(err)
	sort.Strings(paths)
	return paths
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name     string
		schema   string
		instance string
		want     []string // nil means valid
	}{
		{"const keeps big integers exact", `{"const":9007199254740993}`, `9007199254740992`, []string{""}},
		{"const matches the same big integer", `{"const":9007199254740993}`, `9007199254740993`, nil},
		{"integer accepts 1.0", `{"type":"integer"}`, `1.0`, nil},
		{"integer rejects 1.5", `{"type":"integer"}`, `1.5`, []string{""}},
		{"integer accepts an exponent", `{"type":"integer"}`, `1e3`, nil},
		{"number accepts an integer", `{"type":"number"}`, `3`, nil},
		{"case-different names are distinct",
			`{"type":"object","properties":{"Name":{"type":"string"},"name":{"type":"integer"}},"required":["Name","name"]}`,
			`{"Name":1,"name":"x"}`, []string{"/Name", "/name"}},
		{"required missing", `{"type":"object","required":["a","b"]}`, `{"a":1}`, []string{""}},
		{"required string given null",
			`{"type":"object","properties":{"name":{"type":"string"}},"required":["name"]}`,
			`{"name":null}`, []string{"/name"}},
		{"unknown keyword ignored", `{"type":"string","x-mcp-header":"X"}`, `"v"`, nil},
		{"pattern matches", `{"type":"string","pattern":"^[a-z]+$"}`, `"abc"`, nil},
		{"pattern fails", `{"type":"string","pattern":"^[a-z]+$"}`, `"ABC"`, []string{""}},
		{"additionalProperties false", `{"type":"object","properties":{"a":{}},"additionalProperties":false}`, `{"a":1,"extra":2}`, []string{"/extra"}},
		{"additionalProperties schema", `{"type":"object","additionalProperties":{"type":"integer"}}`, `{"a":1,"b":"x"}`, []string{"/b"}},
		{"maxLength counts runes", `{"type":"string","maxLength":2}`, `"üü"`, nil},
		{"minLength counts runes", `{"type":"string","minLength":3}`, `"üü"`, []string{""}},
		{"type list", `{"type":["string","null"]}`, `null`, nil},
		{"type list rejects", `{"type":["string","null"]}`, `1`, []string{""}},
		{"enum mixes types", `{"enum":["a",1,true,null]}`, `1.0`, nil},
		{"enum rejects", `{"enum":["a","b"]}`, `"c"`, []string{""}},
		{"const object ignores key order", `{"const":{"a":1,"b":[1,2]}}`, `{"b":[1,2],"a":1}`, nil},
		{"minimum", `{"minimum":0}`, `-0.0001`, []string{""}},
		{"maximum", `{"maximum":10}`, `10`, nil},
		{"exclusiveMinimum", `{"exclusiveMinimum":0}`, `0`, []string{""}},
		{"exclusiveMaximum", `{"exclusiveMaximum":10}`, `9.999`, nil},
		{"minimum ignores non-numbers", `{"minimum":5}`, `"x"`, nil},
		{"items", `{"type":"array","items":{"type":"string"}}`, `["a",2,"c",4]`, []string{"/1", "/3"}},
		{"minItems", `{"minItems":2}`, `[1]`, []string{""}},
		{"maxItems", `{"maxItems":1}`, `[1,2]`, []string{""}},
		{"nested paths escape", `{"properties":{"a/b":{"properties":{"c~d":{"type":"string"}}}}}`, `{"a/b":{"c~d":5}}`, []string{"/a~1b/c~0d"}},
		{"local ref", `{"$defs":{"pos":{"type":"integer","minimum":0}},"properties":{"n":{"$ref":"#/$defs/pos"}}}`, `{"n":-1}`, []string{"/n"}},
		{"recursive ref", `{"$defs":{"node":{"type":"object","properties":{"kids":{"type":"array","items":{"$ref":"#/$defs/node"}},"v":{"type":"integer"}}}},"$ref":"#/$defs/node"}`, `{"kids":[{"kids":[{"v":"x"}]}]}`, []string{"/kids/0/kids/0/v"}},
		{"true schema", `true`, `{"anything":1}`, nil},
		{"false schema", `false`, `1`, []string{""}},
		{"false property", `{"properties":{"a":false}}`, `{"a":1}`, []string{"/a"}},
		{"several errors at once", `{"type":"object","properties":{"a":{"type":"string"},"b":{"type":"integer"}}}`, `{"a":1,"b":"x"}`, []string{"/a", "/b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := Basic().Compile(json.RawMessage(tt.schema))
			if err != nil {
				t.Fatalf("Compile: %v", err)
			}
			err = s.Validate(json.RawMessage(tt.instance))
			if tt.want == nil {
				if err != nil {
					t.Fatalf("Validate: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Validate accepted %s", tt.instance)
			}
			got := errorPaths(t, err)
			if strings.Join(got, ",") != strings.Join(tt.want, ",") {
				t.Fatalf("error paths = %q, want %q (%v)", got, tt.want, err)
			}
		})
	}
}

func TestValidationErrorString(t *testing.T) {
	if got := (&ValidationError{Path: "/a", Message: "must be a string"}).Error(); got != "/a: must be a string" {
		t.Errorf("Error() = %q", got)
	}
	if got := (&ValidationError{Message: "must be an object"}).Error(); got != "must be an object" {
		t.Errorf("root Error() = %q", got)
	}
}

func TestValidateBadInstance(t *testing.T) {
	s, err := Basic().Compile(json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	for _, in := range []string{``, `{`, `{}{}`} {
		if err := s.Validate(json.RawMessage(in)); err == nil {
			t.Errorf("Validate(%q) accepted invalid JSON", in)
		}
	}
}

func TestCompileRejects(t *testing.T) {
	tests := []struct {
		name   string
		schema string
		want   string
	}{
		{"bad pattern", `{"pattern":"("}`, "pattern"},
		{"remote ref", `{"$ref":"https://example.com/s.json"}`, "$ref"},
		{"relative ref", `{"$ref":"other.json#/a"}`, "$ref"},
		{"unresolved ref", `{"$ref":"#/$defs/missing"}`, "$ref"},
		{"not a schema", `"x"`, "schema"},
		{"bad json", `{`, "schema"},
		{"bad type name", `{"type":"text"}`, "type"},
		{"type not a string", `{"type":5}`, "type"},
		{"minLength not an integer", `{"minLength":1.5}`, "minLength"},
		{"negative maxItems", `{"maxItems":-1}`, "maxItems"},
		{"minimum not a number", `{"minimum":"a"}`, "minimum"},
		{"required not strings", `{"required":[1]}`, "required"},
		{"properties not an object", `{"properties":[]}`, "properties"},
		{"enum not an array", `{"enum":"a"}`, "enum"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Basic().Compile(json.RawMessage(tt.schema))
			if err == nil {
				t.Fatalf("Compile(%s) succeeded", tt.schema)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error %q does not mention %q", err, tt.want)
			}
		})
	}
}

func TestCompileBadPattern(t *testing.T) {
	if _, err := Basic().Compile(json.RawMessage(`{"pattern":"("}`)); err == nil {
		t.Fatal("Compile accepted a bad pattern")
	}
}

func TestCompileRejectsRemoteRef(t *testing.T) {
	if _, err := Basic().Compile(json.RawMessage(`{"$ref":"https://example.com/s.json"}`)); err == nil {
		t.Fatal("Compile accepted a remote $ref")
	}
}

func nestedSchema(depth int) string {
	return strings.Repeat(`{"items":`, depth) + `{}` + strings.Repeat(`}`, depth)
}

func TestBounds(t *testing.T) {
	if _, err := Basic(MaxDepth(64)).Compile(json.RawMessage(nestedSchema(65))); err == nil {
		t.Error("a schema nested 65 levels compiled with MaxDepth(64)")
	}
	if _, err := Basic(MaxDepth(64)).Compile(json.RawMessage(nestedSchema(60))); err != nil {
		t.Errorf("a schema nested 60 levels failed: %v", err)
	}

	s, err := Basic(MaxDepth(64)).Compile(json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	deep := strings.Repeat("[", 65) + strings.Repeat("]", 65)
	if err := s.Validate(json.RawMessage(deep)); err == nil {
		t.Error("an instance nested 65 arrays deep passed with MaxDepth(64)")
	}

	s, err = Basic(MaxSteps(100)).Compile(json.RawMessage(`{"type":"array","items":{"type":"integer"}}`))
	if err != nil {
		t.Fatal(err)
	}
	items := "[" + strings.TrimSuffix(strings.Repeat("1,", 1000), ",") + "]"
	err = s.Validate(json.RawMessage(items))
	if err == nil {
		t.Fatal("1,000 items passed with MaxSteps(100)")
	}
	paths := errorPaths(t, err)
	if len(paths) != 1 || paths[0] != "" || !strings.Contains(err.Error(), "100") {
		t.Fatalf("want one root error naming the limit, got %q: %v", paths, err)
	}
}

func TestRecursiveRefTerminates(t *testing.T) {
	s, err := Basic().Compile(json.RawMessage(`{"$defs":{"a":{"$ref":"#/$defs/a"}},"$ref":"#/$defs/a"}`))
	if err != nil {
		return
	}
	if err := s.Validate(json.RawMessage(`1`)); err == nil {
		t.Fatal("a ref loop with no keywords validated instead of hitting a limit")
	}
}

func FuzzValidate(f *testing.F) {
	f.Add([]byte(`{"type":"object","properties":{"a":{"type":"string","maxLength":3}},"required":["a"]}`), []byte(`{"a":"xy"}`))
	f.Add([]byte(`{"$defs":{"n":{"items":{"$ref":"#/$defs/n"}}},"$ref":"#/$defs/n"}`), []byte(`[[[[]]]]`))
	f.Add([]byte(`{"enum":[1,"a",null],"pattern":"^a"}`), []byte(`"a"`))
	f.Fuzz(func(t *testing.T, schema, instance []byte) {
		s, err := Basic(MaxSteps(10_000)).Compile(schema)
		if err != nil {
			return
		}
		_ = s.Validate(instance)
	})
}
