package database

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"graduation/apps/server/migrations"
)

var ErrNotFound = errors.New("not found")

type Store struct{ db *sql.DB }

func Open(ctx context.Context, dsn string) (*Store, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)
	if err = db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("connect mysql: %w", err)
	}
	return &Store{db: db}, nil
}
func (s *Store) Close() error { return s.db.Close() }
func (s *Store) Migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, migrations.Initial); err != nil {
		return err
	}
	var columns int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='room_settings' AND COLUMN_NAME='display_background'`).Scan(&columns); err != nil {
		return err
	}
	if columns == 0 {
		if _, err := s.db.ExecContext(ctx, migrations.DisplayBackground); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) CountAdmins(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM admins").Scan(&n)
	return n, err
}
func (s *Store) CreateAdmin(ctx context.Context, email, hash string) error {
	_, err := s.db.ExecContext(ctx, "INSERT INTO admins(email,password_hash) VALUES(?,?)", strings.ToLower(email), hash)
	return err
}
func (s *Store) AdminByEmail(ctx context.Context, email string) (uint64, string, error) {
	var id uint64
	var hash string
	err := s.db.QueryRowContext(ctx, "SELECT id,password_hash FROM admins WHERE email=?", strings.ToLower(email)).Scan(&id, &hash)
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrNotFound
	}
	return id, hash, err
}
func (s *Store) CreateSession(ctx context.Context, adminID uint64, tokenHash [32]byte, expires time.Time) error {
	_, err := s.db.ExecContext(ctx, "INSERT INTO sessions(admin_id,token_hash,expires_at) VALUES(?,?,?)", adminID, tokenHash[:], expires)
	return err
}
func (s *Store) SessionAdmin(ctx context.Context, token string) (uint64, error) {
	h := sha256.Sum256([]byte(token))
	var id uint64
	err := s.db.QueryRowContext(ctx, "SELECT admin_id FROM sessions WHERE token_hash=? AND expires_at>NOW()", h[:]).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrNotFound
	}
	return id, err
}
func (s *Store) DeleteSession(ctx context.Context, token string) error {
	h := sha256.Sum256([]byte(token))
	_, err := s.db.ExecContext(ctx, "DELETE FROM sessions WHERE token_hash=?", h[:])
	return err
}

const roomSelect = `SELECT r.id,r.code,r.name,r.title,r.status,r.locked,r.created_by,r.created_at,r.started_at,r.ended_at,s.enabled_reactions,s.reaction_rate_limit,s.default_display_mode,s.display_background FROM rooms r JOIN room_settings s ON s.room_id=r.id`

func scanRoom(scanner interface{ Scan(...any) error }) (RoomRecord, error) {
	var r RoomRecord
	var raw []byte
	err := scanner.Scan(&r.ID, &r.Code, &r.Name, &r.Title, &r.Status, &r.Locked, &r.CreatedBy, &r.CreatedAt, &r.StartedAt, &r.EndedAt, &raw, &r.RateLimit, &r.DisplayMode, &r.DisplayBackground)
	if err != nil {
		return r, err
	}
	if err = json.Unmarshal(raw, &r.Reactions); err != nil {
		return r, err
	}
	return r, nil
}
func (s *Store) ListRooms(ctx context.Context) ([]RoomRecord, error) {
	rows, err := s.db.QueryContext(ctx, roomSelect+" ORDER BY r.created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []RoomRecord{}
	for rows.Next() {
		r, e := scanRoom(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
func (s *Store) RoomByCode(ctx context.Context, code string) (RoomRecord, error) {
	r, err := scanRoom(s.db.QueryRowContext(ctx, roomSelect+" WHERE r.code=?", strings.ToUpper(code)))
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrNotFound
	}
	return r, err
}
func (s *Store) CreateRoom(ctx context.Context, r RoomRecord) (RoomRecord, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return r, err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, "INSERT INTO rooms(code,name,title,created_by) VALUES(?,?,?,?)", r.Code, r.Name, r.Title, r.CreatedBy)
	if err != nil {
		return r, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return r, err
	}
	raw, _ := json.Marshal(r.Reactions)
	_, err = tx.ExecContext(ctx, "INSERT INTO room_settings(room_id,enabled_reactions,reaction_rate_limit,default_display_mode,display_background) VALUES(?,?,?,?,?)", id, raw, r.RateLimit, r.DisplayMode, r.DisplayBackground)
	if err != nil {
		return r, err
	}
	if err = tx.Commit(); err != nil {
		return r, err
	}
	return s.RoomByCode(ctx, r.Code)
}
func (s *Store) UpdateRoom(ctx context.Context, r RoomRecord) error {
	raw, _ := json.Marshal(r.Reactions)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, "UPDATE rooms SET name=?,title=?,status=?,locked=?,started_at=?,ended_at=? WHERE id=?", r.Name, r.Title, r.Status, r.Locked, r.StartedAt, r.EndedAt, r.ID)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "UPDATE room_settings SET enabled_reactions=?,reaction_rate_limit=?,default_display_mode=?,display_background=? WHERE room_id=?", raw, r.RateLimit, r.DisplayMode, r.DisplayBackground, r.ID)
	if err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) DeleteRoom(ctx context.Context, id uint64) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM rooms WHERE id=?", id)
	return err
}

func (s *Store) CreateDisplayToken(ctx context.Context, roomID uint64, name string, tokenHash [32]byte) (uint64, error) {
	res, err := s.db.ExecContext(ctx, "INSERT INTO display_tokens(room_id,name,token_hash) VALUES(?,?,?)", roomID, name, tokenHash[:])
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	return uint64(id), err
}
func (s *Store) ValidateDisplayToken(ctx context.Context, roomID uint64, token string) (uint64, string, error) {
	h := sha256.Sum256([]byte(token))
	var id uint64
	var name string
	err := s.db.QueryRowContext(ctx, "SELECT id,name FROM display_tokens WHERE room_id=? AND token_hash=? AND revoked_at IS NULL", roomID, h[:]).Scan(&id, &name)
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrNotFound
	}
	if err == nil {
		_, _ = s.db.ExecContext(ctx, "UPDATE display_tokens SET last_seen_at=NOW() WHERE id=?", id)
	}
	return id, name, err
}
func (s *Store) ListDisplayTokens(ctx context.Context, roomID uint64) ([]DisplayTokenRecord, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id,name,revoked_at IS NOT NULL,last_seen_at,created_at FROM display_tokens WHERE room_id=? ORDER BY created_at", roomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DisplayTokenRecord{}
	for rows.Next() {
		var d DisplayTokenRecord
		if err = rows.Scan(&d.ID, &d.Name, &d.Revoked, &d.LastSeenAt, &d.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
func (s *Store) RevokeDisplayToken(ctx context.Context, roomID, id uint64) error {
	_, err := s.db.ExecContext(ctx, "UPDATE display_tokens SET revoked_at=NOW() WHERE room_id=? AND id=?", roomID, id)
	return err
}

func (s *Store) InsertEvent(ctx context.Context, roomID *uint64, typ, msg string) {
	_, _ = s.db.ExecContext(ctx, "INSERT INTO system_events(room_id,event_type,message) VALUES(?,?,?)", roomID, typ, msg)
}
func (s *Store) RecentEvents(ctx context.Context, limit int) ([]Event, error) {
	if limit < 1 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, "SELECT id,room_id,level,event_type,message,created_at FROM system_events ORDER BY id DESC LIMIT ?", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Event{}
	for rows.Next() {
		var e Event
		if err = rows.Scan(&e.ID, &e.RoomID, &e.Level, &e.Type, &e.Message, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
func (s *Store) FlushStats(ctx context.Context, roomID uint64, at time.Time, counts map[string]uint64) error {
	if len(counts) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for emoji, count := range counts {
		_, err = tx.ExecContext(ctx, "INSERT INTO reaction_stats(room_id,bucket_at,emoji,count) VALUES(?,?,?,?) ON DUPLICATE KEY UPDATE count=count+VALUES(count)", roomID, at.Truncate(5*time.Second), emoji, count)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}
func (s *Store) Stats(ctx context.Context, roomID uint64) ([]StatPoint, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT bucket_at,emoji,count FROM reaction_stats WHERE room_id=? ORDER BY bucket_at", roomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []StatPoint{}
	for rows.Next() {
		var p StatPoint
		if err = rows.Scan(&p.At, &p.Emoji, &p.Count); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
func (s *Store) RoomMetrics(ctx context.Context, roomID uint64) (RoomMetrics, error) {
	var metrics RoomMetrics
	err := s.db.QueryRowContext(ctx, "SELECT total_reactions,peak_reactions_per_second,peak_concurrent_users,unique_clients,rate_limited_reactions FROM room_metrics WHERE room_id=?", roomID).Scan(&metrics.TotalReactions, &metrics.PeakReactionsPerSecond, &metrics.PeakConcurrentUsers, &metrics.UniqueClients, &metrics.RateLimitedReactions)
	if errors.Is(err, sql.ErrNoRows) {
		return RoomMetrics{}, nil
	}
	return metrics, err
}
func (s *Store) FlushRoomMetrics(ctx context.Context, roomID uint64, metrics RoomMetrics) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO room_metrics(room_id,total_reactions,peak_reactions_per_second,peak_concurrent_users,unique_clients,rate_limited_reactions) VALUES(?,?,?,?,?,?)
		ON DUPLICATE KEY UPDATE total_reactions=VALUES(total_reactions),peak_reactions_per_second=GREATEST(peak_reactions_per_second,VALUES(peak_reactions_per_second)),peak_concurrent_users=GREATEST(peak_concurrent_users,VALUES(peak_concurrent_users)),unique_clients=GREATEST(unique_clients,VALUES(unique_clients)),rate_limited_reactions=VALUES(rate_limited_reactions)`,
		roomID, metrics.TotalReactions, metrics.PeakReactionsPerSecond, metrics.PeakConcurrentUsers, metrics.UniqueClients, metrics.RateLimitedReactions)
	return err
}

func IsDuplicate(err error) bool {
	return err != nil && strings.Contains(err.Error(), "Duplicate entry")
}
