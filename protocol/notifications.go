package protocol

import (
	"encoding/json"
	"errors"
)

// NotificationParams is the params of a notification that carries only _meta, such as the list-changed ones.
type NotificationParams struct {
	Meta Object `json:"_meta,omitempty"`
}

type ProgressParams struct {
	Meta          Object        `json:"_meta,omitempty"`
	ProgressToken ProgressToken `json:"progressToken"`
	Progress      float64       `json:"progress"`
	Total         *float64      `json:"total,omitempty"`
	Message       string        `json:"message,omitempty"`
}

type LoggingMessageParams struct {
	Meta   Object          `json:"_meta,omitempty"`
	Level  LoggingLevel    `json:"level"`
	Logger string          `json:"logger,omitempty"`
	Data   json.RawMessage `json:"data"`
}

func (p LoggingMessageParams) MarshalJSON() ([]byte, error) {
	type plain LoggingMessageParams
	if p.Data == nil {
		return nil, errors.New("protocol: a log message needs data")
	}
	return json.Marshal(plain(p))
}

type CancelledParams struct {
	Meta      Object    `json:"_meta,omitempty"`
	RequestID RequestID `json:"requestId"`
	Reason    string    `json:"reason,omitempty"`
}

type ResourceUpdatedParams struct {
	Meta Object `json:"_meta,omitempty"`
	URI  string `json:"uri"`
}

// UnsupportedVersionData is the data of a -32022 error.
type UnsupportedVersionData struct {
	Supported []string `json:"supported"`
	Requested string   `json:"requested"`
}

// MissingCapabilityData is the data of a -32021 error.
type MissingCapabilityData struct {
	RequiredCapabilities ClientCapabilities `json:"requiredCapabilities"`
}
