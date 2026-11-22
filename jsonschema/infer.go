// Package jsonschema builds JSON Schemas (2020-12) from Go types and validates JSON against a bounded
// subset of the keywords. The full keyword set comes with the jsonschema/full module.
package jsonschema

import (
	"bytes"
	"encoding"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"
)

// SchemaProvider lets a type give its own schema, so ForType runs no reflection for it.
type SchemaProvider interface {
	MCPSchema() json.RawMessage
}

var (
	providerType      = reflect.TypeFor[SchemaProvider]()
	timeType          = reflect.TypeFor[time.Time]()
	rawMessageType    = reflect.TypeFor[json.RawMessage]()
	numberType        = reflect.TypeFor[json.Number]()
	marshalerType     = reflect.TypeFor[json.Marshaler]()
	textMarshalerType = reflect.TypeFor[encoding.TextMarshaler]()
)

// For returns the schema of T. See ForType for the rules.
func For[T any]() (json.RawMessage, error) {
	return ForType(reflect.TypeFor[T]())
}

// ForType returns the schema of t with keys in a stable order. Structs follow encoding/json: json tags
// name properties, fields without omitempty or omitzero are required, and embedded structs are flattened.
// Recursive types are placed in $defs and referenced with $ref.
func ForType(t reflect.Type) (json.RawMessage, error) {
	if t == nil {
		return nil, errors.New("jsonschema: nil type")
	}
	b := &builder{
		defs:       map[string]any{},
		names:      map[reflect.Type]string{},
		inProgress: map[reflect.Type]bool{},
		recursive:  map[reflect.Type]bool{},
	}
	s, err := b.schema(t, 0)
	if err != nil {
		return nil, err
	}
	if m, ok := s.(map[string]any); ok && len(b.defs) > 0 {
		m["$defs"] = b.defs
	}
	if raw, ok := s.(json.RawMessage); ok {
		return raw, nil
	}
	return json.Marshal(s)
}

type builder struct {
	defs       map[string]any
	names      map[reflect.Type]string
	inProgress map[reflect.Type]bool
	recursive  map[reflect.Type]bool
}

// schema returns a map[string]any, or a json.RawMessage from a SchemaProvider.
func (b *builder) schema(t reflect.Type, depth int) (any, error) {
	if raw, ok, err := providedSchema(t); ok || err != nil {
		return raw, err
	}
	switch {
	case t.Kind() == reflect.Pointer:
		return b.schema(t.Elem(), depth)
	case t == timeType:
		return map[string]any{"type": "string", "format": "date-time"}, nil
	case t == rawMessageType:
		return map[string]any{}, nil
	case t == numberType:
		return map[string]any{"type": "number"}, nil
	case implements(t, marshalerType):
		return map[string]any{}, nil
	case implements(t, textMarshalerType):
		return map[string]any{"type": "string"}, nil
	}

	switch t.Kind() {
	case reflect.Bool:
		return map[string]any{"type": "boolean"}, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return map[string]any{"type": "integer"}, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return map[string]any{"type": "integer", "minimum": json.Number("0")}, nil
	case reflect.Float32, reflect.Float64:
		return map[string]any{"type": "number"}, nil
	case reflect.String:
		return map[string]any{"type": "string"}, nil
	case reflect.Interface:
		return map[string]any{}, nil
	case reflect.Slice:
		// encoding/json writes a nil slice or map as null, so both schemas accept null.
		if t.Elem().Kind() == reflect.Uint8 && !implements(t.Elem(), marshalerType) && !implements(t.Elem(), textMarshalerType) {
			return map[string]any{"type": []any{"string", "null"}, "contentEncoding": "base64"}, nil
		}
		items, err := b.schema(t.Elem(), depth+1)
		if err != nil {
			return nil, err
		}
		return map[string]any{"type": []any{"array", "null"}, "items": items}, nil
	case reflect.Array:
		items, err := b.schema(t.Elem(), depth+1)
		if err != nil {
			return nil, err
		}
		return map[string]any{"type": "array", "items": items, "minItems": t.Len(), "maxItems": t.Len()}, nil
	case reflect.Map:
		if !validMapKey(t.Key()) {
			return nil, fmt.Errorf("jsonschema: unsupported map key type %s", t.Key())
		}
		values, err := b.schema(t.Elem(), depth+1)
		if err != nil {
			return nil, err
		}
		return map[string]any{"type": []any{"object", "null"}, "additionalProperties": values}, nil
	case reflect.Struct:
		return b.structSchema(t, depth)
	}
	return nil, fmt.Errorf("jsonschema: unsupported type %s", t)
}

