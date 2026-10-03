package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Address            string
	DatabaseURL        string
	FrontendURL        string
	CookieSecure       bool
	SessionTTL         time.Duration
	BatchInterval      time.Duration
	PresenceInterval   time.Duration
	StatsFlushInterval time.Duration
	DefaultRateLimit   int
	BootstrapEmail     string
	BootstrapPassword  string
}

func Load() (Config, error) {
	loadLocalEnvironment()
	cfg := Config{
		Address: env("HTTP_ADDR", ":8080"), DatabaseURL: os.Getenv("DATABASE_URL"),
		FrontendURL:  env("FRONTEND_URL", "http://localhost:3000"),
		CookieSecure: envBool("COOKIE_SECURE", false), SessionTTL: 12 * time.Hour,
		BatchInterval: 150 * time.Millisecond, PresenceInterval: 3 * time.Second,
		StatsFlushInterval: 5 * time.Second, DefaultRateLimit: envInt("REACTION_RATE_LIMIT", 5),
		BootstrapEmail: os.Getenv("BOOTSTRAP_ADMIN_EMAIL"), BootstrapPassword: os.Getenv("BOOTSTRAP_ADMIN_PASSWORD"),
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.DefaultRateLimit < 1 || cfg.DefaultRateLimit > 100 {
		return Config{}, fmt.Errorf("REACTION_RATE_LIMIT must be between 1 and 100")
	}
	return cfg, nil
}
func loadLocalEnvironment() {
	for _, path := range []string{".env", "../../.env"} {
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for line := range strings.SplitSeq(string(content), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			line = strings.TrimPrefix(line, "export ")
			key, value, found := strings.Cut(line, "=")
			if !found {
				continue
			}
			key = strings.TrimSpace(key)
			if key == "" {
				continue
			}
			value = strings.Trim(strings.TrimSpace(value), `"'`)
			if _, exists := os.LookupEnv(key); !exists {
				_ = os.Setenv(key, value)
			}
		}
		return
	}
}

func env(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
func envBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}
func envInt(key string, fallback int) int {
	v, err := strconv.Atoi(os.Getenv(key))
	if err != nil {
		return fallback
	}
	return v
}
