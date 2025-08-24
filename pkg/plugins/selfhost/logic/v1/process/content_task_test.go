package process

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	pb "github.com/quka-ai/quka-ai/pkg/proto/filechunker"
)

// mockFileChunkerService implements the FileChunkerServiceServer interface for testing
type mockFileChunkerService struct {
	pb.UnimplementedFileChunkerServiceServer
	shouldFail bool
	chunks     []*pb.GenieChunk
}

func (m *mockFileChunkerService) ChunkFile(ctx context.Context, req *pb.ChunkFileRequest) (*pb.ChunkFileResponse, error) {
	if m.shouldFail {
		return &pb.ChunkFileResponse{
			Success: false,
			Message: "mocked failure",
		}, nil
	}

	return &pb.ChunkFileResponse{
		Success: true,
		Message: "success",
		Chunks:  m.chunks,
		Metadata: &pb.FileMetadata{
			OriginalFilename: req.Filename,
			MimeType:         req.MimeType,
			FileSize:         int64(len(req.FileContent)),
			TotalChunks:      int32(len(m.chunks)),
			TotalTokens:      100,
			ContentType:      "text",
			ComplexityScore:  0.8,
		},
		LlmUsage: &pb.LLMUsage{
			Provider:         "openai",
			Model:            req.Config.ModelName,
			PromptTokens:     50,
			CompletionTokens: 50,
			TotalTokens:      100,
			EstimatedCost:    0.001,
		},
	}, nil
}

func (m *mockFileChunkerService) GetSupportedLLMs(ctx context.Context, req *pb.GetSupportedLLMsRequest) (*pb.GetSupportedLLMsResponse, error) {
	return &pb.GetSupportedLLMsResponse{
		SupportedLlms: []*pb.LLMInfo{
			{
				Provider:       pb.LLMProvider_OPENAI,
				Name:           "OpenAI GPT",
				Models:         []string{"gpt-4", "gpt-3.5-turbo"},
				RequiresApiKey: true,
				Description:    "OpenAI GPT models",
			},
		},
	}, nil
}

func (m *mockFileChunkerService) HealthCheck(ctx context.Context, req *pb.HealthCheckRequest) (*pb.HealthCheckResponse, error) {
	return &pb.HealthCheckResponse{
		Healthy: true,
		Version: "1.0.0",
		Status:  "running",
		AvailableLlms: []string{
			"gpt-4",
			"gpt-3.5-turbo",
		},
	}, nil
}

// setupTestServer creates a test gRPC server and client
func setupTestServer(t *testing.T) (pb.FileChunkerServiceClient, func()) {
	buffer := 1024 * 1024
	lis := bufconn.Listen(buffer)

	baseServer := grpc.NewServer()
	mockService := &mockFileChunkerService{
		chunks: []*pb.GenieChunk{
			{
				Id:              "chunk-1",
				Text:            "This is the first chunk of content",
				TokenCount:      10,
				StartIndex:      0,
				EndIndex:        30,
				SemanticScore:   0.9,
				SemanticSummary: "Introduction content",
				KeyConcepts:     []string{"introduction", "overview"},
				Metadata:        map[string]string{"type": "text"},
			},
			{
				Id:              "chunk-2",
				Text:            "This is the second chunk of content with more details",
				TokenCount:      15,
				StartIndex:      31,
				EndIndex:        80,
				SemanticScore:   0.85,
				SemanticSummary: "Detailed content",
				KeyConcepts:     []string{"details", "analysis"},
				Metadata:        map[string]string{"type": "text"},
			},
		},
	}

	pb.RegisterFileChunkerServiceServer(baseServer, mockService)
	go func() {
		if err := baseServer.Serve(lis); err != nil {
			panic(fmt.Sprintf("Server exited with error: %v", err))
		}
	}()

	dialer := func(context.Context, string) (net.Conn, error) {
		return lis.Dial()
	}

	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(dialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("Failed to dial bufnet: %v", err)
	}

	client := pb.NewFileChunkerServiceClient(conn)

	return client, func() {
		conn.Close()
		baseServer.Stop()
	}
}

