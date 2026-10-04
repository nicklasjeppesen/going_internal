package socket

import (
	"errors"
	"sync"

	"github.com/google/uuid"
)

// Lock order: Rooms.mu before Room.mu. Methods that take a lock never call
// another method that takes the same lock — they use the *Locked helpers.

type Room struct {
	ID   string     `json:"id"`
	Name string     `json:"name"`
	mu   sync.Mutex // beskytter Clients

	// key: clientId, value: Client
	Clients map[string]*Client `json:"clients"`
}

func (room *Room) GetClientsId() []string {
	room.mu.Lock()
	defer room.mu.Unlock()

	clientsID := make([]string, 0, len(room.Clients))
	for key := range room.Clients {
		clientsID = append(clientsID, key)
	}
	return clientsID
}

// GetClients returns a snapshot of the clients in the room. Use it instead of
// ranging over room.Clients, which other connections change concurrently.
func (room *Room) GetClients() []*Client {
	room.mu.Lock()
	defer room.mu.Unlock()

	clients := make([]*Client, 0, len(room.Clients))
	for _, cl := range room.Clients {
		clients = append(clients, cl)
	}
	return clients
}

// Send a message to all clients in this room
func (room *Room) SendMessage(command string, inputs ...any) {
	// Send outside the lock, so a slow client never blocks the room.
	for _, cl := range room.GetClients() {
		cl.SendMessage(command, inputs...)
	}
}

type Rooms struct {
	mu sync.RWMutex
	// key: Room_id, value: room
	rooms map[string]*Room

	// Client room mapping key: clientID, value: []*Room
	clientInRoom map[string][]*Room
}

func (r *Rooms) CreateRoom(name string) *Room {
	r.mu.Lock()
	defer r.mu.Unlock()

	var room = &Room{
		ID:      uuid.New().String(),
		Name:    name,
		Clients: make(map[string]*Client),
	}
	r.rooms[room.ID] = room
	return room
}

func (r *Rooms) CreateRoomWithCustomUUID(name string, customUUID string) *Room {
	r.mu.Lock()
	defer r.mu.Unlock()

	_room, ok := r.rooms[customUUID]
	if ok {
		return _room
	}

	var room = &Room{
		ID:      customUUID,
		Name:    name,
		Clients: make(map[string]*Client),
	}
	r.rooms[room.ID] = room
	return room
}

func (r *Rooms) Getroom(roomId string) *Room {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.rooms[roomId]
}

/*
- Add a Client to a room
*/
func (r *Rooms) AddClientToRoom(client *Client, roomID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	room, ok := r.rooms[roomID]
	if !ok {
		return errors.New("room does not exists")
	}

	// add Client to room list
	room.mu.Lock()
	room.Clients[client.GetId()] = client
	room.mu.Unlock()

	// Add room to client list
	r.clientInRoom[client.GetId()] = append(r.clientInRoom[client.GetId()], room)

	return nil
}

/*
- Delete a client from one room
*/
func (r *Rooms) RemoveClientFromRoom(client *Client, room *Room) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.removeClientFromRoomLocked(client, room)
}

/*
- Delete client from all its room
- often used when client connection is closed
*/
func (r *Rooms) RemoveClientFromRooms(client *Client) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Iterate over a copy: removeClientFromRoomLocked rewrites the list.
	rooms := append([]*Room(nil), r.clientInRoom[client.GetId()]...)
	for _, room := range rooms {
		r.removeClientFromRoomLocked(client, room)
	}
	delete(r.clientInRoom, client.GetId())
}

// removeClientFromRoomLocked expects r.mu to be held.
func (r *Rooms) removeClientFromRoomLocked(client *Client, room *Room) {
	// Delete client from the room
	if _, ok := r.rooms[room.ID]; ok {
		room.mu.Lock()
		delete(room.Clients, client.GetId())
		room.mu.Unlock()
	}

	// Remove the room from the client's list. Build a new slice instead of
	// shifting in place, so no other holder of the old slice sees it change.
	rooms, ok := r.clientInRoom[client.GetId()]
	if !ok {
		return
	}
	remaining := make([]*Room, 0, len(rooms))
	for _, _room := range rooms {
		if _room.ID != room.ID {
			remaining = append(remaining, _room)
		}
	}
	if len(remaining) == 0 {
		delete(r.clientInRoom, client.GetId())
		return
	}
	r.clientInRoom[client.GetId()] = remaining
}

/*
- Create a new rooms struct
*/
func newRooms() Rooms {
	return Rooms{
		rooms:        make(map[string]*Room),
		clientInRoom: make(map[string][]*Room),
	}
}
