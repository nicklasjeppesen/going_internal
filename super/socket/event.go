package socket

import (
	"encoding/json"
)

// Event is the Messages sent over the websocket
// Used to differ between different actions
type Event struct {
	// Type is the message type sent
	Type string `json:"type"`
	// Payload is the data Based on the Type
	Payload json.RawMessage `json:"payload"`
}

/**
* return array of the reviewed content from  the client
*
 */
func (event *Event) Content() []string {
	var data []string
	json.Unmarshal([]byte(event.Payload), &data)
	return data
}
