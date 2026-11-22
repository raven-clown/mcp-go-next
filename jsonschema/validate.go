package jsonschema

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// ValidationError is one failed keyword. Path is a JSON Pointer into the instance, "" for the root.
type ValidationError struct {
	Path    string
	Message string
}

func (e *ValidationError) Error() string {
	if e.Path == "" {
		return e.Message
	}
	return e.Path + ": " + e.Message
}

type Validator interface {
	Compile(schema json.RawMessage) (Schema, error)
}

// Schema validates instances. Validate returns nil, one *ValidationError, or errors.Join of several.
type Schema interface {
	Validate(instance json.RawMessage) error
}

type BasicOption func(*basicConfig)

type basicConfig struct {
	maxDepth int
	maxSteps int
}

// MaxDepth bounds how deeply schemas and instances may nest. The default is 64.
func MaxDepth(n int) BasicOption { return func(c *basicConfig) { c.maxDepth = n } }

// MaxSteps bounds the schema evaluations one Validate call may run. The default is 1,000,000.
func MaxSteps(n int) BasicOption { return func(c *basicConfig) { c.maxSteps = n } }

// Basic validates type, properties, required, additionalProperties, items, enum, const, minimum,
// maximum, exclusiveMinimum, exclusiveMaximum, minLength, maxLength, pattern, minItems, maxItems,
// boolean schemas and local $ref. Other keywords are ignored. Numbers are compared exactly.
func Basic(opts ...BasicOption) Validator {
	c := basicConfig{maxDepth: 64, maxSteps: 1_000_000}
	for _, o := range opts {
		o(&c)
	}
	return basic{cfg: c}
}

type basic struct{ cfg basicConfig }

type node struct {
	always     *bool
	types      []string
	properties map[string]*node
	required   []string
	additional *node
	items      *node
	enum       []any
	hasEnum    bool
	constVal   any
	hasConst   bool
	minimum    *big.Rat
	maximum    *big.Rat
	exclMin    *big.Rat
	exclMax    *big.Rat
	minLength  *int
	maxLength  *int
	minItems   *int
	maxItems   *int
	pattern    *regexp.Regexp
	ref        *node
}

type compiler struct {
	cfg  basicConfig
	root any
	byJP map[string]*node
}

func (b basic) Compile(schema json.RawMessage) (Schema, error) {
	root, err := decodeValue(schema)
	if err != nil {
		return nil, fmt.Errorf("jsonschema: schema is not valid JSON: %w", err)
	}
	c := &compiler{cfg: b.cfg, root: root, byJP: map[string]*node{}}
	n, err := c.compile(root, "#", 0)
	if err != nil {
		return nil, err
	}
	return &compiled{root: n, cfg: b.cfg}, nil
}

func (c *compiler) compile(v any, pointer string, depth int) (*node, error) {
	if n, ok := c.byJP[pointer]; ok {
		return n, nil
	}
	if depth > c.cfg.maxDepth {
		return nil, fmt.Errorf("jsonschema: schema nests deeper than %d at %s", c.cfg.maxDepth, pointer)
	}
	n := &node{}
	c.byJP[pointer] = n
	switch s := v.(type) {
	case bool:
		n.always = &s
		return n, nil
	case map[string]any:
		if err := c.fill(n, s, pointer, depth); err != nil {
			return nil, err
		}
		return n, nil
	}
	return nil, fmt.Errorf("jsonschema: %s is not a schema (want an object or a boolean)", pointer)
}

