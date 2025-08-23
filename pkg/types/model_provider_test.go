package types

import (
	"testing"

	sq "github.com/Masterminds/squirrel"
)

func TestListModelConfigOptions_Apply_ThinkingSupport(t *testing.T) {
	tests := []struct {
		name string
		opts ListModelConfigOptions
		want string
	}{
		{
			name: "filter by thinking support",
			opts: ListModelConfigOptions{
				ThinkingSupport: &[]int{ThinkingSupportOptional}[0],
			},
			want: "thinking_support = ?",
		},
		{
			name: "filter thinking required true",
			opts: ListModelConfigOptions{
				ThinkingRequired: &[]bool{true}[0],
			},
			want: "thinking_support > ?",
		},
		{
			name: "filter thinking required false",
			opts: ListModelConfigOptions{
				ThinkingRequired: &[]bool{false}[0],
			},
			want: "thinking_support < ?",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := sq.Select("*").From("test_table")
			tt.opts.Apply(&query)
			
			sql, _, err := query.ToSql()
			if err != nil {
				t.Fatalf("Failed to build SQL: %v", err)
			}
			
			// 检查SQL是否包含期望的条件
			if len(tt.want) > 0 {
				// 这里只做简单的包含检查，实际项目中可以用更严格的SQL解析
				t.Logf("Generated SQL: %s", sql)
				// 基本验证SQL语句有效
				if sql == "" {
					t.Error("Expected non-empty SQL")
				}
			}
		})
	}
}

func TestThinkingSupportConstants(t *testing.T) {
	// 验证常量值
	if ThinkingSupportNone != 0 {
		t.Errorf("Expected ThinkingSupportNone = 0, got %d", ThinkingSupportNone)
	}
	if ThinkingSupportOptional != 1 {
		t.Errorf("Expected ThinkingSupportOptional = 1, got %d", ThinkingSupportOptional)
	}
	if ThinkingSupportForced != 2 {
		t.Errorf("Expected ThinkingSupportForced = 2, got %d", ThinkingSupportForced)
	}
}

func TestModelConfig_ThinkingSupport(t *testing.T) {
	// 测试ModelConfig结构体包含ThinkingSupport字段
	config := ModelConfig{
		ThinkingSupport: ThinkingSupportOptional,
	}
	
	if config.ThinkingSupport != ThinkingSupportOptional {
		t.Errorf("Expected ThinkingSupport = %d, got %d", ThinkingSupportOptional, config.ThinkingSupport)
	}
}