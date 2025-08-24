package butler

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/samber/lo"
	"github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"

	"github.com/quka-ai/quka-ai/app/core"
	"github.com/quka-ai/quka-ai/pkg/types"
	"github.com/quka-ai/quka-ai/pkg/utils"
)

type ButlerAgent struct {
	core *core.Core
}

func NewButlerAgent(core *core.Core) *ButlerAgent {
	return &ButlerAgent{core: core}
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
		Role:    types.USER_ROLE_TOOL.String(),
		Content: fmt.Sprintf("已经成功创建了数据表：%s \n 表描述： %s \n 表内容：\n%s\n请将结果总结给用户", tableName, tableDescription, data),
	}}, nil
}

func (b *ButlerAgent) QueryTable(tableID string) ([]openai.ChatCompletionMessage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	data, err := b.core.Store().BulterTableStore().GetTableData(ctx, tableID)
	if err != nil {
		return nil, err
	}

	return []openai.ChatCompletionMessage{{
		Role:    types.USER_ROLE_TOOL.String(),
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
		Role:    types.USER_ROLE_TOOL.String(),
		Content: fmt.Sprintf("已经删除数据表：%s", tableID),
	}}, nil
}

func (b *ButlerAgent) ModifyTable(tableID, data string) ([]openai.ChatCompletionMessage, *openai.Usage, error) {
	// 创建表格
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	table, err := b.core.Store().BulterTableStore().GetTableData(ctx, tableID)
	if err != nil {
		return nil, nil, err
	}

	if err = b.core.Store().BulterTableStore().Update(ctx, tableID, data); err != nil {
		return nil, nil, fmt.Errorf("Failed to modify user table data, %w", err)
	}

	return []openai.ChatCompletionMessage{{
		Role:    types.USER_ROLE_TOOL.String(),
		Content: fmt.Sprintf("已经成功修改了数据表：%s \n 表内容：\n%s\n请将本次更新的结果总结给用户，并告知用户你更新了数据表，若用户没有要求则不必将表中数据完整的展示出来", table.TableName, data),
	}}, nil, nil
}