func (c *compiler) fill(n *node, s map[string]any, pointer string, depth int) error {
	at := func(key string) string { return pointer + "/" + escapePointer(key) }
	for _, key := range sortedKeys(s) {
		v := s[key]
		var err error
		switch key {
		case "type":
			n.types, err = typeList(v)
		case "properties":
			props, ok := v.(map[string]any)
			if !ok {
				return fmt.Errorf("jsonschema: %s: properties must be an object", pointer)
			}
			n.properties = map[string]*node{}
			for _, name := range sortedKeys(props) {
				child, err := c.compile(props[name], at(key)+"/"+escapePointer(name), depth+1)
				if err != nil {
					return err
				}
				n.properties[name] = child
			}
		case "required":
			n.required, err = stringList(v)
		case "additionalProperties":
			n.additional, err = c.compile(v, at(key), depth+1)
		case "items":
			n.items, err = c.compile(v, at(key), depth+1)
		case "enum":
			list, ok := v.([]any)
			if !ok {
				return fmt.Errorf("jsonschema: %s: enum must be an array", pointer)
			}
			n.enum, n.hasEnum = list, true
		case "const":
			n.constVal, n.hasConst = v, true
		case "minimum", "maximum", "exclusiveMinimum", "exclusiveMaximum":
			var r *big.Rat
			r, err = numberKeyword(v)
			switch key {
			case "minimum":
				n.minimum = r
			case "maximum":
				n.maximum = r
			case "exclusiveMinimum":
				n.exclMin = r
			default:
				n.exclMax = r
			}
		case "minLength":
			n.minLength, err = countKeyword(v)
		case "maxLength":
			n.maxLength, err = countKeyword(v)
		case "minItems":
			n.minItems, err = countKeyword(v)
		case "maxItems":
			n.maxItems, err = countKeyword(v)
		case "pattern":
			p, ok := v.(string)
			if !ok {
				return fmt.Errorf("jsonschema: %s: pattern must be a string", pointer)
			}
			if n.pattern, err = regexp.Compile(p); err != nil {
				return fmt.Errorf("jsonschema: %s: bad pattern %q: %w", pointer, p, err)
			}
		case "$ref":
			ref, ok := v.(string)
			if !ok || !strings.HasPrefix(ref, "#") {
				return fmt.Errorf("jsonschema: %s: only local $ref values starting with # are supported, got %v", pointer, v)
			}
			target, ok := resolvePointer(c.root, ref)
			if !ok {
				return fmt.Errorf("jsonschema: %s: $ref %q does not resolve", pointer, ref)
			}
			n.ref, err = c.compile(target, ref, depth+1)
		}
		if err != nil {
			return fmt.Errorf("jsonschema: %s: %s: %w", pointer, key, err)
		}
	}
	return nil
}

var typeNames = map[string]bool{"null": true, "boolean": true, "object": true, "array": true, "number": true, "integer": true, "string": true}

func typeList(v any) ([]string, error) {
	var names []string
	switch t := v.(type) {
	case string:
		names = []string{t}
	case []any:
		for _, e := range t {
			s, ok := e.(string)
			if !ok {
				return nil, errors.New("type entries must be strings")
			}
			names = append(names, s)
		}
	default:
		return nil, errors.New("type must be a string or an array of strings")
	}
	for _, name := range names {
		if !typeNames[name] {
			return nil, fmt.Errorf("unknown type %q", name)
		}
	}
	return names, nil
}

func stringList(v any) ([]string, error) {
	list, ok := v.([]any)
	if !ok {
		return nil, errors.New("must be an array of strings")
	}
	out := make([]string, 0, len(list))
	for _, e := range list {
		s, ok := e.(string)
		if !ok {
			return nil, errors.New("must be an array of strings")
		}
		out = append(out, s)
	}
	return out, nil
}

func numberKeyword(v any) (*big.Rat, error) {
	n, ok := v.(json.Number)
	if !ok {
		return nil, errors.New("must be a number")
	}
	r, ok := ratOf(n)
	if !ok {
		return nil, errors.New("number out of range")
	}
	return r, nil
}

func countKeyword(v any) (*int, error) {
	n, ok := v.(json.Number)
	if !ok {
		return nil, errors.New("must be a non-negative integer")
	}
	i, err := strconv.ParseInt(string(n), 10, 32)
	if err != nil || i < 0 {
		return nil, errors.New("must be a non-negative integer")
	}
	c := int(i)
	return &c, nil
}

