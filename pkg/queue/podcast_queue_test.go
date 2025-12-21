package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/quka-ai/quka-ai/pkg/testutils"
	"github.com/redis/go-redis/v9"
)

// TestMain 用于设置测试环境
func TestMain(m *testing.M) {
	// 加载测试环境变量
	testutils.LoadEnvOrPanic()

	// 设置默认的 Redis 环境变量（如果未设置）
	setDefaultRedisEnv()

	// 运行测试
	m.Run()
}

// setDefaultRedisEnv 设置默认的 Redis 环境变量用于测试
func setDefaultRedisEnv() {
	if os.Getenv("QUKA_TEST_REDIS_ADDR") == "" {
		os.Setenv("QUKA_TEST_REDIS_ADDR", "localhost:6379")
	}
	if os.Getenv("QUKA_TEST_REDIS_PASSWORD") == "" {
		os.Setenv("QUKA_TEST_REDIS_PASSWORD", "")
	}
}

// newTestRedisClient 创建测试用的 Redis 客户端（单例模式）
func newTestRedisClient() *redis.Client {
	addr := os.Getenv("QUKA_TEST_REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}

	password := os.Getenv("QUKA_TEST_REDIS_PASSWORD")
	dbStr := os.Getenv("QUKA_TEST_REDIS_DB")
	db := 0
	if dbStr != "" {
		db = 1 // 测试使用 DB 1
	}

	return redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
}

// newTestAsynqClient 创建测试用的 asynq 客户端
func newTestAsynqClient(redisClient *redis.Client) *asynq.Client {
	return asynq.NewClient(asynq.RedisClientOpt{
		Addr:     redisClient.Options().Addr,
		Password: redisClient.Options().Password,
		DB:       redisClient.Options().DB,
	})
}

// newTestAsynqServer 创建测试用的 asynq 服务器
func newTestAsynqServer(redisClient *redis.Client) *asynq.Server {
	return asynq.NewServer(
		asynq.RedisClientOpt{
			Addr:     redisClient.Options().Addr,
			Password: redisClient.Options().Password,
			DB:       redisClient.Options().DB,
		},
		asynq.Config{
			Concurrency: 2, // 测试时使用较少的并发数
			Queues: map[string]int{
				PodcastQueueName: 10, // 使用 podcast 队列名称
			},
		},
	)
}

// TestPodcastQueue_NewPodcastQueueWithClientServer 测试队列创建
func TestPodcastQueue_NewPodcastQueueWithClientServer(t *testing.T) {
	// 创建测试用 Redis 客户端（单例模式）
	redisClient := newTestRedisClient()
	defer redisClient.Close()

	// 创建 asynq 客户端和服务器
	client := newTestAsynqClient(redisClient)
	defer client.Close()

	// 测试队列创建
	keyPrefix := "test"
	queue := NewPodcastQueueWithClientServer(keyPrefix, client)

	if queue == nil {
		t.Fatal("NewPodcastQueueWithClientServer returned nil")
	}

	if queue.keyPrefix != keyPrefix {
		t.Errorf("Expected keyPrefix %q, got %q", keyPrefix, queue.keyPrefix)
	}

	if queue.client != client {
		t.Error("Client not set correctly")
	}
}

// TestPodcastQueue_NewPodcastQueueWithClientServer_EmptyKeyPrefix 测试空 keyPrefix 的默认值
func TestPodcastQueue_NewPodcastQueueWithClientServer_EmptyKeyPrefix(t *testing.T) {
	redisClient := newTestRedisClient()
	defer redisClient.Close()

	client := newTestAsynqClient(redisClient)
	defer client.Close()

	// 测试空 keyPrefix，应该使用默认值 "quka"
	queue := NewPodcastQueueWithClientServer("", client)

	if queue.keyPrefix != "quka" {
		t.Errorf("Expected default keyPrefix %q, got %q", "quka", queue.keyPrefix)
	}
}

// TestPodcastQueue_EnqueueGenerationTask 测试任务入队
func TestPodcastQueue_EnqueueGenerationTask(t *testing.T) {
	redisClient := newTestRedisClient()
	defer redisClient.Close()

	// 确保测试数据库是空的
	ctx := context.Background()
	redisClient.FlushDB(ctx)

	client := newTestAsynqClient(redisClient)
	defer client.Close()

	queue := NewPodcastQueueWithClientServer("test", client)

	// 测试入队
	podcastID := "test-podcast-123"
	err := queue.EnqueueGenerationTask(ctx, podcastID)
	if err != nil {
		t.Fatalf("EnqueueGenerationTask failed: %v", err)
	}

	// 验证任务是否正确入队
	// 检查队列中的任务数量
	inspector := asynq.NewInspector(asynq.RedisClientOpt{
		Addr:     redisClient.Options().Addr,
		Password: redisClient.Options().Password,
		DB:       redisClient.Options().DB,
	})
	defer inspector.Close()

	// 等待一段时间确保任务被处理
	time.Sleep(100 * time.Millisecond)

	// 检查待处理队列
	queueInfo, err := inspector.GetQueueInfo(PodcastQueueName)
	if err != nil {
		t.Errorf("Failed to get queue info: %v", err)
	} else {
		t.Logf("Queue stats: Size=%d, Active=%d, Pending=%d",
			queueInfo.Size, queueInfo.Active, queueInfo.Pending)
	}
}

// TestPodcastQueue_EnqueueGenerationTask_MarshalError 测试 JSON 序列化错误
func TestPodcastQueue_EnqueueGenerationTask_MarshalError(t *testing.T) {
	redisClient := newTestRedisClient()
	defer redisClient.Close()

	client := newTestAsynqClient(redisClient)
	defer client.Close()

	queue := NewPodcastQueueWithClientServer("test", client)

	// 使用非常长的 podcastID 来测试潜在的序列化问题
	longID := string(make([]byte, 1000000)) // 1MB 的字符串
	err := queue.EnqueueGenerationTask(context.Background(), longID)

	// 这个测试主要验证序列化不会 panic
	// 即使序列化失败也应该返回错误而不是 panic
	if err == nil {
		t.Error("Expected error for marshal failure")
	}
}

// TestPodcastQueue_EnqueueDelayedGenerationTask 测试延迟任务入队
func TestPodcastQueue_EnqueueDelayedGenerationTask(t *testing.T) {
	redisClient := newTestRedisClient()
	defer redisClient.Close()

	redisClient.FlushDB(context.Background())

	client := newTestAsynqClient(redisClient)
	defer client.Close()

	queue := NewPodcastQueueWithClientServer("test", client)

	// 测试延迟入队
	podcastID := "test-podcast-delayed"
	delay := 1 * time.Second
	err := queue.EnqueueDelayedGenerationTask(context.Background(), podcastID, delay)
	if err != nil {
		t.Fatalf("EnqueueDelayedGenerationTask failed: %v", err)
	}

	t.Logf("Delayed task scheduled with delay: %v", delay)
}

// TestPodcastQueue_Shutdown 测试优雅关闭
func TestPodcastQueue_Shutdown(t *testing.T) {
	redisClient := newTestRedisClient()
	defer redisClient.Close()

	client := newTestAsynqClient(redisClient)

	queue := NewPodcastQueueWithClientServer("test", client)

	// 测试关闭不应该 panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Shutdown panicked: %v", r)
		}
	}()

	queue.Shutdown()
	t.Log("Shutdown completed without panic")
}

