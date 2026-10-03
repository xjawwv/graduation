package room

import (
	"context"
	"errors"
	"log/slog"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"graduation/apps/server/internal/database"
	"graduation/apps/server/internal/protocol"
)

const maxRoomConnections = 5_000
const maxTrackedClients = 100_000

var ErrRoomNotFound = errors.New("room not found")
var ErrRoomLocked = errors.New("room locked")
var ErrRoomFull = errors.New("room connection limit reached")
var ErrInvalidReaction = errors.New("invalid reaction")
var ErrReactionsDisabled = errors.New("reactions disabled")

type Client struct {
	ID            string
	Name          string
	Role          protocol.Role
	Send          chan []byte
	lastHeartbeat atomic.Int64
	latencyMS     atomic.Int64
	limitMu       sync.Mutex
	tokens        float64
	lastRefill    time.Time
	limitReady    bool
	queueMu       sync.RWMutex
	closed        atomic.Bool
}

func NewClient(id, name string, role protocol.Role) *Client {
	c := &Client{ID: id, Name: name, Role: role, Send: make(chan []byte, 64), lastRefill: time.Now()}
	c.lastHeartbeat.Store(time.Now().UnixMilli())
	return c
}
func (c *Client) Touch(sentAt int64) {
	now := time.Now()
	c.lastHeartbeat.Store(now.UnixMilli())
	if sentAt > 0 {
		lat := now.UnixMilli() - sentAt
		if lat >= 0 && lat < 60000 {
			c.latencyMS.Store(lat)
		}
	}
}
func (c *Client) Allow(limit int, now time.Time) bool {
	c.limitMu.Lock()
	defer c.limitMu.Unlock()
	if !c.limitReady {
		c.tokens = float64(limit)
		c.limitReady = true
	}
	elapsed := now.Sub(c.lastRefill).Seconds()
	c.tokens = min(float64(limit), c.tokens+elapsed*float64(limit))
	c.lastRefill = now
	if c.tokens < 1 {
		return false
	}
	c.tokens--
	return true
}
func (c *Client) Queue(msg []byte) bool {
	c.queueMu.RLock()
	defer c.queueMu.RUnlock()
	if c.closed.Load() {
		return false
	}
	select {
	case c.Send <- msg:
		return true
	default:
		return false
	}
}
func (c *Client) Close() {
	c.queueMu.Lock()
	defer c.queueMu.Unlock()
	if c.closed.CompareAndSwap(false, true) {
		close(c.Send)
	}
}

type Snapshot struct {
	Room                   database.RoomRecord      `json:"room"`
	Presence               protocol.Presence        `json:"presence"`
	TotalReactions         uint64                   `json:"total_reactions"`
	ReactionCounts         map[string]uint64        `json:"reaction_counts"`
	ReactionsPerSecond     uint64                   `json:"reactions_per_second"`
	PeakReactionsPerSecond uint64                   `json:"peak_reactions_per_second"`
	PeakConcurrent         int                      `json:"peak_concurrent"`
	UniqueUsers            int                      `json:"unique_users"`
	RateLimited            uint64                   `json:"rate_limited"`
	Displays               []protocol.DisplayHealth `json:"displays"`
	Announcement           string                   `json:"announcement"`
	Prompt                 string                   `json:"prompt"`
	CountdownEndsAt        int64                    `json:"countdown_ends_at"`
}

type Runtime struct {
	mu                 sync.RWMutex
	record             database.RoomRecord
	clients            map[*Client]struct{}
	seen               map[string]struct{}
	buffer             map[string]uint64
	unflushed          map[string]uint64
	totals             map[string]uint64
	displayHistory     map[string]protocol.DisplayHealth
	announcement       string
	prompt             string
	countdownEndsAt    int64
	total              uint64
	rateLimited        uint64
	rateCount          uint64
	currentRate        uint64
	peakRate           uint64
	rateSecond         int64
	studentConnections int
	peakConcurrent     int
	uniqueTotal        int
}

