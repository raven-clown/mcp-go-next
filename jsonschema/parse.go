package jsonschema

import (
	"encoding/json"
	"errors"
	"fmt"
	"unicode/utf16"
	"unicode/utf8"
)

// parseInstance decodes one JSON value into the same shapes as a json.Decoder with UseNumber:
// map[string]any, []any, string, json.Number, bool and nil. It refuses data after the value and
// nesting deeper than maxDepth. Validation runs on every tool call, so it avoids reflection.
func parseInstance(b []byte, maxDepth int) (any, error) {
	p := parser{b: b, maxDepth: maxDepth}
	p.skipSpace()
	v, err := p.value(0)
	if err != nil {
		return nil, err
	}
	p.skipSpace()
	if p.i != len(p.b) {
		return nil, p.errorf("data after the value")
	}
	return v, nil
}

var errTooDeep = errors.New("too deep")

type parser struct {
	b        []byte
	i        int
	maxDepth int
}

func (p *parser) errorf(format string, args ...any) error {
	return fmt.Errorf("offset %d: %s", p.i, fmt.Sprintf(format, args...))
}

func (p *parser) skipSpace() {
	for p.i < len(p.b) {
		switch p.b[p.i] {
		case ' ', '\t', '\n', '\r':
			p.i++
		default:
			return
		}
	}
}

func (p *parser) value(depth int) (any, error) {
	if p.i >= len(p.b) {
		return nil, p.errorf("unexpected end of input")
	}
	switch c := p.b[p.i]; {
	case c == '{':
		if depth >= p.maxDepth {
			return nil, errTooDeep
		}
		return p.object(depth + 1)
	case c == '[':
		if depth >= p.maxDepth {
			return nil, errTooDeep
		}
		return p.array(depth + 1)
	case c == '"':
		return p.str()
	case c == '-' || (c >= '0' && c <= '9'):
		return p.number()
	case c == 't':
		return true, p.literal("true")
	case c == 'f':
		return false, p.literal("false")
	case c == 'n':
		return nil, p.literal("null")
	}
	return nil, p.errorf("invalid character %q", p.b[p.i])
}

func (p *parser) literal(word string) error {
	if len(p.b)-p.i < len(word) || string(p.b[p.i:p.i+len(word)]) != word {
		return p.errorf("invalid literal")
	}
	p.i += len(word)
	return nil
}

func (p *parser) object(depth int) (any, error) {
	p.i++
	m := map[string]any{}
	p.skipSpace()
	if p.i < len(p.b) && p.b[p.i] == '}' {
		p.i++
		return m, nil
	}
	for {
		p.skipSpace()
		if p.i >= len(p.b) || p.b[p.i] != '"' {
			return nil, p.errorf("want a string key")
		}
		key, err := p.str()
		if err != nil {
			return nil, err
		}
		p.skipSpace()
		if p.i >= len(p.b) || p.b[p.i] != ':' {
			return nil, p.errorf("want ':'")
		}
		p.i++
		p.skipSpace()
		v, err := p.value(depth)
		if err != nil {
			return nil, err
		}
		m[key] = v
		p.skipSpace()
		if p.i >= len(p.b) {
			return nil, p.errorf("unexpected end of input")
		}
		switch p.b[p.i] {
		case ',':
			p.i++
		case '}':
			p.i++
			return m, nil
		default:
			return nil, p.errorf("want ',' or '}'")
		}
	}
}

func (p *parser) array(depth int) (any, error) {
	p.i++
	a := []any{}
	p.skipSpace()
	if p.i < len(p.b) && p.b[p.i] == ']' {
		p.i++
		return a, nil
	}
	for {
		p.skipSpace()
		v, err := p.value(depth)
		if err != nil {
			return nil, err
		}
		a = append(a, v)
		p.skipSpace()
		if p.i >= len(p.b) {
			return nil, p.errorf("unexpected end of input")
		}
		switch p.b[p.i] {
		case ',':
			p.i++
		case ']':
			p.i++
			return a, nil
		default:
			return nil, p.errorf("want ',' or ']'")
		}
	}
}

