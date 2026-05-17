package search

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"cachesvc/internal/cache"
)

// DefaultCacheTTL — срок жизни записи поиска (см. WRITEUP.md).
const DefaultCacheTTL = 10 * time.Minute

type Result struct {
	Title string `json:"title"`
	Score int    `json:"score"`
}

type Client struct {
	baseURL string
	http    *http.Client
	cache   *cache.Cache
	ttl     time.Duration
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		http:    &http.Client{Timeout: 5 * time.Second},
		cache:   cache.New(),
		ttl:     DefaultCacheTTL,
	}
}

// Search возвращает результаты поиска для запроса q (read-through cache + singleflight).
func (c *Client) Search(ctx context.Context, q string) ([]Result, error) {
	raw, err := c.cache.GetOrLoad(ctx, q, c.ttl, func(ctx context.Context) ([]byte, error) {
		results, err := c.do(ctx, q)
		if err != nil {
			return nil, err
		}
		return json.Marshal(results)
	})
	if err != nil {
		return nil, err
	}
	var out []Result
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) do(ctx context.Context, q string) ([]Result, error) {
	u := c.baseURL + "/search?q=" + url.QueryEscape(q)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		return nil, fmt.Errorf("upstream status %d", resp.StatusCode)
	}
	var out []Result
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}
