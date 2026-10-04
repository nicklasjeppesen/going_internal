package socket

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	auth "github.com/nicklasjeppesen/going_internal/super/auth"
	struct_to_map "github.com/nicklasjeppesen/going_internal/super/util"
)

// ClientList is a map used to help manage a map of clients
type ClientList map[*Client]bool

// Client is a websocket client, basically a frontend visitor
type Client struct {
	connectionID uuid.UUID

	// the websocket connection
	connection *websocket.Conn

	// manager is the manager used to manage the client
	manager *Manager
	// egress is used to avoid concurrent writes on the WebSocket
	egress chan Event
	// room is the room id, the client is current in
	RoomID string
	// The given hub with the logic
	hub IBaseHub

	// The Authenticated user
	Auth auth.Auth

	// client properties; other connections read them (e.g. a room listing
	// peers), so they are guarded by propertiesMu.
	propertiesMu sync.RWMutex
	properties   map[string]string

	closeOnce sync.Once

	// sendMu guards egress against "send on closed channel": senders hold the
	// read lock while sending, closeConnection takes the write lock to close.
	sendMu sync.RWMutex
	closed bool
}

var (
	// pongWait is how long we will await a pong response from client
	pongWait = 15 * time.Second
	// pingInterval has to be less than pongWait, We cant multiply by 0.9 to get 90% of time
	// Because that can make decimals, so instead *9 / 10 to get 90%
	// The reason why it has to be less than PingRequency is becuase otherwise it will send a new Ping before getting response
	pingInterval = (pongWait * 9) / 10
)

// NewClient is used to initialize a new Client with all required values initialized
func NewClient(conn *websocket.Conn, manager *Manager, hub IBaseHub, auth auth.Auth) *Client {
	return &Client{
		connectionID: uuid.New(),
		connection:   conn,
		manager:      manager,
		egress:       make(chan Event, 1000),
		hub:          hub,
		Auth:         auth,
	}
}

func (client *Client) SetProperty(key string, value string) {
	client.propertiesMu.Lock()
	defer client.propertiesMu.Unlock()
	if client.properties == nil {
		client.properties = make(map[string]string)
	}
	client.properties[key] = value
}

func (client *Client) GetProperty(key string) (string, bool) {
	client.propertiesMu.RLock()
	defer client.propertiesMu.RUnlock()
	value, exists := client.properties[key]
	return value, exists
}

func (client *Client) RemoveProperty(key string) {
	client.propertiesMu.Lock()
	defer client.propertiesMu.Unlock()
	delete(client.properties, key)
}

func (client *Client) GetManager() *Manager {
	return client.manager
}

// trySend queues an event for writeMessages. It returns errClientClosed if the
// connection is closed and errEgressFull if the client can't keep up.
func (client *Client) trySend(event Event) error {
	client.sendMu.RLock()
	defer client.sendMu.RUnlock()
	if client.closed {
		return errClientClosed
	}
	select {
	case client.egress <- event:
		return nil
	default:
		return errEgressFull
	}
}

var (
	errClientClosed = errors.New("client connection is closed")
	errEgressFull   = errors.New("client egress channel full")
)

func (client *Client) SendEvent(event Event) {
	if err := client.trySend(event); err == errEgressFull {
		log.Printf("client %s egress channel full, dropping message", client.GetId())
		client.closeConnection()
	}
}

func (client *Client) SendMessage(command string, contents ...any) error {

	// Should properly be able to send without casting first
	response := []any{}
	for i := range contents {
		structValue := struct_to_map.HasJsonFunc(contents[i])
		response = append(response, structValue)
	}

	b, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %v", err)
	}
	var newMessage = Event{Type: command, Payload: b}

	if err := client.trySend(newMessage); err != nil {
		return fmt.Errorf("client %s: %w", client.GetId(), err)
	}
	return nil
}

func (client *Client) GetId() string {
	return client.connectionID.String()
}

func (c *Client) closeConnection() {
	c.closeOnce.Do(func() {
		c.hub.CancelConnection(c) // Custom method.

		c.sendMu.Lock()
		c.closed = true
		close(c.egress)
		c.sendMu.Unlock()

		c.manager.removeClient(c)
		c.hub.unregisterClient(c) // remove client from all rooms and, and messageHub client List
		c.connection.Close()
	})
}

// readMessages will start the client to read messages and handle them
// appropriatly.
// This is suppose to be ran as a goroutine
func (c *Client) readMessages() {
	defer c.closeConnection()

	// Set Max Size of Messages in Bytes
	c.connection.SetReadLimit(512 * 1024)
	// Configure Wait time for Pong response, use Current time + pongWait
	// This has to be done here to set the first initial timer.
	if err := c.connection.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		log.Println(err)
		return
	}
	// Configure how to handle Pong responses
	c.connection.SetPongHandler(c.pongHandler)

	// Loop Forever
	for {
		// ReadMessage is used to read the next message in queue
		// in the connection
		_, payload, err := c.connection.ReadMessage()

		if err != nil {
			// If Connection is closed, we will Recieve an error here
			// We only want to log Strange errors, but simple Disconnection
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error reading message: %v", err)
			}
			break // Break the loop to close conn & Cleanup
		}

		// Marshal incoming data into a Event struct
		var request Event
		if err := json.Unmarshal(payload, &request); err != nil {
			log.Printf("error unmarshalling message: %v", err)
			continue
		}

		//------ CALL ROUTE HUB HERE! -------------//
		c.hub.Routing(request, c)
	}
}

// pongHandler is used to handle PongMessages for the Client
func (c *Client) pongHandler(pongMsg string) error {
	// Current time + Pong Wait time
	return c.connection.SetReadDeadline(time.Now().Add(pongWait))
}

// writeMessages is a process that listens for new messages to output to the Client
func (c *Client) writeMessages() {
	// Create a ticker that triggers a ping at given interval
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	for {
		select {
		case message, ok := <-c.egress:
			// Ok will be false In case the egress channel is closed
			if !ok {
				// Manager has closed this connection channel, so communicate that to frontend
				if err := c.connection.WriteMessage(websocket.CloseMessage, nil); err != nil {
					// Log that the connection is closed and the reason
					log.Println("connection closed: ", err)
				}
				// Return to close the goroutine
				return
			}

			data, err := json.Marshal(message)
			if err != nil {
				log.Println(err)
				continue // closes the connection, should we really
			}
			// Write a Regular text message to the connection
			if err := c.connection.WriteMessage(websocket.TextMessage, data); err != nil {
				log.Println(err)
				return
			}

		case <-ticker.C:
			// Send the Ping
			if err := c.connection.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				log.Println("writemsg: ", err)
				return
			}
		}

	}
}
