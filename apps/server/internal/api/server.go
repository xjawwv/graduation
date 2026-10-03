package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"graduation/apps/server/internal/auth"
	"graduation/apps/server/internal/database"
	"graduation/apps/server/internal/protocol"
	"graduation/apps/server/internal/room"
	ws "graduation/apps/server/internal/websocket"
)

var roomCodePattern = regexp.MustCompile(`^[A-Z0-9]{4,12}$`)

type loginAttempt struct {
	count int
	reset time.Time
}
type Server struct {
	store          *database.Store
	manager        *room.Manager
	auth           *auth.Service
	ws             *ws.Handler
	frontendOrigin string
	loginMu        sync.Mutex
	loginAttempts  map[string]loginAttempt
}

func New(store *database.Store, manager *room.Manager, authService *auth.Service, wsHandler *ws.Handler, origin string) *Server {
	return &Server{store: store, manager: manager, auth: authService, ws: wsHandler, frontendOrigin: strings.TrimSuffix(origin, "/"), loginAttempts: map[string]loginAttempt{}}
}
func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.Recoverer, s.securityHeaders, s.cors)
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	})
	r.Handle("/ws", s.ws)
	r.Route("/api", func(r chi.Router) {
		r.Post("/auth/login", s.login)
		r.Post("/auth/logout", s.logout)
		r.Get("/rooms/{code}/public", s.publicRoom)
		r.Group(func(r chi.Router) {
			r.Use(s.requireAdmin)
			r.Get("/auth/me", func(w http.ResponseWriter, r *http.Request) {
				writeJSON(w, http.StatusOK, map[string]any{"authenticated": true})
			})
			r.Get("/rooms", s.listRooms)
			r.Post("/rooms", s.createRoom)
			r.Get("/rooms/{code}", s.getRoom)
			r.Patch("/rooms/{code}", s.patchRoom)
			r.Delete("/rooms/{code}", s.deleteRoom)
			for _, action := range []string{"start", "pause", "resume", "end", "lock", "unlock"} {
				r.Post("/rooms/{code}/"+action, s.lifecycle(action))
			}
			r.Get("/rooms/{code}/stats", s.stats)
			r.Get("/rooms/{code}/displays", s.listDisplays)
			r.Post("/rooms/{code}/displays", s.createDisplay)
			r.Delete("/rooms/{code}/displays/{id}", s.revokeDisplay)
			r.Get("/system", s.system)
		})
	})
	return r
}
func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}
func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == s.frontendOrigin {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PATCH,DELETE,OPTIONS")
			w.Header().Add("Vary", "Origin")
		}
		if r.Method == http.MethodOptions {
			if origin != s.frontendOrigin {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" && origin != "" && origin != s.frontendOrigin {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
func (s *Server) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, err := s.auth.AdminID(r)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "authentication_required")
			return
		}
		next.ServeHTTP(w, r.WithContext(withAdminID(r.Context(), id)))
	})
}
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	in.Email = strings.TrimSpace(in.Email)
	if in.Email == "" || len([]byte(in.Email)) > 254 || in.Password == "" || len([]byte(in.Password)) > 72 {
		writeError(w, http.StatusBadRequest, "invalid_login")
		return
	}
	loginKey := strings.ToLower(in.Email)
	if !s.allowLogin(loginKey) {
		w.Header().Set("Retry-After", "60")
		writeError(w, http.StatusTooManyRequests, "login_rate_limited")
		return
	}
	token, expires, err := s.auth.Login(r.Context(), in.Email, in.Password)
	if err != nil {
		if errors.Is(err, auth.ErrUnauthorized) {
			writeError(w, http.StatusUnauthorized, "invalid_credentials")
			return
		}
		serverError(w, err)
		return
	}
	s.loginMu.Lock()
	delete(s.loginAttempts, loginKey)
	s.loginMu.Unlock()
	s.auth.SetCookie(w, token, expires)
	writeJSON(w, http.StatusOK, map[string]any{"authenticated": true})
}
func (s *Server) allowLogin(key string) bool {
	now := time.Now()
	s.loginMu.Lock()
	defer s.loginMu.Unlock()
	if len(s.loginAttempts) >= 10_000 {
		for candidate, attempt := range s.loginAttempts {
			if now.After(attempt.reset) {
				delete(s.loginAttempts, candidate)
			}
		}
		if _, exists := s.loginAttempts[key]; !exists && len(s.loginAttempts) >= 10_000 {
			return false
		}
	}
	attempt := s.loginAttempts[key]
	if now.After(attempt.reset) {
		attempt = loginAttempt{reset: now.Add(time.Minute)}
	}
	if attempt.count >= 10 {
		return false
	}
	attempt.count++
	s.loginAttempts[key] = attempt
	return true
}
func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	s.auth.Logout(r)
	s.auth.ClearCookie(w)
	w.WriteHeader(http.StatusNoContent)
}
func (s *Server) listRooms(w http.ResponseWriter, r *http.Request) {
	records, err := s.store.ListRooms(r.Context())
	if err != nil {
		serverError(w, err)
		return
	}
	type item struct {
		database.RoomRecord
		Runtime *room.Snapshot `json:"runtime,omitempty"`
	}
	out := make([]item, 0, len(records))
	for _, rec := range records {
		entry := item{RoomRecord: rec}
		if rt, ok := s.manager.Get(rec.Code); ok {
			snap := rt.Snapshot()
			entry.Runtime = &snap
		}
		out = append(out, entry)
	}
	writeJSON(w, http.StatusOK, out)
}
func (s *Server) getRoom(w http.ResponseWriter, r *http.Request) {
	rec, err := s.store.RoomByCode(r.Context(), codeParam(r))
	if err != nil {
		notFoundOrError(w, err)
		return
	}
	if rt, ok := s.manager.Get(rec.Code); ok {
		writeJSON(w, http.StatusOK, rt.Snapshot())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"room": rec})
}
func (s *Server) publicRoom(w http.ResponseWriter, r *http.Request) {
	rec, err := s.store.RoomByCode(r.Context(), codeParam(r))
	if err != nil {
		notFoundOrError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": rec.Code, "title": rec.Title, "status": rec.Status, "locked": rec.Locked, "reactions": rec.Reactions})
}

type createRoomInput struct {
	Code  string `json:"code"`
	Name  string `json:"name"`
	Title string `json:"title"`
}

func (s *Server) createRoom(w http.ResponseWriter, r *http.Request) {
	var in createRoomInput
	if !decodeJSON(w, r, &in) {
		return
	}
	in.Code = strings.ToUpper(strings.TrimSpace(in.Code))
	if in.Code == "" {
		var err error
		in.Code, err = room.GenerateCode(6)
		if err != nil {
			serverError(w, err)
			return
		}
	}
	in.Name = strings.TrimSpace(in.Name)
	in.Title = strings.TrimSpace(in.Title)
	if !roomCodePattern.MatchString(in.Code) || !validText(in.Name, 120) || !validText(in.Title, 160) {
		writeError(w, http.StatusBadRequest, "invalid_room")
		return
	}
	rec, err := s.store.CreateRoom(r.Context(), database.RoomRecord{Code: in.Code, Name: in.Name, Title: in.Title, CreatedBy: adminID(r.Context()), Status: protocol.StatusWaiting, Reactions: []string{"❤️", "🔥", "😂", "😭", "👏", "🎓"}, RateLimit: 5, DisplayMode: protocol.ModeReactions, DisplayBackground: protocol.DefaultDisplayBackground})
	if err != nil {
		if database.IsDuplicate(err) {
			writeError(w, http.StatusConflict, "room_code_exists")
			return
		}
		serverError(w, err)
		return
	}
	s.manager.Add(rec)
	s.store.InsertEvent(r.Context(), &rec.ID, "room_created", "Room "+rec.Code+" created")
	writeJSON(w, http.StatusCreated, rec)
}

type patchRoomInput struct {
	Name              *string               `json:"name"`
	Title             *string               `json:"title"`
	Reactions         []string              `json:"reactions"`
	RateLimit         *int                  `json:"reaction_rate_limit"`
	DisplayMode       *protocol.DisplayMode `json:"display_mode"`
	DisplayBackground *string               `json:"display_background"`
}

func (s *Server) patchRoom(w http.ResponseWriter, r *http.Request) {
	rec, err := s.store.RoomByCode(r.Context(), codeParam(r))
	if err != nil {
		notFoundOrError(w, err)
		return
	}
	var in patchRoomInput
	if !decodeJSON(w, r, &in) {
		return
	}
	reactionsChanged := in.Reactions != nil
	modeChanged := in.DisplayMode != nil
	backgroundChanged := in.DisplayBackground != nil
	if in.Name != nil {
		rec.Name = strings.TrimSpace(*in.Name)
	}
	if in.Title != nil {
		rec.Title = strings.TrimSpace(*in.Title)
	}
	if in.Reactions != nil {
		if !validReactions(in.Reactions) {
			writeError(w, http.StatusBadRequest, "invalid_reactions")
			return
		}
		rec.Reactions = in.Reactions
	}
	if in.RateLimit != nil {
		if *in.RateLimit < 1 || *in.RateLimit > 100 {
			writeError(w, http.StatusBadRequest, "invalid_rate_limit")
			return
		}
		rec.RateLimit = *in.RateLimit
	}
	if in.DisplayMode != nil {
		if !validDisplayMode(*in.DisplayMode) {
			writeError(w, http.StatusBadRequest, "invalid_display_mode")
			return
		}
		rec.DisplayMode = *in.DisplayMode
	}
	if in.DisplayBackground != nil {
		background := strings.ToLower(strings.TrimSpace(*in.DisplayBackground))
		if !protocol.ValidDisplayBackground(background) {
			writeError(w, http.StatusBadRequest, "invalid_display_background")
			return
		}
		rec.DisplayBackground = background
	}
	if !validText(rec.Name, 120) || !validText(rec.Title, 160) {
		writeError(w, http.StatusBadRequest, "invalid_room")
		return
	}
	if err = s.store.UpdateRoom(r.Context(), rec); err != nil {
		serverError(w, err)
		return
	}
	if rt, ok := s.manager.Get(rec.Code); ok {
		rt.Update(rec)
	}
	s.manager.BroadcastState(rec.Code)
	if reactionsChanged {
		s.manager.BroadcastControl(rec.Code, protocol.Outgoing{Type: "reaction_config", ReactionConfig: rec.Reactions}, protocol.RoleStudent, protocol.RoleAdmin)
	}
	if modeChanged {
		s.manager.SetCountdown(rec.Code, 0)
		s.manager.BroadcastControl(rec.Code, protocol.Outgoing{Type: "display_mode", Mode: rec.DisplayMode}, protocol.RoleDisplay, protocol.RoleAdmin)
	}
	if backgroundChanged {
		s.manager.BroadcastControl(rec.Code, protocol.Outgoing{Type: "display_background", Background: rec.DisplayBackground}, protocol.RoleDisplay, protocol.RoleAdmin)
	}
	writeJSON(w, http.StatusOK, rec)
}
func (s *Server) deleteRoom(w http.ResponseWriter, r *http.Request) {
	rec, err := s.store.RoomByCode(r.Context(), codeParam(r))
	if err != nil {
		notFoundOrError(w, err)
		return
	}
	if err = s.store.DeleteRoom(r.Context(), rec.ID); err != nil {
		serverError(w, err)
		return
	}
	s.manager.Delete(rec.Code)
	w.WriteHeader(http.StatusNoContent)
}
func (s *Server) lifecycle(action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rec, err := s.store.RoomByCode(r.Context(), codeParam(r))
		if err != nil {
			notFoundOrError(w, err)
			return
		}
		now := time.Now()
		valid := true
		switch action {
		case "start":
			valid = rec.Status == protocol.StatusWaiting
			rec.Status = protocol.StatusActive
			rec.StartedAt = &now
		case "pause":
			valid = rec.Status == protocol.StatusActive
			rec.Status = protocol.StatusPaused
		case "resume":
			valid = rec.Status == protocol.StatusPaused
			rec.Status = protocol.StatusActive
		case "end":
			valid = rec.Status != protocol.StatusEnded
			rec.Status = protocol.StatusEnded
			rec.EndedAt = &now
		case "lock":
			rec.Locked = true
		case "unlock":
			rec.Locked = false
		}
		if !valid {
			writeError(w, http.StatusConflict, "invalid_transition")
			return
		}
		if err = s.store.UpdateRoom(r.Context(), rec); err != nil {
			serverError(w, err)
			return
		}
		if rt, ok := s.manager.Get(rec.Code); ok {
			rt.Update(rec)
		}
		s.manager.BroadcastState(rec.Code)
		s.store.InsertEvent(r.Context(), &rec.ID, "room_"+action, "Room "+strings.ToLower(action))
		writeJSON(w, http.StatusOK, rec)
	}
}
func (s *Server) stats(w http.ResponseWriter, r *http.Request) {
	rec, err := s.store.RoomByCode(r.Context(), codeParam(r))
	if err != nil {
		notFoundOrError(w, err)
		return
	}
	points, err := s.store.Stats(r.Context(), rec.ID)
	if err != nil {
		serverError(w, err)
		return
	}
	var snap any = map[string]any{}
	if rt, ok := s.manager.Get(rec.Code); ok {
		snap = rt.Snapshot()
	}
	writeJSON(w, http.StatusOK, map[string]any{"runtime": snap, "series": points})
}
func (s *Server) listDisplays(w http.ResponseWriter, r *http.Request) {
	rec, err := s.store.RoomByCode(r.Context(), codeParam(r))
	if err != nil {
		notFoundOrError(w, err)
		return
	}
	items, err := s.store.ListDisplayTokens(r.Context(), rec.ID)
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}
func (s *Server) createDisplay(w http.ResponseWriter, r *http.Request) {
	rec, err := s.store.RoomByCode(r.Context(), codeParam(r))
	if err != nil {
		notFoundOrError(w, err)
		return
	}
	var in struct {
		Name string `json:"name"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" || len(in.Name) > 120 {
		writeError(w, http.StatusBadRequest, "invalid_name")
		return
	}
	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		serverError(w, err)
		return
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	hash := sha256.Sum256([]byte(token))
	id, err := s.store.CreateDisplayToken(r.Context(), rec.ID, in.Name, hash)
	if err != nil {
		serverError(w, err)
		return
	}
	s.store.InsertEvent(r.Context(), &rec.ID, "display_authorized", in.Name+" authorized")
	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "name": in.Name, "token": token})
}
func (s *Server) revokeDisplay(w http.ResponseWriter, r *http.Request) {
	rec, err := s.store.RoomByCode(r.Context(), codeParam(r))
	if err != nil {
		notFoundOrError(w, err)
		return
	}
	id, err := parseUint(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id")
		return
	}
	if err = s.store.RevokeDisplayToken(r.Context(), rec.ID, id); err != nil {
		serverError(w, err)
		return
	}
	s.manager.DisconnectDisplay(rec.Code, "display-"+strconv.FormatUint(id, 10))
	s.store.InsertEvent(r.Context(), &rec.ID, "display_revoked", "Display authorization revoked")
	w.WriteHeader(http.StatusNoContent)
}
func (s *Server) system(w http.ResponseWriter, r *http.Request) {
	events, err := s.store.RecentEvents(r.Context(), 50)
	if err != nil {
		serverError(w, err)
		return
	}
	metrics := s.manager.SystemMetrics()
	metrics["cpu_count"] = runtime.NumCPU()
	writeJSON(w, http.StatusOK, map[string]any{"metrics": metrics, "events": events})
}
func codeParam(r *http.Request) string { return strings.ToUpper(chi.URLParam(r, "code")) }
func validReactions(v []string) bool {
	if len(v) < 1 || len(v) > 16 {
		return false
	}
	seen := map[string]bool{}
	for _, x := range v {
		if x == "" || len([]byte(x)) > 32 || seen[x] {
			return false
		}
		seen[x] = true
	}
	return true
}
func validText(value string, maxRunes int) bool {
	return value != "" && utf8.RuneCountInString(value) <= maxRunes
}
func validDisplayMode(mode protocol.DisplayMode) bool {
	switch mode {
	case protocol.ModeReactions, protocol.ModeAnnouncement, protocol.ModeCountdown, protocol.ModeCelebration, protocol.ModeQR, protocol.ModeBlank:
		return true
	default:
		return false
	}
}
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return false
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return false
	}
	return true
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func writeError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]string{"error": code})
}
func serverError(w http.ResponseWriter, err error) {
	slog.Error("request failed", "error", err)
	writeError(w, http.StatusInternalServerError, "internal_error")
}
func notFoundOrError(w http.ResponseWriter, err error) {
	if errors.Is(err, database.ErrNotFound) {
		writeError(w, http.StatusNotFound, "room_not_found")
	} else {
		serverError(w, err)
	}
}