func newRuntime(r database.RoomRecord) *Runtime {
	if r.DisplayBackground == "" {
		r.DisplayBackground = protocol.DefaultDisplayBackground
	}
	return &Runtime{record: r, clients: map[*Client]struct{}{}, seen: map[string]struct{}{}, buffer: map[string]uint64{}, unflushed: map[string]uint64{}, totals: map[string]uint64{}, displayHistory: map[string]protocol.DisplayHealth{}, rateSecond: time.Now().Unix()}
}
func (r *Runtime) Record() database.RoomRecord {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := r.record
	out.Reactions = append([]string(nil), r.record.Reactions...)
	return out
}
func (r *Runtime) Update(rec database.RoomRecord) { r.mu.Lock(); r.record = rec; r.mu.Unlock() }
func (r *Runtime) add(c *Client) (reconnect bool, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.clients) >= maxRoomConnections {
		return false, ErrRoomFull
	}
	r.clients[c] = struct{}{}
	if c.Role == protocol.RoleStudent {
		r.studentConnections++
		if c.ID != "" {
			_, reconnect = r.seen[c.ID]
			if !reconnect && len(r.seen) < maxTrackedClients {
				r.seen[c.ID] = struct{}{}
				r.uniqueTotal++
			}
		}
		if r.studentConnections > r.peakConcurrent {
			r.peakConcurrent = r.studentConnections
		}
	}
	if c.Role == protocol.RoleDisplay {
		r.displayHistory[c.ID] = protocol.DisplayHealth{ID: c.ID, Name: c.Name, Online: true, LastHeartbeat: c.lastHeartbeat.Load()}
	}
	return reconnect, nil
}
func (r *Runtime) remove(c *Client) {
	r.mu.Lock()
	if _, existed := r.clients[c]; existed {
		delete(r.clients, c)
		if c.Role == protocol.RoleStudent {
			r.studentConnections--
		}
	}
	if c.Role == protocol.RoleDisplay {
		r.displayHistory[c.ID] = protocol.DisplayHealth{ID: c.ID, Name: c.Name, Online: false, LatencyMS: c.latencyMS.Load(), LastHeartbeat: c.lastHeartbeat.Load()}
	}
	r.mu.Unlock()
	c.Close()
}
func (r *Runtime) broadcast(msg []byte, roles ...protocol.Role) {
	allowed := map[protocol.Role]bool{}
	for _, x := range roles {
		allowed[x] = true
	}
	r.mu.RLock()
	targets := make([]*Client, 0, len(r.clients))
	for c := range r.clients {
		if allowed[c.Role] {
			targets = append(targets, c)
		}
	}
	r.mu.RUnlock()
	for _, c := range targets {
		if !c.Queue(msg) {
			r.remove(c)
		}
	}
}
func (r *Runtime) presenceLocked() protocol.Presence {
	p := protocol.Presence{}
	currentUnique := map[string]struct{}{}
	for c := range r.clients {
		p.Online++
		switch c.Role {
		case protocol.RoleStudent:
			p.Students++
			currentUnique[c.ID] = struct{}{}
		case protocol.RoleAdmin:
			p.Admins++
		case protocol.RoleDisplay:
			p.Displays++
		}
	}
	p.UniqueClients = len(currentUnique)
	return p
}
func (r *Runtime) Snapshot() Snapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()
	counts := make(map[string]uint64, len(r.totals))
	for k, v := range r.totals {
		counts[k] = v
	}
	rate := r.currentRate
	nowSecond := time.Now().Unix()
	if nowSecond == r.rateSecond {
		rate = r.rateCount
	} else if nowSecond > r.rateSecond+1 {
		rate = 0
	}
	peak := max(r.peakRate, rate)
	displays := r.displaysLocked()
	return Snapshot{Room: r.record, Presence: r.presenceLocked(), TotalReactions: r.total, ReactionCounts: counts, ReactionsPerSecond: rate, PeakReactionsPerSecond: peak, PeakConcurrent: r.peakConcurrent, UniqueUsers: r.uniqueTotal, RateLimited: r.rateLimited, Displays: displays, Announcement: r.announcement, Prompt: r.prompt, CountdownEndsAt: r.countdownEndsAt}
}
func (r *Runtime) displaysLocked() []protocol.DisplayHealth {
	health := make(map[string]protocol.DisplayHealth, len(r.displayHistory))
	for id, item := range r.displayHistory {
		health[id] = item
	}
	for c := range r.clients {
		if c.Role == protocol.RoleDisplay {
			health[c.ID] = protocol.DisplayHealth{ID: c.ID, Name: c.Name, Online: true, LatencyMS: c.latencyMS.Load(), LastHeartbeat: c.lastHeartbeat.Load()}
		}
	}
	out := make([]protocol.DisplayHealth, 0, len(health))
	for _, item := range health {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
func (r *Runtime) initialMessages() [][]byte {
	s := r.Snapshot()
	return [][]byte{protocol.Encode(protocol.Outgoing{Type: "joined", Room: &protocol.RoomState{Code: s.Room.Code, Title: s.Room.Title, Status: s.Room.Status, Locked: s.Room.Locked, Background: s.Room.DisplayBackground}, Presence: &s.Presence, ReactionConfig: s.Room.Reactions, Mode: s.Room.DisplayMode, Background: s.Room.DisplayBackground, Message: s.Announcement, CountdownEndsAt: s.CountdownEndsAt}), protocol.Encode(protocol.Outgoing{Type: "prompt", Message: s.Prompt}), protocol.Encode(protocol.Outgoing{Type: "display_health", Displays: s.Displays})}
}
func (r *Runtime) react(c *Client, emoji string) error {
	now := time.Now()
	r.mu.RLock()
	status := r.record.Status
	limit := r.record.RateLimit
	valid := false
	for _, e := range r.record.Reactions {
		if e == emoji {
			valid = true
			break
		}
	}
	r.mu.RUnlock()
	if status != protocol.StatusActive {
		return ErrReactionsDisabled
	}
	if !valid {
		return ErrInvalidReaction
	}
	if !c.Allow(limit, now) {
		r.mu.Lock()
		r.rateLimited++
		r.mu.Unlock()
		return nil
	}
	r.mu.Lock()
	sec := now.Unix()
	if sec != r.rateSecond {
		r.currentRate = r.rateCount
		if r.currentRate > r.peakRate {
			r.peakRate = r.currentRate
		}
		r.rateCount = 0
		r.rateSecond = sec
	}
	r.buffer[emoji]++
	r.unflushed[emoji]++
	r.totals[emoji]++
	r.total++
	r.rateCount++
	r.mu.Unlock()
	return nil
}
func (r *Runtime) drainBuffer() map[string]uint64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.buffer) == 0 {
		return nil
	}
	out := r.buffer
	r.buffer = map[string]uint64{}
	return out
}
func (r *Runtime) drainStats() map[string]uint64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.unflushed) == 0 {
		return nil
	}
	out := r.unflushed
	r.unflushed = map[string]uint64{}
	return out
}
func (r *Runtime) restoreStats(v map[string]uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for k, n := range v {
		r.unflushed[k] += n
	}
}
func (r *Runtime) seedStatistics(metrics database.RoomMetrics, points []database.StatPoint) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, point := range points {
		r.totals[point.Emoji] += point.Count
	}
	sum := uint64(0)
	for _, count := range r.totals {
		sum += count
	}
	r.total = max(metrics.TotalReactions, sum)
	r.peakRate = metrics.PeakReactionsPerSecond
	r.peakConcurrent = metrics.PeakConcurrentUsers
	r.uniqueTotal = metrics.UniqueClients
	r.rateLimited = metrics.RateLimitedReactions
}

