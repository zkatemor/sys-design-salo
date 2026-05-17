package upstream

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"cbservice/internal/breaker"
)

type Client struct {
	baseURL string
	http    *http.Client
	brk     *breaker.Breaker
}

func NewClient(baseURL string) *Client {
	brk, err := breaker.New(breaker.Config{
		FailureThreshold:  5,
		OpenTimeout:       2 * time.Second,
		HalfOpenMaxProbes: 2,
		SuccessThreshold:  2,
	})
	if err != nil {
		panic(err)
	}
	return &Client{
		baseURL: baseURL,
		http:    &http.Client{Timeout: 2 * time.Second},
		brk:     brk,
	}
}

// Fetch делает GET /data на upstream.
func (c *Client) Fetch(ctx context.Context) (string, error) {
	var body string
	err := c.brk.Execute(func() error {
		var err error
		body, err = c.do(ctx)
		return err
	})
	if err != nil {
		return "", err
	}
	return body, nil
}

func (c *Client) do(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/data", nil)
	if err != nil {
		return "", err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		return "", fmt.Errorf("upstream status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	return string(body), nil
}