// TestPodcastQueue_Integration 集成测试
func TestPodcastQueue_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	redisClient := newTestRedisClient()
	defer redisClient.Close()

	// redisClient.FlushDB(context.Background())

	// client := newTestAsynqClient(redisClient)
	// server := newTestAsynqServer(redisClient)
	// defer client.Close()

	// queue := NewPodcastQueueWithClientServer("integration-test", client, server)

	// // 测试多个任务入队
	// const numTasks = 5
	// for i := range numTasks {
	// 	podcastID := "podcast-integration-test"
	// 	err := queue.EnqueueGenerationTask(context.Background(), podcastID)
	// 	if err != nil {
	// 		t.Fatalf("Failed to enqueue task %d: %v", i, err)
	// 	}
	// }

	// t.Logf("Enqueued %d tasks for integration test", numTasks)

	// // 等待任务被处理
	// time.Sleep(500 * time.Millisecond)

	// 检查队列状态
	inspector := asynq.NewInspector(asynq.RedisClientOpt{
		Addr:     redisClient.Options().Addr,
		Password: redisClient.Options().Password,
		DB:       redisClient.Options().DB,
	})
	defer inspector.Close()

	queueInfo, err := inspector.GetQueueInfo(PodcastQueueName)
	if err != nil {
		t.Errorf("Failed to get queue info: %v", err)
	} else {
		t.Logf("Integration test queue stats: Size=%d, Active=%d, Pending=%d, Processed=%d, Failed=%d",
			queueInfo.Size, queueInfo.Active, queueInfo.Pending, queueInfo.Processed, queueInfo.Failed)

		// 通过Redis直接查看失败任务详情
		t.Logf("\n=== 通过Redis查看所有队列状态 ===")

		// 检查所有相关队列
		queues := map[string]string{
			"pending":   "asynq:pending:podcast",
			"active":    "asynq:active:podcast",
			"failed":    "asynq:{podcast}:failed",
			"completed": "asynq:completed:podcast",
			"retry":     "asynq:retry:podcast",
			"stat":      "asynq:stat:podcast",
		}

		for name, key := range queues {
			// 检查key的类型
			keyType, err := redisClient.Type(context.Background(), key).Result()
			if err != nil {
				t.Errorf("Failed to get key type for %s: %v", name, err)
				continue
			}

			t.Logf("%s队列 (%s): 类型=%s", name, key, keyType)

			// 根据类型使用不同的方法
			if name == "stat" {
				statKeys, err := redisClient.HKeys(context.Background(), key).Result()
				if err != nil {
					t.Errorf("Failed to get %s stat keys: %v", name, err)
				} else {
					t.Logf("  %d 个统计项", len(statKeys))
					if len(statKeys) > 0 {
						t.Logf("  统计项: %v", statKeys)
						for _, statKey := range statKeys {
							value, err := redisClient.HGet(context.Background(), key, statKey).Result()
							if err == nil {
								t.Logf("    %s: %s", statKey, value)
							}
						}
					}
				}
			} else if keyType == "list" {
				count, err := redisClient.LLen(context.Background(), key).Result()
				if err != nil {
					t.Errorf("Failed to get %s queue count: %v", name, err)
				} else {
					t.Logf("  %d 个任务", count)

					// 如果队列中有任务，获取任务ID
					if count > 0 {
						taskIDs, err := redisClient.LRange(context.Background(), key, 0, 2).Result()
						if err != nil {
							t.Errorf("Failed to get %s task IDs: %v", name, err)
						} else {
							t.Logf("  前3个任务ID: %v", taskIDs)

							// 获取第一个任务的详细信息
							if len(taskIDs) > 0 {
								taskDetails, err := redisClient.HGetAll(context.Background(), fmt.Sprintf("asynq:task:%s", taskIDs[0])).Result()
								if err != nil {
									t.Errorf("Failed to get task details for %s: %v", taskIDs[0], err)
								} else {
									t.Logf("  任务详情 (%s):", taskIDs[0])
									for k, v := range taskDetails {
										t.Logf("    %s: %s", k, v)
									}
								}
							}
						}
					}
				}
			} else if keyType == "string" && name == "failed" {
				// 特殊情况：failed key是string类型，可能是计数器
				value, err := redisClient.Get(context.Background(), key).Result()
				if err == nil {
					t.Logf("  字符串值: %s", value)
				}
			} else {
				t.Logf("  非List类型，跳过")
			}
		}

		// 查找所有包含"failed"的键
		t.Logf("\n=== 搜索所有包含'failed'的键 ===")
		failedKeys, err := redisClient.Keys(context.Background(), "*failed*").Result()
		if err != nil {
			t.Errorf("Failed to search failed keys: %v", err)
		} else {
			t.Logf("找到 %d 个包含'failed'的键:", len(failedKeys))
			for _, key := range failedKeys {
				keyType, _ := redisClient.Type(context.Background(), key).Result()
				if keyType == "list" {
					count, _ := redisClient.LLen(context.Background(), key).Result()
					t.Logf("  %s (list): %d 个任务", key, count)

					if count > 0 {
						taskIDs, _ := redisClient.LRange(context.Background(), key, 0, 2).Result()
						t.Logf("    任务ID: %v", taskIDs)

						// 显示任务详情
						for _, taskID := range taskIDs {
							taskDetails, _ := redisClient.HGetAll(context.Background(), fmt.Sprintf("asynq:task:%s", taskID)).Result()
							if len(taskDetails) > 0 {
								t.Logf("    任务详情 (%s):", taskID)
								for k, v := range taskDetails {
									t.Logf("      %s: %s", k, v)
								}
							}
						}
					}
				} else {
					value, _ := redisClient.Get(context.Background(), key).Result()
					t.Logf("  %s (%s): %s", key, keyType, value)
				}
			}
		}

		// 检查是否有key prefix
		t.Logf("\n=== 检查Redis连接信息 ===")
		t.Logf("Redis地址: %s", redisClient.Options().Addr)
		t.Logf("Redis密码: %s", redisClient.Options().Password)
		t.Logf("Redis数据库: %d", redisClient.Options().DB)

		// 检查所有asynq相关的键
		t.Logf("\n=== 检查所有asynq键 ===")
		pattern := "asynq:*"
		keys, err := redisClient.Keys(context.Background(), pattern).Result()
		if err != nil {
			t.Errorf("Failed to get asynq keys: %v", err)
		} else {
			t.Logf("找到 %d 个asynq相关的键:", len(keys))
			for _, key := range keys {
				// 显示所有键
				keyType, _ := redisClient.Type(context.Background(), key).Result()
				if keyType == "list" {
					count, _ := redisClient.LLen(context.Background(), key).Result()
					t.Logf("  %s (list): %d 个元素", key, count)

					// 如果有任务，显示任务ID
					if count > 0 {
						taskIDs, _ := redisClient.LRange(context.Background(), key, 0, 2).Result()
						t.Logf("    任务ID: %v", taskIDs)

						// 显示任务详情
						if len(taskIDs) > 0 {
							taskDetails, _ := redisClient.HGetAll(context.Background(), fmt.Sprintf("asynq:task:%s", taskIDs[0])).Result()
							if len(taskDetails) > 0 {
								t.Logf("    任务详情 (%s):", taskIDs[0])
								for k, v := range taskDetails {
									t.Logf("      %s: %s", k, v)
								}
							}
						}
					}
				} else if keyType == "hash" {
					// hash类型可能包含任务详情
					fields, _ := redisClient.HGetAll(context.Background(), key).Result()
					t.Logf("  %s (hash): %d 个字段", key, len(fields))
					if len(fields) > 0 {
						// 显示state字段（如果有的话）
						if state, ok := fields["state"]; ok {
							t.Logf("    *** 任务状态: %s", state)
						}

						for k, v := range fields {
							// 只显示部分内容，避免过长
							if len(v) > 100 {
								t.Logf("    %s: %s...", k, v[:100])
							} else {
								t.Logf("    %s: %s", k, v)
							}
						}
					}
				} else if keyType == "string" {
					value, _ := redisClient.Get(context.Background(), key).Result()
					// 跳过过长的字符串
					if len(value) > 100 {
						t.Logf("  %s (string): %s...", key, value[:100])
					} else {
						t.Logf("  %s (string): %s", key, value)
					}
				} else if keyType == "zset" {
					// zset类型，可能包含archived任务ID
					members, _ := redisClient.ZRange(context.Background(), key, 0, 9).Result()
					t.Logf("  %s (zset): %d 个成员", key, len(members))
					if len(members) > 0 {
						t.Logf("    成员: %v", members)
					}
				} else {
					t.Logf("  %s (%s): 跳过", key, keyType)
				}
			}
		}
	}
}

