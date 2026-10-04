package socket

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gorilla/websocket"
	auth "github.com/nicklasjeppesen/going_internal/super/auth"
	"github.com/nicklasjeppesen/going_internal/super/constants"
)

// Run with -race: these tests exercise the shared hub/room/manager state from
// many goroutines at once, the way concurrent websocket connections do.

type testHub struct{ BaseHub }

func (h *testHub) RegisterRoutes()          {}
func (h *testHub) CancelConnection(*Client) {}
func newTestHub() *testHub                  { h := &testHub{}; h.SetupDefaultHub(); return h }
func authFor(userID string) auth.Auth {
	r := httptest.NewRequest(http.MethodGet, "/ws", nil)
	ctx := context.WithValue(r.Context(), constants.Auth_id, userID) //nolint:staticcheck // same key the JWT middleware uses
	return auth.Auth{R: r.WithContext(ctx)}
}

// fakeClient is a client without a websocket connection, for map/room tests.
func fakeClient(hub IBaseHub, userID string) *Client {
	return NewClient(nil, NewManager(), hub, authFor(userID))
}

// realClient is a client backed by a real websocket connection, so
// closeConnection can run for real.
func realClient(t *testing.T, hub IBaseHub, userID string) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		go func() {
			defer c.Close()
			for {
				if _, _, err := c.ReadMessage(); err != nil {
					return
				}
			}
		}()
	}))
	t.Cleanup(srv.Close)
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	manager := NewManager()
	client := NewClient(conn, manager, hub, authFor(userID))
	manager.addClient(client)
	return client
}

func TestHubClientsConcurrentAccess(t *testing.T) {
	hub := newTestHub()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			c := fakeClient(hub, fmt.Sprint(i%10))
			hub.AddClient(c)
			_ = hub.IsOnline(fmt.Sprint(i % 10))
			hub.SendToUser(fmt.Sprint(i%10), "ping", i)
			hub.unregisterClient(c)
		}(i)
	}
	wg.Wait()
}

// A user with several tabs gets messages in all of them, and closing one tab
// keeps the others registered.
func TestUserWithSeveralConnections(t *testing.T) {
	hub := newTestHub()
	tab1 := fakeClient(hub, "7")
	tab2 := fakeClient(hub, "7")
	hub.AddClient(tab1)
	hub.AddClient(tab2)

	if !hub.SendToUser("7", "hello") {
		t.Fatal("SendToUser(7) = false, want true")
	}
	for name, tab := range map[string]*Client{"tab1": tab1, "tab2": tab2} {
		if got := len(tab.egress); got != 1 {
			t.Errorf("%s received %d messages, want 1", name, got)
		}
	}

	hub.unregisterClient(tab1)
	if got := hub.ClientsFor("7"); len(got) != 1 || got[0] != tab2 {
		t.Fatalf("after closing tab1: ClientsFor(7) = %v, want [tab2]", got)
	}

	hub.unregisterClient(tab2)
	if hub.IsOnline("7") || hub.SendToUser("7", "hello") {
		t.Fatal("user 7 still online after the last connection closed")
	}
}

func TestClientPropertiesConcurrentAccess(t *testing.T) {
	hub := newTestHub()
	c := fakeClient(hub, "1")
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			c.SetProperty("peerId", fmt.Sprint(i))
			_, _ = c.GetProperty("peerId")
			c.RemoveProperty("peerId")
		}(i)
	}
	wg.Wait()
}

func TestRoomsConcurrentAccess(t *testing.T) {
	hub := newTestHub()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			c := fakeClient(hub, fmt.Sprint(i))
			room := hub.Rooms.CreateRoomWithCustomUUID("r", fmt.Sprint("room-", i%5))
			_ = hub.Rooms.AddClientToRoom(c, room.ID)
			room.SendMessage("ping", i)
			_ = room.GetClientsId()
			_ = room.GetClients()
			hub.Rooms.RemoveClientFromRooms(c)
		}(i)
	}
	wg.Wait()
}

func TestRemoveClientFromRoomsLeavesEveryRoom(t *testing.T) {
	hub := newTestHub()
	c := fakeClient(hub, "1")
	var rooms []*Room
	for i := 0; i < 4; i++ {
		room := hub.Rooms.CreateRoom(fmt.Sprint("room-", i))
		if err := hub.Rooms.AddClientToRoom(c, room.ID); err != nil {
			t.Fatal(err)
		}
		rooms = append(rooms, room)
	}

	hub.Rooms.RemoveClientFromRooms(c)

	for _, room := range rooms {
		if ids := room.GetClientsId(); len(ids) != 0 {
			t.Errorf("%s still has clients %v", room.Name, ids)
		}
	}
}

// Sending to a client while its connection closes must never panic with
// "send on closed channel" — that would crash the whole server.
func TestSendWhileClosing(t *testing.T) {
	for round := 0; round < 20; round++ {
		hub := newTestHub()
		client := realClient(t, hub, "1")
		hub.AddClient(client)

		var wg sync.WaitGroup
		panics := make(chan any, 10)
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer func() {
					if p := recover(); p != nil {
						panics <- p
					}
				}()
				for j := 0; j < 50; j++ {
					_ = client.SendMessage("msg", j)
					client.SendEvent(Event{Type: "evt"})
				}
			}()
		}
		client.closeConnection()
		wg.Wait()
		close(panics)
		for p := range panics {
			t.Fatalf("round %d: send panicked: %v", round, p)
		}
	}
}

func TestManagerBroadcastConcurrentWithAddRemove(t *testing.T) {
	hub := newTestHub()
	manager := NewManager()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			c := fakeClient(hub, fmt.Sprint(i))
			manager.addClient(c)
			manager.removeClient(c)
		}(i)
		go func() {
			defer wg.Done()
			manager.Broadcast("hello")
		}()
	}
	wg.Wait()
}
