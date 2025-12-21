package rss

import (
	"time"

	"github.com/mmcdole/gofeed"

	"github.com/quka-ai/quka-ai/pkg/types"
)

// Parser RSS/Atom 解析器
type Parser struct {
	parser *gofeed.Parser
}

// NewParser 创建新的 RSS 解析器实例
func NewParser() *Parser {
	return &Parser{
		parser: gofeed.NewParser(),
	}
}

// Parse 解析 RSS/Atom 内容
func (p *Parser) Parse(content string) (*types.RSSFeed, error) {
	feed, err := p.parser.ParseString(content)
	if err != nil {
		return nil, err
	}

	return p.convertFeed(feed), nil
}

// ParseURL 直接从 URL 解析 RSS/Atom
func (p *Parser) ParseURL(url string) (*types.RSSFeed, error) {
	feed, err := p.parser.ParseURL(url)
	if err != nil {
		return nil, err
	}

	return p.convertFeed(feed), nil
}

// convertFeed 将 gofeed.Feed 转换为 types.RSSFeed
func (p *Parser) convertFeed(feed *gofeed.Feed) *types.RSSFeed {
	rssFeed := &types.RSSFeed{
		Title:       feed.Title,
		Description: feed.Description,
		Link:        feed.Link,
		Items:       make([]*types.RSSFeedItem, 0, len(feed.Items)),
	}

	for _, item := range feed.Items {
		rssFeed.Items = append(rssFeed.Items, p.convertItem(item))
	}

	return rssFeed
}

// convertItem 将 gofeed.Item 转换为 types.RSSFeedItem
func (p *Parser) convertItem(item *gofeed.Item) *types.RSSFeedItem {
	rssItem := &types.RSSFeedItem{
		GUID:        item.GUID,
		Title:       item.Title,
		Link:        item.Link,
		Description: item.Description,
		Content:     item.Content,
	}

	// 如果没有 GUID，使用 Link 作为唯一标识
	if rssItem.GUID == "" {
		rssItem.GUID = item.Link
	}

	// 处理作者信息
	if item.Author != nil {
		rssItem.Author = item.Author.Name
	}

	// 处理发布时间
	if item.PublishedParsed != nil {
		rssItem.PublishedAt = item.PublishedParsed.Unix()
	} else if item.UpdatedParsed != nil {
		rssItem.PublishedAt = item.UpdatedParsed.Unix()
	} else {
		rssItem.PublishedAt = time.Now().Unix()
	}

	return rssItem
}
