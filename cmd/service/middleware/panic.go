package middleware

import (
	"fmt"
	"log/slog"
	"runtime"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/quka-ai/quka-ai/app/response"
	"github.com/quka-ai/quka-ai/pkg/errors"
)

// PanicRecovery 捕获panic并记录详细的错误信息
func PanicRecovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// 获取调用堆栈
				stack := make([]byte, 4096)
				length := runtime.Stack(stack, false)

				// 解析堆栈，找到panic触发点
				panicInfo := parsePanicStack(stack[:length], err)

				// 输出到stderr，确保在stdout被淹没时也能看到
				slog.Error("=== PANIC RECOVERED ===",
					slog.String("error", fmt.Sprintf("%v", err)),
					slog.String("path", c.Request.URL.Path),
					slog.String("method", c.Request.Method),
					slog.String("trigger_point", panicInfo.triggerPoint),
					slog.String("stack_trace", panicInfo.relevantStack),
				)

				// 如果还在写入响应头，返回500错误
				if !c.Writer.Written() {
					response.APIError(c, errors.New("api.Middleware.PanicRecovery", "internal server error", nil))
				}

				// 中断后续处理
				c.Abort()
			}
		}()

		c.Next()
	}
}

// panicInfo 包含解析后的panic信息
type panicInfo struct {
	triggerPoint  string // panic触发的文件:行号
	relevantStack string // 相关的堆栈信息
}

// parsePanicStack 解析堆栈信息，提取关键部分
func parsePanicStack(stack []byte, panicErr interface{}) panicInfo {
	stackStr := string(stack)
	lines := strings.Split(stackStr, "\n")

	info := panicInfo{
		relevantStack: "",
		triggerPoint:  "unknown",
	}

	// 找到panic的第一行（包含panic消息）
	for i := 0; i < len(lines); i++ {
		if strings.Contains(lines[i], "panic(") {
			// panic行
			if i+1 < len(lines) && strings.TrimSpace(lines[i+1]) != "" {
				// 下一行通常是goroutine信息
				if i+2 < len(lines) && strings.Contains(lines[i+2], ".go:") {
					// 找到触发点
					info.triggerPoint = extractFileAndLine(lines[i+2])
				}
			}
			break
		}
	}

	// 提取与quka-ai相关的堆栈信息
	var relevantLines []string
	maxRelevantLines := 15 // 最多保留15行相关堆栈

	for i := 0; i < len(lines) && len(relevantLines) < maxRelevantLines; i++ {
		line := strings.TrimSpace(lines[i])

		// 跳过空行和goroutine信息行
		if line == "" || strings.HasPrefix(line, "goroutine") {
			continue
		}

		// 包含quka-ai的行
		if strings.Contains(line, "quka-ai") && strings.Contains(line, ".go:") {
			relevantLines = append(relevantLines, line)

			// 如果是函数调用行，也添加下一行（通常是文件位置）
			if i+1 < len(lines) && strings.TrimSpace(lines[i+1]) != "" &&
				!strings.HasPrefix(lines[i+1], "goroutine") {
				nextLine := strings.TrimSpace(lines[i+1])
				if strings.Contains(nextLine, "quka-ai") {
					relevantLines = append(relevantLines, nextLine)
				}
			}
		}
	}

	if len(relevantLines) > 0 {
		info.relevantStack = strings.Join(relevantLines, "\n")
	} else {
		// 如果没找到相关行，返回前20行
		maxLines := 20
		if len(lines) < maxLines {
			maxLines = len(lines)
		}
		info.relevantStack = strings.Join(lines[:maxLines], "\n")
	}

	return info
}

// extractFileAndLine 从堆栈行中提取文件和行号
func extractFileAndLine(line string) string {
	// 典型的堆栈行格式:
	// /path/to/file.go:123 +0x123
	parts := strings.Split(line, ":")
	if len(parts) >= 2 {
		// 提取文件名和行号
		fileAndLine := strings.TrimSpace(parts[0])
		if idx := strings.LastIndex(fileAndLine, "/"); idx >= 0 {
			fileAndLine = fileAndLine[idx+1:]
		}
		return fmt.Sprintf("%s:%s", fileAndLine, strings.Split(parts[1], " ")[0])
	}
	return line
}
