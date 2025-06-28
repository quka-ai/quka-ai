package ai

import (
	"fmt"

	"github.com/quka-ai/quka-ai/pkg/types"
	"github.com/sashabaranov/go-openai"
)

type ToolHandlerFunc func(openai.FunctionCall) ([]openai.ChatCompletionMessage, error)

// 或者更通用的版本
func WrapToolHandler[T any](genContextFunc func() T, handler func(context T, args openai.FunctionCall) ([]openai.ChatCompletionMessage, error)) ToolHandlerFunc {
	return func(args openai.FunctionCall) ([]openai.ChatCompletionMessage, error) {
		appendMessages, err := handler(genContextFunc(), args)
		if err != nil {
			return nil, err
		}
		return appendMessages, nil
	}
}

func HandleToolCall(aiResp openai.ChatCompletionResponse, messages []openai.ChatCompletionMessage, toolHandler map[string]ToolHandlerFunc, receiveFunc types.ReceiveFunc) ([]openai.ChatCompletionMessage, error) {
	for _, choice := range aiResp.Choices {
		if choice.Message.Content != "" {
			messages = append(messages, openai.ChatCompletionMessage{
				Role:    types.USER_ROLE_ASSISTANT.String(),
				Content: choice.Message.Content,
			})

			if err := receiveFunc(&types.TextMessage{Text: choice.Message.Content}, types.MESSAGE_PROGRESS_GENERATING); err != nil {
				return nil, err
			}

			if len(choice.Message.ToolCalls) == 0 {
				continue
			}
		}

		for _, toolCall := range choice.Message.ToolCalls {
			if handler, ok := toolHandler[toolCall.Function.Name]; ok {

				appendMessages, err := handler(toolCall.Function)
				if err != nil {
					return nil, err
				}

				if len(appendMessages) > 0 {
					messages = append(messages, appendMessages...)
				}

			} else {
				messages = append(messages, openai.ChatCompletionMessage{
					Role:    types.USER_ROLE_SYSTEM.String(),
					Content: fmt.Sprintf("Tool %s not found", toolCall.Function.Name),
				})
			}
		}
	}
	fmt.Println(len(messages))
	return messages, nil
}
