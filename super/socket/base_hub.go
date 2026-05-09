package socket

import (
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
	CancleConnecetion(*Client)
	AppReceiver(channels.Socket)
}

func NewHub[T IBaseHub](hub T) T {

	hub.SetupDefaultHub()
	hub.RegisterRoutes()
	return hub
}

type BaseHub struct {
	BaseURL  string
	Clients  map[string]*Client
	Rooms    Rooms // client
	handlers map[string]func(parameter []string, c *Client) error
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
  - Function to handle when connection to client is closed
  - Left empty on purpose, because it should be the programmer
    Who handle what should be done.
*/
func (hub *BaseHub) CancleConnecetion(*Client) {

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
	hub.Clients = make(map[string]*Client)
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
