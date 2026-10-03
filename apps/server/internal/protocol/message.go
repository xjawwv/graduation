package protocol

import "encoding/json"

type Role string
type RoomStatus string
type DisplayMode string

const (
	RoleStudent              Role        = "student"
	RoleDisplay              Role        = "display"
	RoleAdmin                Role        = "admin"
	StatusWaiting            RoomStatus  = "WAITING"
	StatusActive             RoomStatus  = "ACTIVE"
	StatusPaused             RoomStatus  = "PAUSED"
	StatusEnded              RoomStatus  = "ENDED"
	ModeReactions            DisplayMode = "REACTIONS"
	ModeAnnouncement         DisplayMode = "ANNOUNCEMENT"
	ModeCountdown            DisplayMode = "COUNTDOWN"
	ModeCelebration          DisplayMode = "CELEBRATION"
	ModeQR                   DisplayMode = "QR"
	ModeBlank                DisplayMode = "BLANK"
	DefaultDisplayBackground             = "#09090b"
)

type Incoming struct {
	Type       string      `json:"type"`
	Role       Role        `json:"role,omitempty"`
	Room       string      `json:"room,omitempty"`
	ClientID   string      `json:"client_id,omitempty"`
	Token      string      `json:"token,omitempty"`
	Name       string      `json:"name,omitempty"`
	Emoji      string      `json:"emoji,omitempty"`
	Mode       DisplayMode `json:"mode,omitempty"`
	Message    string      `json:"message,omitempty"`
	Background string      `json:"background,omitempty"`
	Reactions  []string    `json:"reactions,omitempty"`
	Duration   int         `json:"duration,omitempty"`
	SentAt     int64       `json:"sent_at,omitempty"`
}

type Presence struct {
	Online        int `json:"online"`
	Students      int `json:"students"`
	Admins        int `json:"admins"`
	Displays      int `json:"displays"`
	UniqueClients int `json:"unique_clients"`
}
type DisplayHealth struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Online        bool   `json:"online"`
	LatencyMS     int64  `json:"latency_ms"`
	LastHeartbeat int64  `json:"last_heartbeat"`
}
type RoomState struct {
	Code       string     `json:"code"`
	Title      string     `json:"title"`
	Status     RoomStatus `json:"status"`
	Locked     bool       `json:"locked"`
	Background string     `json:"background,omitempty"`
}

type Outgoing struct {
	Type            string            `json:"type"`
	Message         string            `json:"message,omitempty"`
	Room            *RoomState        `json:"room,omitempty"`
	Presence        *Presence         `json:"presence,omitempty"`
	Reactions       map[string]uint64 `json:"reactions,omitempty"`
	ReactionConfig  []string          `json:"reaction_config,omitempty"`
	Mode            DisplayMode       `json:"mode,omitempty"`
	Background      string            `json:"background,omitempty"`
	Duration        int               `json:"duration,omitempty"`
	CountdownEndsAt int64             `json:"countdown_ends_at,omitempty"`
	Displays        []DisplayHealth   `json:"displays,omitempty"`
	ServerTime      int64             `json:"server_time,omitempty"`
	SentAt          int64             `json:"sent_at,omitempty"`
	Code            string            `json:"code,omitempty"`
}

func Encode(v Outgoing) []byte { b, _ := json.Marshal(v); return b }

func ValidDisplayBackground(value string) bool {
	if value == "transparent" {
		return true
	}
	if len(value) != 7 || value[0] != '#' {
		return false
	}
	for i := 1; i < len(value); i++ {
		c := value[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}
