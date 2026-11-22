package protocol

// Error codes added by MCP. The JSON-RPC 2.0 codes live in package jsonrpc.
const (
	CodeHeaderMismatch                  = -32020
	CodeMissingRequiredClientCapability = -32021
	CodeUnsupportedProtocolVersion      = -32022
)
