package websocket

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"graduation/apps/server/internal/auth"
	"graduation/apps/server/internal/database"
	"graduation/apps/server/internal/protocol"
	"graduation/apps/server/internal/room"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 45 * time.Second
	pingPeriod     = 20 * time.Second
	maxMessageSize = 4096
)

type Handler struct {
	manager  *room.Manager
	store    *database.Store
	auth     *auth.Service
	origin   string
	upgrader websocket.Upgrader
}

func New(manager *room.Manager, store *database.Store, authService *auth.Service, origin string) *Handler {
	h := &Handler{manager: manager, store: store, auth: authService, origin: strings.TrimSuffix(origin, "/")}
	h.upgrader = websocket.Upgrader{ReadBufferSize: 1024, WriteBufferSize: 1024, CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		return origin == "" || origin == h.origin
	}}
	return h
}
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	cctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	conn.SetReadLimit(maxMessageSize)
	_ = conn.SetReadDeadline(time.Now().Add(15 * time.Second))
	var first protocol.Incoming
	if err = conn.ReadJSON(&first); err != nil || first.Type != "join" {
		closeWith(conn, websocket.ClosePolicyViolation, "join required")
		return
	}
	code := strings.ToUpper(strings.TrimSpace(first.Room))
	if code == "" {
		closeWith(conn, websocket.ClosePolicyViolation, "room required")
		return
	}
	runtimeRoom, ok := h.manager.Get(code)
	if !ok {
		closeWith(conn, websocket.ClosePolicyViolation, "room not found")
		return
	}
	rec := runtimeRoom.Record()
	var client *room.Client
	switch first.Role {
	case protocol.RoleStudent:
		if first.ClientID == "" || len(first.ClientID) > 80 {
			closeWith(conn, websocket.ClosePolicyViolation, "client_id required")
			return
		}
		client = room.NewClient(first.ClientID, "", first.Role)
	case protocol.RoleAdmin:
		if _, err = h.auth.AdminID(r); err != nil {
			closeWith(conn, websocket.ClosePolicyViolation, "unauthorized")
			return
		}
		client = room.NewClient("admin-"+time.Now().Format("150405.000"), "Admin", first.Role)
	case protocol.RoleDisplay:
		id, name, e := h.store.ValidateDisplayToken(r.Context(), rec.ID, first.Token)
		if e != nil {
			closeWith(conn, websocket.ClosePolicyViolation, "invalid display token")
			return
		}
		client = room.NewClient(formatID(id), name, first.Role)
	default:
		closeWith(conn, websocket.ClosePolicyViolation, "invalid role")
		return
	}
	if err = h.manager.Join(code, client); err != nil {
		closeWith(conn, websocket.ClosePolicyViolation, err.Error())
		return
	}
	defer h.manager.Leave(code, client)
	client.Touch(time.Now().UnixMilli())
	_ = conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error { client.Touch(0); return conn.SetReadDeadline(time.Now().Add(pongWait)) })
	done := make(chan struct{})
	go h.writePump(cctx, conn, client, done)
	h.readPump(conn, client, code)
	cancel()
	<-done
}
func (h *Handler) readPump(conn *websocket.Conn, c *room.Client, code string) {
	for {
		var msg protocol.Incoming
		if err := conn.ReadJSON(&msg); err != nil {
			return
		}
		switch msg.Type {
		case "reaction":
			if c.Role == protocol.RoleStudent {
				_ = h.manager.React(code, c, msg.Emoji)
			}
		case "ping":
			c.Touch(msg.SentAt)
			c.Queue(protocol.Encode(protocol.Outgoing{Type: "pong", SentAt: msg.SentAt, ServerTime: time.Now().UnixMilli()}))
		default:
			if c.Role == protocol.RoleAdmin {
				h.handleAdmin(code, msg)
			}
		}
	}
}
func (h *Handler) handleAdmin(code string, msg protocol.Incoming) {
	runtimeRoom, ok := h.manager.Get(code)
	if !ok {
		return
	}
	rec := runtimeRoom.Record()
	changed := false
	eventType := ""
	eventMessage := ""
	var postControl protocol.Outgoing
	var controlRoles []protocol.Role
	hasPostControl := false
	now := time.Now()
	switch msg.Type {
	case "pause":
		if rec.Status == protocol.StatusActive {
			rec.Status = protocol.StatusPaused
			changed = true
			eventType, eventMessage = "reactions_paused", "Reactions paused"
		}
	case "resume":
		if rec.Status == protocol.StatusPaused || rec.Status == protocol.StatusWaiting {
			rec.Status = protocol.StatusActive
			if rec.StartedAt == nil {
				rec.StartedAt = &now
			}
			changed = true
			eventType, eventMessage = "reactions_resumed", "Reactions resumed"
		}
	case "lock_room":
		rec.Locked = true
		changed = true
		eventType, eventMessage = "room_locked", "Room locked"
	case "unlock_room":
		rec.Locked = false
		changed = true
		eventType, eventMessage = "room_unlocked", "Room unlocked"
	case "clear_display":
		h.manager.ClearDisplay(code)
		eventType, eventMessage = "display_cleared", "Display cleared"
	case "set_display_mode":
		if validMode(msg.Mode) {
			rec.DisplayMode = msg.Mode
			changed = true
			endsAt := int64(0)
			if msg.Mode == protocol.ModeCountdown {
				duration := min(max(msg.Duration, 1), 3600)
				endsAt = time.Now().Add(time.Duration(duration) * time.Second).UnixMilli()
			}
			postControl = protocol.Outgoing{Type: "display_mode", Mode: msg.Mode, CountdownEndsAt: endsAt}
			controlRoles = []protocol.Role{protocol.RoleDisplay, protocol.RoleAdmin}
			hasPostControl = true
			eventType, eventMessage = "display_mode_changed", "Display mode changed to "+string(msg.Mode)
		}
	case "set_display_background":
		background := strings.ToLower(strings.TrimSpace(msg.Background))
		if protocol.ValidDisplayBackground(background) {
			rec.DisplayBackground = background
			changed = true
			postControl = protocol.Outgoing{Type: "display_background", Background: background}
			controlRoles = []protocol.Role{protocol.RoleDisplay, protocol.RoleAdmin}
			hasPostControl = true
			eventType, eventMessage = "display_background_changed", "Display background changed"
		}
	case "announcement":
		if len([]byte(msg.Message)) <= 300 {
			duration := clampAnnouncementDuration(msg.Duration)
			h.manager.SetAnnouncement(code, msg.Message, duration)
			eventType, eventMessage = "announcement_sent", "Display announcement updated"
		}
	case "prompt":
		if len([]byte(msg.Message)) <= 160 {
			h.manager.SetPrompt(code, msg.Message)
			eventType, eventMessage = "prompt_sent", "Audience prompt updated"
		}
	case "reaction_config":
		if validReactions(msg.Reactions) {
			rec.Reactions = append([]string(nil), msg.Reactions...)
			changed = true
			postControl = protocol.Outgoing{Type: "reaction_config", ReactionConfig: rec.Reactions}
			controlRoles = []protocol.Role{protocol.RoleStudent, protocol.RoleAdmin}
			hasPostControl = true
			eventType, eventMessage = "reaction_config_changed", "Allowed reactions updated"
		}
	case "disconnect_all":
		h.manager.DisconnectAll(code)
		eventType, eventMessage = "clients_disconnected", "Students and displays disconnected"
	}
	if changed {
		if err := h.store.UpdateRoom(context.Background(), rec); err != nil {
			return
		}
		runtimeRoom.Update(rec)
		h.manager.BroadcastState(code)
	}
	if hasPostControl {
		if postControl.Type == "display_mode" {
			h.manager.SetCountdown(code, postControl.CountdownEndsAt)
		}
		h.manager.BroadcastControl(code, postControl, controlRoles...)
	}
	if eventType != "" {
		h.store.InsertEvent(context.Background(), &rec.ID, eventType, eventMessage)
	}
}
func (h *Handler) writePump(ctx context.Context, conn *websocket.Conn, c *room.Client, done chan<- struct{}) {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()
	defer close(done)
	defer conn.Close()
	for {
		select {
		case <-ctx.Done():
			_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
			_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseGoingAway, "closing"))
			return
		case msg, ok := <-c.Send:
			_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "closed"))
				return
			}
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
func closeWith(c *websocket.Conn, code int, msg string) {
	_ = c.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(code, msg), time.Now().Add(time.Second))
	_ = c.Close()
}
func formatID(id uint64) string { return "display-" + strconv.FormatUint(id, 10) }
func validMode(m protocol.DisplayMode) bool {
	switch m {
	case protocol.ModeReactions, protocol.ModeAnnouncement, protocol.ModeCountdown, protocol.ModeCelebration, protocol.ModeQR, protocol.ModeBlank:
		return true
	}
	return false
}
func clampAnnouncementDuration(duration int) int {
	return min(max(duration, 5), 10)
}
func validReactions(v []string) bool {
	if len(v) < 1 || len(v) > 16 {
		return false
	}
	seen := map[string]bool{}
	for _, e := range v {
		if e == "" || len([]byte(e)) > 32 || seen[e] {
			return false
		}
		seen[e] = true
	}
	return true
}
