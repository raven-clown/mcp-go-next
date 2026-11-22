package jsonschema

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

// splitTag splits a jsonschema tag on commas. "\," is a literal comma and "\\" a literal backslash.
func splitTag(tag string) []string {
	var parts []string
	var cur strings.Builder
	for i := 0; i < len(tag); i++ {
		c := tag[i]
		if c == '\\' && i+1 < len(tag) && (tag[i+1] == ',' || tag[i+1] == '\\') {
			cur.WriteByte(tag[i+1])
			i++
			continue
		}
		if c == ',' {
			parts = append(parts, cur.String())
			cur.Reset()
			continue
		}
		cur.WriteByte(c)
	}
	return append(parts, cur.String())
}

// applyTag adds the keywords of a jsonschema struct tag to s. base is the field type without pointers.
func applyTag(s map[string]any, tag string, base reflect.Type, asString bool) error {
	for _, part := range splitTag(tag) {
		if strings.TrimSpace(part) == "" {
			continue
		}
		key, value, ok := strings.Cut(part, "=")
		if !ok {
			return fmt.Errorf("jsonschema tag %q: want key=value", part)
		}
		switch key {
		case "title", "description", "format":
			s[key] = value
		case "pattern":
			if _, err := regexp.Compile(value); err != nil {
				return fmt.Errorf("jsonschema tag pattern %q: %w", value, err)
			}
			s[key] = value
		case "minimum", "maximum", "exclusiveMinimum", "exclusiveMaximum":
			n, err := parseNumber(value)
			if err != nil {
				return fmt.Errorf("jsonschema tag %s=%q: %w", key, value, err)
			}
			s[key] = n
		case "minLength", "maxLength", "minItems", "maxItems":
			n, err := strconv.ParseUint(value, 10, 31)
			if err != nil {
				return fmt.Errorf("jsonschema tag %s=%q: want a non-negative integer", key, value)
			}
			s[key] = n
		case "enum":
			target, valueType := s, base
			if isList(base) {
				items, _ := s["items"].(map[string]any)
				if items == nil {
					return fmt.Errorf("jsonschema tag enum: the items of %s have no schema to extend", base)
				}
				target, valueType = items, base.Elem()
			}
			var values []any
			for _, raw := range strings.Split(value, "|") {
				v, err := parseValue(raw, valueType, asString)
				if err != nil {
					return fmt.Errorf("jsonschema tag enum value %q: %w", raw, err)
				}
				values = append(values, v)
			}
			target["enum"] = values
		case "default":
			v, err := parseValue(value, base, asString)
			if err != nil {
				return fmt.Errorf("jsonschema tag default %q: %w", value, err)
			}
			s[key] = v
		default:
			return fmt.Errorf("jsonschema tag: unknown key %q", key)
		}
	}
	return nil
}

func isList(t reflect.Type) bool {
	return (t.Kind() == reflect.Slice && t.Elem().Kind() != reflect.Uint8) || t.Kind() == reflect.Array
}

func parseNumber(s string) (json.Number, error) {
	if _, ok := ratOf(json.Number(s)); !ok {
		return "", fmt.Errorf("not a number")
	}
	return json.Number(s), nil
}

// parseValue turns tag text into a JSON value of the field's type.
func parseValue(s string, t reflect.Type, asString bool) (any, error) {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if asString {
		return s, nil
	}
	switch t.Kind() {
	case reflect.String:
		return s, nil
	case reflect.Bool:
		b, err := strconv.ParseBool(s)
		if err != nil {
			return nil, fmt.Errorf("want true or false")
		}
		return b, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if _, err := strconv.ParseInt(s, 10, t.Bits()); err != nil {
			return nil, fmt.Errorf("want an integer that fits %s", t)
		}
		return json.Number(s), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		if _, err := strconv.ParseUint(s, 10, t.Bits()); err != nil {
			return nil, fmt.Errorf("want a non-negative integer that fits %s", t)
		}
		return json.Number(s), nil
	case reflect.Float32, reflect.Float64:
		if _, err := strconv.ParseFloat(s, t.Bits()); err != nil {
			return nil, fmt.Errorf("want a number")
		}
		return json.Number(s), nil
	}
	dec := json.NewDecoder(bytes.NewReader([]byte(s)))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil || dec.More() {
		return nil, fmt.Errorf("want a JSON value for %s", t)
	}
	return v, nil
}
