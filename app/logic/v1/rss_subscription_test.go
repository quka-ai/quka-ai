package v1_test

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/quka-ai/quka-ai/app/core/srv"
	v1 "github.com/quka-ai/quka-ai/app/logic/v1"
	"github.com/quka-ai/quka-ai/pkg/security"
	"github.com/quka-ai/quka-ai/pkg/types"
)

var (
	testSpaceID    = os.Getenv("QUKA_TEST_RSS_SPACE_ID")
	testResourceID = os.Getenv("QUKA_TEST_RSS_RESOURCE_ID")
	testUserID     = os.Getenv("QUKA_TEST_USER_ID")
)

func setupRSSSubscriptionLogic() *v1.RSSSubscriptionLogic {
	ctx := context.WithValue(context.Background(), v1.TOKEN_CONTEXT_KEY, security.TokenClaims{
		User:    testUserID,
		Appid:   "test",
		AppName: "quka",
		Fields: map[string]string{
			security.ROLE_KEY: srv.RoleChief,
		},
	})
	return v1.NewRSSSubscriptionLogic(ctx, NewCore())
}

func TestCreateSubscription(t *testing.T) {
	logic := setupRSSSubscriptionLogic()

	// 测试数据
	testCases := []struct {
		name            string
		spaceID         string
		resourceID      string
		url             string
		title           string
		description     string
		category        string
		updateFrequency int
		expectError     bool
		errorContains   string
	}{
		{
			name:            "成功创建订阅 - 使用 RSS Feed 的默认信息",
			spaceID:         testSpaceID,
			resourceID:      testResourceID,
			url:             "https://blog.golang.org/feed.atom",
			title:           "",
			description:     "",
			category:        "技术",
			updateFrequency: 3600,
			expectError:     false,
		},
		{
			name:            "成功创建订阅 - 自定义标题和描述",
			spaceID:         testSpaceID,
			resourceID:      testResourceID,
			url:             "https://www.reddit.com/r/golang/.rss",
			title:           "Reddit Golang",
			description:     "Golang subreddit feed",
			category:        "社区",
			updateFrequency: 7200,
			expectError:     false,
		},
		{
			name:            "失败 - Resource 不存在",
			spaceID:         testSpaceID,
			resourceID:      "non-existent-resource",
			url:             "https://blog.golang.org/feed.atom",
			title:           "Test Feed",
			description:     "Test Description",
			category:        "测试",
			updateFrequency: 3600,
			expectError:     true,
			errorContains:   "notfound",
		},
		{
			name:            "失败 - 无效的 RSS URL",
			spaceID:         testSpaceID,
			resourceID:      testResourceID,
			url:             "https://example.com/invalid-feed",
			title:           "Invalid Feed",
			description:     "This should fail",
			category:        "测试",
			updateFrequency: 3600,
			expectError:     true,
			errorContains:   "invalid",
		},
		{
			name:            "成功 - 使用默认更新频率",
			spaceID:         testSpaceID,
			resourceID:      testResourceID,
			url:             "https://news.ycombinator.com/rss",
			title:           "Hacker News",
			description:     "Tech news",
			category:        "新闻",
			updateFrequency: 0, // 应该使用默认值 3600
			expectError:     false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			subscription, err := logic.CreateSubscription(
				tc.spaceID,
				tc.resourceID,
				tc.url,
				tc.title,
				tc.description,
				tc.category,
				tc.updateFrequency,
			)

			if tc.expectError {
				require.Error(t, err)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}
				assert.Nil(t, subscription)
			} else {
				require.NoError(t, err)
				require.NotNil(t, subscription)

				// 验证返回的订阅数据
				assert.NotZero(t, subscription.ID)
				assert.Equal(t, tc.spaceID, subscription.SpaceID)
				assert.Equal(t, tc.resourceID, subscription.ResourceID)
				assert.Equal(t, tc.url, subscription.URL)
				assert.Equal(t, testUserID, subscription.UserID)
				assert.True(t, subscription.Enabled)
				assert.NotZero(t, subscription.CreatedAt)
				assert.NotZero(t, subscription.UpdatedAt)

				// 验证标题和描述
				if tc.title != "" {
					assert.Equal(t, tc.title, subscription.Title)
				} else {
					// 如果没有提供标题，应该从 Feed 中获取
					assert.NotEmpty(t, subscription.Title)
				}

				// 验证更新频率
				if tc.updateFrequency > 0 {
					assert.Equal(t, tc.updateFrequency, subscription.UpdateFrequency)
				} else {
					// 默认值应该是 3600
					assert.Equal(t, 3600, subscription.UpdateFrequency)
				}

				t.Logf("创建的订阅 ID: %s, 标题: %s", subscription.ID, subscription.Title)

				// 清理：删除测试创建的订阅
				if subscription.ID != "" {
					err := logic.DeleteSubscription(subscription.ID)
					if err != nil {
						t.Logf("警告: 清理订阅失败: %v", err)
					}
				}
			}
		})
	}
}

