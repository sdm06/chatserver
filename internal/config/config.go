package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds every tunable the server needs. Values come from environment
// variables so the same binary works locally, in tests, and in containers.
type Config struct {
	TCPAddr       string
	HTTPAddr      string
	TLSCert       string
	TLSKey        string
	AuthToken     string
	MaxFrame      int
	MaxClients    int
	HistorySize   int
	ReadTimeout   time.Duration
	WriteTimeout  time.Duration
	RatePerSecond float64
	RateBurst     int
	Dev           bool
}

// Load reads the environment and applies safe defaults.
func Load() Config {
	c := Config{
		TCPAddr:       env("CHAT_TCP_ADDR", ":4000"),
		HTTPAddr:      env("CHAT_HTTP_ADDR", ":8080"),
		TLSCert:       os.Getenv("CHAT_TLS_CERT"),
		TLSKey:        os.Getenv("CHAT_TLS_KEY"),
		AuthToken:     os.Getenv("CHAT_AUTH_TOKEN"),
		MaxFrame:      envInt("CHAT_MAX_FRAME", 4096),
		MaxClients:    envInt("CHAT_MAX_CLIENTS", 1000),
		HistorySize:   envInt("CHAT_HISTORY", 50),
		ReadTimeout:   envDuration("CHAT_READ_TIMEOUT", 10*time.Minute),
		WriteTimeout:  envDuration("CHAT_WRITE_TIMEOUT", 10*time.Second),
		RatePerSecond: envFloat("CHAT_RATE", 5),
		RateBurst:     envInt("CHAT_RATE_BURST", 10),
		Dev:           envBool("CHAT_DEV", false),
	}
	c.MaxFrame = clamp(c.MaxFrame, 512, 4096)
	c.MaxClients = clamp(c.MaxClients, 1, 1_000_000)
	c.HistorySize = clamp(c.HistorySize, 0, 10_000)
	return c
}

// TLSEnabled reports whether both certificate files were configured.
func (c Config) TLSEnabled() bool { return c.TLSCert != "" && c.TLSKey != "" }

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envFloat(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseFloat(v, 64); err == nil {
			return n
		}
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

func envBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
