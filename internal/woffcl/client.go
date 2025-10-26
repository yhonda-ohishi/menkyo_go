package woffcl

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// BackendURLResponse woff-clからのレスポンス
type BackendURLResponse struct {
	URL string `json:"url"`
}

// Client woff-clクライアント
type Client struct {
	endpoint string
	secret   string
	httpClient *http.Client
}

// NewClient 新しいClientを作成
func NewClient(endpoint, secret string) *Client {
	return &Client{
		endpoint: endpoint,
		secret:   secret,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GetBackendURL バックエンドURLを取得
func (c *Client) GetBackendURL() (string, error) {
	req, err := http.NewRequest("GET", c.endpoint+"/get-backend-url", nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// シークレット認証
	req.Header.Set("Authorization", "Bearer "+c.secret)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var result BackendURLResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if result.URL == "" {
		return "", fmt.Errorf("empty backend URL in response")
	}

	return result.URL, nil
}