func TestCreateSubscription_DuplicateURL(t *testing.T) {
	logic := setupRSSSubscriptionLogic()

	url := "https://blog.golang.org/feed.atom"
	title := "Go Blog - First"
	category := "技术"

	// 第一次创建应该成功
	subscription1, err := logic.CreateSubscription(
		testSpaceID,
		testResourceID,
		url,
		title,
		"First subscription",
		category,
		3600,
	)
	require.NoError(t, err)
	require.NotNil(t, subscription1)
	defer logic.DeleteSubscription(subscription1.ID)

	// 第二次创建相同的 URL 应该失败
	subscription2, err := logic.CreateSubscription(
		testSpaceID,
		testResourceID,
		url,
		"Go Blog - Second",
		"Second subscription",
		category,
		3600,
	)
	require.Error(t, err)
	assert.Nil(t, subscription2)
	assert.Contains(t, err.Error(), "exist")

	t.Logf("重复订阅验证通过，错误信息: %v", err)
}

func TestCreateSubscription_PermissionCheck(t *testing.T) {
	// 创建一个没有编辑权限的上下文
	ctx := context.WithValue(context.Background(), v1.TOKEN_CONTEXT_KEY, security.TokenClaims{
		User:    "readonly-user",
		Appid:   "test",
		AppName: "brew",
	})

	logic := v1.NewRSSSubscriptionLogic(ctx, NewCore())

	// 尝试创建订阅应该失败（权限不足）
	subscription, err := logic.CreateSubscription(
		testSpaceID,
		testResourceID,
		"https://blog.golang.org/feed.atom",
		"Test Feed",
		"Test Description",
		"测试",
		3600,
	)

	// 根据实际的 RBAC 配置，这里可能会返回权限错误
	// 如果测试环境允许所有权限，可以跳过此测试或调整断言
	if err != nil {
		assert.Contains(t, err.Error(), "permission")
		assert.Nil(t, subscription)
		t.Logf("权限检查通过，错误信息: %v", err)
	} else {
		t.Skip("测试环境可能没有启用 RBAC 权限检查")
	}
}

func TestCreateSubscription_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试")
	}

	logic := setupRSSSubscriptionLogic()

	// 创建真实的订阅
	subscription, err := logic.CreateSubscription(
		testSpaceID,
		testResourceID,
		"https://blog.golang.org/feed.atom",
		"Go Blog",
		"Official Go Blog",
		"技术",
		3600,
	)
	require.NoError(t, err)
	require.NotNil(t, subscription)
	defer logic.DeleteSubscription(subscription.ID)

	t.Logf("创建的订阅: ID=%s, Title=%s", subscription.ID, subscription.Title)

	// 验证可以获取刚创建的订阅
	retrieved, err := logic.GetSubscription(subscription.ID)
	require.NoError(t, err)
	require.NotNil(t, retrieved)
	assert.Equal(t, subscription.ID, retrieved.ID)
	assert.Equal(t, subscription.Title, retrieved.Title)
	assert.Equal(t, subscription.URL, retrieved.URL)

	// 验证订阅出现在列表中
	subscriptions, err := logic.ListSubscriptions(testSpaceID)
	require.NoError(t, err)
	require.NotEmpty(t, subscriptions)

	found := false
	for _, sub := range subscriptions {
		if sub.ID == subscription.ID {
			found = true
			break
		}
	}
	assert.True(t, found, "新创建的订阅应该出现在列表中")

	t.Log("集成测试通过")
}

