package socket

import (
	"errors"
	"log"
	"net/http"
	"os"
	"sync"

	auth "github.com/nicklasjeppesen/going_internal/super/auth"
	channels "github.com/nicklasjeppesen/going_internal/super/channels"
	constants "github.com/nicklasjeppesen/going_internal/super/constants"

	"github.com/gorilla/websocket"
)

//-----------------------------------------------------------------
// 							Manager
//-----------------------------------------------------------------
//
// This manager is responsible for holding a list of Clients register to a socket.
// Also responsible for holding a list of hubs, that so a manager can send message
// to that person

// Manager is used to hold references to all Clients and hubs Registered,
// and Broadcasting
type Manager struct {

	//
	// List of all registeres clients
	clients ClientList

	//
	// Using a syncMutex here to be able to lcok state before editing clients
	// Could also use Channels to block
	sync.RWMutex

	//
	// List os all messages
	AllHubs map[string]IBaseHub
}

// NewManager is used to initalize all the values inside the manager
func NewManager() *Manager {
	m := &Manager{
		clients: make(ClientList),
		AllHubs: make(map[string]IBaseHub),
	}
	return m
}

var (
	//
	//websocketUpgrader is used to upgrade incomming HTTP requests into a persitent websocket connection
	//
	websocketUpgrader = websocket.Upgrader{
		CheckOrigin:     checkOrigin,
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}

	//
	// Variable for not supported event tpye
	//
	ErrEventNotSupported = errors.New("this event type is not supported")
)

// Check origin and return true if its allowed
func checkOrigin(r *http.Request) bool {

	// Get the URL where the request is coming from
	RecivedRequest := r.Header.Get("Origin")

	var host = os.Getenv(constants.APP_URL)
	var port = os.Getenv(constants.APP_PORT)
	var appUrl = host + ":" + port
	return RecivedRequest == appUrl
}

// serveWS is a HTTP Handler that the has the Manager that allows connections
func (m *Manager) serveWS(w http.ResponseWriter, r *http.Request, hub IBaseHub) {

	// Begin by upgrading the HTTP request
	conn, err := websocketUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	auth_ := auth.Auth{R: r, W: w}
	client := NewClient(conn, m, hub, auth_)

	// Add the newly created client to the manager
	m.addClient(client)

	go client.readMessages()
	go client.writeMessages()
}

// addClient will add clients to our clientList
func (m *Manager) addClient(client *Client) {
	// Lock so we can manipulate
	m.Lock()
	defer m.Unlock()

	// Add Client
	m.clients[client] = true
}

// Remove a client from the managers lists os clients
func (m *Manager) removeClient(client *Client) {
	m.Lock()
	defer m.Unlock()

	// Check if Client exists, then delete it
	if _, ok := m.clients[client]; ok {
		// close connection
		client.connection.Close()
		// remove
		delete(m.clients, client)
	}
}

// Send a message to all clients listen on this managers hub
func (m *Manager) Broadcast(command string, args ...any) {
	for client := range m.clients {
		client.SendMessage(command, args...)
	}
}

// Reciving a message from the app, and send it to a hub
//
// A message be send by a job or a controller
func (m *Manager) GetAppMessages() {
	websocketChannels := channels.WebSocketMessageProvider{}
	go func() {
		for {
			var websocketMessage = websocketChannels.GetMessageToSocket()
			if hub, ok := m.AllHubs[websocketMessage.URL]; ok {
				hub.AppReceiver(websocketMessage)
			}
		}
	}()
}
