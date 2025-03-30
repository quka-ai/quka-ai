package butler

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/samber/lo"
	"github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"

	"github.com/quka-ai/quka-ai/app/core"
	"github.com/quka-ai/quka-ai/pkg/ai"
	"github.com/quka-ai/quka-ai/pkg/safe"
	"github.com/quka-ai/quka-ai/pkg/types"
	"github.com/quka-ai/quka-ai/pkg/utils"
)

type ButlerAgent struct {
	core    *core.Core
	client  *openai.Client
	Model   string
	VlModel string
}

func NewButlerAgent(core *core.Core, client *openai.Client, model, vlModel string) *ButlerAgent {
	return &ButlerAgent{core: core, client: client, Model: model, VlModel: vlModel}
}

var FunctionDefine = lo.Map([]*openai.FunctionDefinition{
	{
		Name:        "createTable",
		Description: "如果没有合适的记录表，请使用该方法创建新的表",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"tableName": {
					Type:        jsonschema.String,
					Description: "新创建的表名",
				},
				"data": {
					Type:        jsonschema.String,
					Description: "数据表内容，markdown格式",
				},
				"tableDesc": {
					Type:        jsonschema.String,
					Description: "该数据表的描述信息，简介",
				},
			},
			Required: []string{"tableName", "data", "tableDesc"},
		},
	},
	{
		Name:        "queryTable",
		Description: "查询数据表情况",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"tableID": {
					Type:        jsonschema.String,
					Description: "需要查询的数据表ID",
					Items: &jsonschema.Definition{
						Type: jsonschema.String,
					},
				},
			},
			Required: []string{"tableID"},
		},
	},
	{
		Name:        "updateTable",
		Description: "如果已经存在相关的数据表，则使用该方法来对数据表内容进行变更，包括增、删、改",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"tableID": {
					Type:        jsonschema.String,
					Description: "需要修改的数据表ID",
				},
			},
			Required: []string{"tableID"},
		},
	},
	{
		Name:        "deleteTable",
		Description: "用户明确指定需要删除的数据表",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"tableID": {
					Type:        jsonschema.String,
					Description: "需要删除的数据表ID",
				},
			},
			Required: []string{"tableID"},
		},
	},
	{
		Name:        "chat",
		Description: "Just chat about anything",
	},
}, func(item *openai.FunctionDefinition, _ int) openai.Tool {
	return openai.Tool{
		Function: item,
	}
})

func (b *ButlerAgent) Query(userID string, reqMsg *types.ChatMessage) ([]openai.ChatCompletionMessage, []*openai.Usage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	butlerTables, err := b.core.Store().BulterTableStore().ListButlerTables(ctx, userID)
	if err != nil && err != sql.ErrNoRows {
		return nil, nil, err
	}

	userTables := strings.Builder{}
	for i, v := range butlerTables {
		if i == 0 {
			userTables.WriteString("| 表ID | 表名 | 表描述 |  \n")
			userTables.WriteString("| --- | --- | --- |  \n")
		}
		userTables.WriteString(fmt.Sprintf("| %s | %s | %s |  \n", v.TableID, v.TableName, v.TableDescription))
	}

	userData := userTables.String()

	// 获取session 历史记录
	list, err := b.core.Store().ChatMessageStore().ListSessionMessageUpToGivenID(ctx, reqMsg.SpaceID, reqMsg.SessionID, reqMsg.ID, 1, 10)
	if err != nil {
		slog.Error("Butler: failed to load session history message", slog.String("error", err.Error()), slog.String("session_id", reqMsg.SessionID))
		list = append(list, reqMsg)
	}

	list = lo.Reverse(list)

	req := []openai.ChatCompletionMessage{
		{
			Role:    types.USER_ROLE_SYSTEM.String(),
			Content: BUTLER_PROMPT_CN,
		},
		{
			Role:    types.USER_ROLE_SYSTEM.String(),
			Content: ai.GenerateTimeListAtNowCN(),
		},
		{
			Role:    types.USER_ROLE_SYSTEM.String(),
			Content: fmt.Sprintf("这是用户当前所有的数据表情况：\n%s\n，如果已经存在相同的表，请不要再创建，而是需要修改", lo.If(userData != "", userData).Else("用户当前没有任何数据表")),
		},
	}

	var (
		usages                []*openai.Usage
		needToUpdateMsgAttach bool
	)

	for _, item := range list {
		var msgContent = item.Message
		if item.IsEncrypt == types.MESSAGE_IS_ENCRYPT {
			msg, _ := b.core.DecryptData([]byte(item.Message))
			msgContent = string(msg)
		}
		if len(item.Attach) > 0 {
			if item.Attach[0].AIDescription != "" {
				imageAIDescriptions := strings.Join(lo.Map(item.Attach, func(item types.ChatAttach, i int) string {
					return fmt.Sprintf("第%d副图：%s", i, item.AIDescription)
				}), "\n")
				req = append(req, openai.ChatCompletionMessage{
					Role:    item.Role.String(),
					Content: fmt.Sprintf("%s\n%s", msgContent, imageAIDescriptions),
				})
				continue
			}

			if item.ID == reqMsg.ID {
				req = append(req, openai.ChatCompletionMessage{
					Role:    item.Role.String(),
					Content: item.Message,
				})
				for i, v := range reqMsg.Attach.ToMultiContent(reqMsg.Message) {
					if v.Type != openai.ChatMessagePartTypeImageURL {
						continue
					}
					resp, err := b.core.Srv().AI().DescribeImage(ctx, "中文", v.ImageURL.URL)
					if resp.Usage != nil {
						usages = append(usages, resp.Usage)
					}
					if err != nil {
						return nil, usages, err
					}

					req = append(req, openai.ChatCompletionMessage{
						Role:    types.USER_ROLE_SYSTEM.String(),
						Content: resp.Message(),
					})

					reqMsg.Attach[i].AIDescription = resp.Message()
					needToUpdateMsgAttach = true
				}
				continue
			}

			req = append(req, openai.ChatCompletionMessage{
				Role:         item.Role.String(),
				MultiContent: item.Attach.ToMultiContent(""),
			},
				openai.ChatCompletionMessage{
					Role:    item.Role.String(),
					Content: item.Message,
				})
			continue
		}

		req = append(req, openai.ChatCompletionMessage{
			Role:    item.Role.String(),
			Content: msgContent,
		})
	}

	if needToUpdateMsgAttach {
		go safe.Run(func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
			defer cancel()
			if err := b.core.Store().ChatMessageStore().UpdateMessageAttach(ctx, reqMsg.SessionID, reqMsg.ID, reqMsg.Attach); err != nil {
				slog.Error("Failed to update message attach for ai description", slog.String("error", err.Error()), slog.String("msg_id", reqMsg.ID))
			}
		})
	}

	appendMessage, usage, err := b.HandleUserRequest(userID, req)
	if usage != nil {
		usages = append(usages, usage)
	}

	if err != nil {
		return nil, usages, err
	}

	if len(appendMessage) > 0 {
		if appendMessage[0].Role == types.USER_ROLE_ASSISTANT.String() {
			return appendMessage, usages, nil
		}
	}

	return append(req, appendMessage...), usages, nil
}

