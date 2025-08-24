package ai

import (
	"fmt"

	"github.com/quka-ai/quka-ai/pkg/types"
	"github.com/samber/lo"
	"github.com/sashabaranov/go-openai"
)

type ToolHandlerFunc func(openai.FunctionCall) ([]*types.MessageContext, error)

// 或者更通用的版本
func WrapToolHandler[T any](genContextFunc func() T, handler func(context T, args openai.FunctionCall) ([]*types.MessageContext, error)) ToolHandlerFunc {
	return func(args openai.FunctionCall) ([]*types.MessageContext, error) {
		appendMessages, err := handler(genContextFunc(), args)
		if err != nil {
			return nil, err
		}
		return appendMessages, nil
	}
}

func HandleToolCallOnly(tools []*openai.ToolCall, toolHandler map[string]ToolHandlerFunc) ([]*types.MessageContext, error) {
	if toolHandler == nil {
		toolHandler = make(map[string]ToolHandlerFunc)
	}

	var messages []*types.MessageContext
	for _, tool := range tools {
		messages = append(messages, &types.MessageContext{
			Role:    types.USER_ROLE_ASSISTANT,
			Content: fmt.Sprintf("I will use the tool %s to answer the question", tool.Function.Name),
		})
		if handler, ok := toolHandler[tool.Function.Name]; ok {
			appendMessages, err := handler(tool.Function)
			if err != nil {
				return nil, err
			}

			if len(appendMessages) > 0 {
				messages = append(messages, appendMessages...)
			} else {
				messages = append(messages, &types.MessageContext{
					Role:    types.USER_ROLE_SYSTEM,
					Content: fmt.Sprintf("Tool %s not responsing", tool.Function.Name),
				})
			}
		} else {
			messages = append(messages, &types.MessageContext{
				Role:    types.USER_ROLE_SYSTEM,
				Content: fmt.Sprintf("Tool %s not found", tool.Function.Name),
			})
		}
	}
	return messages, nil
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
					messages = append(messages, lo.Map(appendMessages, func(item *types.MessageContext, _ int) openai.ChatCompletionMessage {
						return openai.ChatCompletionMessage{
							Role:    item.Role.String(),
							Content: item.Content,
						}
					})...)
				}

			} else {
				messages = append(messages, openai.ChatCompletionMessage{
					Role:    types.USER_ROLE_SYSTEM.String(),
					Content: fmt.Sprintf("Tool %s not found", toolCall.Function.Name),
				})
			}
		}
	}
	return messages, nil
}
