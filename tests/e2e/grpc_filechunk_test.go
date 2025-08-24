package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/joho/godotenv"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/quka-ai/quka-ai/pkg/proto/filechunker"
)

// TestConfig holds configuration for e2e tests
type TestConfig struct {
	GRPCAddress    string
	OpenAIAPIKey   string
	OpenAIModel    string
	OpenAIEndpoint string
}

// getTestConfig loads test configuration from environment variables
func getTestConfig() *TestConfig {
	// Load .env file if it exists
	if envPath := filepath.Join(".", ".env"); fileExists(envPath) {
		_ = godotenv.Load(envPath)
	}

	return &TestConfig{
		GRPCAddress:    getEnvOrDefault("TEST_GRPC_ADDRESS", "localhost:35051"),
		OpenAIAPIKey:   getEnvOrDefault("TEST_OPENAI_API_KEY", ""),
		OpenAIModel:    getEnvOrDefault("TEST_OPENAI_MODEL", "gpt-3.5-turbo"),
		OpenAIEndpoint: getEnvOrDefault("TEST_OPENAI_ENDPOINT", "https://api.openai.com/v1"),
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// saveChunksToFile saves chunks to a JSON file
func saveChunksToFile(filename string, chunks []*filechunker.GenieChunk) error {
	// Create output directory if it doesn't exist
	outputDir := "test_output"
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Convert chunks to a more readable format
	type ChunkOutput struct {
		ID              string `json:"id"`
		Text            string `json:"text"`
		TokenCount      int32  `json:"token_count"`
		SemanticSummary string `json:"semantic_summary"`
	}

	output := make([]ChunkOutput, len(chunks))
	for i, chunk := range chunks {
		output[i] = ChunkOutput{
			ID:              chunk.Id,
			Text:            chunk.Text,
			TokenCount:      chunk.TokenCount,
			SemanticSummary: chunk.SemanticSummary,
		}
	}

	// Marshal to JSON with indentation
	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal chunks: %w", err)
	}

	// Write to file
	outputPath := filepath.Join(outputDir, filename)
	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// E2ETestSuite holds the test suite for e2e testing
type E2ETestSuite struct {
	config     *TestConfig
	grpcClient filechunker.FileChunkerServiceClient
	grpcConn   *grpc.ClientConn
}

// NewE2ETestSuite creates a new e2e test suite
func NewE2ETestSuite(t *testing.T) *E2ETestSuite {
	testConfig := getTestConfig()

	fmt.Println(testConfig)
	// Check if OpenAI API key is provided
	if testConfig.OpenAIAPIKey == "" {
		t.Skip("TEST_OPENAI_API_KEY not set, skipping e2e tests")
	}

	conn, err := grpc.NewClient(testConfig.GRPCAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err, "Failed to connect to gRPC service at %s", testConfig.GRPCAddress)

	client := filechunker.NewFileChunkerServiceClient(conn)

	return &E2ETestSuite{
		config:     testConfig,
		grpcClient: client,
		grpcConn:   conn,
	}
}

// Close cleans up resources
func (s *E2ETestSuite) Close() error {
	if s.grpcConn != nil {
		s.grpcConn.Close()
	}
	return nil
}

// TestGRPCFileChunkService tests the complete e2e flow
func TestGRPCFileChunkService(t *testing.T) {
	suite := NewE2ETestSuite(t)
	defer suite.Close()

	t.Run("HealthCheck", suite.TestHealthCheck)
	t.Run("GetSupportedLLMs", suite.TestGetSupportedLLMs)
	t.Run("ChunkTextFile", suite.TestChunkTextFile)
	t.Run("ChunkPDF", suite.TestChunkPDF)
	t.Run("ConcurrentChunking", suite.TestConcurrentChunking)
}

// TestHealthCheck tests the health check endpoint
func (s *E2ETestSuite) TestHealthCheck(t *testing.T) {
	ctx := context.Background()

	resp, err := s.grpcClient.HealthCheck(ctx, &filechunker.HealthCheckRequest{})
	require.NoError(t, err)
	require.True(t, resp.Healthy)
	require.NotEmpty(t, resp.Version)
	t.Logf("Service version: %s, status: %s", resp.Version, resp.Status)
}

// TestGetSupportedLLMs tests the supported LLMs endpoint
func (s *E2ETestSuite) TestGetSupportedLLMs(t *testing.T) {
	ctx := context.Background()

	resp, err := s.grpcClient.GetSupportedLLMs(ctx, &filechunker.GetSupportedLLMsRequest{})
	require.NoError(t, err)
	require.NotEmpty(t, resp.SupportedLlms)

	foundOpenAI := lo.SomeBy(resp.SupportedLlms, func(llm *filechunker.LLMInfo) bool {
		return llm.Provider == filechunker.LLMProvider_OPENAI
	})
	require.True(t, foundOpenAI)
	t.Logf("Found %d supported LLMs", len(resp.SupportedLlms))
}

// TestChunkTextFile tests chunking a text file
func (s *E2ETestSuite) TestChunkTextFile(t *testing.T) {
	ctx := context.Background()

	// Test content
	content := `This is a comprehensive document about artificial intelligence.
	
	# Introduction
	Artificial Intelligence (AI) is rapidly transforming our world, creating new possibilities across industries and reshaping how we live and work.
	
	## Machine Learning Fundamentals
	Machine learning is a subset of AI that enables systems to learn and improve from experience without being explicitly programmed.
	
	### Deep Learning
	Deep learning uses artificial neural networks with multiple layers to progressively extract higher-level features from raw input.
	
	# Applications
	AI applications span across healthcare, finance, transportation, and many other sectors.
	
	## Healthcare
	In healthcare, AI is revolutionizing diagnostics, drug discovery, and personalized medicine.
	
	### Diagnostic Imaging
	AI algorithms can now detect cancers and other diseases from medical imaging with accuracy comparable to human experts.`

	req := &filechunker.ChunkFileRequest{
		FileContent: []byte(content),
		Filename:    "ai_documentation.txt",
		MimeType:    "text/plain",
		Strategy:    filechunker.GenieStrategy_SLUMBER,
		Config: &filechunker.GenieConfig{
			LlmProvider:     filechunker.LLMProvider_OPENAI,
			ModelName:       s.config.OpenAIModel,
			ApiKey:          s.config.OpenAIAPIKey,
			LlmHost:         s.config.OpenAIEndpoint,
			TargetChunkSize: 800,
		},
	}

	resp, err := s.grpcClient.ChunkFile(ctx, req)
	require.NoError(t, err)
	require.True(t, resp.Success)
	require.NotEmpty(t, resp.Chunks)
	require.GreaterOrEqual(t, len(resp.Chunks), 1)

	// Validate chunk structure
	for _, chunk := range resp.Chunks {
		require.NotEmpty(t, chunk.Text)
		require.Greater(t, chunk.TokenCount, int32(0))
		require.NotEmpty(t, chunk.SemanticSummary)
		t.Logf("Chunk %s: %d tokens, summary: %s", chunk.Id, chunk.TokenCount, chunk.SemanticSummary)
	}

	// Save chunks to file
	err = saveChunksToFile("text_file_chunks.json", resp.Chunks)
	require.NoError(t, err, "Failed to save chunks to file")
	t.Logf("Chunks saved to test_output/text_file_chunks.json")

	t.Logf("Successfully chunked %d characters into %d chunks", len(content), len(resp.Chunks))
}

// TestChunkPDF tests chunking a PDF file (simplified)
func (s *E2ETestSuite) TestChunkPDF(t *testing.T) {
	ctx := context.Background()

	// PDF content simulation
	pdfContent := `This is a PDF document about cloud computing.
	
	# Cloud Computing Overview
	Cloud computing delivers computing services over the internet.
	
	## Service Models
	- IaaS: Infrastructure as a Service
	- PaaS: Platform as a Service
	- SaaS: Software as a Service
	
	## Deployment Models
	- Public Cloud
	- Private Cloud
	- Hybrid Cloud
	
	# Benefits
	Scalability, cost-effectiveness, and flexibility are key benefits of cloud computing.`

	req := &filechunker.ChunkFileRequest{
		FileContent: []byte(pdfContent),
		Filename:    "cloud_computing.pdf",
		MimeType:    "application/pdf",
		Strategy:    filechunker.GenieStrategy_SLUMBER,
		Config: &filechunker.GenieConfig{
			LlmProvider:     filechunker.LLMProvider_OPENAI,
			ModelName:       s.config.OpenAIModel,
			ApiKey:          s.config.OpenAIAPIKey,
			LlmHost:         s.config.OpenAIEndpoint,
			TargetChunkSize: 600,
		},
	}

	resp, err := s.grpcClient.ChunkFile(ctx, req)
	require.NoError(t, err)
	require.True(t, resp.Success)
	require.NotEmpty(t, resp.Chunks)

	// Save PDF chunks to file
	err = saveChunksToFile("pdf_file_chunks.json", resp.Chunks)
	require.NoError(t, err, "Failed to save PDF chunks to file")
	t.Logf("PDF chunks saved to test_output/pdf_file_chunks.json")

	t.Logf("PDF chunked into %d chunks", len(resp.Chunks))
	t.Logf("File metadata: %s, %d bytes, %d total tokens",
		resp.Metadata.OriginalFilename, resp.Metadata.FileSize, resp.Metadata.TotalTokens)
}

// TestConcurrentChunking tests concurrent chunking requests
func (s *E2ETestSuite) TestConcurrentChunking(t *testing.T) {
	ctx := context.Background()

	content1 := "Document 1: This is the first document for concurrent testing. It contains multiple sentences to ensure proper chunking behavior."
	content2 := "Document 2: This is the second document for concurrent testing. It also contains multiple sentences for testing purposes."
	content3 := "Document 3: This is the third document for concurrent testing. The content is designed to test concurrent processing capabilities."

	contents := []string{content1, content2, content3}

	// Concurrent requests using channels for synchronization
	results := make([]*filechunker.ChunkFileResponse, len(contents))
	errors := make([]error, len(contents))
	done := make(chan int, len(contents))

	// Launch goroutines for concurrent requests
	for i, content := range contents {
		go func(index int, text string) {
			defer func() { done <- index }()

			req := &filechunker.ChunkFileRequest{
				FileContent: []byte(text),
				Filename:    fmt.Sprintf("concurrent_test_%d.txt", index+1),
				MimeType:    "text/plain",
				Strategy:    filechunker.GenieStrategy_SLUMBER,
				Config: &filechunker.GenieConfig{
					LlmProvider:     filechunker.LLMProvider_OPENAI,
					ModelName:       s.config.OpenAIModel,
					ApiKey:          s.config.OpenAIAPIKey,
					LlmHost:         s.config.OpenAIEndpoint,
					TargetChunkSize: 300,
				},
			}

			results[index], errors[index] = s.grpcClient.ChunkFile(ctx, req)
		}(i, content)
	}

	// Wait for all requests to complete
	for i := 0; i < len(contents); i++ {
		select {
		case <-done:
			// Request completed
		case <-time.After(30 * time.Second):
			t.Fatal("Timeout waiting for concurrent requests")
		}
	}

	// Verify all requests completed successfully
	for i, err := range errors {
		require.NoError(t, err, "Request %d failed", i+1)
		require.True(t, results[i].Success, "Request %d not successful", i+1)
		t.Logf("Concurrent request %d processed %d chunks", i+1, len(results[i].Chunks))

		// Save each concurrent result to separate files
		filename := fmt.Sprintf("concurrent_chunks_%d.json", i+1)
		err = saveChunksToFile(filename, results[i].Chunks)
		require.NoError(t, err, "Failed to save concurrent chunks to file")
		t.Logf("Concurrent chunks %d saved to test_output/%s", i+1, filename)
	}
}