type Manager struct {
	mu                                  sync.RWMutex
	statsMu                             sync.Mutex
	rooms                               map[string]*Runtime
	store                               *database.Store
	started                             time.Time
	messages                            atomic.Uint64
	reconnects                          atomic.Uint64
	shuttingDown                        atomic.Bool
	batchEvery, timePresence, timeStats time.Duration
}

func NewManager(records []database.RoomRecord, store *database.Store, batch, presence, stats time.Duration) *Manager {
	m := &Manager{rooms: map[string]*Runtime{}, store: store, started: time.Now(), batchEvery: batch, timePresence: presence, timeStats: stats}
	for _, r := range records {
		m.rooms[r.Code] = newRuntime(r)
	}
	return m
}
func (m *Manager) Get(code string) (*Runtime, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	r, ok := m.rooms[strings.ToUpper(code)]
	return r, ok
}
func (m *Manager) SeedStatistics(code string, metrics database.RoomMetrics, points []database.StatPoint) {
	if runtimeRoom, ok := m.Get(code); ok {
		runtimeRoom.seedStatistics(metrics, points)
	}
}
func (m *Manager) Add(rec database.RoomRecord) {
	m.mu.Lock()
	m.rooms[rec.Code] = newRuntime(rec)
	m.mu.Unlock()
}
func (m *Manager) Delete(code string) {
	m.mu.Lock()
	r := m.rooms[code]
	delete(m.rooms, code)
	m.mu.Unlock()
	if r != nil {
		r.broadcast(protocol.Encode(protocol.Outgoing{Type: "system", Code: "room_deleted", Message: "Room was deleted"}), protocol.RoleStudent, protocol.RoleDisplay, protocol.RoleAdmin)
		r.mu.RLock()
		clients := make([]*Client, 0, len(r.clients))
		for c := range r.clients {
			clients = append(clients, c)
		}
		r.mu.RUnlock()
		for _, c := range clients {
			r.remove(c)
		}
	}
}
func (m *Manager) Join(code string, c *Client) error {
	if m.shuttingDown.Load() {
		return errors.New("service shutting down")
	}
	r, ok := m.Get(code)
	if !ok {
		return ErrRoomNotFound
	}
	r.mu.RLock()
	locked := r.record.Locked
	r.mu.RUnlock()
	if locked && c.Role == protocol.RoleStudent {
		return ErrRoomLocked
	}
	reconnect, err := r.add(c)
	if err != nil {
		return err
	}
	if reconnect {
		m.reconnects.Add(1)
	}
	for _, msg := range r.initialMessages() {
		if !c.Queue(msg) {
			r.remove(c)
			return errors.New("client queue full")
		}
	}
	m.messages.Add(1)
	return nil
}
func (m *Manager) Leave(code string, c *Client) {
	if r, ok := m.Get(code); ok {
		r.remove(c)
	}
}
func (m *Manager) React(code string, c *Client, emoji string) error {
	r, ok := m.Get(code)
	if !ok {
		return ErrRoomNotFound
	}
	m.messages.Add(1)
	return r.react(c, emoji)
}
func (m *Manager) BroadcastState(code string) {
	if r, ok := m.Get(code); ok {
		s := r.Snapshot()
		r.broadcast(protocol.Encode(protocol.Outgoing{Type: "room_state", Room: &protocol.RoomState{Code: s.Room.Code, Title: s.Room.Title, Status: s.Room.Status, Locked: s.Room.Locked, Background: s.Room.DisplayBackground}}), protocol.RoleStudent, protocol.RoleDisplay, protocol.RoleAdmin)
	}
}
func (m *Manager) BroadcastControl(code string, event protocol.Outgoing, roles ...protocol.Role) {
	if r, ok := m.Get(code); ok {
		r.broadcast(protocol.Encode(event), roles...)
	}
}
func (m *Manager) SetAnnouncement(code, msg string, duration int) {
	if r, ok := m.Get(code); ok {
		r.mu.Lock()
		r.announcement = msg
		r.mu.Unlock()
		r.broadcast(protocol.Encode(protocol.Outgoing{Type: "announcement", Message: msg, Duration: duration}), protocol.RoleDisplay, protocol.RoleAdmin)
	}
}
func (m *Manager) SetPrompt(code, msg string) {
	if r, ok := m.Get(code); ok {
		r.mu.Lock()
		r.prompt = msg
		r.mu.Unlock()
		r.broadcast(protocol.Encode(protocol.Outgoing{Type: "prompt", Message: msg}), protocol.RoleStudent, protocol.RoleAdmin)
	}
}
func (m *Manager) SetCountdown(code string, ends int64) {
	if r, ok := m.Get(code); ok {
		r.mu.Lock()
		r.countdownEndsAt = ends
		r.mu.Unlock()
	}
}
func (m *Manager) ClearDisplay(code string) {
	if r, ok := m.Get(code); ok {
		r.mu.Lock()
		r.buffer = map[string]uint64{}
		r.mu.Unlock()
		r.broadcast(protocol.Encode(protocol.Outgoing{Type: "clear_display"}), protocol.RoleDisplay, protocol.RoleAdmin)
	}
}
func (m *Manager) DisconnectAll(code string) {
	if r, ok := m.Get(code); ok {
		r.mu.RLock()
		clients := make([]*Client, 0, len(r.clients))
		for c := range r.clients {
			if c.Role != protocol.RoleAdmin {
				clients = append(clients, c)
			}
		}
		r.mu.RUnlock()
		for _, c := range clients {
			r.remove(c)
		}
	}
}
func (m *Manager) DisconnectDisplay(code, id string) {
	if r, ok := m.Get(code); ok {
		r.mu.RLock()
		clients := make([]*Client, 0, 1)
		for c := range r.clients {
			if c.Role == protocol.RoleDisplay && c.ID == id {
				clients = append(clients, c)
			}
		}
		r.mu.RUnlock()
		for _, c := range clients {
			r.remove(c)
		}
	}
}
func (m *Manager) Start(ctx context.Context) {
	batch := time.NewTicker(m.batchEvery)
	presence := time.NewTicker(m.timePresence)
	defer batch.Stop()
	defer presence.Stop()
	go m.runStats(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-batch.C:
			m.flushBatches()
		case <-presence.C:
			m.broadcastPresence()
		}
	}
}
func (m *Manager) runStats(ctx context.Context) {
	ticker := time.NewTicker(m.timeStats)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			flushCtx, cancel := context.WithTimeout(ctx, min(m.timeStats, 4*time.Second))
			m.flushStats(flushCtx)
			cancel()
		}
	}
}
func (m *Manager) all() []*Runtime {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*Runtime, 0, len(m.rooms))
	for _, r := range m.rooms {
		out = append(out, r)
	}
	return out
}
func (m *Manager) flushBatches() {
	for _, r := range m.all() {
		if b := r.drainBuffer(); len(b) > 0 {
			r.broadcast(protocol.Encode(protocol.Outgoing{Type: "reaction_batch", Reactions: b}), protocol.RoleDisplay, protocol.RoleAdmin)
		}
	}
}
func (m *Manager) broadcastPresence() {
	for _, r := range m.all() {
		s := r.Snapshot()
		r.broadcast(protocol.Encode(protocol.Outgoing{Type: "presence", Presence: &s.Presence, Displays: s.Displays}), protocol.RoleStudent, protocol.RoleDisplay, protocol.RoleAdmin)
	}
}
func (m *Manager) flushStats(ctx context.Context) {
	m.statsMu.Lock()
	defer m.statsMu.Unlock()
	for _, r := range m.all() {
		counts := r.drainStats()
		rec := r.Record()
		if len(counts) > 0 {
			if err := m.store.FlushStats(ctx, rec.ID, time.Now(), counts); err != nil {
				r.restoreStats(counts)
				slog.Error("reaction statistics flush failed", "room", rec.Code, "error", err)
			}
		}
		snapshot := r.Snapshot()
		if err := m.store.FlushRoomMetrics(ctx, rec.ID, database.RoomMetrics{TotalReactions: snapshot.TotalReactions, PeakReactionsPerSecond: snapshot.PeakReactionsPerSecond, PeakConcurrentUsers: snapshot.PeakConcurrent, UniqueClients: snapshot.UniqueUsers, RateLimitedReactions: snapshot.RateLimited}); err != nil {
			slog.Error("room metrics flush failed", "room", rec.Code, "error", err)
		}
	}
}
func (m *Manager) Shutdown(ctx context.Context) {
	m.shuttingDown.Store(true)
	m.flushStats(ctx)
	for _, r := range m.all() {
		r.broadcast(protocol.Encode(protocol.Outgoing{Type: "system", Code: "shutdown", Message: "Server is restarting"}), protocol.RoleStudent, protocol.RoleDisplay, protocol.RoleAdmin)
		r.mu.RLock()
		clients := make([]*Client, 0, len(r.clients))
		for c := range r.clients {
			clients = append(clients, c)
		}
		r.mu.RUnlock()
		for _, c := range clients {
			r.remove(c)
		}
	}
}
func (m *Manager) SystemMetrics() map[string]any {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	connections := 0
	reactionsPerSecond := uint64(0)
	for _, r := range m.all() {
		snapshot := r.Snapshot()
		connections += snapshot.Presence.Online
		reactionsPerSecond += snapshot.ReactionsPerSecond
	}
	uptime := max(int64(time.Since(m.started).Seconds()), 1)
	return map[string]any{"status": map[bool]string{true: "shutting_down", false: "online"}[m.shuttingDown.Load()], "uptime_seconds": uptime, "connections": connections, "messages_per_second": m.messages.Load() / uint64(uptime), "messages_total": m.messages.Load(), "reactions_per_second": reactionsPerSecond, "reconnects": m.reconnects.Load(), "memory_bytes": mem.Alloc, "goroutines": runtime.NumGoroutine()}
}
