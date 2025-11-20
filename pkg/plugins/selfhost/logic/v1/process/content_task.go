package process

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pgvector/pgvector-go"
	"github.com/samber/lo"
	"github.com/sashabaranov/go-openai"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/quka-ai/quka-ai/app/core"
	"github.com/quka-ai/quka-ai/app/logic/v1/process"
	"github.com/quka-ai/quka-ai/pkg/plugins/selfhost/srv"
	pb "github.com/quka-ai/quka-ai/pkg/proto/filechunker"
	"github.com/quka-ai/quka-ai/pkg/safe"
	"github.com/quka-ai/quka-ai/pkg/types"
	"github.com/quka-ai/quka-ai/pkg/utils"
)

type ContentTaskProcess struct {
	core         *core.Core
	grpcConn     *grpc.ClientConn
	grpcClient   pb.FileChunkerServiceClient
	PreChunkChan chan *types.ContentTask
	ChunkChan    chan *types.ContentTask
}

func NewContentTaskProcess(core *core.Core, cfg srv.ChunkService) *ContentTaskProcess {
	p := &ContentTaskProcess{
		core:         core,
		PreChunkChan: make(chan *types.ContentTask, 4),
		ChunkChan:    make(chan *types.ContentTask, 4),
	}

	// Initialize gRPC client if enabled
	if cfg.Enabled {
		timeout := time.Duration(cfg.Timeout) * time.Second
		if timeout == 0 {
			timeout = 30 * time.Second // default timeout
		}

		conn, err := grpc.NewClient(cfg.Address,
			grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			slog.Error("Failed to connect to gRPC filechunk service",
				slog.String("address", cfg.Address),
				slog.String("error", err.Error()))
		} else {
			p.grpcConn = conn
			p.grpcClient = pb.NewFileChunkerServiceClient(conn)
			slog.Info("gRPC filechunk client initialized successfully",
				slog.String("address", cfg.Address))
		}
	}

	go safe.Run(func() {
		p.loop()
	})

	return p
}

// Close closes the gRPC connection if it exists
func (p *ContentTaskProcess) Close() {
	if p.grpcConn != nil {
		if err := p.grpcConn.Close(); err != nil {
			slog.Error("Failed to close gRPC connection", slog.String("error", err.Error()))
		}
	}
}

func removeEmptyLine(text string) string {
	var newStr strings.Builder
	lines := strings.Split(text, "\n")
	// 遍历每一行，去除空行
	for _, line := range lines {
		// 如果行不为空，添加到结果中
		trim := strings.TrimSpace(line)
		if trim != "" {
			newStr.WriteString(trim)
		}
	}
	return newStr.String()
}

func (p *ContentTaskProcess) ProcessTasks() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	list, err := p.core.Store().ContentTaskStore().ListUnprocessedTasks(ctx, types.NO_PAGINATION, types.NO_PAGINATION)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("Failed to load unprocessed tasks, %w", err)
	}

	for _, v := range list {
		switch v.Step {
		case types.LONG_CONTENT_STEP_CREATE_CHUNK:
			select {
			case p.ChunkChan <- &types.ContentTask{
				TaskID:     v.TaskID,
				SpaceID:    v.SpaceID,
				UserID:     v.UserID,
				Resource:   v.Resource,
				MetaInfo:   v.MetaInfo,
				FileURL:    v.FileURL,
				FileName:   v.FileName,
				AIFileID:   v.AIFileID,
				Step:       v.Step,
				TaskType:   v.TaskType,
				CreatedAt:  v.CreatedAt,
				UpdatedAt:  v.UpdatedAt,
				RetryTimes: v.RetryTimes,
			}:
			default:
			}
		default:
		}
	}
	return nil
}

