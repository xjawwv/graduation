package websocket

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"graduation/apps/server/internal/database"
	"graduation/apps/server/internal/protocol"
	"graduation/apps/server/internal/room"
)

func TestStudentReactionUsesJoinedRoom(t *testing.T) {
	records := []database.RoomRecord{
		{Code: "GRAD26", Status: protocol.StatusActive, Reactions: []string{"🔥"}, RateLimit: 5},
		{Code: "OTHER", Status: protocol.StatusActive, Reactions: []string{"🔥"}, RateLimit: 5},
	}
	manager := room.NewManager(records, nil, time.Hour, time.Hour, time.Hour)
	server := httptest.NewServer(New(manager, nil, nil, "http://localhost:3000"))
	defer server.Close()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err = conn.WriteJSON(protocol.Incoming{Type: "join", Role: protocol.RoleStudent, Room: "GRAD26", ClientID: "test-client"}); err != nil {
		t.Fatal(err)
	}
	_, joinedRaw, err := conn.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	var joined protocol.Outgoing
	if err = json.Unmarshal(joinedRaw, &joined); err != nil {
		t.Fatal(err)
	}
	if joined.Type != "joined" {
		t.Fatalf("first event = %q, want joined", joined.Type)
	}
	if err = conn.WriteJSON(map[string]string{"type": "reaction", "room": "OTHER", "emoji": "🔥"}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		joinedRoom, _ := manager.Get("GRAD26")
		otherRoom, _ := manager.Get("OTHER")
		if joinedRoom.Snapshot().TotalReactions == 1 {
			if otherRoom.Snapshot().TotalReactions != 0 {
				t.Fatal("reaction trusted message room")
			}
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("reaction was not accepted")
}

func TestClampAnnouncementDuration(t *testing.T) {
	cases := []struct {
		input int
		want  int
	}{{0, 5}, {5, 5}, {7, 7}, {10, 10}, {30, 10}}
	for _, tc := range cases {
		if got := clampAnnouncementDuration(tc.input); got != tc.want {
			t.Fatalf("clampAnnouncementDuration(%d)=%d, want %d", tc.input, got, tc.want)
		}
	}
}
