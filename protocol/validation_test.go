package protocol

import (
	"encoding/json"
	"reflect"
	"testing"
)

// A zero value either refuses to encode or encodes with every required key set to something other than null.
func TestZeroValuesHaveNoNullRequiredKeys(t *testing.T) {
	defs := loadSchemaDefs(t)
	for _, tc := range schemaFieldCases {
		t.Run(tc.def, func(t *testing.T) {
			zero := reflect.New(reflect.TypeOf(tc.value)).Interface()
			out, err := json.Marshal(zero)
			if err != nil {
				return
			}
			var m map[string]json.RawMessage
			if err := json.Unmarshal(out, &m); err != nil {
				t.Fatalf("zero value is not an object: %s", out)
			}
			for _, name := range defs[tc.def].Required {
				if v, ok := m[name]; ok && string(v) == "null" {
					t.Errorf("required key %q is null in %s", name, out)
				}
			}
		})
	}
}

func TestToolDefaultInputSchema(t *testing.T) {
	out, err := json.Marshal(Tool{Name: "x"})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	assertSameJSON(t, []byte(`{"name":"x","inputSchema":{"type":"object"}}`), out)
}

func TestTokensMustBeStringOrNumber(t *testing.T) {
	bad := []struct {
		name string
		in   string
		v    any
	}{
		{"object progress token", `{"progressToken":{"a":1},"progress":1}`, new(ProgressParams)},
		{"null progress token", `{"progressToken":null,"progress":1}`, new(ProgressParams)},
		{"bool progress token", `{"progressToken":true,"progress":1}`, new(ProgressParams)},
		{"object request id", `{"requestId":[1]}`, new(CancelledParams)},
		{"null request id", `{"requestId":null}`, new(CancelledParams)},
		{"object token in meta", `{"progressToken":{}}`, new(RequestMeta)},
	}
	for _, tt := range bad {
		t.Run(tt.name, func(t *testing.T) {
			if err := json.Unmarshal([]byte(tt.in), tt.v); err == nil {
				t.Fatalf("Unmarshal(%s) succeeded", tt.in)
			}
		})
	}

	var m RequestMeta
	if err := json.Unmarshal([]byte(`{"progressToken":null}`), &m); err != nil {
		t.Fatalf("null token in meta: %v", err)
	}
	if len(m.ProgressToken) != 0 {
		t.Errorf("null token in meta was kept as %s", m.ProgressToken)
	}

	var p ProgressParams
	if err := json.Unmarshal([]byte(`{"progressToken":7,"progress":1}`), &p); err != nil || string(p.ProgressToken) != "7" {
		t.Errorf("numeric token = %s, %v", p.ProgressToken, err)
	}

	if _, err := json.Marshal(ProgressParams{Progress: 1}); err == nil {
		t.Error("ProgressParams without a token encoded")
	}
	if _, err := json.Marshal(CancelledParams{}); err == nil {
		t.Error("CancelledParams without a request id encoded")
	}
}

func TestElicitParamsValidation(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"name":{"type":"string"}}}`)
	ok := []ElicitParams{
		{Message: "m", RequestedSchema: schema},
		{Mode: ElicitForm, Message: "m", RequestedSchema: schema},
		{Mode: ElicitURL, Message: "m", URL: "https://example.com"},
	}
	for _, p := range ok {
		if _, err := json.Marshal(p); err != nil {
			t.Errorf("Marshal(%+v): %v", p, err)
		}
	}
	bad := []ElicitParams{
		{Message: "m"},
		{Message: "m", RequestedSchema: json.RawMessage(`"x"`)},
		{Mode: ElicitURL, Message: "m"},
		{Message: "m", URL: "https://example.com"},
		{Mode: ElicitURL, Message: "m", URL: "https://example.com", RequestedSchema: schema},
		{Mode: "bogus", Message: "m"},
	}
	for _, p := range bad {
		if out, err := json.Marshal(p); err == nil {
			t.Errorf("Marshal(%+v) = %s, want error", p, out)
		}
	}
}

func TestCompleteReferenceValidation(t *testing.T) {
	for _, r := range []CompleteReference{{Type: RefPrompt, Name: "p"}, {Type: RefResource, URI: "file:///{x}"}} {
		if _, err := json.Marshal(r); err != nil {
			t.Errorf("Marshal(%+v): %v", r, err)
		}
	}
	for _, r := range []CompleteReference{{Type: RefPrompt}, {Type: RefResource}, {Type: "ref/other", Name: "x"}, {}} {
		if out, err := json.Marshal(r); err == nil {
			t.Errorf("Marshal(%+v) = %s, want error", r, out)
		}
	}
}
