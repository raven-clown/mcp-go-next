package jsonschema

import (
	"encoding/json"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func mustCompile(t *testing.T, schema string) Schema {
	t.Helper()
	s, err := Basic().Compile(json.RawMessage(schema))
	if err != nil {
		t.Fatalf("Compile(%s): %v", schema, err)
	}
	return s
}

func within(t *testing.T, limit time.Duration, f func()) {
	t.Helper()
	limit *= timeScale
	start := time.Now()
	f()
	if d := time.Since(start); d > limit {
		t.Fatalf("took %v, want under %v", d, limit)
	}
}

// allocated reports the bytes f allocates, which unlike time does not depend on the machine.
func allocated(f func()) uint64 {
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	f()
	runtime.ReadMemStats(&after)
	return after.TotalAlloc - before.TotalAlloc
}

func TestLongNumbersAreBounded(t *testing.T) {
	long := strings.Repeat("7", 1<<20)
	cases := []struct {
		schema   string
		instance string
		valid    bool
	}{
		{`{"type":"integer"}`, long, true},
		{`{"type":"integer"}`, "0." + long, false},
		{`{"enum":[1,2,3,4,5,6,7,8,9,10]}`, long, false},
		{`{"minimum":0}`, long, false},
		{`{"const":1}`, long, false},
	}
	for _, c := range cases {
		s := mustCompile(t, c.schema)
		within(t, time.Second, func() {
			if err := s.Validate(json.RawMessage(c.instance)); (err == nil) != c.valid {
				t.Errorf("%s with a 1 MiB number: err = %v, want valid=%v", c.schema, err, c.valid)
			}
		})
	}
}

func TestIntegerTypeFromText(t *testing.T) {
	s := mustCompile(t, `{"type":"integer"}`)
	for _, ok := range []string{`1e1001`, `10.000`, `1.5e1`, `-0`, `12E+2`} {
		if err := s.Validate(json.RawMessage(ok)); err != nil {
			t.Errorf("%s: %v", ok, err)
		}
	}
	for _, bad := range []string{`1.05e1`, `1e-1`, `0.5`} {
		if err := s.Validate(json.RawMessage(bad)); err == nil {
			t.Errorf("%s accepted as an integer", bad)
		}
	}
}

func TestErrorsAreCapped(t *testing.T) {
	s := mustCompile(t, `{"items":{"type":"integer"}}`)
	items := `[` + strings.TrimSuffix(strings.Repeat(`"x",`, 10_000), ",") + `]`
	err := s.Validate(json.RawMessage(items))
	if err == nil {
		t.Fatal("no error")
	}
	var n int
	var walk func(error)
	walk = func(e error) {
		if j, ok := e.(interface{ Unwrap() []error }); ok {
			for _, inner := range j.Unwrap() {
				walk(inner)
			}
			return
		}
		n++
	}
	walk(err)
	if n > 101 {
		t.Fatalf("got %d errors, want at most 101", n)
	}
	if !strings.Contains(err.Error(), "more") {
		t.Fatalf("the last error does not say more were left out: %.200s", err)
	}
}

func TestCappedErrorsAreStable(t *testing.T) {
	s := mustCompile(t, `{"additionalProperties":{"type":"integer"}}`)
	var b strings.Builder
	b.WriteString("{")
	for i := 0; i < 300; i++ {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(`"k` + strings.Repeat("x", i%7) + string(rune('a'+i%26)) + `` + `` + `":"no"`)
	}
	b.WriteString("}")
	first := s.Validate(json.RawMessage(b.String())).Error()
	for i := 0; i < 20; i++ {
		if got := s.Validate(json.RawMessage(b.String())).Error(); got != first {
			t.Fatalf("run %d reported different errors", i)
		}
	}
}

// Failures past the error cap must not build their long paths.
func TestManyFailuresUnderLongPaths(t *testing.T) {
	key := strings.Repeat("k", 50_000)
	var b strings.Builder
	for i := 0; i < 50; i++ {
		b.WriteString(`{"` + key + `":`)
	}
	b.WriteString("[" + strings.TrimSuffix(strings.Repeat(`"x",`, 20_000), ",") + "]")
	b.WriteString(strings.Repeat("}", 50))
	instance := json.RawMessage(b.String())
	// Building each path would allocate about 2.5 MB per failure, so 20,000 failures would need ~50 GB.
	const limit = 200 << 20
	s := mustCompile(t, `{"$defs":{"n":{"anyOf":[{"type":"array","items":{"type":"integer"}},{"additionalProperties":{"$ref":"#/$defs/n"}}]}},"additionalProperties":{"$ref":"#/$defs/n"}}`)
	if n := allocated(func() { _ = s.Validate(instance) }); n > limit {
		t.Errorf("anyOf case allocated %d MB", n>>20)
	}
	s = mustCompile(t, `{"$defs":{"n":{"type":["object","array"],"items":{"type":"integer"},"additionalProperties":{"$ref":"#/$defs/n"}}},"$ref":"#/$defs/n"}`)
	var err error
	if n := allocated(func() { err = s.Validate(instance) }); n > limit {
		t.Errorf("failing case allocated %d MB", n>>20)
	}
	if err == nil {
		t.Fatal("20,000 bad items passed")
	}
	if len(err.Error()) > 200_000 {
		t.Errorf("error message is %d bytes", len(err.Error()))
	}
}

// Walking keys is work too, so MaxSteps must bound it even when no key matches a property.
func TestObjectKeysCostSteps(t *testing.T) {
	var b strings.Builder
	b.WriteString("{")
	for i := 0; i < 100_000; i++ {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(`"k` + strconv.Itoa(i) + `":1`)
	}
	b.WriteString("}")
	instance := json.RawMessage(b.String())
	s := mustCompile(t, `{"properties":{"bad":{"type":"integer"}},"patternProperties":{"^z":{}},"anyOf":[{"$ref":"#"},{"$ref":"#"},{"type":"null"}]}`)
	within(t, 5*time.Second, func() {
		_ = s.Validate(instance)
	})
}

func TestReportedPathsAreShort(t *testing.T) {
	key := strings.Repeat("k", 500)
	s := mustCompile(t, `{"properties":{"`+key+`":{"properties":{"b":{"type":"string"}}}}}`)
	err := s.Validate(json.RawMessage(`{"` + key + `":{"b":1}}`))
	if err == nil {
		t.Fatal("no error")
	}
	if got := err.(*ValidationError).Path; got != "/"+strings.Repeat("k", 64)+".../b" {
		t.Errorf("path = %q", got)
	}
	var b strings.Builder
	for i := 0; i < 200; i++ {
		b.WriteString(`{"abcdefghij":`)
	}
	b.WriteString(`1` + strings.Repeat("}", 200))
	s, err = Basic(MaxDepth(300)).Compile(json.RawMessage(`{"additionalProperties":{"$ref":"#"},"type":"object"}`))
	if err != nil {
		t.Fatal(err)
	}
	err = s.Validate(json.RawMessage(b.String()))
	if err == nil {
		t.Fatal("no error")
	}
	got := err.(*ValidationError).Path
	if len(got) > maxPathLen+4 || !strings.HasPrefix(got, "/...") || !strings.HasSuffix(got, "/abcdefghij") {
		t.Errorf("deep path = %d bytes, %.40q...", len(got), got)
	}
}

func TestLongNumberAllowedWhereAnythingIs(t *testing.T) {
	long := strings.Repeat("9", 5000)
	for _, schema := range []string{`{}`, `true`, `{"properties":{"raw":{}}}`, `{"type":"number"}`, `{"type":"integer"}`} {
		in := long
		if strings.Contains(schema, "raw") {
			in = `{"raw":` + long + `}`
		}
		if err := mustCompile(t, schema).Validate(json.RawMessage(in)); err != nil {
			t.Errorf("%s rejected a long number: %v", schema, err)
		}
	}
	if err := mustCompile(t, `{"maximum":10}`).Validate(json.RawMessage(long)); err == nil {
		t.Error("a long number passed a numeric bound")
	}
}

func TestPatternPropertiesAndPrefixItems(t *testing.T) {
	tests := []struct {
		schema, instance string
		valid            bool
	}{
		{`{"patternProperties":{"^x-":{}},"additionalProperties":false}`, `{"x-a":1}`, true},
		{`{"patternProperties":{"^x-":{"type":"string"}},"additionalProperties":false}`, `{"x-a":1}`, false},
		{`{"patternProperties":{"^x-":{}},"additionalProperties":false}`, `{"y":1}`, false},
		{`{"prefixItems":[{"type":"string"}],"items":false}`, `["a"]`, true},
		{`{"prefixItems":[{"type":"string"}],"items":false}`, `["a",1]`, false},
		{`{"prefixItems":[{"type":"string"},{"type":"integer"}],"items":{"type":"boolean"}}`, `["a",1,true,false]`, true},
		{`{"prefixItems":[{"type":"string"}]}`, `[1]`, false},
	}
	for _, tt := range tests {
		err := mustCompile(t, tt.schema).Validate(json.RawMessage(tt.instance))
		if (err == nil) != tt.valid {
			t.Errorf("%s with %s: err = %v, want valid=%v", tt.schema, tt.instance, err, tt.valid)
		}
	}
}

func TestAnyOfAndAllOf(t *testing.T) {
	tests := []struct {
		schema, instance string
		valid            bool
	}{
		{`{"anyOf":[{"type":"string"},{"type":"null"}]}`, `null`, true},
		{`{"anyOf":[{"type":"string"},{"type":"null"}]}`, `1`, false},
		{`{"allOf":[{"minimum":1},{"maximum":3}]}`, `2`, true},
		{`{"allOf":[{"minimum":1},{"maximum":3}]}`, `4`, false},
	}
	for _, tt := range tests {
		err := mustCompile(t, tt.schema).Validate(json.RawMessage(tt.instance))
		if (err == nil) != tt.valid {
			t.Errorf("%s with %s: err = %v, want valid=%v", tt.schema, tt.instance, err, tt.valid)
		}
	}
}

func TestCountKeywordsAcceptIntegralNumbers(t *testing.T) {
	if _, err := Basic().Compile(json.RawMessage(`{"minLength":1.0,"maxItems":2e0}`)); err != nil {
		t.Fatalf("Compile: %v", err)
	}
}

func TestDeepPathsStayCheap(t *testing.T) {
	key := strings.Repeat("k", 60_000)
	var b strings.Builder
	for i := 0; i < 60; i++ {
		b.WriteString(`{"` + key + `":`)
	}
	b.WriteString(`1`)
	b.WriteString(strings.Repeat("}", 60))
	s := mustCompile(t, `{"additionalProperties":{"$ref":"#"}}`)
	within(t, 2*time.Second, func() {
		if err := s.Validate(json.RawMessage(b.String())); err != nil {
			t.Fatalf("Validate: %v", err)
		}
	})
}

type SliceNode []SliceNode
type MapNode map[string]MapNode
type PtrNode *PtrNode

func TestSelfReferencingNonStructTypes(t *testing.T) {
	for _, typ := range []reflect.Type{reflect.TypeFor[SliceNode](), reflect.TypeFor[MapNode](), reflect.TypeFor[PtrNode]()} {
		schema, err := ForType(typ)
		if err != nil {
			t.Errorf("%s: %v", typ, err)
			continue
		}
		if _, err := Basic().Compile(schema); err != nil {
			t.Errorf("%s schema does not compile: %v\n%s", typ, err, schema)
		}
	}
	schema, err := For[SliceNode]()
	if err != nil {
		t.Fatal(err)
	}
	s := mustCompile(t, string(schema))
	if err := s.Validate(json.RawMessage(`[[],[[]]]`)); err != nil {
		t.Errorf("nested SliceNode: %v", err)
	}
	if err := s.Validate(json.RawMessage(`[[1]]`)); err == nil {
		t.Error("a number inside a SliceNode passed")
	}
}

type Leaf struct {
	X int
}
type LeftWrap struct{ Leaf }
type RightWrap struct{ Leaf }
type Dup struct {
	LeftWrap
	RightWrap
	Own string `json:"own"`
}

type Colored struct {
	Color *string `json:"color" jsonschema:"enum=red|blue"`
	Shade *int    `json:"shade,omitempty" jsonschema:"enum=1|2"`
}

type Chain struct {
	Name string `json:"name"`
	Next *Chain `json:"next"`
}

type quoted struct {
	A  int   `json:"a,string"`
	PA *int  `json:"pa,string"`
	PP **int `json:"pp,string"` //nolint:staticcheck // encoding/json ignores the option here, which the test checks
}

type oddTags struct {
	Bad  string `json:"c'd"` //nolint:staticcheck // a name Go releases treat differently, which inference must refuse
	Good string `json:"x-y.z"`
}

type punctuatedTags struct {
	Good string `json:"x-y.z"`
	Spa  string `json:"e f"`
	Acc  string `json:"é"`
}

func TestInferenceMatchesEncodingJSON(t *testing.T) {
	red, one, two := "red", 1, 2
	pone := &one
	values := []any{
		Dup{},
		Colored{},
		Colored{Color: &red, Shade: &two},
		Chain{Name: "a", Next: &Chain{Name: "b"}},
		quoted{A: 1, PA: &one, PP: &pone},
		quoted{},
		punctuatedTags{Good: "g", Spa: "s", Acc: "a"},
		flat{Name: "n", Age: 3, Score: 1.5, Active: true, Count: 9},
		slicesMaps{Ints: []int{1}, Scores: map[string]float64{"a": 1}, ByID: map[int]string{1: "a"}, Blob: []byte("x"), Matrix: [][]int{{1}}},
	}
	for _, v := range values {
		typ := reflect.TypeOf(v)
		schema, err := ForType(typ)
		if err != nil {
			t.Fatalf("%s: %v", typ, err)
		}
		s := mustCompile(t, string(schema))
		instance, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.Validate(instance); err != nil {
			t.Errorf("%s: %s fails its own schema: %v\n%s", typ, instance, err, schema)
		}
	}

	schema, err := For[Dup]()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(schema), `"X"`) {
		t.Errorf("a field embedded twice at the same depth is kept: %s", schema)
	}

	schema, err = For[Chain]()
	if err != nil {
		t.Fatal(err)
	}
	err = mustCompile(t, string(schema)).Validate(json.RawMessage(`{"name":"a","next":{"name":5,"next":"nope"}}`))
	if err == nil {
		t.Error("a bad value inside a recursive pointer field passed")
	}

	if _, err := For[oddTags](); err == nil || !strings.Contains(err.Error(), "c'd") {
		t.Errorf("a tag name Go releases treat differently was accepted: %v", err)
	}
}

