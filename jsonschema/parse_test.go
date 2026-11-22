package jsonschema

import (
	"errors"
	"reflect"
	"testing"
)

func TestParseInstanceMatchesEncodingJSON(t *testing.T) {
	for _, in := range []string{
		`{"a":[1,2.5,-3e2,{"b":null}],"c":"é😀\"\\\/\b\f\n\r\t","d":true,"e":false}`,
		`"\ud800"`, `"\ud800x"`, `"\ud800A"`, `"\udc00"`, "\"\xff\xfe\"", `[]`, `{}`, ` 0 `, `-0.0e-0`,
		`{"a":1,"a":2}`,
	} {
		assertSameParse(t, []byte(in))
	}
	for _, bad := range []string{``, `0A`, `1 x`, `{`, `[1,]`, `{"a"}`, `01`, `1.`, `1e`, `-`, `"\x"`, "\"\x01\"", `tru`, `{}x`, `"abc`, `"\u12"`, `{"a":1,}`, `nul`} {
		if _, err := parseInstance([]byte(bad), 64); err == nil {
			t.Errorf("parseInstance(%q) accepted invalid JSON", bad)
		}
	}
}

func TestParseInstanceDepth(t *testing.T) {
	if _, err := parseInstance([]byte(`[[[]]]`), 2); !errors.Is(err, errTooDeep) {
		t.Errorf("depth 3 with limit 2: %v", err)
	}
	if _, err := parseInstance([]byte(`[[]]`), 2); err != nil {
		t.Errorf("depth 2 with limit 2: %v", err)
	}
	if _, err := parseInstance([]byte(`["[[[[["]`), 2); err != nil {
		t.Errorf("brackets inside a string count as depth: %v", err)
	}
}

func assertSameParse(t *testing.T, b []byte) {
	t.Helper()
	got, gotErr := parseInstance(b, 10_000)
	want, wantErr := decodeValue(b)
	if (gotErr == nil) != (wantErr == nil) {
		t.Fatalf("%q: parseInstance err = %v, encoding/json err = %v", b, gotErr, wantErr)
	}
	if gotErr == nil && !reflect.DeepEqual(got, want) {
		t.Fatalf("%q: parseInstance = %#v, encoding/json = %#v", b, got, want)
	}
}

// FuzzParseInstance checks the hand-written parser against encoding/json on any input.
func FuzzParseInstance(f *testing.F) {
	for _, seed := range []string{
		`{"a":[1,2.5,-3e2,{"b":null}],"c":"é😀"}`, `"\ud800A"`, "\"\xff\"", `[1,]`, `01`, `{"a":1,"a":2}`,
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		assertSameParse(t, b)
	})
}
