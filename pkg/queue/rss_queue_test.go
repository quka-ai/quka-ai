package queue

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
)

// TestRSSQueue_NewRSSQueueWithClientServer 测试 RSS 队列创建
func TestRSSQueue_NewRSSQueueWithClientServer(t *testing.T) {
	// 创建测试用 Redis 客户端（单例模式）
	redisClient := newTestRedisClient()
	defer redisClient.Close()

	// 创建 asynq 客户端和服务器
	client := newTestAsynqClient(redisClient)
	defer client.Close()

	// 测试队列创建
	keyPrefix := "test-rss"
	queue := NewRSSQueueWithClientServer(keyPrefix, client)

	if queue == nil {
		t.Fatal("NewRSSQueueWithClientServer returned nil")
	}

	if queue.keyPrefix != keyPrefix {
		t.Errorf("Expected keyPrefix %q, got %q", keyPrefix, queue.keyPrefix)
	}

	if queue.client != client {
		t.Error("Client not set correctly")
	}
}

// TestRSSQueue_EnqueueTask 测试 RSS 任务入队
func TestRSSQueue_EnqueueTask(t *testing.T) {
	redisClient := newTestRedisClient()
	defer redisClient.Close()

	// 确保测试数据库是空的
	ctx := context.Background()
	redisClient.FlushDB(ctx)

	client := newTestAsynqClient(redisClient)
	defer client.Close()

	queue := NewRSSQueueWithClientServer("test", client)

	// 测试入队
	subscriptionID := "12345"
	err := queue.EnqueueTask(ctx, subscriptionID)
	if err != nil {
		t.Fatalf("EnqueueTask failed: %v", err)
	}

	// 验证任务是否正确入队到 "rss" 队列
	inspector := asynq.NewInspector(asynq.RedisClientOpt{
		Addr:     redisClient.Options().Addr,
		Password: redisClient.Options().Password,
		DB:       redisClient.Options().DB,
	})
	defer inspector.Close()

	// 等待一段时间确保任务被处理
	time.Sleep(100 * time.Millisecond)

	// 检查 "rss" 队列的状态
	queueInfo, err := inspector.GetQueueInfo(RSSQueueName)
	if err != nil {
		t.Errorf("Failed to get RSS queue info: %v", err)
	} else {
		t.Logf("RSS Queue stats: Size=%d, Active=%d, Pending=%d",
			queueInfo.Size, queueInfo.Active, queueInfo.Pending)
	}
}

// TestRSSQueue_EnqueueDelayedTask 测试延迟任务入队
func TestRSSQueue_EnqueueDelayedTask(t *testing.T) {
	redisClient := newTestRedisClient()
	defer redisClient.Close()

	redisClient.FlushDB(context.Background())

	client := newTestAsynqClient(redisClient)
	defer client.Close()

	queue := NewRSSQueueWithClientServer("test", client)

	// 测试延迟入队
	subscriptionID := "54321"
	delay := 1 * time.Second
	err := queue.EnqueueDelayedTask(context.Background(), subscriptionID, delay)
	if err != nil {
		t.Fatalf("EnqueueDelayedTask failed: %v", err)
	}

	t.Logf("RSS delayed task scheduled with delay: %v", delay)
}

// TestRSSQueue_SetupHandler 测试处理器设置
func TestRSSQueue_SetupHandler(t *testing.T) {
	redisClient := newTestRedisClient()
	defer redisClient.Close()

	client := newTestAsynqClient(redisClient)
	defer client.Close()

	queue := NewRSSQueueWithClientServer("test", client)

	// 创建测试处理器
	testHandler := func(ctx context.Context, task *asynq.Task) error {
		// 验证任务类型
		if task.Type() != TaskTypeRSSFetch {
			t.Errorf("Expected task type %q, got %q", TaskTypeRSSFetch, task.Type())
		}

		// 验证任务载荷
		var payload RSSFetchTask
		err := json.Unmarshal(task.Payload(), &payload)
		if err != nil {
			t.Errorf("Failed to unmarshal task payload: %v", err)
		}

		t.Logf("RSS Handler received task: %+v", payload)
		return nil
	}

	// 设置处理器
	mux := queue.SetupHandler(testHandler)
	if mux == nil {
		t.Fatal("SetupHandler returned nil")
	}

	t.Log("RSS Handler setup completed")
}

// TestRSSFetchTask_JSONMarshaling 测试任务结构的 JSON 序列化
func TestRSSFetchTask_JSONMarshaling(t *testing.T) {
	task := &RSSFetchTask{
		SubscriptionID: "99999",
	}

	// 测试序列化
	payload, err := json.Marshal(task)
	if err != nil {
		t.Fatalf("Failed to marshal task: %v", err)
	}

	// 测试反序列化
	var deserializedTask RSSFetchTask
	err = json.Unmarshal(payload, &deserializedTask)
	if err != nil {
		t.Fatalf("Failed to unmarshal task: %v", err)
	}

	// 验证数据一致性
	if deserializedTask.SubscriptionID != task.SubscriptionID {
		t.Errorf("SubscriptionID mismatch: expected %d, got %d", task.SubscriptionID, deserializedTask.SubscriptionID)
	}

	t.Logf("RSS JSON marshaling test passed: %+v", deserializedTask)
}

// newTestAsynqServerForRSS 创建用于 RSS 测试的 asynq 服务器
func newTestAsynqServerForRSS(redisClient *redis.Client) *asynq.Server {
	return asynq.NewServer(
		asynq.RedisClientOpt{
			Addr:     redisClient.Options().Addr,
			Password: redisClient.Options().Password,
			DB:       redisClient.Options().DB,
		},
		asynq.Config{
			Concurrency: 2, // 测试时使用较少的并发数
			Queues: map[string]int{
				RSSQueueName: 10, // 使用 rss 队列名称
			},
		},
	)
}