func TestInferredKeywordsAreAllValidated(t *testing.T) {
	annotations := map[string]bool{"title": true, "description": true, "default": true, "format": true, "contentEncoding": true, "$defs": true}
	supported := map[string]bool{
		"type": true, "properties": true, "required": true, "additionalProperties": true, "items": true,
		"enum": true, "minimum": true, "maximum": true, "minLength": true, "maxLength": true, "pattern": true,
		"minItems": true, "maxItems": true, "$ref": true, "anyOf": true,
	}
	var check func(path string, v any)
	check = func(path string, v any) {
		m, ok := v.(map[string]any)
		if !ok {
			return
		}
		for k, child := range m {
			switch {
			case k == "properties" || k == "$defs":
				for name, sub := range child.(map[string]any) {
					check(path+"/"+k+"/"+name, sub)
				}
			case k == "anyOf":
				for _, sub := range child.([]any) {
					check(path+"/anyOf", sub)
				}
			case k == "items" || k == "additionalProperties":
				check(path+"/"+k, child)
			case annotations[k] || supported[k]:
			default:
				t.Errorf("%s: inference emitted %q, which Basic does not validate", path, k)
			}
		}
	}
	for _, typ := range []reflect.Type{
		reflect.TypeFor[flat](), reflect.TypeFor[tagged](), reflect.TypeFor[slicesMaps](), reflect.TypeFor[pointers](),
		reflect.TypeFor[timeAndRaw](), reflect.TypeFor[embedded](), reflect.TypeFor[Tree](), reflect.TypeFor[Chain](),
		reflect.TypeFor[Colored](), reflect.TypeFor[quoted](),
	} {
		schema, err := ForType(typ)
		if err != nil {
			t.Fatal(err)
		}
		var v any
		if err := json.Unmarshal(schema, &v); err != nil {
			t.Fatal(err)
		}
		check(typ.String(), v)
	}
}

func TestQuotedOnlyOneLevel(t *testing.T) {
	schema, err := For[quoted]()
	if err != nil {
		t.Fatal(err)
	}
	var s struct {
		Properties map[string]struct {
			Type any `json:"type"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(schema, &s); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(s.Properties["pp"].Type, []any{"integer", "null"}) {
		t.Errorf("**int with ,string = %v, want an integer like encoding/json", s.Properties["pp"].Type)
	}
	if !reflect.DeepEqual(s.Properties["pa"].Type, []any{"string", "null"}) {
		t.Errorf("*int with ,string = %v, want a string", s.Properties["pa"].Type)
	}
}
