package protocol

import (
	"encoding/json"
	"fmt"
)

const (
	MetaProtocolVersion    = "io.modelcontextprotocol/protocolVersion"
	MetaClientInfo         = "io.modelcontextprotocol/clientInfo"
	MetaClientCapabilities = "io.modelcontextprotocol/clientCapabilities"
	MetaLogLevel           = "io.modelcontextprotocol/logLevel"
	MetaServerInfo         = "io.modelcontextprotocol/serverInfo"
	MetaSubscriptionID     = "io.modelcontextprotocol/subscriptionId"
	metaProgressToken      = "progressToken"
)

// RequestMeta is the _meta object of a request. Keys it does not know stay in Extra and survive a round trip.
type RequestMeta struct {
	ProgressToken      ProgressToken
	ProtocolVersion    string
	ClientInfo         *Implementation
	ClientCapabilities ClientCapabilities
	LogLevel           LoggingLevel
	Extra              Object
}

func (m RequestMeta) MarshalJSON() ([]byte, error) {
	out := make(map[string]any, len(m.Extra)+5)
	for k, v := range m.Extra {
		out[k] = v
	}
	if len(m.ProgressToken) > 0 {
		out[metaProgressToken] = m.ProgressToken
	}
	out[MetaProtocolVersion] = m.ProtocolVersion
	out[MetaClientCapabilities] = m.ClientCapabilities
	if m.ClientInfo != nil {
		out[MetaClientInfo] = m.ClientInfo
	}
	if m.LogLevel != "" {
		out[MetaLogLevel] = m.LogLevel
	}
	return json.Marshal(out)
}

func (m *RequestMeta) UnmarshalJSON(b []byte) error {
	var raw Object
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	*m = RequestMeta{}
	for k, v := range raw {
		var err error
		switch k {
		case metaProgressToken:
			if string(v) != "null" {
				err = json.Unmarshal(v, &m.ProgressToken)
			}
		case MetaProtocolVersion:
			err = json.Unmarshal(v, &m.ProtocolVersion)
		case MetaClientInfo:
			err = json.Unmarshal(v, &m.ClientInfo)
		case MetaClientCapabilities:
			err = json.Unmarshal(v, &m.ClientCapabilities)
		case MetaLogLevel:
			err = json.Unmarshal(v, &m.LogLevel)
		default:
			if m.Extra == nil {
				m.Extra = Object{}
			}
			m.Extra[k] = v
		}
		if err != nil {
			return fmt.Errorf("_meta %q: %w", k, err)
		}
	}
	return nil
}