func implements(t, iface reflect.Type) bool {
	return t.Implements(iface) || (t.Kind() != reflect.Pointer && reflect.PointerTo(t).Implements(iface))
}

func validMapKey(k reflect.Type) bool {
	switch k.Kind() {
	case reflect.String, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return true
	}
	return implements(k, textMarshalerType)
}

// providedSchema calls MCPSchema on a fresh value, so a nil pointer never reaches a value receiver.
func providedSchema(t reflect.Type) (raw json.RawMessage, ok bool, err error) {
	var v reflect.Value
	switch {
	case t.Kind() == reflect.Pointer && t.Implements(providerType):
		v = reflect.New(t.Elem())
	case t.Kind() != reflect.Pointer && t.Implements(providerType):
		v = reflect.Zero(t)
	case t.Kind() != reflect.Pointer && reflect.PointerTo(t).Implements(providerType):
		v = reflect.New(t)
	default:
		return nil, false, nil
	}
	defer func() {
		if r := recover(); r != nil {
			raw, ok, err = nil, true, fmt.Errorf("jsonschema: MCPSchema of %s panicked: %v", t, r)
		}
	}()
	out := v.Interface().(SchemaProvider).MCPSchema()
	if !json.Valid(out) {
		return nil, true, fmt.Errorf("jsonschema: MCPSchema of %s returned invalid JSON", t)
	}
	return bytes.Clone(out), true, nil
}

func (b *builder) structSchema(t reflect.Type, depth int) (any, error) {
	named := t.Name() != ""
	if named && b.inProgress[t] {
		b.recursive[t] = true
		return map[string]any{"$ref": "#/$defs/" + b.defName(t)}, nil
	}
	if named {
		b.inProgress[t] = true
		defer delete(b.inProgress, t)
	}

	properties := map[string]any{}
	var required []string
	for _, f := range structFields(t) {
		s, err := b.schema(f.typ, depth+1)
		if err != nil {
			return nil, fmt.Errorf("%s.%s: %w", t, f.goName, err)
		}
		if f.asString || f.tag != "" {
			m, err := asMap(s)
			if err != nil {
				return nil, fmt.Errorf("%s.%s: %w", t, f.goName, err)
			}
			if f.asString {
				m = map[string]any{"type": "string"}
			}
			if err := applyTag(m, f.tag, deref(f.typ), f.asString); err != nil {
				return nil, fmt.Errorf("%s.%s: %w", t, f.goName, err)
			}
			s = m
		}
		// encoding/json writes a nil pointer as null, so a pointer field is optional and accepts null.
		optional := f.omit
		if f.typ.Kind() == reflect.Pointer {
			s = nullable(s)
			optional = true
		}
		properties[f.name] = s
		if !optional {
			required = append(required, f.name)
		}
	}

	s := map[string]any{"type": "object", "additionalProperties": false}
	if len(properties) > 0 {
		s["properties"] = properties
	}
	if len(required) > 0 {
		sort.Strings(required)
		s["required"] = required
	}
	if named && b.recursive[t] {
		if depth > 0 {
			b.defs[b.defName(t)] = s
			return map[string]any{"$ref": "#/$defs/" + b.defName(t)}, nil
		}
		// The root stays inline and gains $defs, so $defs holds a copy to avoid a cycle.
		def := make(map[string]any, len(s))
		for k, v := range s {
			def[k] = v
		}
		b.defs[b.defName(t)] = def
	}
	return s, nil
}

// asMap gives a schema that tag keywords can be added to. A provider's raw schema is decoded first.
func asMap(s any) (map[string]any, error) {
	if m, ok := s.(map[string]any); ok {
		return m, nil
	}
	dec := json.NewDecoder(bytes.NewReader(s.(json.RawMessage)))
	dec.UseNumber()
	var m map[string]any
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("jsonschema: tag keywords need an object schema: %w", err)
	}
	return m, nil
}

