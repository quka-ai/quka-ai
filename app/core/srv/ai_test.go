package srv

import (
	"testing"

	"github.com/quka-ai/quka-ai/pkg/types"
)

func TestSetupAI_NilProvider(t *testing.T) {
	providers := []types.ModelConfig{
		{
			ID:        "test-1",
			ModelName: "test-model",
			ModelType: "chat",
			Provider:  nil, // This should not cause a panic
		},
		{
			ID:        "test-2",
			ModelName: "test-model-2",
			ModelType: "embedding",
			Provider:  nil, // This should not cause a panic
		},
	}

	usage := Usage{}

	// This should not panic
	ai, err := SetupAI(providers, []types.ModelProvider{}, usage)
	if err != nil {
		t.Fatalf("SetupAI failed: %v", err)
	}

	if ai == nil {
		t.Fatal("SetupAI returned nil AI")
	}

	// Should have empty drivers since all providers were nil
	if len(ai.chatDrivers) != 0 {
		t.Errorf("Expected 0 chat drivers, got %d", len(ai.chatDrivers))
	}
	if len(ai.embedDrivers) != 0 {
		t.Errorf("Expected 0 embed drivers, got %d", len(ai.embedDrivers))
	}
}
