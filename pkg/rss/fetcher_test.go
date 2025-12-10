package rss

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetcher_Fetch_Success(t *testing.T) {
	// 创建测试服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证 User-Agent
		assert.Contains(t, r.Header.Get("User-Agent"), "QukaAI-RSS-Fetcher")

		// 返回模拟的 RSS feed
		w.Header().Set("Content-Type", "application/rss+xml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Test Feed</title>
    <link>https://example.com</link>
    <description>Test Description</description>
    <item>
      <guid>https://example.com/post1</guid>
      <title>Test Post</title>
      <link>https://example.com/post1</link>
      <description>Test post description</description>
    </item>
  </channel>
</rss>`))
	}))
	defer server.Close()

	fetcher := NewFetcher()
	ctx := context.Background()

	feed, err := fetcher.Fetch(ctx, server.URL)
	require.NoError(t, err)
	require.NotNil(t, feed)

	assert.Equal(t, "Test Feed", feed.Title)
	assert.Equal(t, "https://example.com", feed.Link)
	assert.Equal(t, "Test Description", feed.Description)
	assert.Len(t, feed.Items, 1)
	assert.Equal(t, "Test Post", feed.Items[0].Title)
}

func TestFetcher_Fetch_InvalidURL(t *testing.T) {
	fetcher := NewFetcher()
	ctx := context.Background()

	feed, err := fetcher.Fetch(ctx, "://invalid-url")
	assert.Error(t, err)
	assert.Nil(t, feed)
	assert.Contains(t, err.Error(), "failed to create request")
}

func TestFetcher_Fetch_ServerError(t *testing.T) {
	// 创建返回错误状态码的测试服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	fetcher := NewFetcher()
	ctx := context.Background()

	feed, err := fetcher.Fetch(ctx, server.URL)
	assert.Error(t, err)
	assert.Nil(t, feed)
	assert.Contains(t, err.Error(), "unexpected status code: 500")
}

func TestFetcher_Fetch_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	fetcher := NewFetcher()
	ctx := context.Background()

	feed, err := fetcher.Fetch(ctx, server.URL)
	assert.Error(t, err)
	assert.Nil(t, feed)
	assert.Contains(t, err.Error(), "unexpected status code: 404")
}

func TestFetcher_Fetch_InvalidRSSContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("This is not valid RSS content"))
	}))
	defer server.Close()

	fetcher := NewFetcher()
	ctx := context.Background()

	feed, err := fetcher.Fetch(ctx, server.URL)
	assert.Error(t, err)
	assert.Nil(t, feed)
	assert.Contains(t, err.Error(), "failed to parse RSS feed")
}

func TestFetcher_Fetch_ContextCancellation(t *testing.T) {
	// 创建一个延迟响应的服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	fetcher := NewFetcher()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	feed, err := fetcher.Fetch(ctx, server.URL)
	assert.Error(t, err)
	assert.Nil(t, feed)
}

func TestFetcher_Fetch_RedirectHandling(t *testing.T) {
	// 最终目标服务器
	finalServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Redirected Feed</title>
    <link>https://example.com</link>
    <description>Feed after redirect</description>
  </channel>
</rss>`))
	}))
	defer finalServer.Close()

	// 重定向服务器
	redirectServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, finalServer.URL, http.StatusMovedPermanently)
	}))
	defer redirectServer.Close()

	fetcher := NewFetcher()
	ctx := context.Background()

	feed, err := fetcher.Fetch(ctx, redirectServer.URL)
	require.NoError(t, err)
	require.NotNil(t, feed)
	assert.Equal(t, "Redirected Feed", feed.Title)
}

func TestFetcher_Fetch_TooManyRedirects(t *testing.T) {
	// 创建循环重定向的服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, r.URL.String(), http.StatusMovedPermanently)
	}))
	defer server.Close()

	fetcher := NewFetcher()
	ctx := context.Background()

	feed, err := fetcher.Fetch(ctx, server.URL)
	assert.Error(t, err)
	assert.Nil(t, feed)
}

func TestFetcher_FetchWithRetry_Success(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount < 2 {
			// 第一次请求失败
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		// 第二次请求成功
		w.Header().Set("Content-Type", "application/rss+xml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Retry Success</title>
    <link>https://example.com</link>
    <description>Success after retry</description>
  </channel>
</rss>`))
	}))
	defer server.Close()

	fetcher := NewFetcher()
	ctx := context.Background()

	feed, err := fetcher.FetchWithRetry(ctx, server.URL, 3)
	require.NoError(t, err)
	require.NotNil(t, feed)
	assert.Equal(t, "Retry Success", feed.Title)
	assert.Equal(t, 2, callCount)
}

func TestFetcher_FetchWithRetry_AllFailed(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	fetcher := NewFetcher()
	ctx := context.Background()

	feed, err := fetcher.FetchWithRetry(ctx, server.URL, 3)
	assert.Error(t, err)
	assert.Nil(t, feed)
	assert.Contains(t, err.Error(), "failed after 3 retries")
	assert.Equal(t, 3, callCount)
}

func TestFetcher_FetchWithRetry_ContextCancellation(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	fetcher := NewFetcher()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	feed, err := fetcher.FetchWithRetry(ctx, server.URL, 10)
	assert.Error(t, err)
	assert.Nil(t, feed)
	// 由于 context 取消，不应该重试 10 次
	assert.Less(t, callCount, 10)
}

func TestFetcher_UserAgent(t *testing.T) {
	var receivedUserAgent string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedUserAgent = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/rss+xml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Test</title>
    <link>https://example.com</link>
    <description>Test</description>
  </channel>
</rss>`))
	}))
	defer server.Close()

	fetcher := NewFetcher()
	ctx := context.Background()

	_, err := fetcher.Fetch(ctx, server.URL)
	require.NoError(t, err)
	assert.Equal(t, "QukaAI-RSS-Fetcher/1.0", receivedUserAgent)
}

func TestFetcher_AcceptHeader(t *testing.T) {
	var receivedAccept string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAccept = r.Header.Get("Accept")
		w.Header().Set("Content-Type", "application/rss+xml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Test</title>
    <link>https://example.com</link>
    <description>Test</description>
  </channel>
</rss>`))
	}))
	defer server.Close()

	fetcher := NewFetcher()
	ctx := context.Background()

	_, err := fetcher.Fetch(ctx, server.URL)
	require.NoError(t, err)
	assert.Contains(t, receivedAccept, "application/rss+xml")
	assert.Contains(t, receivedAccept, "application/atom+xml")
}
