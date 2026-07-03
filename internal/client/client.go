package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/abevz/af-coordinator/internal/core"
)

type Client struct {
	socketPath string
	httpClient *http.Client
}

func New(socketPath string) *Client {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var dialer net.Dialer
			return dialer.DialContext(ctx, "unix", socketPath)
		},
	}

	return &Client{
		socketPath: socketPath,
		httpClient: &http.Client{
			Timeout:   5 * time.Second,
			Transport: transport,
		},
	}
}

func (c *Client) Health() (core.Health, error) {
	req, err := http.NewRequest(http.MethodGet, "http://af-coordinator/healthz", nil)
	if err != nil {
		return core.Health{}, fmt.Errorf("build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return core.Health{}, fmt.Errorf("perform request over %s: %w", c.socketPath, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return core.Health{}, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	var payload core.Health
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return core.Health{}, fmt.Errorf("decode response: %w", err)
	}

	return payload, nil
}
