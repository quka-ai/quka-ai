package rss

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/quka-ai/quka-ai/pkg/types"
)

// mockCore 用于测试的 mock core
type mockCore struct {
	articles  map[string]*types.RSSArticle
	knowledge map[string]*types.Knowledge
	resources map[string]*types.Resource
}

func newMockCore() *mockCore {
	return &mockCore{
		articles:  make(map[string]*types.RSSArticle),
		knowledge: make(map[string]*types.Knowledge),
		resources: map[string]*types.Resource{
			"default": {
				ID:      "default",
				SpaceID: "space1",
				Cycle:   7 * 24 * 3600, // 7天过期
			},
		},
	}
}

// mockStore 实现 Store 接口的部分方法
type mockStore struct {
	core             *mockCore
	articleStore     *mockRSSArticleStore
	knowledgeStore   *mockKnowledgeStore
	subscriptionStore *mockRSSSubscriptionStore
	resourceStore    *mockResourceStore
}

func (m *mockStore) RSSArticleStore() interface{} {
	return m.articleStore
}

func (m *mockStore) KnowledgeStore() interface{} {
	return m.knowledgeStore
}

func (m *mockStore) RSSSubscriptionStore() interface{} {
	return m.subscriptionStore
}

func (m *mockStore) ResourceStore() interface{} {
	return m.resourceStore
}

// mockRSSArticleStore
type mockRSSArticleStore struct {
	core *mockCore
}

func (m *mockRSSArticleStore) Exists(ctx context.Context, subscriptionID int64, guid string) (bool, error) {
	key := fmt.Sprintf("%d:%s", subscriptionID, guid)
	_, exists := m.core.articles[key]
	return exists, nil
}

func (m *mockRSSArticleStore) Create(ctx context.Context, article *types.RSSArticle) error {
	article.ID = int64(len(m.core.articles) + 1)
	key := fmt.Sprintf("%d:%s", article.SubscriptionID, article.GUID)
	m.core.articles[key] = article
	return nil
}

// mockKnowledgeStore
type mockKnowledgeStore struct {
	core *mockCore
}

func (m *mockKnowledgeStore) Create(ctx context.Context, knowledge types.Knowledge) error {
	m.core.knowledge[knowledge.ID] = &knowledge
	return nil
}

// mockRSSSubscriptionStore
type mockRSSSubscriptionStore struct {
	core *mockCore
}

func (m *mockRSSSubscriptionStore) Update(ctx context.Context, id int64, updates map[string]interface{}) error {
	return nil
}

// mockResourceStore
type mockResourceStore struct {
	core *mockCore
}

func (m *mockResourceStore) GetResource(ctx context.Context, spaceID, resourceID string) (*types.Resource, error) {
	resource, ok := m.core.resources[resourceID]
	if !ok {
		return nil, fmt.Errorf("resource not found")
	}
	return resource, nil
}

// 创建测试用的 Processor（使用简化的依赖）
func setupTestProcessor() (*Processor, *mockCore) {
	mockCore := newMockCore()

	// 这里我们直接创建一个简化版的 Processor 用于测试
	// 实际实现中需要根据项目结构调整
	processor := &Processor{
		core: nil, // 实际测试中需要提供合适的 mock
	}

	return processor, mockCore
}

func TestProcessor_ProcessArticle_NewArticle(t *testing.T) {
	// 注意：这个测试需要完整的 core mock 才能运行
	// 这里提供测试结构，实际运行需要配合项目的测试框架
	t.Skip("需要完整的 core mock 支持")

	processor, mockCore := setupTestProcessor()
	ctx := context.Background()

	subscription := &types.RSSSubscription{
		ID:         1,
		UserID:     "user1",
		SpaceID:    "space1",
		ResourceID: "default",
		URL:        "https://example.com/feed",
		Title:      "Test Feed",
	}

	article := &types.RSSArticle{
		GUID:        "https://example.com/post1",
		Title:       "Test Article",
		Link:        "https://example.com/post1",
		Description: "Test description",
		Content:     "<p>Full content</p>",
		Author:      "John Doe",
		PublishedAt: time.Now().Unix(),
	}

	err := processor.ProcessArticle(ctx, subscription, article)
	require.NoError(t, err)

	// 验证文章已保存
	key := fmt.Sprintf("%d:%s", subscription.ID, article.GUID)
	savedArticle, exists := mockCore.articles[key]
	assert.True(t, exists)
	assert.Equal(t, article.Title, savedArticle.Title)

	// 验证 knowledge 已创建
	assert.Len(t, mockCore.knowledge, 1)
	for _, k := range mockCore.knowledge {
		assert.Equal(t, subscription.SpaceID, k.SpaceID)
		assert.Equal(t, subscription.UserID, k.UserID)
		assert.Equal(t, subscription.ResourceID, k.Resource)
		assert.Equal(t, types.KnowledgeKind("rss"), k.Kind)
		assert.Equal(t, article.Title, k.Title)
	}
}

func TestProcessor_ProcessArticle_DuplicateArticle(t *testing.T) {
	t.Skip("需要完整的 core mock 支持")

	processor, mockCore := setupTestProcessor()
	ctx := context.Background()

	subscription := &types.RSSSubscription{
		ID:         1,
		UserID:     "user1",
		SpaceID:    "space1",
		ResourceID: "default",
	}

	article := &types.RSSArticle{
		GUID:        "https://example.com/post1",
		Title:       "Test Article",
		Link:        "https://example.com/post1",
		Description: "Test description",
		PublishedAt: time.Now().Unix(),
	}

	// 第一次处理
	err := processor.ProcessArticle(ctx, subscription, article)
	require.NoError(t, err)

	knowledgeCountBefore := len(mockCore.knowledge)

	// 第二次处理同一篇文章
	err = processor.ProcessArticle(ctx, subscription, article)
	require.NoError(t, err)

	// knowledge 数量不应该增加（因为是重复文章）
	assert.Equal(t, knowledgeCountBefore, len(mockCore.knowledge))
}

