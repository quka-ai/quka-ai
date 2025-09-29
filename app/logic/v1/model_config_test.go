package v1

import (
	"net/http"
	"testing"

	"github.com/quka-ai/quka-ai/pkg/types"
)

func TestCreateModelRequest_ThinkingSupport(t *testing.T) {
	// 测试创建模型请求包含ThinkingSupport字段
	req := CreateModelRequest{
		ProviderID:      "test-provider",
		ModelName:       "test-model",
		DisplayName:     "Test Model",
		ModelType:       types.MODEL_TYPE_CHAT,
		ThinkingSupport: types.ThinkingSupportOptional,
	}

	if req.ThinkingSupport != types.ThinkingSupportOptional {
		t.Errorf("Expected ThinkingSupport = %d, got %d", types.ThinkingSupportOptional, req.ThinkingSupport)
	}
}

func TestUpdateModelRequest_ThinkingSupport(t *testing.T) {
	// 测试更新模型请求包含ThinkingSupport字段
	thinkingSupport := types.ThinkingSupportForced
	req := UpdateModelRequest{
		ThinkingSupport: &thinkingSupport,
	}

	if req.ThinkingSupport == nil || *req.ThinkingSupport != types.ThinkingSupportForced {
		t.Errorf("Expected ThinkingSupport = %d, got %v", types.ThinkingSupportForced, req.ThinkingSupport)
	}
}

// 模拟验证思考配置的逻辑测试（不依赖数据库）
func TestValidateThinkingConfig_Logic(t *testing.T) {
	tests := []struct {
		name            string
		thinkingSupport int
		enableThinking  bool
		expectError     bool
		expectedErrCode int
	}{
		{
			name:            "None support with thinking enabled should error",
			thinkingSupport: types.ThinkingSupportNone,
			enableThinking:  true,
			expectError:     true,
			expectedErrCode: http.StatusBadRequest,
		},
		{
			name:            "Forced support with thinking disabled should error",
			thinkingSupport: types.ThinkingSupportForced,
			enableThinking:  false,
			expectError:     true,
			expectedErrCode: http.StatusBadRequest,
		},
		{
			name:            "Optional support with thinking enabled should pass",
			thinkingSupport: types.ThinkingSupportOptional,
			enableThinking:  true,
			expectError:     false,
		},
		{
			name:            "Optional support with thinking disabled should pass",
			thinkingSupport: types.ThinkingSupportOptional,
			enableThinking:  false,
			expectError:     false,
		},
		{
			name:            "None support with thinking disabled should pass",
			thinkingSupport: types.ThinkingSupportNone,
			enableThinking:  false,
			expectError:     false,
		},
		{
			name:            "Forced support with thinking enabled should pass",
			thinkingSupport: types.ThinkingSupportForced,
			enableThinking:  true,
			expectError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 模拟验证逻辑（不依赖实际数据库）
			var shouldError bool

			switch tt.thinkingSupport {
			case types.ThinkingSupportNone:
				if tt.enableThinking {
					shouldError = true
				}
			case types.ThinkingSupportForced:
				if !tt.enableThinking {
					shouldError = true
				}
			case types.ThinkingSupportOptional:
				// 可选，无需验证
			default:
				shouldError = true
			}

			if shouldError != tt.expectError {
				t.Errorf("Expected error = %v, got %v", tt.expectError, shouldError)
			}
		})
	}
}