// ratOf parses a JSON number exactly. Exponents beyond ±1000 are refused, because big.Rat would
// expand them in full and a tiny input like 1e1000000000 could take minutes.
func ratOf(n json.Number) (*big.Rat, bool) {
	s := string(n)
	if i := strings.IndexAny(s, "eE"); i >= 0 {
		exp, err := strconv.Atoi(strings.TrimPrefix(s[i+1:], "+"))
		if err != nil || exp > 1000 || exp < -1000 {
			return nil, false
		}
	}
	r, ok := new(big.Rat).SetString(s)
	return r, ok
}

func resolvePointer(root any, ref string) (any, bool) {
	if ref == "#" {
		return root, true
	}
	if !strings.HasPrefix(ref, "#/") {
		return nil, false
	}
	cur := root
	for _, part := range strings.Split(ref[2:], "/") {
		part = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
		switch v := cur.(type) {
		case map[string]any:
			next, ok := v[part]
			if !ok {
				return nil, false
			}
			cur = next
		case []any:
			i, err := strconv.Atoi(part)
			if err != nil || i < 0 || i >= len(v) {
				return nil, false
			}
			cur = v[i]
		default:
			return nil, false
		}
	}
	return cur, true
}

func escapePointer(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "~", "~0"), "/", "~1")
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// decodeValue reads exactly one JSON value, keeping numbers as json.Number.
func decodeValue(b []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err == nil {
		return nil, errors.New("data after the value")
	}
	return v, nil
}

type compiled struct {
	root *node
	cfg  basicConfig
}

type run struct {
	cfg     basicConfig
	steps   int
	stopped bool
	errs    []error
}

func (s *compiled) Validate(instance json.RawMessage) error {
	if d := nestingDepth(instance); d > s.cfg.maxDepth {
		return &ValidationError{Message: fmt.Sprintf("value nests deeper than %d levels", s.cfg.maxDepth)}
	}
	v, err := decodeValue(instance)
	if err != nil {
		return &ValidationError{Message: "not valid JSON: " + err.Error()}
	}
	r := &run{cfg: s.cfg}
	r.validate(s.root, v, "", 0)
	if r.stopped {
		return &ValidationError{Message: fmt.Sprintf("validation stopped after %d steps", s.cfg.maxSteps)}
	}
	switch len(r.errs) {
	case 0:
		return nil
	case 1:
		return r.errs[0]
	}
	return errors.Join(r.errs...)
}

// nestingDepth counts the deepest [ or { outside strings, before the value is decoded at all.
func nestingDepth(b []byte) int {
	depth, deepest, inString, escaped := 0, 0, false, false
	for _, c := range b {
		switch {
		case inString:
			switch {
			case escaped:
				escaped = false
			case c == '\\':
				escaped = true
			case c == '"':
				inString = false
			}
		case c == '"':
			inString = true
		case c == '[' || c == '{':
			depth++
			if depth > deepest {
				deepest = depth
			}
		case c == ']' || c == '}':
			depth--
		}
	}
	return deepest
}

func (r *run) fail(path, format string, args ...any) {
	r.errs = append(r.errs, &ValidationError{Path: path, Message: fmt.Sprintf(format, args...)})
}

