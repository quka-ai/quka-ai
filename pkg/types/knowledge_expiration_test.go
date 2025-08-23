package types

import (
	"testing"
	"time"
)

func TestCalculateExpiredAt(t *testing.T) {
	createdAt := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).Unix()

	tests := []struct {
		name      string
		createdAt int64
		cycle     int
		want      int64
	}{
		{
			name:      "永不过期 - cycle为0",
			createdAt: createdAt,
			cycle:     0,
			want:      0,
		},
		{
			name:      "永不过期 - cycle为负数",
			createdAt: createdAt,
			cycle:     -1,
			want:      0,
		},
		{
			name:      "30天后过期",
			createdAt: createdAt,
			cycle:     30,
			want:      createdAt + 30*24*3600,
		},
		{
			name:      "1天后过期",
			createdAt: createdAt,
			cycle:     1,
			want:      createdAt + 1*24*3600,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateExpiredAt(tt.createdAt, tt.cycle)
			if got != tt.want {
				t.Errorf("CalculateExpiredAt() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestKnowledge_IsExpired(t *testing.T) {
	// Mock GetCurrentTimestamp 函数用于测试
	originalGetCurrentTimestamp := GetCurrentTimestamp
	defer func() {
		GetCurrentTimestamp = originalGetCurrentTimestamp
	}()

	currentTime := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC).Unix()
	GetCurrentTimestamp = func() int64 {
		return currentTime
	}

	tests := []struct {
		name      string
		knowledge *Knowledge
		want      bool
	}{
		{
			name: "永不过期的知识",
			knowledge: &Knowledge{
				ExpiredAt: 0,
			},
			want: false,
		},
		{
			name: "未过期的知识",
			knowledge: &Knowledge{
				ExpiredAt: currentTime + 86400, // 1天后过期
			},
			want: false,
		},
		{
			name: "刚好到期的知识",
			knowledge: &Knowledge{
				ExpiredAt: currentTime, // 当前时间过期
			},
			want: true,
		},
		{
			name: "已过期的知识",
			knowledge: &Knowledge{
				ExpiredAt: currentTime - 86400, // 1天前过期
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.knowledge.IsExpired()
			if got != tt.want {
				t.Errorf("Knowledge.IsExpired() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestKnowledge_SetExpiredAt(t *testing.T) {
	createdAt := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).Unix()

	tests := []struct {
		name  string
		cycle int
		want  int64
	}{
		{
			name:  "设置30天有效期",
			cycle: 30,
			want:  createdAt + 30*24*3600,
		},
		{
			name:  "设置永不过期",
			cycle: 0,
			want:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k := &Knowledge{
				CreatedAt: createdAt,
			}
			k.SetExpiredAt(tt.cycle)

			if k.ExpiredAt != tt.want {
				t.Errorf("Knowledge.SetExpiredAt() ExpiredAt = %v, want %v", k.ExpiredAt, tt.want)
			}
		})
	}
}
