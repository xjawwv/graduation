package database

import (
	"graduation/apps/server/internal/protocol"
	"time"
)

type RoomRecord struct {
	ID                uint64               `json:"id"`
	Code              string               `json:"code"`
	Name              string               `json:"name"`
	Title             string               `json:"title"`
	Status            protocol.RoomStatus  `json:"status"`
	Locked            bool                 `json:"locked"`
	CreatedBy         uint64               `json:"created_by"`
	CreatedAt         time.Time            `json:"created_at"`
	StartedAt         *time.Time           `json:"started_at"`
	EndedAt           *time.Time           `json:"ended_at"`
	Reactions         []string             `json:"reactions"`
	RateLimit         int                  `json:"reaction_rate_limit"`
	DisplayMode       protocol.DisplayMode `json:"display_mode"`
	DisplayBackground string               `json:"display_background"`
}

type Event struct {
	ID        uint64    `json:"id"`
	RoomID    *uint64   `json:"room_id,omitempty"`
	Level     string    `json:"level"`
	Type      string    `json:"type"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}
type StatPoint struct {
	At    time.Time `json:"at"`
	Emoji string    `json:"emoji"`
	Count uint64    `json:"count"`
}
type RoomMetrics struct {
	TotalReactions         uint64 `json:"total_reactions"`
	PeakReactionsPerSecond uint64 `json:"peak_reactions_per_second"`
	PeakConcurrentUsers    int    `json:"peak_concurrent"`
	UniqueClients          int    `json:"unique_users"`
	RateLimitedReactions   uint64 `json:"rate_limited"`
}
type DisplayTokenRecord struct {
	ID         uint64     `json:"id"`
	Name       string     `json:"name"`
	Revoked    bool       `json:"revoked"`
	LastSeenAt *time.Time `json:"last_seen_at"`
	CreatedAt  time.Time  `json:"created_at"`
}