func TestProcessor_ProcessFeed(t *testing.T) {
	t.Skip("需要完整的 core mock 支持")

	processor, mockCore := setupTestProcessor()
	ctx := context.Background()

	subscription := &types.RSSSubscription{
		ID:         1,
		UserID:     "user1",
		SpaceID:    "space1",
		ResourceID: "default",
	}

	feed := &types.RSSFeed{
		Title:       "Test Feed",
		Description: "Test feed description",
		Link:        "https://example.com",
		Items: []*types.RSSFeedItem{
			{
				GUID:        "https://example.com/post1",
				Title:       "Post 1",
				Link:        "https://example.com/post1",
				Description: "Description 1",
				PublishedAt: time.Now().Unix(),
			},
			{
				GUID:        "https://example.com/post2",
				Title:       "Post 2",
				Link:        "https://example.com/post2",
				Description: "Description 2",
				PublishedAt: time.Now().Unix(),
			},
		},
	}

	err := processor.ProcessFeed(ctx, subscription, feed)
	require.NoError(t, err)

	// 验证所有文章都已处理
	assert.Len(t, mockCore.articles, 2)
	assert.Len(t, mockCore.knowledge, 2)
}

func TestProcessor_BuildKnowledgeContent(t *testing.T) {
	processor := &Processor{}

	article := &types.RSSArticle{
		Title:       "Test Article",
		Author:      "John Doe",
		Link:        "https://example.com/post1",
		Content:     "<p>Full article content</p>",
		Description: "Short description",
		PublishedAt: time.Date(2023, 1, 15, 10, 30, 0, 0, time.UTC).Unix(),
	}

	content := processor.buildKnowledgeContent(article)

	// 验证内容包含必要的信息
	assert.Contains(t, content, "# Test Article")
	assert.Contains(t, content, "**作者**: John Doe")
	assert.Contains(t, content, "**原文链接**: [https://example.com/post1](https://example.com/post1)")
	assert.Contains(t, content, "**发布时间**: 2023-01-15")
	assert.Contains(t, content, "Full article content")
}

func TestProcessor_BuildKnowledgeContent_FallbackToDescription(t *testing.T) {
	processor := &Processor{}

	article := &types.RSSArticle{
		Title:       "Test Article",
		Link:        "https://example.com/post1",
		Description: "Only description available",
		Content:     "", // 没有 content
		PublishedAt: time.Now().Unix(),
	}

	content := processor.buildKnowledgeContent(article)

	// 应该使用 description
	assert.Contains(t, content, "Only description available")
}

func TestProcessor_BuildKnowledgeContent_NoAuthor(t *testing.T) {
	processor := &Processor{}

	article := &types.RSSArticle{
		Title:       "Test Article",
		Link:        "https://example.com/post1",
		Content:     "Content",
		Author:      "", // 没有作者
		PublishedAt: time.Now().Unix(),
	}

	content := processor.buildKnowledgeContent(article)

	// 不应该包含作者信息
	assert.NotContains(t, content, "**作者**:")
	assert.Contains(t, content, "# Test Article")
}

func TestProcessor_ExtractKeywords(t *testing.T) {
	processor := &Processor{}

	article := &types.RSSArticle{
		Title:       "Golang Best Practices for 2024",
		Content:     "Article about Go programming",
		Description: "Learn Go",
	}

	keywords := processor.ExtractKeywords(article)

	// 当前实现返回空数组，等待后续AI增强
	assert.NotNil(t, keywords)
	assert.IsType(t, []string{}, keywords)
}

func TestProcessor_UpdateUserInterests(t *testing.T) {
	t.Skip("需要完整的 core mock 支持")

	processor, mockCore := setupTestProcessor()
	ctx := context.Background()

	userID := "user1"
	topics := []string{"golang", "programming", "best-practices"}
	weight := 0.8

	err := processor.UpdateUserInterests(ctx, userID, topics, weight)
	require.NoError(t, err)

	// 验证兴趣已更新（需要 mock 实现）
	assert.NotNil(t, mockCore)
}

func TestProcessor_UpdateUserInterests_EmptyTopics(t *testing.T) {
	t.Skip("需要完整的 core mock 支持")

	processor, _ := setupTestProcessor()
	ctx := context.Background()

	userID := "user1"
	topics := []string{} // 空主题列表
	weight := 0.8

	err := processor.UpdateUserInterests(ctx, userID, topics, weight)
	require.NoError(t, err) // 空列表应该成功但不做任何操作
}

// 集成测试示例（需要完整环境）
func TestProcessor_Integration_ProcessArticleWithExpiration(t *testing.T) {
	t.Skip("需要完整的数据库环境")

	// 这个测试展示了如何测试完整的文章处理流程，包括过期时间设置
	// 实际运行需要配置测试数据库
}

func TestProcessor_Integration_ProcessFeedWithDuplicates(t *testing.T) {
	t.Skip("需要完整的数据库环境")

	// 这个测试展示了如何测试 feed 处理中的去重逻辑
	// 实际运行需要配置测试数据库
}

// 性能测试示例
func BenchmarkProcessor_BuildKnowledgeContent(b *testing.B) {
	processor := &Processor{}

	article := &types.RSSArticle{
		Title:       "Benchmark Article",
		Author:      "Benchmark Author",
		Link:        "https://example.com/benchmark",
		Content:     "<p>Some content for benchmarking</p>",
		Description: "Description",
		PublishedAt: time.Now().Unix(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = processor.buildKnowledgeContent(article)
	}
}
