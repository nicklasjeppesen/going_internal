package socket

import (
	"sync"

	"github.com/nicklasjeppesen/going_internal/super/channels"
)

// https://programmingpercy.tech/blog/mastering-websockets-with-go/
// https://www.youtube.com/watch?v=pKpKv9MKN-E&list=PLySj4sIKv1zfO7Byp3onXFVfoGeXlmdW_&index=17&t=1435s

type IBaseHub interface {
	SetbaseURL(string)
	HasThisURL(string) bool
	GetBaseURl() string
	Routing(Event, *Client) error
	RegisterRoutes()
	SetupDefaultHub()
	CancelConnection(*Client)
	AppReceiver(channels.Socket)
	unregisterClient(client *Client)
}

type BaseHub struct {
	BaseURL  string
	Rooms    Rooms // client
	handlers map[string]func(parameter []string, c *Client) error

	// clients maps a user id to that user's open connections (one per tab or
	// device). Every connection runs its handlers in its own goroutine, so the
	// map is only touched through the methods below, which hold clientsMu.
	clientsMu sync.RWMutex
	clients   map[string]map[*Client]struct{}
}

// AddClient registers the connection under its authenticated user id. A user
// may have several connections at once (tabs, devices); each is kept until it
// closes.
func (hub *BaseHub) AddClient(client *Client) {
	hub.clientsMu.Lock()
	defer hub.clientsMu.Unlock()

	userId := client.Auth.UserIdAsString()
	if hub.clients[userId] == nil {
		hub.clients[userId] = make(map[*Client]struct{})
	}
	hub.clients[userId][client] = struct{}{}
}

// ClientsFor returns the user's open connections (empty if offline).
func (hub *BaseHub) ClientsFor(userId string) []*Client {
	hub.clientsMu.RLock()
	defer hub.clientsMu.RUnlock()

	clients := make([]*Client, 0, len(hub.clients[userId]))
	for client := range hub.clients[userId] {
		clients = append(clients, client)
	}
	return clients
}

// IsOnline reports whether the user has at least one open connection.
func (hub *BaseHub) IsOnline(userId string) bool {
	hub.clientsMu.RLock()
	defer hub.clientsMu.RUnlock()
	return len(hub.clients[userId]) > 0
}

// SendToUser sends the message to every open connection of the user. It
// returns true if at least one connection accepted it, so the caller can fall
// back to e.g. web push when the user is offline.
func (hub *BaseHub) SendToUser(userId string, command string, contents ...any) bool {
	delivered := false
	for _, client := range hub.ClientsFor(userId) {
		if client.SendMessage(command, contents...) == nil {
			delivered = true
		}
	}
	return delivered
}

func (hub *BaseHub) SetbaseURL(url string) {
	hub.BaseURL = url
}

func (hub *BaseHub) GetBaseURl() string {
	return hub.BaseURL
}

func (hub *BaseHub) HasThisURL(urlCheck string) bool {
	return hub.BaseURL == urlCheck
}

/*
  CancleConnecetion handle when connection to client is closed
  Left empty on purpose, because it should be the programmer
  Who handle what should be done.
*/
func (hub *BaseHub) CancleConnecetion(*Client) {

}

/*
  - Function to handle when connection to client is closed
  - Remove client from all rooms, and remove client from the hub client array
    Who handle what should be done.
*/
func (hub *BaseHub) unregisterClient(client *Client) {
	hub.Rooms.RemoveClientFromRooms(client)

	hub.clientsMu.Lock()
	defer hub.clientsMu.Unlock()
	// Remove only this connection; the user's other tabs stay registered.
	userId := client.Auth.UserIdAsString()
	delete(hub.clients[userId], client)
	if len(hub.clients[userId]) == 0 {
		delete(hub.clients, userId)
	}
}

/*
- Registers events
*/
func (hub *BaseHub) On(command string, callback func([]string, *Client) error) {
	hub.handlers[command] = callback
}

func (hub *BaseHub) SetupDefaultHub() {
	hub.Rooms = newRooms()
	hub.handlers = make(map[string]func(event []string, c *Client) error)
	hub.clients = make(map[string]map[*Client]struct{})
}

/*
- Get a handler method
*/
func (hub *BaseHub) Routing(event Event, client *Client) error {

	if handler, ok := hub.handlers[event.Type]; ok {
		if err := handler(event.Content(), client); err != nil {
			return err
		}
		return nil
	} else {
		return ErrEventNotSupported
	}
}

// Method to handle events from the app,
func (hub *BaseHub) AppReceiver(message channels.Socket) {

}
