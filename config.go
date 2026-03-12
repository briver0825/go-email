package main

import (
	"os"
	"strconv"
	"time"
)

// Config holds all configuration.
type Config struct {
	IMAP          IMAPConfig
	HTTPAddr      string
	DBPath        string
	AttachmentDir string
	PollInterval  time.Duration
}

// IMAPConfig holds IMAP server connection details.
type IMAPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	TLS      bool
	Mailbox  string
}

// LoadConfig builds configuration from environment variables with defaults.
func LoadConfig() Config {
	return Config{
		IMAP: IMAPConfig{
			Host:     envStr("IMAP_HOST", "mailserver"),
			Port:     envInt("IMAP_PORT", 143),
			Username: envStr("IMAP_USER", "user@example.com"),
			Password: envStr("IMAP_PASS", "password"),
			TLS:      envBool("IMAP_TLS", false),
			Mailbox:  envStr("IMAP_MAILBOX", "INBOX"),
		},
		HTTPAddr:      envStr("HTTP_ADDR", ":8080"),
		DBPath:        envStr("DB_PATH", "emails.db"),
		AttachmentDir: envStr("ATTACHMENT_DIR", "attachments"),
		PollInterval:  time.Duration(envInt("POLL_INTERVAL", 30)) * time.Second,
	}
}

func envStr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}