func (p *ContentTaskProcess) loop() {
	for range 10 {
		go safe.Run(func() {
			for req := range p.ChunkChan {
				var err error
				// Use gRPC service
				if p.grpcClient != nil {
					err = p.chunkByGRPC(req)
				} else {
					err = fmt.Errorf("gRPC chunk service is not available")
				}

				if err != nil {
					slog.Error("Failed to process chunk task", slog.String("error", err.Error()))

					ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
					if err = p.core.Store().ContentTaskStore().SetRetryTimes(ctx, req.TaskID, req.RetryTimes+1); err != nil {
						slog.Error("Failed to set content task process retry times", slog.String("stage", types.KNOWLEDGE_STAGE_EMBEDDING.String()),
							slog.String("error", err.Error()),
							slog.String("space_id", req.SpaceID),
							slog.String("task_id", req.TaskID),
							slog.String("component", "ContentTaskProcess.chunkByGRPC"))
					}
					cancel()
				}
			}
		})
	}
}

func (p *ContentTaskProcess) chunkByGRPC(task *types.ContentTask) error {
	if task.FileURL == "" || task.Step != types.LONG_CONTENT_STEP_CREATE_CHUNK {
		return fmt.Errorf("Failed to do chunk, please dispose pre chunk")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*10)
	defer cancel()

	ok, err := p.core.Plugins.TryLock(ctx, fmt.Sprintf("task:chunk:%s", task.TaskID))
	if err != nil || !ok {
		return err
	}

	// check task exist
	taskData, err := p.core.Store().ContentTaskStore().GetTask(ctx, task.TaskID)
	if err != nil {
		return err
	}

	if taskData.Step != types.LONG_CONTENT_STEP_CREATE_CHUNK {
		return nil
	}

	// Check cache first
	chunkResult, err := readFileCache(task.UserID, task.FileName)
	if err != nil {
		slog.Error("Failed to read file cache", slog.String("type", "long-content-chunk"), slog.String("task_id", task.TaskID), slog.String("error", err.Error()))
	}

	var chunks []*pb.GenieChunk
	if chunkResult == "" {
		// Download file
		downloadCtx, downloadCtxCancel := context.WithTimeout(context.Background(), time.Minute)
		defer downloadCtxCancel()
		res, err := p.core.Plugins.FileStorage().DownloadFile(downloadCtx, task.FileURL)
		if err != nil {
			return err
		}

		// Determine MIME type based on file extension
		// mimeType := "application/octet-stream"
		// if ext := filepath.Ext(task.FileName); ext != "" {
		// 	switch ext {
		// 	case ".pdf":
		// 		mimeType = "application/pdf"
		// 	case ".docx":
		// 		mimeType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
		// 	case ".txt":
		// 		mimeType = "text/plain"
		// 	case ".md":
		// 		mimeType = "text/markdown"
		// 	}
		// }

		// Get enhance model configuration
		enhanceModel, err := p.core.GetActiveModelConfig(ctx, types.AI_USAGE_ENHANCE)
		if err != nil {
			slog.Error("Failed to get enhance model configuration", slog.String("error", err.Error()))
			return err
		}

		// Prepare gRPC request with enhance model configuration
		grpcReq := &pb.ChunkFileRequest{
			FileContent: res.File,
			Filename:    task.FileName,
			// MimeType:    mimeType,
			Strategy: pb.GenieStrategy_SLUMBER, // Use LLM-driven semantic chunking
			Config: &pb.GenieConfig{
				LlmProvider:     pb.LLMProvider_OPENAI,
				ModelName:       enhanceModel.ModelName,
				ApiKey:          enhanceModel.Provider.ApiKey,
				LlmHost:         enhanceModel.Provider.ApiUrl,
				TargetChunkSize: 1000, // Default chunk size
				CustomPrompt:    "",   // Use default prompt
				LlmParams:       make(map[string]string),
			},
		}

		// Call gRPC service
		grpcResp, err := p.grpcClient.ChunkFile(ctx, grpcReq)
		if err != nil {
			return fmt.Errorf("Failed to chunk file via gRPC: %w", err)
		}

		if !grpcResp.Success {
			return fmt.Errorf("gRPC chunking failed: %s", grpcResp.Message)
		}

		chunks = grpcResp.Chunks

		// Record LLM usage if available
		if grpcResp.LlmUsage != nil {
			usage := &openai.Usage{
				PromptTokens:     int(grpcResp.LlmUsage.PromptTokens),
				CompletionTokens: int(grpcResp.LlmUsage.CompletionTokens),
				TotalTokens:      int(grpcResp.LlmUsage.TotalTokens),
			}
			process.NewRecordUsageRequest(grpcResp.LlmUsage.Model, types.USAGE_TYPE_SYSTEM, types.USAGE_SUB_TYPE_SUMMARY, task.SpaceID, task.UserID, usage)
		}

		// Cache the result
		raw, _ := json.Marshal(chunks)
		if err = storeFileCache(task.UserID, task.FileName, string(raw)); err != nil {
			slog.Error("Failed to save chunk result to file", slog.String("error", err.Error()),
				slog.String("task_id", task.TaskID), slog.String("task", "long-content-chunk"))
		}
	} else {
		// Load from cache
		if err = json.Unmarshal([]byte(chunkResult), &chunks); err != nil {
			return fmt.Errorf("Failed to unmarshal from file cache: %w", err)
		}
	}

	// Convert gRPC chunks to strings for handlerChunks
	chunkTexts := lo.Map(chunks, func(item *pb.GenieChunk, _ int) string {
		return item.Text
	})

	// Use taskData directly for handlerChunks
	err = p.handlerChunks(taskData, chunkTexts)
	if err != nil {
		return fmt.Errorf("Failed to handler chunks: %w", err)
	}

	return nil
}