func (b *ButlerAgent) HandleUserRequest(userID string, messages []openai.ChatCompletionMessage) ([]openai.ChatCompletionMessage, *openai.Usage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	resp, err := b.client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model:    b.Model,
			Messages: messages,
			Tools:    FunctionDefine,
		},
	)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to request ai: %w", err)
	}

	// 解析OpenAI的响应
	message := resp.Choices[0].Message
	if message.ToolCalls != nil {
		for _, v := range message.ToolCalls {
			switch v.Function.Name {
			case "createTable":
				var params struct {
					TableName string `json:"tableName"`
					TableDesc string `json:"tableDesc"`
					Data      string `json:"data"`
				}
				if err = json.Unmarshal([]byte(v.Function.Arguments), &params); err != nil {
					return nil, nil, err
				}
				res, err := b.CreateTable(userID, params.TableName, params.TableDesc, params.Data)
				return res, &resp.Usage, err
			case "queryTable":
				var params struct {
					TableID string `json:"tableID"`
				}
				if err = json.Unmarshal([]byte(v.Function.Arguments), &params); err != nil {
					return nil, nil, err
				}
				res, err := b.QueryTable(params.TableID, messages)
				return res, &resp.Usage, err
			case "updateTable":
				var params struct {
					TableID string `json:"tableID"`
				}

				if err = json.Unmarshal([]byte(v.Function.Arguments), &params); err != nil {
					return nil, nil, err
				}
				res, usage, err := b.ModifyTable(params.TableID, messages)
				if usage != nil {
					resp.Usage.TotalTokens += usage.TotalTokens
					resp.Usage.CompletionTokens += usage.CompletionTokens
					resp.Usage.PromptTokens += usage.PromptTokens
				}
				return res, &resp.Usage, err
			case "deleteTable":
				var params struct {
					TableID string `json:"tableID"`
				}

				if err = json.Unmarshal([]byte(v.Function.Arguments), &params); err != nil {
					return nil, nil, err
				}
				res, err := b.DeleteTable(params.TableID)
				return res, &resp.Usage, err
			case "chat":
				slog.Warn("Butler: continue chat")
				// TODO
			default:

			}
		}
	}

	slog.Warn("Butler: unknown function call", slog.Any("response", resp))
	return []openai.ChatCompletionMessage{
		{
			Role:    types.USER_ROLE_ASSISTANT.String(),
			Content: message.Content,
		},
	}, nil, nil
}

