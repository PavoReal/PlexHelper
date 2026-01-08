package main

import (
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

type QBittorrentClient struct {
	baseURL  string
	username string
	password string
	client   *http.Client
}

func NewQBittorrentClient(baseURL, username, password string) (*QBittorrentClient, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("creating cookie jar: %w", err)
	}

	return &QBittorrentClient{
		baseURL:  strings.TrimSuffix(baseURL, "/"),
		username: username,
		password: password,
		client: &http.Client{
			Timeout: 10 * time.Second,
			Jar:     jar,
		},
	}, nil
}

func (q *QBittorrentClient) Login() error {
	data := url.Values{}
	data.Set("username", q.username)
	data.Set("password", q.password)

	req, err := http.NewRequest("POST", q.baseURL+"/api/v2/auth/login", strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", q.baseURL)

	resp, err := q.client.Do(req)
	if err != nil {
		return fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("login failed (403) - IP may be banned from too many attempts")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	return nil
}

func (q *QBittorrentClient) SetUploadLimit(bytesPerSec int) error {
	err := q.setUploadLimitInternal(bytesPerSec)
	if err != nil && strings.Contains(err.Error(), "403") {
		if loginErr := q.Login(); loginErr != nil {
			return fmt.Errorf("re-login failed: %w", loginErr)
		}
		return q.setUploadLimitInternal(bytesPerSec)
	}
	return err
}

func (q *QBittorrentClient) setUploadLimitInternal(bytesPerSec int) error {
	data := url.Values{}
	data.Set("limit", fmt.Sprintf("%d", bytesPerSec))

	req, err := http.NewRequest("POST", q.baseURL+"/api/v2/transfer/setUploadLimit", strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", q.baseURL)

	resp, err := q.client.Do(req)
	if err != nil {
		return fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("forbidden (403) - session may have expired")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	return nil
}