// validate walks one schema node. depth counts nodes on the current path, refs included, so a $ref
// loop that never consumes the instance stops at the depth limit instead of recursing forever.
func (r *run) validate(n *node, v any, path string, depth int) {
	if r.stopped {
		return
	}
	r.steps++
	if r.steps > r.cfg.maxSteps {
		r.stopped = true
		return
	}
	if depth > 4*r.cfg.maxDepth {
		r.fail(path, "schema references nest deeper than %d", 4*r.cfg.maxDepth)
		return
	}
	if n.always != nil {
		if !*n.always {
			r.fail(path, "no value is allowed here")
		}
		return
	}
	if len(n.types) > 0 && !matchesType(n.types, v) {
		r.fail(path, "must be of type %s, got %s", strings.Join(n.types, " or "), typeOf(v))
	}
	if n.hasConst && !equal(n.constVal, v) {
		r.fail(path, "must equal the const value")
	}
	if n.hasEnum {
		found := false
		for _, e := range n.enum {
			if equal(e, v) {
				found = true
				break
			}
		}
		if !found {
			r.fail(path, "must be one of the enum values")
		}
	}
	switch x := v.(type) {
	case json.Number:
		r.number(n, x, path)
	case string:
		r.str(n, x, path)
	case []any:
		if n.minItems != nil && len(x) < *n.minItems {
			r.fail(path, "must have at least %d items", *n.minItems)
		}
		if n.maxItems != nil && len(x) > *n.maxItems {
			r.fail(path, "must have at most %d items", *n.maxItems)
		}
		if n.items != nil {
			for i, item := range x {
				r.validate(n.items, item, path+"/"+strconv.Itoa(i), depth+1)
			}
		}
	case map[string]any:
		for _, name := range n.required {
			if _, ok := x[name]; !ok {
				r.fail(path, "missing required property %q", name)
			}
		}
		for _, key := range sortedKeys(x) {
			child := path + "/" + escapePointer(key)
			if p, ok := n.properties[key]; ok {
				r.validate(p, x[key], child, depth+1)
			} else if n.additional != nil {
				if n.additional.always != nil && !*n.additional.always {
					r.fail(child, "property is not allowed")
				} else {
					r.validate(n.additional, x[key], child, depth+1)
				}
			}
		}
	}
	if n.ref != nil {
		r.validate(n.ref, v, path, depth+1)
	}
}

func (r *run) number(n *node, x json.Number, path string) {
	if n.minimum == nil && n.maximum == nil && n.exclMin == nil && n.exclMax == nil {
		return
	}
	v, ok := ratOf(x)
	if !ok {
		r.fail(path, "number is out of range")
		return
	}
	if n.minimum != nil && v.Cmp(n.minimum) < 0 {
		r.fail(path, "must be at least %s", n.minimum.RatString())
	}
	if n.maximum != nil && v.Cmp(n.maximum) > 0 {
		r.fail(path, "must be at most %s", n.maximum.RatString())
	}
	if n.exclMin != nil && v.Cmp(n.exclMin) <= 0 {
		r.fail(path, "must be greater than %s", n.exclMin.RatString())
	}
	if n.exclMax != nil && v.Cmp(n.exclMax) >= 0 {
		r.fail(path, "must be less than %s", n.exclMax.RatString())
	}
}

func (r *run) str(n *node, x string, path string) {
	if n.minLength != nil || n.maxLength != nil {
		length := utf8.RuneCountInString(x)
		if n.minLength != nil && length < *n.minLength {
			r.fail(path, "must be at least %d characters", *n.minLength)
		}
		if n.maxLength != nil && length > *n.maxLength {
			r.fail(path, "must be at most %d characters", *n.maxLength)
		}
	}
	if n.pattern != nil && !n.pattern.MatchString(x) {
		r.fail(path, "must match the pattern %q", n.pattern.String())
	}
}

func typeOf(v any) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case string:
		return "string"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	case json.Number:
		if r, ok := ratOf(x); ok && r.IsInt() {
			return "integer"
		}
		return "number"
	}
	return "unknown"
}

func matchesType(types []string, v any) bool {
	actual := typeOf(v)
	for _, t := range types {
		if t == actual || (t == "number" && actual == "integer") {
			return true
		}
	}
	return false
}

// equal compares JSON values, with numbers compared by value so 1 equals 1.0.
func equal(a, b any) bool {
	switch x := a.(type) {
	case json.Number:
		y, ok := b.(json.Number)
		if !ok {
			return false
		}
		rx, okx := ratOf(x)
		ry, oky := ratOf(y)
		if !okx || !oky {
			return string(x) == string(y)
		}
		return rx.Cmp(ry) == 0
	case []any:
		y, ok := b.([]any)
		if !ok || len(x) != len(y) {
			return false
		}
		for i := range x {
			if !equal(x[i], y[i]) {
				return false
			}
		}
		return true
	case map[string]any:
		y, ok := b.(map[string]any)
		if !ok || len(x) != len(y) {
			return false
		}
		for k, xv := range x {
			yv, ok := y[k]
			if !ok || !equal(xv, yv) {
				return false
			}
		}
		return true
	}
	return a == b
}
