package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type PlexClient struct {
	baseURL string
	token   string
	client  *http.Client
}

type plexSessionsResponse struct {
	MediaContainer struct {
		Size     int `json:"size"`
		Metadata []struct {
			Player struct {
				Local bool   `json:"local"`
				State string `json:"state"`
			} `json:"Player"`
			Session struct {
				Location string `json:"location"`
			} `json:"Session"`
		} `json:"Metadata"`
	} `json:"MediaContainer"`
}

func NewPlexClient(baseURL, token string) *PlexClient {
	return &PlexClient{
		baseURL: baseURL,
		token:   token,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (p *PlexClient) GetRemoteStreamCount() (int, error) {
	req, err := http.NewRequest("GET", p.baseURL+"/status/sessions", nil)
	if err != nil {
		return 0, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("X-Plex-Token", p.token)
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return 0, fmt.Errorf("invalid plex token (401)")
	}
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var sessions plexSessionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&sessions); err != nil {
		return 0, fmt.Errorf("decoding response: %w", err)
	}

	count := 0
	for _, meta := range sessions.MediaContainer.Metadata {
		isRemote := meta.Session.Location == "wan" || !meta.Player.Local
		isActive := meta.Player.State == "playing" || meta.Player.State == "buffering"
		if isRemote && isActive {
			count++
		}
	}

	return count, nil
}