// str reads a string. Like encoding/json, invalid UTF-8 and lone surrogates become U+FFFD.
func (p *parser) str() (string, error) {
	p.i++
	start := p.i
	for p.i < len(p.b) {
		c := p.b[p.i]
		if c == '"' {
			s := p.b[start:p.i]
			p.i++
			if utf8.Valid(s) {
				return string(s), nil
			}
			return string([]rune(string(s))), nil
		}
		if c == '\\' {
			return p.strSlow(start)
		}
		if c < 0x20 {
			return "", p.errorf("control character in string")
		}
		p.i++
	}
	return "", p.errorf("unterminated string")
}

func (p *parser) strSlow(start int) (string, error) {
	out := make([]byte, 0, p.i-start+16)
	out = append(out, p.b[start:p.i]...)
	for p.i < len(p.b) {
		c := p.b[p.i]
		switch {
		case c == '"':
			p.i++
			if utf8.Valid(out) {
				return string(out), nil
			}
			return string([]rune(string(out))), nil
		case c < 0x20:
			return "", p.errorf("control character in string")
		case c != '\\':
			out = append(out, c)
			p.i++
			continue
		}
		p.i++
		if p.i >= len(p.b) {
			break
		}
		switch e := p.b[p.i]; e {
		case '"', '\\', '/':
			out = append(out, e)
		case 'b':
			out = append(out, '\b')
		case 'f':
			out = append(out, '\f')
		case 'n':
			out = append(out, '\n')
		case 'r':
			out = append(out, '\r')
		case 't':
			out = append(out, '\t')
		case 'u':
			r, ok := p.hex4(p.i + 1)
			if !ok {
				return "", p.errorf("bad \\u escape")
			}
			p.i += 4
			if utf16.IsSurrogate(r) {
				if r2, ok := p.lowSurrogate(); ok {
					if dec := utf16.DecodeRune(r, r2); dec != utf8.RuneError {
						p.i += 6
						out = utf8.AppendRune(out, dec)
						break
					}
				}
				r = utf8.RuneError
			}
			out = utf8.AppendRune(out, r)
		default:
			return "", p.errorf("bad escape")
		}
		p.i++
	}
	return "", p.errorf("unterminated string")
}

// lowSurrogate reads a \uXXXX right after the current escape, without consuming it.
func (p *parser) lowSurrogate() (rune, bool) {
	j := p.i + 1
	if j+6 > len(p.b) || p.b[j] != '\\' || p.b[j+1] != 'u' {
		return 0, false
	}
	return p.hex4(j + 2)
}

func (p *parser) hex4(at int) (rune, bool) {
	if at+4 > len(p.b) {
		return 0, false
	}
	var r rune
	for _, c := range p.b[at : at+4] {
		switch {
		case c >= '0' && c <= '9':
			r = r<<4 | rune(c-'0')
		case c >= 'a' && c <= 'f':
			r = r<<4 | rune(c-'a'+10)
		case c >= 'A' && c <= 'F':
			r = r<<4 | rune(c-'A'+10)
		default:
			return 0, false
		}
	}
	return r, true
}

// number checks the JSON number grammar and keeps the text, like json.Number.
func (p *parser) number() (any, error) {
	start := p.i
	if p.b[p.i] == '-' {
		p.i++
	}
	switch {
	case p.i < len(p.b) && p.b[p.i] == '0':
		p.i++
	case p.i < len(p.b) && p.b[p.i] >= '1' && p.b[p.i] <= '9':
		p.digits()
	default:
		return nil, p.errorf("bad number")
	}
	if p.i < len(p.b) && p.b[p.i] == '.' {
		p.i++
		if !p.digits() {
			return nil, p.errorf("bad number")
		}
	}
	if p.i < len(p.b) && (p.b[p.i] == 'e' || p.b[p.i] == 'E') {
		p.i++
		if p.i < len(p.b) && (p.b[p.i] == '+' || p.b[p.i] == '-') {
			p.i++
		}
		if !p.digits() {
			return nil, p.errorf("bad number")
		}
	}
	return json.Number(p.b[start:p.i]), nil
}

func (p *parser) digits() bool {
	start := p.i
	for p.i < len(p.b) && p.b[p.i] >= '0' && p.b[p.i] <= '9' {
		p.i++
	}
	return p.i > start
}
