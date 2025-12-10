package rss

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParser_ParseRSS(t *testing.T) {
	parser := NewParser()

	// 模拟一个简单的 RSS 2.0 feed
	rssFeed := `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Test RSS Feed</title>
    <link>https://example.com</link>
    <description>A test RSS feed</description>
    <item>
      <guid>https://example.com/post1</guid>
      <title>First Post</title>
      <link>https://example.com/post1</link>
      <description>This is the first post</description>
      <author>John Doe</author>
      <pubDate>Mon, 02 Jan 2023 15:04:05 GMT</pubDate>
    </item>
    <item>
      <guid>https://example.com/post2</guid>
      <title>Second Post</title>
      <link>https://example.com/post2</link>
      <description>This is the second post</description>
      <author>Jane Doe</author>
      <pubDate>Tue, 03 Jan 2023 10:30:00 GMT</pubDate>
    </item>
  </channel>
</rss>`

	feed, err := parser.Parse(rssFeed)
	require.NoError(t, err)
	require.NotNil(t, feed)

	// 验证 feed 信息
	assert.Equal(t, "Test RSS Feed", feed.Title)
	assert.Equal(t, "https://example.com", feed.Link)
	assert.Equal(t, "A test RSS feed", feed.Description)

	// 验证 items
	require.Len(t, feed.Items, 2)

	// 验证第一篇文章
	item1 := feed.Items[0]
	assert.Equal(t, "https://example.com/post1", item1.GUID)
	assert.Equal(t, "First Post", item1.Title)
	assert.Equal(t, "https://example.com/post1", item1.Link)
	assert.Equal(t, "This is the first post", item1.Description)
	assert.Equal(t, "John Doe", item1.Author)
	assert.Greater(t, item1.PublishedAt, int64(0))

	// 验证第二篇文章
	item2 := feed.Items[1]
	assert.Equal(t, "https://example.com/post2", item2.GUID)
	assert.Equal(t, "Second Post", item2.Title)
	assert.Equal(t, "https://example.com/post2", item2.Link)
	assert.Equal(t, "This is the second post", item2.Description)
	assert.Equal(t, "Jane Doe", item2.Author)
	assert.Greater(t, item2.PublishedAt, int64(0))
}

func TestParser_ParseAtom(t *testing.T) {
	parser := NewParser()

	// 模拟一个简单的 Atom feed
	atomFeed := `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Test Atom Feed</title>
  <link href="https://example.com"/>
  <subtitle>A test Atom feed</subtitle>
  <entry>
    <id>https://example.com/entry1</id>
    <title>First Entry</title>
    <link href="https://example.com/entry1"/>
    <summary>This is the first entry</summary>
    <author>
      <name>John Doe</name>
    </author>
    <published>2023-01-02T15:04:05Z</published>
  </entry>
</feed>`

	feed, err := parser.Parse(atomFeed)
	require.NoError(t, err)
	require.NotNil(t, feed)

	// 验证 feed 信息
	assert.Equal(t, "Test Atom Feed", feed.Title)
	assert.Equal(t, "https://example.com", feed.Link)
	assert.Equal(t, "A test Atom feed", feed.Description)

	// 验证 items
	require.Len(t, feed.Items, 1)

	// 验证文章
	item := feed.Items[0]
	assert.Equal(t, "https://example.com/entry1", item.GUID)
	assert.Equal(t, "First Entry", item.Title)
	assert.Equal(t, "https://example.com/entry1", item.Link)
	assert.Equal(t, "This is the first entry", item.Description)
	assert.Equal(t, "John Doe", item.Author)
	assert.Greater(t, item.PublishedAt, int64(0))
}

func TestParser_ParseInvalidFeed(t *testing.T) {
	parser := NewParser()

	invalidFeed := `This is not a valid RSS or Atom feed`

	feed, err := parser.Parse(invalidFeed)
	assert.Error(t, err)
	assert.Nil(t, feed)
}

func TestParser_ParseEmptyFeed(t *testing.T) {
	parser := NewParser()

	emptyFeed := `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Empty Feed</title>
    <link>https://example.com</link>
    <description>A feed with no items</description>
  </channel>
</rss>`

	feed, err := parser.Parse(emptyFeed)
	require.NoError(t, err)
	require.NotNil(t, feed)

	assert.Equal(t, "Empty Feed", feed.Title)
	assert.Len(t, feed.Items, 0)
}

func TestParser_ParseItemWithoutGUID(t *testing.T) {
	parser := NewParser()

	// RSS item 没有 GUID，应该使用 Link 作为 GUID
	rssFeed := `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Test Feed</title>
    <link>https://example.com</link>
    <description>Test</description>
    <item>
      <title>Post Without GUID</title>
      <link>https://example.com/post-no-guid</link>
      <description>This post has no GUID</description>
    </item>
  </channel>
</rss>`

	feed, err := parser.Parse(rssFeed)
	require.NoError(t, err)
	require.NotNil(t, feed)
	require.Len(t, feed.Items, 1)

	// GUID 应该使用 Link
	assert.Equal(t, "https://example.com/post-no-guid", feed.Items[0].GUID)
	assert.Equal(t, "https://example.com/post-no-guid", feed.Items[0].Link)
}

func TestParser_ParseItemWithContent(t *testing.T) {
	parser := NewParser()

	// RSS item 包含 content:encoded 字段
	rssFeed := `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:content="http://purl.org/rss/1.0/modules/content/">
  <channel>
    <title>Test Feed</title>
    <link>https://example.com</link>
    <description>Test</description>
    <item>
      <guid>https://example.com/post1</guid>
      <title>Post With Content</title>
      <link>https://example.com/post1</link>
      <description>Short description</description>
      <content:encoded><![CDATA[<p>Full HTML content goes here</p>]]></content:encoded>
    </item>
  </channel>
</rss>`

	feed, err := parser.Parse(rssFeed)
	require.NoError(t, err)
	require.NotNil(t, feed)
	require.Len(t, feed.Items, 1)

	item := feed.Items[0]
	assert.Equal(t, "Short description", item.Description)
	assert.Contains(t, item.Content, "Full HTML content goes here")
}