// nullable lets a schema accept null. A provider's raw schema is left as the type wrote it.
func nullable(s any) any {
	m, ok := s.(map[string]any)
	if !ok {
		return s
	}
	if t, ok := m["type"].(string); ok {
		c := make(map[string]any, len(m))
		for k, v := range m {
			c[k] = v
		}
		c["type"] = []any{t, "null"}
		return c
	}
	if _, ok := m["$ref"]; ok {
		return map[string]any{"anyOf": []any{m, map[string]any{"type": "null"}}}
	}
	return m
}

func deref(t reflect.Type) reflect.Type {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t
}

func (b *builder) defName(t reflect.Type) string {
	if name, ok := b.names[t]; ok {
		return name
	}
	base := strings.Map(func(r rune) rune {
		if r == '_' || (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			return r
		}
		return '_'
	}, t.Name())
	name := base
	for i := 2; ; i++ {
		taken := false
		for other, n := range b.names {
			if n == name && other != t {
				taken = true
				break
			}
		}
		if !taken {
			break
		}
		name = fmt.Sprintf("%s_%d", base, i)
	}
	b.names[t] = name
	return name
}

type field struct {
	name     string
	goName   string
	tagged   bool
	index    []int
	typ      reflect.Type
	omit     bool
	asString bool
	tag      string
}

// structFields lists the fields encoding/json would encode, with its rules for embedded structs: the
// shallowest name wins, a tagged field beats an untagged one at the same depth, and other ties drop the name.
func structFields(t reflect.Type) []field {
	type queued struct {
		typ   reflect.Type
		index []int
		// viaPointer is set below an embedded pointer, whose fields vanish when the pointer is nil.
		viaPointer bool
	}
	next := []queued{{typ: t}}
	visited := map[reflect.Type]bool{}
	var all []field
	for len(next) > 0 {
		current := next
		next = nil
		for _, q := range current {
			if visited[q.typ] {
				continue
			}
			visited[q.typ] = true
			for i := 0; i < q.typ.NumField(); i++ {
				sf := q.typ.Field(i)
				if sf.Anonymous {
					ft := deref(sf.Type)
					if !sf.IsExported() && ft.Kind() != reflect.Struct {
						continue
					}
				} else if !sf.IsExported() {
					continue
				}
				tag := sf.Tag.Get("json")
				if tag == "-" {
					continue
				}
				name, opts, _ := strings.Cut(tag, ",")
				index := append(append([]int(nil), q.index...), i)
				if name == "" && sf.Anonymous && deref(sf.Type).Kind() == reflect.Struct {
					next = append(next, queued{
						typ:        deref(sf.Type),
						index:      index,
						viaPointer: q.viaPointer || sf.Type.Kind() == reflect.Pointer,
					})
					continue
				}
				f := field{
					name:   name,
					goName: sf.Name,
					tagged: name != "",
					index:  index,
					typ:    sf.Type,
					omit:   q.viaPointer,
					tag:    sf.Tag.Get("jsonschema"),
				}
				if f.name == "" {
					f.name = sf.Name
				}
				for _, o := range strings.Split(opts, ",") {
					switch o {
					case "omitempty", "omitzero":
						f.omit = true
					case "string":
						switch deref(sf.Type).Kind() {
						case reflect.Bool, reflect.String,
							reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
							reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
							reflect.Float32, reflect.Float64:
							f.asString = true
						}
					}
				}
				all = append(all, f)
			}
		}
	}

	sort.SliceStable(all, func(i, j int) bool {
		if all[i].name != all[j].name {
			return all[i].name < all[j].name
		}
		if len(all[i].index) != len(all[j].index) {
			return len(all[i].index) < len(all[j].index)
		}
		return all[i].tagged && !all[j].tagged
	})
	var out []field
	for i := 0; i < len(all); {
		j := i + 1
		for j < len(all) && all[j].name == all[i].name {
			j++
		}
		group := all[i:j]
		i = j
		best := group[0]
		depth := len(best.index)
		var top []field
		for _, f := range group {
			if len(f.index) == depth {
				top = append(top, f)
			}
		}
		if len(top) == 1 {
			out = append(out, best)
			continue
		}
		var taggedTop []field
		for _, f := range top {
			if f.tagged {
				taggedTop = append(taggedTop, f)
			}
		}
		if len(taggedTop) == 1 {
			out = append(out, taggedTop[0])
		}
	}
	return out
}
