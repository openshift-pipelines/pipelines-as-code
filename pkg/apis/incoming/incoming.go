package incoming

import "encoding/json"

type (
	Params  map[string]any
	Payload struct {
		Params Params `json:"params"`
	}
)

// ParseIncomingPayload parses the payload from the incoming webhook, in json format and has only one key params.
func ParseIncomingPayload(payload []byte) (Payload, error) {
	var incomingPayload Payload
	err := json.Unmarshal(payload, &incomingPayload)
	if err != nil {
		return Payload{}, err
	}
	return incomingPayload, nil
}