func (b *ButlerAgent) CreateTable(userID, tableName, tableDescription, data string) ([]openai.ChatCompletionMessage, error) {
	// 创建表格
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	err := b.core.Store().BulterTableStore().Create(ctx, types.ButlerTable{
		TableID:          utils.GenUniqIDStr(),
		UserID:           userID,
		TableName:        tableName,
		TableDescription: tableDescription,
		TableData:        data,
		CreatedAt:        time.Now().Unix(),
		UpdatedAt:        time.Now().Unix(),
	})
	if err != nil {
		return nil, err
	}
	return []openai.ChatCompletionMessage{{
		Role:    "system",
		Content: fmt.Sprintf("已经成功创建了数据表：%s \n 表描述： %s \n 表内容：\n%s\n请将结果总结给用户", tableName, tableDescription, data),
	}}, nil
}

func (b *ButlerAgent) QueryTable(tableID string, messages []openai.ChatCompletionMessage) ([]openai.ChatCompletionMessage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	data, err := b.core.Store().BulterTableStore().GetTableData(ctx, tableID)
	if err != nil {
		return nil, err
	}

	return []openai.ChatCompletionMessage{{
		Role:    "system",
		Content: fmt.Sprintf("查询到的数据表情况如下：\n表名：%s\n表描述：%s\n表内容：\n%s", data.TableName, data.TableDescription, lo.If(len(strings.Split(data.TableData, "\n")) >= 3, data.TableData).Else("空")),
	}}, nil
}

func (b *ButlerAgent) DeleteTable(tableID string) ([]openai.ChatCompletionMessage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	err := b.core.Store().BulterTableStore().Delete(ctx, tableID)
	if err != nil {
		return nil, err
	}

	return []openai.ChatCompletionMessage{{
		Role:    "system",
		Content: fmt.Sprintf("已经删除数据表：%s", tableID),
	}}, nil
}

func (b *ButlerAgent) ModifyTable(tableID string, messages []openai.ChatCompletionMessage) ([]openai.ChatCompletionMessage, *openai.Usage, error) {
	// 创建表格
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	table, err := b.core.Store().BulterTableStore().GetTableData(ctx, tableID)
	if err != nil {
		return nil, nil, err
	}

	_, userMessageIndex, ok := lo.FindIndexOf(messages, func(item openai.ChatCompletionMessage) bool {
		if item.Role == types.USER_ROLE_USER.String() {
			return true
		}
		return false
	})
	reqMessages := []openai.ChatCompletionMessage{
		{
			Role:    "system",
			Content: BUTLER_MODIFY_PROMPT_CN,
		},
		{
			Role:    "system",
			Content: ai.GenerateTimeListAtNowCN(),
		},
		{
			Role:    "system",
			Content: fmt.Sprintf("这是用户当前的数据表情况：\n%s", table.TableData),
		},
	}
	if ok {
		reqMessages = append(reqMessages, messages[userMessageIndex:]...)
	}

	resp, err := b.client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model:    b.Model,
			Messages: reqMessages,
			Tools: []openai.Tool{
				{
					Function: &openai.FunctionDefinition{
						Name:        "update",
						Description: "修改结果",
						Parameters: jsonschema.Definition{
							Type: jsonschema.Object,
							Properties: map[string]jsonschema.Definition{
								"data": {
									Type:        jsonschema.String,
									Description: "修改后的数据表内容，markdown格式",
								},
							},
							Required: []string{"tableID", "data"},
						},
					},
				},
			},
		},
	)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to request ai: %w", err)
	}

	message := resp.Choices[0].Message
	if len(message.ToolCalls) > 0 {
		for _, v := range message.ToolCalls {
			switch v.Function.Name {
			case "update":
				var params struct {
					Data string `json:"data"`
				}
				if err = json.Unmarshal([]byte(v.Function.Arguments), &params); err != nil {
					return nil, nil, fmt.Errorf("Failed to unmarshal modify params: %w", err)
				}

				ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
				defer cancel()
				if err = b.core.Store().BulterTableStore().Update(ctx, tableID, params.Data); err != nil {
					return nil, nil, fmt.Errorf("Failed to modify user table data, %w", err)
				}

				return []openai.ChatCompletionMessage{{
					Role:    types.USER_ROLE_SYSTEM.String(),
					Content: fmt.Sprintf("已经成功修改了数据表：%s \n 表内容：\n%s\n请将本次更新的结果总结给用户，并告知用户你更新了数据表，若用户没有要求则不必将表中数据完整的展示出来", table.TableName, params.Data),
				}}, &resp.Usage, nil
			default:
			}
		}
	}

	slog.Info("Butler: unknown function call", slog.Any("request", reqMessages), slog.Any("response", resp))

	return lo.Map(resp.Choices, func(item openai.ChatCompletionChoice, _ int) openai.ChatCompletionMessage {
		return openai.ChatCompletionMessage{
			Role:    item.Message.Role,
			Content: item.Message.Content,
		}
	}), &resp.Usage, nil
}
