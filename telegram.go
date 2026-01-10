package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type TelegramClient struct {
	botToken string
	chatID   string
	client   *http.Client
}

func NewTelegramClient(botToken, chatID string) *TelegramClient {
	if botToken == "" || chatID == "" {
		return nil
	}

	return &TelegramClient{
		botToken: botToken,
		chatID:   chatID,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (t *TelegramClient) SendMessage(text string) error {
	if t == nil {
		return nil
	}

	payload := map[string]interface{}{
		"chat_id":    t.chatID,
		"text":       text,
		"parse_mode": "Markdown",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling payload: %w", err)
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.botToken)
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	return nil
}

type TelegramUpdate struct {
	UpdateID int              `json:"update_id"`
	Message  *TelegramMessage `json:"message"`
}

type TelegramMessage struct {
	Chat struct {
		ID int64 `json:"id"`
	} `json:"chat"`
	From struct {
		Username string `json:"username"`
	} `json:"from"`
	Text string `json:"text"`
}

type TelegramCommand struct {
	Command  string
	Duration time.Duration
	Username string
	ChatID   int64
}

func (t *TelegramClient) GetUpdates(offset, timeout int) ([]TelegramUpdate, error) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates?timeout=%d&offset=%d",
		t.botToken, timeout, offset)

	client := &http.Client{Timeout: time.Duration(timeout+10) * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("getting updates: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		OK     bool             `json:"ok"`
		Result []TelegramUpdate `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	if !result.OK {
		return nil, fmt.Errorf("telegram API returned not OK")
	}

	return result.Result, nil
}

func (t *TelegramClient) SendReply(chatID int64, text string) error {
	payload := map[string]interface{}{
		"chat_id":    chatID,
		"text":       text,
		"parse_mode": "Markdown",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling payload: %w", err)
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.botToken)
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	return nil
}

func (t *TelegramClient) StartPolling(cmdCh chan<- TelegramCommand, defaultDuration time.Duration) {
	if t == nil {
		return
	}

	offset := 0
	for {
		updates, err := t.GetUpdates(offset, 30)
		if err != nil {
			log.Printf("Error getting Telegram updates: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}

		for _, update := range updates {
			offset = update.UpdateID + 1

			if update.Message == nil {
				continue
			}

			chatIDStr := fmt.Sprintf("%d", update.Message.Chat.ID)
			if chatIDStr != t.chatID {
				continue
			}

			cmd := parseCommand(update.Message.Text, defaultDuration)
			if cmd == nil {
				continue
			}

			cmd.ChatID = update.Message.Chat.ID
			cmd.Username = update.Message.From.Username

			select {
			case cmdCh <- *cmd:
			default:
			}
		}
	}
}

func parseCommand(text string, defaultDuration time.Duration) *TelegramCommand {
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, "/") {
		return nil
	}

	parts := strings.Fields(text)
	command := strings.TrimPrefix(parts[0], "/")
	command = strings.Split(command, "@")[0]

	switch command {
	case "limit":
		duration := defaultDuration
		if len(parts) > 1 {
			if parsed := parseDuration(parts[1]); parsed > 0 {
				duration = parsed
			}
		}
		return &TelegramCommand{Command: "limit", Duration: duration}
	case "unlimit":
		return &TelegramCommand{Command: "unlimit"}
	case "status":
		return &TelegramCommand{Command: "status"}
	}

	return nil
}

func parseDuration(s string) time.Duration {
	if d, err := time.ParseDuration(s); err == nil {
		return d
	}

	re := regexp.MustCompile(`^(\d+)$`)
	if matches := re.FindStringSubmatch(s); len(matches) == 2 {
		if mins, err := strconv.Atoi(matches[1]); err == nil {
			return time.Duration(mins) * time.Minute
		}
	}

	return 0
}
