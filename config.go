package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type Config struct {
	PlexURL                  string `json:"plex_url"`
	PlexToken                string `json:"plex_token"`
	QBittorrentURL           string `json:"qbittorrent_url"`
	QBittorrentUsername      string `json:"qbittorrent_username"`
	QBittorrentPassword      string `json:"qbittorrent_password"`
	IdleUploadKbps           int    `json:"idle_upload_kbps"`
	StreamingUploadKbps      int    `json:"streaming_upload_kbps"`
	PollIntervalSec          int    `json:"poll_interval_sec"`
	StreamingThreshold       int    `json:"streaming_threshold"`
	IdleThreshold            int    `json:"idle_threshold"`
	TelegramBotToken         string `json:"telegram_bot_token"`
	TelegramChatID           string `json:"telegram_chat_id"`
	HealthPort               int    `json:"health_port"`
	CooldownMaxTransitions   int    `json:"cooldown_max_transitions"`
	CooldownWindowMinutes    int    `json:"cooldown_window_minutes"`
	CooldownStatePath        string `json:"cooldown_state_path"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config JSON: %w", err)
	}

	if env := os.Getenv("PLEX_TOKEN"); env != "" {
		cfg.PlexToken = env
	}
	if env := os.Getenv("QBITTORRENT_PASSWORD"); env != "" {
		cfg.QBittorrentPassword = env
	}
	if env := os.Getenv("TELEGRAM_BOT_TOKEN"); env != "" {
		cfg.TelegramBotToken = env
	}
	if env := os.Getenv("TELEGRAM_CHAT_ID"); env != "" {
		cfg.TelegramChatID = env
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	cfg.applyDefaults()

	return &cfg, nil
}

func (c *Config) validate() error {
	if c.PlexURL == "" {
		return fmt.Errorf("plex_url is required")
	}
	if c.PlexToken == "" {
		return fmt.Errorf("plex_token is required (set in config or PLEX_TOKEN env var)")
	}
	if c.QBittorrentURL == "" {
		return fmt.Errorf("qbittorrent_url is required")
	}
	return nil
}

func (c *Config) applyDefaults() {
	if c.PollIntervalSec <= 0 {
		c.PollIntervalSec = 60
	}
	if c.StreamingThreshold <= 0 {
		c.StreamingThreshold = 2
	}
	if c.IdleThreshold <= 0 {
		c.IdleThreshold = 3
	}
	if c.CooldownMaxTransitions <= 0 {
		c.CooldownMaxTransitions = 2
	}
	if c.CooldownWindowMinutes <= 0 {
		c.CooldownWindowMinutes = 60
	}
	if c.CooldownStatePath == "" {
		c.CooldownStatePath = "cooldown_state.json"
	}
}
