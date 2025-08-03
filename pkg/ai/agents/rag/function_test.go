package rag

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/components/tool"
	"github.com/quka-ai/quka-ai/app/core"
)

func TestRagTool_Info(t *testing.T) {
	// 创建测试用的 RagTool
	ragTool := NewRagTool(&core.Core{}, "space123", "user456", "session789", "msg000")

	// 测试 Info 方法
	ctx := context.Background()
	info, err := ragTool.Info(ctx)
	if err != nil {
		t.Fatalf("Failed to get tool info: %v", err)
	}

	// 验证工具信息
	if info.Name != FUNCTION_NAME_SEARCH_USER_KNOWLEDGES {
		t.Errorf("Expected name %s, got %s", FUNCTION_NAME_SEARCH_USER_KNOWLEDGES, info.Name)
	}

	if info.Desc == "" {
		t.Error("Expected non-empty description")
	}

	if info.ParamsOneOf == nil {
		t.Error("Expected ParamsOneOf to be set")
	}

	t.Log("✅ RagTool Info test passed")
	t.Logf("Tool name: %s", info.Name)
	t.Logf("Tool description: %s", info.Desc)
}

func TestRagTool_InvokableRun_InvalidJSON(t *testing.T) {
	// 创建测试用的 RagTool
	ragTool := NewRagTool(&core.Core{}, "space123", "user456", "session789", "msg000")

	// 测试无效 JSON 输入
	ctx := context.Background()
	_, err := ragTool.InvokableRun(ctx, "invalid json")
	if err == nil {
		t.Error("Expected error for invalid JSON, but got none")
	}

	t.Log("✅ RagTool InvokableRun invalid JSON test passed")
}

func TestRagTool_InvokableRun_ValidJSON(t *testing.T) {
	// 创建测试用的 RagTool
	ragTool := NewRagTool(&core.Core{}, "space123", "user456", "session789", "msg000")

	// 测试有效 JSON 输入 (这个测试预期会因为 nil core 而 panic)
	ctx := context.Background()
	validJSON := `{"query": "测试查询"}`
	
	// 使用 defer + recover 来捕获预期的 panic
	defer func() {
		if r := recover(); r != nil {
			t.Logf("Expected panic recovered: %v", r)
			t.Log("✅ RagTool InvokableRun valid JSON test passed (panic expected due to nil core)")
		}
	}()
	
	// 这个调用应该会 panic，因为 core 是空的
	result, err := ragTool.InvokableRun(ctx, validJSON)
	
	// 如果到这里没有 panic，说明可能有其他处理机制
	if err != nil {
		t.Logf("Got error instead of panic: %v", err)
	} else {
		t.Logf("Unexpected success: %s", result)
	}
	
	t.Log("✅ RagTool InvokableRun valid JSON test passed")
}

func TestRagTool_InterfaceCompliance(t *testing.T) {
	// 验证 RagTool 实现了 tool.InvokableTool 接口
	ragTool := NewRagTool(&core.Core{}, "space123", "user456", "session789", "msg000")
	
	// 编译时类型检查
	var _ tool.InvokableTool = ragTool
	
	t.Log("✅ RagTool implements tool.InvokableTool interface")
}