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
	cooldown := NewCooldownTracker(cfg.CooldownMaxTransitions, cfg.CooldownWindowMinutes, cfg.CooldownStatePath)
	manualThrottle := NewManualThrottle()
	eventCh := make(chan string, 1)
	telegramCmdCh := make(chan TelegramCommand, 1)
	manualExpiryCh := make(chan struct{}, 1)
	var expiryTimer *time.Timer

	if cfg.HealthPort > 0 {
		server := NewServer(cfg.HealthPort, appState, plex, qbt, eventCh, manualThrottle)
		server.Start()
	}

	if telegram != nil {
		defaultDuration := time.Duration(cfg.ManualThrottleDefaultMinutes) * time.Minute
		go telegram.StartPolling(telegramCmdCh, defaultDuration)
		log.Println("Telegram command polling started")
	}

	state := StateIdle
	currentLimitKbps := cfg.IdleUploadKbps

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	check := func() bool {
		if manualThrottle.IsActive() {
			if *verbose {
				log.Println("Manual throttle active, skipping Plex check")
			}
			return false
		}

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

		if state == StateStreaming && newState == StateIdle {
			if !cooldown.CanTransitionToIdle() {
				log.Printf("Cooldown active: blocking streaming -> idle transition (%d/%d transitions used in window)",
					cooldown.TransitionsInWindow(), cfg.CooldownMaxTransitions)
				return false
			}
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

		if state == StateStreaming && newState == StateIdle {
			cooldown.RecordTransition()
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

	handleTelegramCommand := func(cmd TelegramCommand) {
		switch cmd.Command {
		case "limit":
			if expiryTimer != nil {
				expiryTimer.Stop()
			}

			manualThrottle.Activate(cmd.Duration, cmd.Username)

			limitKbps := cfg.StreamingUploadKbps
			limitBytes := limitKbps * 1024
			limitStr := fmt.Sprintf("%d KB/s", limitKbps)
			if limitKbps == 0 {
				limitStr = "unlimited"
			}

			log.Printf("Manual throttle activated by %s for %s", cmd.Username, cmd.Duration)

			if !*dryRun {
				if err := qbt.SetUploadLimit(limitBytes); err != nil {
					log.Printf("Error setting upload limit: %v", err)
					telegram.SendReply(cmd.ChatID, fmt.Sprintf("Error setting limit: %v", err))
					return
				}
			}

			currentLimitKbps = limitKbps
			state = StateStreaming
			appState.Update(state, 0, currentLimitKbps)

			expiryTimer = time.AfterFunc(cmd.Duration, func() {
				select {
				case manualExpiryCh <- struct{}{}:
				default:
				}
			})

			msg := fmt.Sprintf("*Manual throttle activated*\nDuration: %s\nUpload limited to %s", formatDuration(cmd.Duration), limitStr)
			telegram.SendReply(cmd.ChatID, msg)

		case "unlimit":
			if !manualThrottle.IsActive() {
				telegram.SendReply(cmd.ChatID, "Manual throttle is not currently active.")
				return
			}

			if expiryTimer != nil {
				expiryTimer.Stop()
			}
			manualThrottle.Deactivate()

			log.Printf("Manual throttle cancelled by %s", cmd.Username)

			check()

			limitStr := fmt.Sprintf("%d KB/s", currentLimitKbps)
			if currentLimitKbps == 0 {
				limitStr = "unlimited"
			}
			msg := fmt.Sprintf("*Manual throttle cancelled*\nRestored to %s state (%s)", state, limitStr)
			telegram.SendReply(cmd.ChatID, msg)

		case "status":
			_, _, remoteStreams, uploadLimit, startTime := appState.Get()
			uptime := time.Since(startTime).Round(time.Second)

			limitStr := fmt.Sprintf("%d KB/s", uploadLimit)
			if uploadLimit == 0 {
				limitStr = "unlimited"
			}

			var statusMsg string
			if manualThrottle.IsActive() {
				remaining := manualThrottle.TimeRemaining()
				statusMsg = fmt.Sprintf("*Status*\nState: manual throttle\nUpload limit: %s\nTime remaining: %s\nRemote streams: %d\nUptime: %s",
					limitStr, formatDuration(remaining), remoteStreams, uptime)
			} else {
				statusMsg = fmt.Sprintf("*Status*\nState: %s\nUpload limit: %s\nRemote streams: %d\nUptime: %s",
					state, limitStr, remoteStreams, uptime)
			}
			telegram.SendReply(cmd.ChatID, statusMsg)
		}
	}

	handleManualExpiry := func() {
		if !manualThrottle.IsActive() {
			return
		}

		manualThrottle.Deactivate()
		log.Println("Manual throttle expired")

		check()

		limitStr := fmt.Sprintf("%d KB/s", currentLimitKbps)
		if currentLimitKbps == 0 {
			limitStr = "unlimited"
		}
		msg := fmt.Sprintf("*Manual throttle expired*\nRestored to %s state (%s)", state, limitStr)
		telegram.SendMessage(msg)
	}

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
		case cmd := <-telegramCmdCh:
			if *verbose {
				log.Printf("Telegram command: %s", cmd.Command)
			}
			handleTelegramCommand(cmd)
		case <-manualExpiryCh:
			handleManualExpiry()
		case sig := <-sigCh:
			log.Printf("Received %v, shutting down", sig)
			return
		}
	}
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second

	if h > 0 {
		return fmt.Sprintf("%dh%dm", h, m)
	}
	if m > 0 {
		return fmt.Sprintf("%dm%ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}