func (p *ContentTaskProcess) handlerChunks(task *types.ContentTask, knowledgeChunks []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*5)
	defer cancel()

	err := p.core.Store().Transaction(ctx, func(ctx context.Context) error {
		// chunk的第一块存为knowledge meta
		knowledgeMeta := fmt.Sprintf("%s\n%s", removeEmptyLine(task.MetaInfo), knowledgeChunks[0])
		metaID := utils.GenUniqIDStr()
		err := p.core.Store().KnowledgeMetaStore().Create(ctx, types.KnowledgeMeta{
			ID:        metaID,
			SpaceID:   task.SpaceID,
			MetaInfo:  knowledgeMeta,
			CreatedAt: time.Now().Unix(),
		})
		if err != nil {
			return fmt.Errorf("Failed to create content meta, %w", err)
		}

		var (
			newKnowledgeIDs []string
			vectors         []types.Vector
			chunks          []string
			inserts         []*types.Knowledge
		)
		for i, item := range knowledgeChunks {
			item = removeEmptyLine(item)
			if item == "" {
				continue
			}
			encryptContent, _ := p.core.Plugins.EncryptData([]byte(item))
			id := utils.GenUniqIDStr()
			newKnowledgeIDs = append(newKnowledgeIDs, id)
			inserts = append(inserts, &types.Knowledge{
				Title:       fmt.Sprintf("%s-Chunk-%d", task.FileName, i+1),
				ID:          id,
				SpaceID:     task.SpaceID,
				UserID:      task.UserID,
				Resource:    task.Resource,
				Content:     encryptContent,
				ContentType: types.KNOWLEDGE_CONTENT_TYPE_MARKDOWN,
				Kind:        types.KNOWLEDGE_KIND_CHUNK,
				Stage:       types.KNOWLEDGE_STAGE_DONE,
				MaybeDate:   time.Now().Local().Format("2006-01-02 15:04"),
				RelDocID:    task.TaskID,
				CreatedAt:   time.Now().Unix(),
				UpdatedAt:   time.Now().Unix(),
			})

			vectors = append(vectors, types.Vector{
				ID:             utils.GenUniqIDStr(),
				KnowledgeID:    id,
				SpaceID:        task.SpaceID,
				UserID:         task.UserID,
				Resource:       task.Resource,
				OriginalLength: len([]rune(item)),
				// Embedding:   pgvector.NewVector(vector),
				CreatedAt: time.Now().Unix(),
				UpdatedAt: time.Now().Unix(),
			})

			chunks = append(chunks, fmt.Sprintf("## FileMeta:  \n%s  \n## Content  \n%s", knowledgeMeta, item))

			if (i != 0 && i%5 == 0) || i+1 == len(knowledgeChunks) {
				vectorResults, err := p.core.Srv().AI().EmbeddingForDocument(ctx, "", chunks)
				if err != nil {
					slog.Error("Failed to embedding for document", slog.String("error", err.Error()))
					return err
				}

				process.NewRecordUsageRequest(vectorResults.Model, "file_chunk", types.USAGE_SUB_TYPE_EMBEDDING, task.SpaceID, task.UserID, vectorResults.Usage)

				if len(vectorResults.Data) != len(vectors) {
					slog.Error("Embedding results count not matched chunks count", slog.String("error", "embedding result length not match"))
					return fmt.Errorf("embedding result length not match")
				}

				for i, v := range vectorResults.Data {
					vectors[i].Embedding = pgvector.NewVector(v)
				}

				if err = p.core.Store().KnowledgeStore().BatchCreate(ctx, inserts); err != nil {
					return fmt.Errorf("Failed to batch create knowledge, %w", err)
				}

				err = p.core.Store().VectorStore().BatchCreate(ctx, vectors)
				if err != nil {
					slog.Error("Failed to insert vector data into vector store", slog.String("error", err.Error()))
					return err
				}

				vectors = []types.Vector{}
				chunks = []string{}
				inserts = []*types.Knowledge{}
			}
		}

		batchRels := lo.Chunk(newKnowledgeIDs, 10)
		chunkIndex := 1
		for _, v := range batchRels {
			inserts := lo.Map(v, func(item string, _ int) types.KnowledgeRelMeta {
				res := types.KnowledgeRelMeta{
					KnowledgeID: item,
					SpaceID:     task.SpaceID,
					MetaID:      metaID,
					ChunkIndex:  chunkIndex,
					CreatedAt:   time.Now().Unix(),
				}
				chunkIndex++
				return res
			})
			if err = p.core.Store().KnowledgeRelMetaStore().BatchCreate(ctx, inserts); err != nil {
				return fmt.Errorf("Failed to batch create knolwedge relevance meta, %w", err)
			}
		}

		if err = p.core.Store().ContentTaskStore().UpdateStep(ctx, task.TaskID, types.LONG_CONTENT_STEP_FINISHED); err != nil {
			return fmt.Errorf("Failed to update task step, %w", err)
		}

		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

// NewRecordUsageRequest is imported from process/knowledge.go
// func NewRecordUsageRequest(model, _type, subType, spaceID, userID string, usage *openai.Usage)

// 存储文件缓存
func storeFileCache(userID, fileName, content string) error {
	// 构建存储路径
	dirPath := "./tmp"
	filePath := filepath.Join(dirPath, userID, fileName)

	dirPath = filepath.Dir(filePath)
	// 如果目录不存在，则创建目录
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		if err := os.MkdirAll(dirPath, os.ModePerm); err != nil {
			return fmt.Errorf("无法创建目录: %w", err)
		}
	}

	// 将内容写入文件
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("无法写入文件: %w", err)
	}

	return nil
}

// 读取文件缓存
func readFileCache(userID, fileName string) (string, error) {
	// 构建文件路径
	dirPath := "./tmp"
	filePath := filepath.Join(dirPath, userID, fileName)

	// 检查文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return "", nil
	}

	// 读取文件内容
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("无法读取文件: %w", err)
	}

	return string(content), nil
}
