package room

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"graduation/apps/server/internal/database"
	"graduation/apps/server/internal/protocol"
)

func TestReactionsAreBatchedForDisplays(t *testing.T) {
	manager := NewManager([]database.RoomRecord{{Code: "GRAD26", Status: protocol.StatusActive, Reactions: []string{"👏"}, RateLimit: 10}}, nil, 10*time.Millisecond, time.Hour, time.Hour)
	student := NewClient("student", "", protocol.RoleStudent)
	display := NewClient("display", "Main", protocol.RoleDisplay)
	if err := manager.Join("GRAD26", student); err != nil {
		t.Fatal(err)
	}
	if err := manager.Join("GRAD26", display); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		<-display.Send
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go manager.Start(ctx)
	for range 4 {
		if err := manager.React("GRAD26", student, "👏"); err != nil {
			t.Fatal(err)
		}
	}
	select {
	case raw := <-display.Send:
		var message protocol.Outgoing
		if err := json.Unmarshal(raw, &message); err != nil {
			t.Fatal(err)
		}
		if message.Type != "reaction_batch" || message.Reactions["👏"] != 4 {
			t.Fatalf("unexpected batch: %+v", message)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for reaction batch")
	}
}
