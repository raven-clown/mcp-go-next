package jsonrpc

import (
	"encoding/json"
	"unicode/utf8"
)

// maxNesting matches encoding/json, which refuses values nested deeper than this.
const maxNesting = 10000

type scanResult int

const (
	scanOK scanResult = iota
	scanSyntax
	scanNotObject
	scanBatch
)

// scanEnvelope reads the top-level object of b in one pass and returns its members as slices of b.
// It checks the whole input is valid JSON, as encoding/json would, so a syntax error anywhere wins
// over any other problem. Values are not copied; callers clone what they keep.
func scanEnvelope(b []byte) (map[string][]byte, scanResult) {
	i := skipSpace(b, 0)
	if i >= len(b) {
		return nil, scanSyntax
	}
	if b[i] != '{' {
		end, ok := skipValue(b, i, 0)
		if !ok || skipSpace(b, end) != len(b) {
			return nil, scanSyntax
		}
		if b[i] == '[' {
			return nil, scanBatch
		}
		return nil, scanNotObject
	}
	fields := make(map[string][]byte, 6)
	i = skipSpace(b, i+1)
	if i < len(b) && b[i] == '}' {
		if skipSpace(b, i+1) != len(b) {
			return nil, scanSyntax
		}
		return fields, scanOK
	}
	for {
		i = skipSpace(b, i)
		if i >= len(b) || b[i] != '"' {
			return nil, scanSyntax
		}
		keyEnd, ok := skipString(b, i)
		if !ok {
			return nil, scanSyntax
		}
		key, ok := decodeKey(b[i:keyEnd])
		if !ok {
			return nil, scanSyntax
		}
		i = skipSpace(b, keyEnd)
		if i >= len(b) || b[i] != ':' {
			return nil, scanSyntax
		}
		i = skipSpace(b, i+1)
		valueEnd, ok := skipValue(b, i, 1)
		if !ok {
			return nil, scanSyntax
		}
		fields[key] = b[i:valueEnd]
		i = skipSpace(b, valueEnd)
		if i >= len(b) {
			return nil, scanSyntax
		}
		switch b[i] {
		case ',':
			i++
		case '}':
			if skipSpace(b, i+1) != len(b) {
				return nil, scanSyntax
			}
			return fields, scanOK
		default:
			return nil, scanSyntax
		}
	}
}

// decodeKey returns a key as encoding/json would decode it. Plain ASCII keys need no work.
func decodeKey(raw []byte) (string, bool) {
	plain := true
	for _, c := range raw[1 : len(raw)-1] {
		if c == '\\' || c >= utf8.RuneSelf {
			plain = false
			break
		}
	}
	if plain {
		return string(raw[1 : len(raw)-1]), true
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", false
	}
	return s, true
}

func skipSpace(b []byte, i int) int {
	for i < len(b) {
		switch b[i] {
		case ' ', '\t', '\n', '\r':
			i++
		default:
			return i
		}
	}
	return i
}

// skipValue returns the end of the JSON value starting at i, checking its grammar.
func skipValue(b []byte, i, depth int) (int, bool) {
	if i >= len(b) {
		return 0, false
	}
	switch c := b[i]; {
	case c == '"':
		return skipString(b, i)
	case c == '{':
		if depth >= maxNesting {
			return 0, false
		}
		return skipObject(b, i, depth+1)
	case c == '[':
		if depth >= maxNesting {
			return 0, false
		}
		return skipArray(b, i, depth+1)
	case c == '-' || (c >= '0' && c <= '9'):
		return skipNumber(b, i)
	case c == 't':
		return skipLiteral(b, i, "true")
	case c == 'f':
		return skipLiteral(b, i, "false")
	case c == 'n':
		return skipLiteral(b, i, "null")
	}
	return 0, false
}

func skipLiteral(b []byte, i int, word string) (int, bool) {
	if len(b)-i < len(word) || string(b[i:i+len(word)]) != word {
		return 0, false
	}
	return i + len(word), true
}

// skipString checks a string the way encoding/json does: no raw control characters and only valid
// escapes. Invalid UTF-8 is allowed, as encoding/json allows it.
func skipString(b []byte, i int) (int, bool) {
	i++
	for i < len(b) {
		c := b[i]
		switch {
		case c == '"':
			return i + 1, true
		case c < 0x20:
			return 0, false
		case c == '\\':
			i++
			if i >= len(b) {
				return 0, false
			}
			switch b[i] {
			case '"', '\\', '/', 'b', 'f', 'n', 'r', 't':
				i++
			case 'u':
				if len(b)-i < 5 || !isHex(b[i+1]) || !isHex(b[i+2]) || !isHex(b[i+3]) || !isHex(b[i+4]) {
					return 0, false
				}
				i += 5
			default:
				return 0, false
			}
		default:
			i++
		}
	}
	return 0, false
}

func isHex(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

func skipNumber(b []byte, i int) (int, bool) {
	if b[i] == '-' {
		i++
	}
	switch {
	case i < len(b) && b[i] == '0':
		i++
	case i < len(b) && b[i] >= '1' && b[i] <= '9':
		i = skipDigits(b, i)
	default:
		return 0, false
	}
	if i < len(b) && b[i] == '.' {
		j := skipDigits(b, i+1)
		if j == i+1 {
			return 0, false
		}
		i = j
	}
	if i < len(b) && (b[i] == 'e' || b[i] == 'E') {
		i++
		if i < len(b) && (b[i] == '+' || b[i] == '-') {
			i++
		}
		j := skipDigits(b, i)
		if j == i {
			return 0, false
		}
		i = j
	}
	return i, true
}

func skipDigits(b []byte, i int) int {
	for i < len(b) && b[i] >= '0' && b[i] <= '9' {
		i++
	}
	return i
}

func skipObject(b []byte, i, depth int) (int, bool) {
	i = skipSpace(b, i+1)
	if i < len(b) && b[i] == '}' {
		return i + 1, true
	}
	for {
		i = skipSpace(b, i)
		if i >= len(b) || b[i] != '"' {
			return 0, false
		}
		end, ok := skipString(b, i)
		if !ok {
			return 0, false
		}
		i = skipSpace(b, end)
		if i >= len(b) || b[i] != ':' {
			return 0, false
		}
		i = skipSpace(b, i+1)
		if i, ok = skipValue(b, i, depth); !ok {
			return 0, false
		}
		i = skipSpace(b, i)
		if i >= len(b) {
			return 0, false
		}
		switch b[i] {
		case ',':
			i++
		case '}':
			return i + 1, true
		default:
			return 0, false
		}
	}
}

func skipArray(b []byte, i, depth int) (int, bool) {
	i = skipSpace(b, i+1)
	if i < len(b) && b[i] == ']' {
		return i + 1, true
	}
	for {
		var ok bool
		if i, ok = skipValue(b, skipSpace(b, i), depth); !ok {
			return 0, false
		}
		i = skipSpace(b, i)
		if i >= len(b) {
			return 0, false
		}
		switch b[i] {
		case ',':
			i++
		case ']':
			return i + 1, true
		default:
			return 0, false
		}
	}
}
