package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type State int

const (
	StateIdle State = iota
	StateStreaming
)

func (s State) String() string {
	if s == StateStreaming {
		return "streaming"
	}
	return "idle"
}

func main() {
	configPath := flag.String("config", "config.json", "path to config file")
	dryRun := flag.Bool("dry-run", false, "log actions without changing limits")
	once := flag.Bool("once", false, "run once and exit")
	verbose := flag.Bool("verbose", false, "enable verbose logging")
	flag.Parse()

	cfg, err := LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	plex := NewPlexClient(cfg.PlexURL, cfg.PlexToken)

	qbt, err := NewQBittorrentClient(cfg.QBittorrentURL, cfg.QBittorrentUsername, cfg.QBittorrentPassword)
	if err != nil {
		log.Fatalf("Failed to create qBittorrent client: %v", err)
	}

	if cfg.QBittorrentUsername != "" {
		if err := qbt.Login(); err != nil {
			log.Fatalf("Failed to login to qBittorrent: %v", err)
		}
		log.Println("Logged in to qBittorrent")
	}

	telegram := NewTelegramClient(cfg.TelegramBotToken, cfg.TelegramChatID)
	if telegram != nil {
		log.Println("Telegram notifications enabled")
	}

	appState := NewAppState()
	eventCh := make(chan string, 1)

	if cfg.HealthPort > 0 {
		healthServer := NewHealthServer(cfg.HealthPort, appState, plex, qbt, eventCh)
		healthServer.Start()
	}

	state := StateIdle
	currentLimitKbps := cfg.IdleUploadKbps

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	check := func() bool {
		remoteStreams, err := plex.GetRemoteStreamCount()
		if err != nil {
			log.Printf("Error checking Plex: %v", err)
			return false
		}

		appState.Update(state, remoteStreams, currentLimitKbps)

		if *verbose {
			log.Printf("Remote streams: %d, state: %s", remoteStreams, state)
		}

		var newState State
		if remoteStreams > 0 {
			newState = StateStreaming
		} else {
			newState = StateIdle
		}

		if newState == state {
			return false
		}

		var limitKbps int
		if newState == StateStreaming {
			limitKbps = cfg.StreamingUploadKbps
		} else {
			limitKbps = cfg.IdleUploadKbps
		}

		limitBytes := limitKbps * 1024

		limitStr := fmt.Sprintf("%d KB/s", limitKbps)
		if limitKbps == 0 {
			limitStr = "unlimited"
		}

		log.Printf("State change: %s -> %s (setting upload limit to %s)", state, newState, limitStr)

		if !*dryRun {
			if err := qbt.SetUploadLimit(limitBytes); err != nil {
				log.Printf("Error setting upload limit: %v", err)
				return false
			}

			var msg string
			if newState == StateStreaming {
				msg = fmt.Sprintf("*Streaming detected*\nThrottling upload to %s", limitStr)
			} else {
				msg = fmt.Sprintf("*Streaming ended*\nRestoring upload to %s", limitStr)
			}
			if err := telegram.SendMessage(msg); err != nil {
				log.Printf("Error sending Telegram notification: %v", err)
			}
		} else {
			log.Printf("[DRY RUN] Would set upload limit to %s", limitStr)
		}

		state = newState
		currentLimitKbps = limitKbps
		return true
	}

	check()

	if *once {
		return
	}

	log.Printf("Starting plex-helper with webhooks (fallback poll: %ds)", cfg.PollIntervalSec)

	fallbackTicker := time.NewTicker(time.Duration(cfg.PollIntervalSec) * time.Second)
	defer fallbackTicker.Stop()

	for {
		select {
		case event := <-eventCh:
			if *verbose {
				log.Printf("Webhook event: %s", event)
			}
			for i := 0; i < 5; i++ {
				time.Sleep(500 * time.Millisecond)
				if check() {
					break
				}
			}
		case <-fallbackTicker.C:
			if *verbose {
				log.Println("Fallback poll triggered")
			}
			check()
		case sig := <-sigCh:
			log.Printf("Received %v, shutting down", sig)
			return
		}
	}
}