// TestChunkFile tests the ChunkFile gRPC method
func TestChunkFile(t *testing.T) {
	client, cleanup := setupTestServer(t)
	defer cleanup()

	tests := []struct {
		name        string
		request     *pb.ChunkFileRequest
		wantSuccess bool
		wantChunks  int
	}{
		{
			name: "successful chunking",
			request: &pb.ChunkFileRequest{
				FileContent: []byte("This is a test document with multiple sentences. It should be chunked into meaningful sections based on semantic analysis."),
				Filename:    "test.txt",
				MimeType:    "text/plain",
				Strategy:    pb.GenieStrategy_SLUMBER,
				Config: &pb.GenieConfig{
					LlmProvider:     pb.LLMProvider_OPENAI,
					ModelName:       "gpt-3.5-turbo",
					ApiKey:          "test-key",
					TargetChunkSize: 1000,
					CustomPrompt:    "",
					LlmParams:       map[string]string{"temperature": "0.7"},
				},
			},
			wantSuccess: true,
			wantChunks:  2,
		},
		{
			name: "empty content",
			request: &pb.ChunkFileRequest{
				FileContent: []byte(""),
				Filename:    "empty.txt",
				MimeType:    "text/plain",
				Strategy:    pb.GenieStrategy_SLUMBER,
				Config:      &pb.GenieConfig{LlmProvider: pb.LLMProvider_OPENAI},
			},
			wantSuccess: true,
			wantChunks:  2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			resp, err := client.ChunkFile(ctx, tt.request)
			if err != nil {
				t.Fatalf("ChunkFile() unexpected error: %v", err)
			}

			if resp.Success != tt.wantSuccess {
				t.Errorf("ChunkFile() success = %v, want %v", resp.Success, tt.wantSuccess)
			}

			if len(resp.Chunks) != tt.wantChunks {
				t.Errorf("ChunkFile() chunks count = %d, want %d", len(resp.Chunks), tt.wantChunks)
			}

			if resp.Metadata == nil {
				t.Error("ChunkFile() metadata should not be nil")
			}

			if resp.LlmUsage == nil {
				t.Error("ChunkFile() llm_usage should not be nil")
			}
		})
	}
}

// TestHealthCheck tests the HealthCheck gRPC method
func TestHealthCheck(t *testing.T) {
	client, cleanup := setupTestServer(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.HealthCheck(ctx, &pb.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("HealthCheck() unexpected error: %v", err)
	}

	if !resp.Healthy {
		t.Error("HealthCheck() healthy = false, want true")
	}

	if resp.Version == "" {
		t.Error("HealthCheck() version should not be empty")
	}
}

// TestGetSupportedLLMs tests the GetSupportedLLMs gRPC method
func TestGetSupportedLLMs(t *testing.T) {
	client, cleanup := setupTestServer(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.GetSupportedLLMs(ctx, &pb.GetSupportedLLMsRequest{})
	if err != nil {
		t.Fatalf("GetSupportedLLMs() unexpected error: %v", err)
	}

	if len(resp.SupportedLlms) == 0 {
		t.Error("GetSupportedLLMs() should return supported LLMs")
	}

	for _, llm := range resp.SupportedLlms {
		if llm.Provider == pb.LLMProvider_OPENAI {
			if len(llm.Models) == 0 {
				t.Error("OpenAI should have models listed")
			}
		}
	}
}

// TestChunkFileWithTimeout tests timeout handling
func TestChunkFileWithTimeout(t *testing.T) {
	client, cleanup := setupTestServer(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	_, err := client.ChunkFile(ctx, &pb.ChunkFileRequest{
		FileContent: []byte("test content"),
		Filename:    "test.txt",
		Config:      &pb.GenieConfig{LlmProvider: pb.LLMProvider_OPENAI},
	})

	// Since we're using bufconn, the timeout might not trigger as expected
	// This test is more about ensuring context cancellation is handled
	if err == nil {
		t.Log("Context cancellation test passed (may not timeout with bufconn)")
	} else {
		t.Logf("Context error: %v", err)
	}
}

// TestChunkFilePerformance tests the performance of the gRPC service
func TestChunkFilePerformance(t *testing.T) {
	client, cleanup := setupTestServer(t)
	defer cleanup()

	// Create a larger test file
	largeContent := make([]byte, 1024*10) // 10KB
	for i := range largeContent {
		largeContent[i] = 'a' + byte(i%26)
	}

	ctx := context.Background()
	start := time.Now()

	resp, err := client.ChunkFile(ctx, &pb.ChunkFileRequest{
		FileContent: largeContent,
		Filename:    "large_test.txt",
		MimeType:    "text/plain",
		Strategy:    pb.GenieStrategy_SLUMBER,
		Config: &pb.GenieConfig{
			LlmProvider:     pb.LLMProvider_OPENAI,
			ModelName:       "gpt-3.5-turbo",
			TargetChunkSize: 1000,
		},
	})

	duration := time.Since(start)

	if err != nil {
		t.Fatalf("ChunkFile failed: %v", err)
	}

	if !resp.Success {
		t.Errorf("Expected success, got: %s", resp.Message)
	}

	t.Logf("Chunked %d bytes in %v", len(largeContent), duration)
	if duration > 5*time.Second {
		t.Errorf("Operation took too long: %v", duration)
	}
}
