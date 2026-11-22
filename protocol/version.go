// Package protocol holds the wire types of MCP revision 2026-07-28. It is the only place wire shapes live.
package protocol

import "encoding/json"

const LatestVersion = "2026-07-28"

// Object is a JSON object kept raw. A nil Object is left out of the output.
type Object = map[string]json.RawMessage
