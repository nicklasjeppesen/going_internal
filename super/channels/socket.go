package channels

//
//------------------------------------------------------------------------
// 			WebSocketMessageProvider / WebSocketMessage
//------------------------------------------------------------------------
//
// WebSocketMessageProvider is reponsible for providing logic
// That allowed a section in the code to send a message to a
// Websocket hub,
//

// WebSocket Message
//
// This is the Message type a websocket can receive from other parts of
// of the applications
type Socket struct {
	URL     string
	Message string
	Data    map[string]any
}

// This channel is used to send message to a websocket.
// The messsage is handle by the websocket manager
var (
	channel = make(chan Socket, 1000)
)

// WebSocketMessageProvider
//
// Provider with the logic for send a a message to a Websocket provider
// A websocket manager, is listen for new message from this
type WebSocketMessageProvider struct {
}

// SendMessageToSocket
//
// Send a message to a websocket hub
func (websocketChannel *WebSocketMessageProvider) SendMessageToSocket(websocket Socket) {
	channel <- websocket
}

// The Websocket manager calling this method, and waiting for a new messages.
func (websocketChannel *WebSocketMessageProvider) GetMessageToSocket() Socket {
	var message = <-channel
	return message
}

func (websocketChannel *WebSocketMessageProvider) CloseChannel() {
	close(channel)
}
