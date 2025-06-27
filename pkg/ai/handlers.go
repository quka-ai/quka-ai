package ai

import (
	"fmt"

	"github.com/quka-ai/quka-ai/pkg/types"
	"github.com/sashabaranov/go-openai"
)

type ToolHandlerFunc func(openai.FunctionCall) error

// 或者更通用的版本
func WrapToolHandler[T any](genContextFunc func() T, handler func(context T, args openai.FunctionCall) error) ToolHandlerFunc {
	return func(args openai.FunctionCall) error {
		return handler(genContextFunc(), args)
	}
}

func HandleToolCall(aiResp openai.ChatCompletionResponse, messages []openai.ChatCompletionMessage, toolHandler map[string]ToolHandlerFunc, receiveFunc types.ReceiveFunc) error {
	for _, choice := range aiResp.Choices {
		if choice.Message.ToolCalls == nil {
			messages = append(messages, openai.ChatCompletionMessage{
				Role:    types.USER_ROLE_ASSISTANT.String(),
				Content: choice.Message.Content,
			})

			if err := receiveFunc(&types.TextMessage{Text: choice.Message.Content}, types.MESSAGE_PROGRESS_GENERATING); err != nil {
				return err
			}
			continue
		}

		for _, toolCall := range choice.Message.ToolCalls {
			if handler, ok := toolHandler[toolCall.Function.Name]; ok {
				if err := handler(toolCall.Function); err != nil {
					return err
				}
			} else {
				messages = append(messages, openai.ChatCompletionMessage{
					Role:    types.USER_ROLE_TOOL.String(),
					Content: fmt.Sprintf("Tool %s not found", toolCall.Function.Name),
				})
			}
		}
	}
	return nil
}
