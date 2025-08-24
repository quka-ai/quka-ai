package process

import (
	"context"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	pb "github.com/quka-ai/quka-ai/pkg/proto/filechunker"
)

// TestGRPCClientIntegration tests the gRPC client integration
func TestGRPCClientIntegration(t *testing.T) {
	buffer := 1024 * 1024
	lis := bufconn.Listen(buffer)

	baseServer := grpc.NewServer()
	mockService := &mockFileChunkerService{
		chunks: []*pb.GenieChunk{
			{
				Id:         "integration-chunk-1",
				Text:       "Integration test chunk content",
				TokenCount: 10,
			},
		},
	}

	pb.RegisterFileChunkerServiceServer(baseServer, mockService)
	go func() {
		baseServer.Serve(lis)
	}()
	defer baseServer.Stop()

	dialer := func(context.Context, string) (net.Conn, error) {
		return lis.Dial()
	}

	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(dialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("Failed to dial bufnet: %v", err)
	}
	defer conn.Close()

	client := pb.NewFileChunkerServiceClient(conn)

	// Test actual gRPC client functionality
	resp, err := client.ChunkFile(ctx, &pb.ChunkFileRequest{
		FileContent: []byte("Integration test content for gRPC client testing"),
		Filename:    "integration_test.txt",
		MimeType:    "text/plain",
		Strategy:    pb.GenieStrategy_SLUMBER,
		Config: &pb.GenieConfig{
			LlmProvider:     pb.LLMProvider_OPENAI,
			ModelName:       "gpt-3.5-turbo",
			ApiKey:          "integration-test-key",
			TargetChunkSize: 1000,
		},
	})

	if err != nil {
		t.Fatalf("ChunkFile failed: %v", err)
	}

	if !resp.Success {
		t.Errorf("Expected success, got: %s", resp.Message)
	}

	if len(resp.Chunks) != 1 {
		t.Errorf("Expected 1 chunk, got %d", len(resp.Chunks))
	}

	if resp.Chunks[0].Text != "Integration test chunk content" {
		t.Errorf("Unexpected chunk content: %s", resp.Chunks[0].Text)
	}
}

// TestErrorHandling tests error handling scenarios
func TestErrorHandling(t *testing.T) {
	buffer := 1024 * 1024
	lis := bufconn.Listen(buffer)

	baseServer := grpc.NewServer()
	mockService := &mockFileChunkerService{
		shouldFail: true,
	}

	pb.RegisterFileChunkerServiceServer(baseServer, mockService)
	go func() {
		baseServer.Serve(lis)
	}()
	defer baseServer.Stop()

	dialer := func(context.Context, string) (net.Conn, error) {
		return lis.Dial()
	}

	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(dialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}
	defer conn.Close()

	client := pb.NewFileChunkerServiceClient(conn)

	resp, err := client.ChunkFile(ctx, &pb.ChunkFileRequest{
		FileContent: []byte("test content"),
		Filename:    "test.txt",
		Config: &pb.GenieConfig{
			LlmProvider: pb.LLMProvider_OPENAI,
		},
	})

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if resp.Success {
		t.Error("Expected failure, got success")
	}

	if resp.Message == "" {
		t.Error("Expected error message, got empty")
	}
}
