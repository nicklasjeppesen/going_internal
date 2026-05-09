package socket

import (
	"errors"
	"sync"

	"github.com/google/uuid"
)

type Room struct {
	ID   string     `json:"id"`
	Name string     `json:"name"`
	mu   sync.Mutex // beskytter Clients

	// key: clientId, value: Client
	Clients map[string]*Client `json:"clients"`
}

func (room *Room) GetClientsId() []string {

	clientsID := []string{}
	for key := range room.Clients {
		clientsID = append(clientsID, key)
	}
	return clientsID
}

// Send a message to all clients in this room
func (room *Room) SendMessage(command string, inputs ...any) {

	for _, cl := range room.Clients {
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

	var room = &Room{
		ID:      uuid.New().String(),
		Name:    name,
		Clients: make(map[string]*Client),
	}
	r.rooms[room.ID] = room
	return room
}

func (r *Rooms) CreateRoomWithCustomUUID(name string, customUUID string) *Room {

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

	if room, ok := r.rooms[roomId]; !ok {
		return nil
	} else {
		return room
	}
}

/*
- Add a Client to a room
*/
func (r *Rooms) AddClientToRoom(client *Client, roomID string) error {

	//	r.mu.Lock() // Casuing a deadlock problem, if user try to create a new room while already in a room
	//	defer r.mu.Unlock()

	if _, ok := r.rooms[roomID]; !ok {
		return errors.New("room does not exists")
	}

	var room = r.rooms[roomID]

	// add Client to room list
	room.Clients[client.GetId()] = client

	// Add room to client list
	if existingRooms, ok := r.clientInRoom[client.GetId()]; ok {
		r.clientInRoom[client.GetId()] = append(existingRooms, room)
	} else {
		r.clientInRoom[client.GetId()] = []*Room{room}
	}

	return nil
}

/*
- Delete a client from its room(s)
*/
func (r *Rooms) RemoveClientFromRoom(client *Client, room *Room) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Delete client from the room
	if _, ok := r.rooms[room.ID]; ok {

		room.mu.Lock()
		delete(room.Clients, client.GetId())
		room.mu.Unlock()
	}

	// remove room from client list.
	rooms, ok := r.clientInRoom[client.GetId()]
	if !ok {
		return
	}

	// Delete the client from the list of where it belongs
	for i, _room := range rooms {
		if _room.ID == room.ID {
			rooms = append(rooms[:i], rooms[i+1:]...)
			break
		}
	}

	r.clientInRoom[client.GetId()] = rooms
}

/*
- Delete client from all its room
- often used when client connection is closed
*/
func (r *Rooms) RemoveClientFromRooms(client *Client) {

	//r.mu.Lock() // Warning casing deadlock
	//r.mu.Unlock() // Warning casing deadlock

	// Delete client from all its rooms
	var rooms, ok = r.clientInRoom[client.GetId()]
	if !ok {
		return
	}
	for _, room := range rooms {
		//	room.mu.Lock() // Warning casing deadlock
		r.RemoveClientFromRoom(client, room)
		//	room.mu.Lock() // Warning casing deadlock
	}
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
