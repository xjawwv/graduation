package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"graduation/apps/server/internal/database"
)

const CookieName = "graduation_session"

var ErrUnauthorized = errors.New("unauthorized")
var dummyPasswordHash = func() []byte {
	hash, err := bcrypt.GenerateFromPassword([]byte("constant-time-login-placeholder"), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}
	return hash
}()

type Service struct {
	store  *database.Store
	ttl    time.Duration
	secure bool
}

func New(store *database.Store, ttl time.Duration, secure bool) *Service {
	return &Service{store: store, ttl: ttl, secure: secure}
}
func HashPassword(password string) (string, error) {
	if len(password) < 12 || len(password) > 72 {
		return "", errors.New("password must be between 12 and 72 bytes")
	}
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(b), err
}
func (s *Service) Login(ctx context.Context, email, password string) (string, time.Time, error) {
	id, hash, err := s.store.AdminByEmail(ctx, strings.TrimSpace(email))
	if errors.Is(err, database.ErrNotFound) {
		_ = bcrypt.CompareHashAndPassword(dummyPasswordHash, []byte(password))
		return "", time.Time{}, ErrUnauthorized
	}
	if err != nil {
		return "", time.Time{}, err
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return "", time.Time{}, ErrUnauthorized
	}
	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		return "", time.Time{}, err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	expires := time.Now().Add(s.ttl)
	h := sha256Token(token)
	if err = s.store.CreateSession(ctx, id, h, expires); err != nil {
		return "", time.Time{}, err
	}
	return token, expires, nil
}
func sha256Token(token string) [32]byte { return sha256Sum([]byte(token)) }
func (s *Service) SetCookie(w http.ResponseWriter, token string, expires time.Time) {
	sameSite := http.SameSiteLaxMode
	if s.secure {
		sameSite = http.SameSiteNoneMode
	}
	http.SetCookie(w, &http.Cookie{Name: CookieName, Value: token, Path: "/", HttpOnly: true, Secure: s.secure, SameSite: sameSite, Expires: expires, MaxAge: int(time.Until(expires).Seconds())})
}
func (s *Service) ClearCookie(w http.ResponseWriter) {
	sameSite := http.SameSiteLaxMode
	if s.secure {
		sameSite = http.SameSiteNoneMode
	}
	http.SetCookie(w, &http.Cookie{Name: CookieName, Path: "/", HttpOnly: true, Secure: s.secure, SameSite: sameSite, MaxAge: -1, Expires: time.Unix(0, 0)})
}
func (s *Service) AdminID(r *http.Request) (uint64, error) {
	c, err := r.Cookie(CookieName)
	if err != nil {
		return 0, ErrUnauthorized
	}
	id, err := s.store.SessionAdmin(r.Context(), c.Value)
	if err != nil {
		return 0, ErrUnauthorized
	}
	return id, nil
}
func (s *Service) Logout(r *http.Request) {
	if c, err := r.Cookie(CookieName); err == nil {
		_ = s.store.DeleteSession(r.Context(), c.Value)
	}
}

// local wrapper keeps token hashing identical without exporting session internals.
func sha256Sum(data []byte) [32]byte { return sha256.Sum256(data) }
