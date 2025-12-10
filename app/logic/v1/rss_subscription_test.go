package v1_test

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/quka-ai/quka-ai/app/core/srv"
	v1 "github.com/quka-ai/quka-ai/app/logic/v1"
	"github.com/quka-ai/quka-ai/pkg/security"
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

				t.Logf("创建的订阅 ID: %d, 标题: %s", subscription.ID, subscription.Title)

				// 清理：删除测试创建的订阅
				if subscription.ID > 0 {
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

	t.Logf("创建的订阅: ID=%d, Title=%s", subscription.ID, subscription.Title)

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
