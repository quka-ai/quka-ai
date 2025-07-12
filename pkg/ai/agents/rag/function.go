package rag

import (
	"github.com/samber/lo"
	"github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"
)

const (
	FUNCTION_NAME_SEARCH_USER_KNOWLEDGES = "SearchUserKnowledges"
)

var FunctionDefine = lo.Map([]*openai.FunctionDefinition{
	{
		Name:        FUNCTION_NAME_SEARCH_USER_KNOWLEDGES,
		Description: "查询用户知识库中的相关知识",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"query": {
					Type:        jsonschema.String,
					Description: "用户的问题",
				},
			},
			Required: []string{"query"},
		},
	},
}, func(item *openai.FunctionDefinition, _ int) openai.Tool {
	return openai.Tool{
		Function: item,
	}
})
