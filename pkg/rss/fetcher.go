package rss

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/quka-ai/quka-ai/pkg/types"
)

// Fetcher RSS 抓取器
type Fetcher struct {
	client *http.Client
	parser *Parser
}

// NewFetcher 创建新的 RSS 抓取器实例
func NewFetcher() *Fetcher {
	return &Fetcher{
		client: &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return fmt.Errorf("stopped after 10 redirects")
				}
				return nil
			},
		},
		parser: NewParser(),
	}
}

// Fetch 从 URL 抓取 RSS/Atom 内容
func (f *Fetcher) Fetch(ctx context.Context, url string) (*types.RSSFeed, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// 设置 User-Agent 以避免某些网站拒绝请求
	req.Header.Set("User-Agent", "QukaAI-RSS-Fetcher/1.0")
	req.Header.Set("Accept", "application/rss+xml, application/atom+xml, application/xml, text/xml")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	feed, err := f.parser.Parse(string(body))
	if err != nil {
		return nil, fmt.Errorf("failed to parse RSS feed: %w", err)
	}

	return feed, nil
}

// FetchWithRetry 带重试的抓取
func (f *Fetcher) FetchWithRetry(ctx context.Context, url string, maxRetries int) (*types.RSSFeed, error) {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		feed, err := f.Fetch(ctx, url)
		if err == nil {
			return feed, nil
		}

		lastErr = err
		if i < maxRetries-1 {
			// 等待一段时间后重试，使用指数退避策略
			waitTime := time.Duration(i+1) * 5 * time.Second
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(waitTime):
				continue
			}
		}
	}

	return nil, fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}
