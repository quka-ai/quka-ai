package safe

import (
	"log/slog"
	"runtime/debug"
	"strings"
)

func Run(fn func()) {
	defer func() {
		if r := recover(); r != nil {
			stack := getStackTrace(3) // Skip 3 frames: defer, Run, and the immediate caller
			slog.Error("panic recovered",
				slog.Any("recover", r),
				slog.String("component", "safe.Run"),
				slog.String("stack", stack),
			)
		}
	}()

	fn()
}

// RunWithLog is a wrapper that executes fn and logs any panic with full stack trace
func RunWithLog(fn func(), component string) {
	defer func() {
		if r := recover(); r != nil {
			stack := getStackTrace(3)
			slog.Error("panic recovered",
				slog.Any("recover", r),
				slog.String("component", component),
				slog.String("stack", stack),
			)
		}
	}()

	fn()
}

// getStackTrace returns a formatted stack trace
// skipFrames specifies how many initial frames to skip
func getStackTrace(skipFrames int) string {
	// Get stack information
	stackBytes := debug.Stack()

	// Convert to string and split into lines
	stackStr := string(stackBytes)
	lines := strings.Split(stackStr, "\n")

	// Format: remove the first few frames and format nicely
	var formatted []string
	formatted = append(formatted, "Stack trace:")

	// Skip the first 'skipFrames' frames (defer, Run, etc.)
	startIdx := skipFrames
	if startIdx < len(lines) {
		// The first line is just "goroutine X [running]:"
		if startIdx == 0 && len(lines) > 0 {
			formatted = append(formatted, "  "+lines[0])
			startIdx = 1
		}

		// Add the rest of the stack frames
		for i := startIdx; i < len(lines) && i < startIdx+20; i++ { // Limit to 20 frames
			line := strings.TrimSpace(lines[i])
			if line != "" {
				formatted = append(formatted, "  "+line)
			}
		}

		if len(lines) > startIdx+20 {
			formatted = append(formatted, "  ... (truncated)")
		}
	}

	return strings.Join(formatted, "\n")
}

// PrintStack prints the stack trace to standard error
func PrintStack() {
	debug.PrintStack()
}

// GetStack returns the stack trace as a string
func GetStack() string {
	return string(debug.Stack())
}
