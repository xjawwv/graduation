package room

import (
	"encoding/json"
	"graduation/apps/server/internal/database"
	"graduation/apps/server/internal/protocol"
	"testing"
	"time"
)

func TestReactionValidationAndRateLimit(t *testing.T) {
	r := newRuntime(database.RoomRecord{Code: "TEST", Status: protocol.StatusActive, RateLimit: 2, Reactions: []string{"🔥"}})
	c := NewClient("one", "", protocol.RoleStudent)
	if err := r.react(c, "🔥"); err != nil {
		t.Fatal(err)
	}
	if err := r.react(c, "nope"); err != ErrInvalidReaction {
		t.Fatalf("got %v", err)
	}
	for range 10 {
		_ = r.react(c, "🔥")
	}
	s := r.Snapshot()
	if s.TotalReactions > 3 {
		t.Fatalf("rate limit accepted too many: %d", s.TotalReactions)
	}
	if s.RateLimited == 0 {
		t.Fatal("expected rate limited count")
	}
}
func TestSlowClientDoesNotBlockBroadcast(t *testing.T) {
	r := newRuntime(database.RoomRecord{})
	c := NewClient("slow", "", protocol.RoleDisplay)
	r.add(c)
	for range cap(c.Send) {
		c.Send <- []byte("x")
	}
	start := time.Now()
	r.broadcast([]byte("x"), protocol.RoleDisplay)
	if time.Since(start) > 50*time.Millisecond {
		t.Fatal("broadcast blocked")
	}
	if !c.closed.Load() {
		t.Fatal("slow client should be closed")
	}
}

func TestControlBroadcastStaysInTargetRoom(t *testing.T) {
	manager := NewManager([]database.RoomRecord{{Code: "GRAD26"}, {Code: "OTHER"}}, nil, time.Hour, time.Hour, time.Hour)
	target := NewClient("target-display", "Target", protocol.RoleDisplay)
	other := NewClient("other-display", "Other", protocol.RoleDisplay)
	if err := manager.Join("GRAD26", target); err != nil {
		t.Fatal(err)
	}
	if err := manager.Join("OTHER", other); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		<-target.Send
		<-other.Send
	}

	manager.BroadcastControl("GRAD26", protocol.Outgoing{Type: "display_background", Background: "transparent"}, protocol.RoleDisplay)

	select {
	case raw := <-target.Send:
		var message protocol.Outgoing
		if err := json.Unmarshal(raw, &message); err != nil {
			t.Fatal(err)
		}
		if message.Type != "display_background" || message.Background != "transparent" {
			t.Fatalf("target received %#v", message)
		}
	case <-time.After(time.Second):
		t.Fatal("target room did not receive control")
	}
	select {
	case raw := <-other.Send:
		t.Fatalf("other room received control %s", raw)
	case <-time.After(20 * time.Millisecond):
	}
}