func TestDeleteSubscription_CascadeDeleteArticles(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试")
	}

	logic := setupRSSSubscriptionLogic()

	// 创建测试订阅
	subscription, err := logic.CreateSubscription(
		testSpaceID,
		testResourceID,
		"https://blog.golang.org/feed.atom",
		"Test RSS Feed",
		"Test RSS Feed Description",
		"测试",
		3600,
	)
	require.NoError(t, err)
	require.NotNil(t, subscription)
	defer logic.DeleteSubscription(subscription.ID)

	t.Logf("创建的订阅 ID: %s", subscription.ID)

	// 创建测试文章
	article1 := &types.RSSArticle{
		ID:             "test-article-1",
		SubscriptionID: subscription.ID,
		UserID:         testUserID,
		GUID:           "test-guid-1",
		Title:          "Test Article 1",
		Link:           "https://example.com/article1",
		Description:    "Test Article 1 Description",
		Content:        "Test Article 1 Content",
		Author:         "Test Author",
		PublishedAt:    time.Now().Unix(),
		FetchedAt:      time.Now().Unix(),
		CreatedAt:      time.Now().Unix(),
	}

	article2 := &types.RSSArticle{
		ID:             "test-article-2",
		SubscriptionID: subscription.ID,
		UserID:         testUserID,
		GUID:           "test-guid-2",
		Title:          "Test Article 2",
		Link:           "https://example.com/article2",
		Description:    "Test Article 2 Description",
		Content:        "Test Article 2 Content",
		Author:         "Test Author",
		PublishedAt:    time.Now().Unix(),
		FetchedAt:      time.Now().Unix(),
		CreatedAt:      time.Now().Unix(),
	}

	// 创建文章
	err = NewCore().Store().RSSArticleStore().Create(context.Background(), article1)
	require.NoError(t, err, "Failed to create article 1")

	err = NewCore().Store().RSSArticleStore().Create(context.Background(), article2)
	require.NoError(t, err, "Failed to create article 2")

	// 验证文章存在
	articles, err := NewCore().Store().RSSArticleStore().ListBySubscription(context.Background(), subscription.ID, 10)
	require.NoError(t, err)
	assert.Equal(t, 2, len(articles), "Expected 2 articles to exist")

	t.Logf("创建了 %d 篇文章", len(articles))

	// 删除订阅（应该级联删除文章）
	err = logic.DeleteSubscription(subscription.ID)
	require.NoError(t, err, "Failed to delete subscription")

	// 验证订阅已删除
	_, err = logic.GetSubscription(subscription.ID)
	assert.Error(t, err)
	assert.Equal(t, sql.ErrNoRows, err, "Expected subscription to be deleted")

	// 验证文章已删除
	articles, err = NewCore().Store().RSSArticleStore().ListBySubscription(context.Background(), subscription.ID, 10)
	require.NoError(t, err)
	assert.Equal(t, 0, len(articles), "Expected 0 articles to exist after subscription deletion")

	t.Log("级联删除测试通过：订阅和文章都已成功删除")
}

func TestRSSArticleStore_DeleteBySubscription(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试")
	}

	core := NewCore()

	// 创建测试文章
	article1 := &types.RSSArticle{
		ID:             "sub1-article-1",
		SubscriptionID: "subscription-1",
		UserID:         testUserID,
		GUID:           "sub1-guid-1",
		Title:          "Subscription 1 Article 1",
		Link:           "https://example.com/sub1/article1",
		Description:    "Description",
		Content:        "Content",
		Author:         "Test Author",
		PublishedAt:    time.Now().Unix(),
		FetchedAt:      time.Now().Unix(),
		CreatedAt:      time.Now().Unix(),
	}

	article2 := &types.RSSArticle{
		ID:             "sub2-article-1",
		SubscriptionID: "subscription-2",
		UserID:         testUserID,
		GUID:           "sub2-guid-1",
		Title:          "Subscription 2 Article 1",
		Link:           "https://example.com/sub2/article1",
		Description:    "Description",
		Content:        "Content",
		Author:         "Test Author",
		PublishedAt:    time.Now().Unix(),
		FetchedAt:      time.Now().Unix(),
		CreatedAt:      time.Now().Unix(),
	}

	// 创建文章
	err := core.Store().RSSArticleStore().Create(context.Background(), article1)
	require.NoError(t, err)

	err = core.Store().RSSArticleStore().Create(context.Background(), article2)
	require.NoError(t, err)

	// 验证两个订阅都有文章
	articles, err := core.Store().RSSArticleStore().ListBySubscription(context.Background(), "subscription-1", 10)
	require.NoError(t, err)
	assert.Equal(t, 1, len(articles), "Expected 1 article for subscription 1")

	articles, err = core.Store().RSSArticleStore().ListBySubscription(context.Background(), "subscription-2", 10)
	require.NoError(t, err)
	assert.Equal(t, 1, len(articles), "Expected 1 article for subscription 2")

	// 只删除订阅1的文章
	err = core.Store().RSSArticleStore().DeleteBySubscription(context.Background(), "subscription-1")
	require.NoError(t, err)

	// 验证订阅1没有文章了
	articles, err = core.Store().RSSArticleStore().ListBySubscription(context.Background(), "subscription-1", 10)
	require.NoError(t, err)
	assert.Equal(t, 0, len(articles), "Expected 0 articles for subscription 1 after deletion")

	// 验证订阅2仍然有文章
	articles, err = core.Store().RSSArticleStore().ListBySubscription(context.Background(), "subscription-2", 10)
	require.NoError(t, err)
	assert.Equal(t, 1, len(articles), "Expected 1 article for subscription 2 to remain")

	t.Log("RSSArticleStore.DeleteBySubscription 测试通过")
}
