//go:build race

package jsonschema

// The race detector slows code down several times, so time limits in tests stretch with it.
const timeScale = 10
