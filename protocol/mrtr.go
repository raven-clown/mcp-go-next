package protocol

import "encoding/json"

// InputRequest is a request the server needs answered before it can finish: elicitation, sampling or roots.
type InputRequest struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

type InputRequests map[string]InputRequest

// InputResponses maps each input request key to the client's result, kept raw until the handler decodes it.
type InputResponses map[string]json.RawMessage

type InputRequiredResult struct {
	ResultBase
	InputRequests InputRequests `json:"inputRequests,omitempty"`
	RequestState  string        `json:"requestState,omitempty"`
}