// TestPodcastGenerationTask_JSONMarshaling 测试任务结构的 JSON 序列化
func TestPodcastGenerationTask_JSONMarshaling(t *testing.T) {
	task := &PodcastGenerationTask{
		PodcastID: "test-podcast-456",
	}

	// 测试序列化
	payload, err := json.Marshal(task)
	if err != nil {
		t.Fatalf("Failed to marshal task: %v", err)
	}

	// 测试反序列化
	var deserializedTask PodcastGenerationTask
	err = json.Unmarshal(payload, &deserializedTask)
	if err != nil {
		t.Fatalf("Failed to unmarshal task: %v", err)
	}

	// 验证数据一致性
	if deserializedTask.PodcastID != task.PodcastID {
		t.Errorf("PodcastID mismatch: expected %q, got %q", task.PodcastID, deserializedTask.PodcastID)
	}

	t.Logf("JSON marshaling test passed: %+v", deserializedTask)
}

// TestPodcastQueue_DequeueTask 测试任务出队（消费）流程
func TestPodcastQueue_DequeueTask(t *testing.T) {
	redisClient := newTestRedisClient()
	defer redisClient.Close()

	// 确保测试数据库是空的
	ctx := context.Background()
	redisClient.FlushDB(ctx)

	client := newTestAsynqClient(redisClient)
	defer client.Close()

	// 创建服务器（包含 worker）
	server := newTestAsynqServer(redisClient)
	defer server.Shutdown()

	queue := NewPodcastQueueWithClientServer("dequeue-test", client)

	// 用于同步的通道
	taskProcessed := make(chan bool, 1)
	var processedPodcastID string

	// 设置处理器，验证任务被正确消费
	handler := func(ctx context.Context, task *asynq.Task) error {
		var payload PodcastGenerationTask
		err := json.Unmarshal(task.Payload(), &payload)
		if err != nil {
			return fmt.Errorf("failed to unmarshal task: %w", err)
		}

		processedPodcastID = payload.PodcastID
		t.Logf("Worker processed task: %s", processedPodcastID)

		// 发送信号表示任务已处理
		taskProcessed <- true
		return nil
	}

	// 设置处理器
	mux := asynq.NewServeMux()
	mux.HandleFunc(TaskTypePodcastGeneration, handler)

	// 启动 worker（在 goroutine 中运行）
	workerDone := make(chan error, 1)
	go func() {
		workerDone <- server.Run(mux)
	}()

	// 等待 worker 启动
	time.Sleep(200 * time.Millisecond)

	// 入队一个任务
	testPodcastID := "test-podcast-dequeue-123"
	err := queue.EnqueueGenerationTask(ctx, testPodcastID)
	if err != nil {
		t.Fatalf("EnqueueGenerationTask failed: %v", err)
	}

	t.Logf("Enqueued task: %s", testPodcastID)

	// 等待任务被处理（最多等待 5 秒）
	select {
	case <-taskProcessed:
		t.Logf("Task processed successfully")
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for task to be processed")
	}

	// 验证任务被正确处理
	if processedPodcastID != testPodcastID {
		t.Errorf("Expected processed podcast ID %q, got %q", testPodcastID, processedPodcastID)
	}

	// 验证队列状态
	inspector := asynq.NewInspector(asynq.RedisClientOpt{
		Addr:     redisClient.Options().Addr,
		Password: redisClient.Options().Password,
		DB:       redisClient.Options().DB,
	})
	defer inspector.Close()

	queueInfo, err := inspector.GetQueueInfo(PodcastQueueName)
	if err != nil {
		t.Errorf("Failed to get queue info: %v", err)
	} else {
		t.Logf("Queue info after processing: Size=%d, Active=%d, Pending=%d, Processed=%d",
			queueInfo.Size, queueInfo.Active, queueInfo.Pending, queueInfo.Processed)

		// 验证队列状态正确
		if queueInfo.Active > 0 {
			t.Errorf("Expected 0 active tasks, got %d", queueInfo.Active)
		}
		if queueInfo.Pending > 0 {
			t.Errorf("Expected 0 pending tasks, got %d", queueInfo.Pending)
		}
	}

	// 关闭 worker
	server.Shutdown()
	select {
	case err := <-workerDone:
		if err != nil {
			t.Logf("Worker exited with error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Log("Worker did not exit within timeout")
	}
}

// TestPodcastQueue_DequeueDelayedTask 测试延迟任务出队
func TestPodcastQueue_DequeueDelayedTask(t *testing.T) {
	redisClient := newTestRedisClient()
	defer redisClient.Close()

	redisClient.FlushDB(context.Background())

	client := newTestAsynqClient(redisClient)
	defer client.Close()

	server := newTestAsynqServer(redisClient)
	defer server.Shutdown()

	queue := NewPodcastQueueWithClientServer("delayed-dequeue-test", client)

	taskProcessed := make(chan bool, 1)
	var processedPodcastID string

	handler := func(ctx context.Context, task *asynq.Task) error {
		var payload PodcastGenerationTask
		err := json.Unmarshal(task.Payload(), &payload)
		if err != nil {
			return err
		}

		processedPodcastID = payload.PodcastID
		taskProcessed <- true
		return nil
	}

	mux := asynq.NewServeMux()
	mux.HandleFunc(TaskTypePodcastGeneration, handler)

	// 启动 worker
	workerDone := make(chan error, 1)
	go func() {
		workerDone <- server.Run(mux)
	}()

	time.Sleep(200 * time.Millisecond)

	// 入队延迟任务（延迟 1 秒）
	testPodcastID := "test-podcast-delayed-dequeue"
	delay := 1 * time.Second
	err := queue.EnqueueDelayedGenerationTask(context.Background(), testPodcastID, delay)
	if err != nil {
		t.Fatalf("EnqueueDelayedGenerationTask failed: %v", err)
	}

	t.Logf("Enqueued delayed task (delay=%v): %s", delay, testPodcastID)

	// 延迟任务应该在 1 秒后被处理
	select {
	case <-taskProcessed:
		t.Logf("Delayed task processed successfully")
	case <-time.After(3 * time.Second):
		t.Fatal("Timeout waiting for delayed task to be processed")
	}

	// 验证任务被正确处理
	if processedPodcastID != testPodcastID {
		t.Errorf("Expected processed podcast ID %q, got %q", testPodcastID, processedPodcastID)
	}

	t.Logf("Delayed task processing verified: %s", processedPodcastID)

	// 关闭 worker
	server.Shutdown()
	select {
	case err := <-workerDone:
		if err != nil {
			t.Logf("Worker exited with error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Log("Worker did not exit within timeout")
	}
}

// Benchmark_EnqueueGenerationTask 性能基准测试
func Benchmark_EnqueueGenerationTask(b *testing.B) {
	redisClient := newTestRedisClient()
	defer redisClient.Close()

	redisClient.FlushDB(context.Background())

	client := newTestAsynqClient(redisClient)
	defer client.Close()

	queue := NewPodcastQueueWithClientServer("benchmark", client)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		podcastID := "benchmark-podcast"
		err := queue.EnqueueGenerationTask(context.Background(), podcastID)
		if err != nil {
			b.Fatalf("EnqueueGenerationTask failed: %v", err)
		}
	}
}
