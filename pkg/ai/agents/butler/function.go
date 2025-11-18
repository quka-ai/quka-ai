package butler

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"github.com/quka-ai/quka-ai/app/core"
)

// GetButlerTools 返回所有Butler工具的列表
func GetButlerTools(core *core.Core, userID string, agent *ButlerAgent) []tool.InvokableTool {
	return []tool.InvokableTool{
		NewListTableTool(core, userID, agent),
		NewCreateTableTool(core, userID, agent),
		NewQueryTableTool(core, userID, agent),
		NewUpdateTableTool(core, userID, agent),
		NewDeleteTableTool(core, userID, agent),
	}
}

const (
	FUNCTION_NAME_CREATE_TABLE = "createTable"
	FUNCTION_NAME_QUERY_TABLE  = "queryTable"
	FUNCTION_NAME_UPDATE_TABLE = "updateTable"
	FUNCTION_NAME_DELETE_TABLE = "deleteTable"
	FUNCTION_NAME_LIST_TABLE   = "listTable"
)

// ListTableTool 列出表格工具
type ListTableTool struct {
	core   *core.Core
	userID string
	agent  *ButlerAgent
}

// NewListTableTool 创建新的列出表格工具实例
func NewListTableTool(core *core.Core, userID string, agent *ButlerAgent) *ListTableTool {
	return &ListTableTool{
		core:   core,
		userID: userID,
		agent:  agent,
	}
}

var _ tool.InvokableTool = (*ListTableTool)(nil)

func (t *ListTableTool) Info(ctx context.Context) (*schema.ToolInfo, error) {

	return &schema.ToolInfo{
		Name: FUNCTION_NAME_LIST_TABLE,
		Desc: "该方法主要用于列出用户所有的数据表，显示表ID、表名和表描述",
	}, nil
}

func (t *ListTableTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	// 获取用户所有的butler表格
	butlerTables, err := t.core.Store().BulterTableStore().ListButlerTables(ctx, t.userID)
	if err != nil && err != sql.ErrNoRows {
		return "", fmt.Errorf("failed to list butler tables: %w", err)
	}

	if len(butlerTables) == 0 {
		return "用户当前没有任何数据表", nil
	}

	// 构建表格列表，使用Markdown表格格式
	userTables := strings.Builder{}
	userTables.WriteString("用户当前的数据表列表：\n\n")
	userTables.WriteString("| 表ID | 表名 | 表描述 |\n")
	userTables.WriteString("| --- | --- | --- |\n")

	for _, v := range butlerTables {
		userTables.WriteString(fmt.Sprintf("| %s | %s | %s |\n", v.TableID, v.TableName, v.TableDescription))
	}

	return userTables.String(), nil
}

// CreateTableTool 创建表格工具
type CreateTableTool struct {
	core   *core.Core
	userID string
	agent  *ButlerAgent
}

// NewCreateTableTool 创建新的创建表格工具实例
func NewCreateTableTool(core *core.Core, userID string, agent *ButlerAgent) *CreateTableTool {
	return &CreateTableTool{
		core:   core,
		userID: userID,
		agent:  agent,
	}
}

var _ tool.InvokableTool = (*CreateTableTool)(nil)

func (t *CreateTableTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	params := map[string]*schema.ParameterInfo{
		"tableName": {
			Type:     schema.String,
			Desc:     "新创建的表名",
			Required: true,
		},
		"tableDesc": {
			Type:     schema.String,
			Desc:     "该数据表的描述信息，简介",
			Required: true,
		},
		"data": {
			Type:     schema.String,
			Desc:     "数据表内容，markdown格式",
			Required: true,
		},
	}

	paramsOneOf := schema.NewParamsOneOfByParams(params)

	return &schema.ToolInfo{
		Name:        FUNCTION_NAME_CREATE_TABLE,
		Desc:        "如果没有合适的记录表，请使用该方法创建新的表",
		ParamsOneOf: paramsOneOf,
	}, nil
}

func (t *CreateTableTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	slog.Info("CreateTableTool InvokableRun called", slog.String("arguments", argumentsInJSON))
	var params struct {
		TableName string `json:"tableName"`
		TableDesc string `json:"tableDesc"`
		Data      string `json:"data"`
	}
	if err := json.Unmarshal([]byte(argumentsInJSON), &params); err != nil {
		slog.Error("CreateTableTool InvokableRun unmarshal error", slog.String("error", err.Error()))
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	messages, err := t.agent.CreateTable(t.userID, params.TableName, params.TableDesc, params.Data)
	if err != nil {
		return "", fmt.Errorf("failed to create table: %w", err)
	}
	if len(messages) > 0 {
		return messages[0].Content, nil
	}
	return "Table created successfully", nil
}

// QueryTableTool 查询表格工具
type QueryTableTool struct {
	core   *core.Core
	userID string
	agent  *ButlerAgent
}

// NewQueryTableTool 创建新的查询表格工具实例
func NewQueryTableTool(core *core.Core, userID string, agent *ButlerAgent) *QueryTableTool {
	return &QueryTableTool{
		core:   core,
		userID: userID,
		agent:  agent,
	}
}

var _ tool.InvokableTool = (*QueryTableTool)(nil)

func (t *QueryTableTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	params := map[string]*schema.ParameterInfo{
		"tableID": {
			Type:     schema.String,
			Desc:     "需要查询的数据表ID",
			Required: true,
		},
	}

	paramsOneOf := schema.NewParamsOneOfByParams(params)

	return &schema.ToolInfo{
		Name:        FUNCTION_NAME_QUERY_TABLE,
		Desc:        "查询数据表情况",
		ParamsOneOf: paramsOneOf,
	}, nil
}

func (t *QueryTableTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var params struct {
		TableID string `json:"tableID"`
	}
	if err := json.Unmarshal([]byte(argumentsInJSON), &params); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	messages, err := t.agent.QueryTable(params.TableID)
	if err != nil {
		return "", fmt.Errorf("failed to query table: %w", err)
	}
	if len(messages) > 0 {
		return messages[0].Content, nil
	}
	return "Query completed", nil
}

// UpdateTableTool 更新表格工具
type UpdateTableTool struct {
	core   *core.Core
	userID string
	agent  *ButlerAgent
}

// NewUpdateTableTool 创建新的更新表格工具实例
func NewUpdateTableTool(core *core.Core, userID string, agent *ButlerAgent) *UpdateTableTool {
	return &UpdateTableTool{
		core:   core,
		userID: userID,
		agent:  agent,
	}
}

var _ tool.InvokableTool = (*UpdateTableTool)(nil)

func (t *UpdateTableTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	params := map[string]*schema.ParameterInfo{
		"tableID": {
			Type:     schema.String,
			Desc:     "需要修改的数据表ID",
			Required: true,
		},
		"data": {
			Type:     schema.String,
			Desc:     "修改后的数据表内容，markdown格式",
			Required: true,
		},
	}

	paramsOneOf := schema.NewParamsOneOfByParams(params)

	return &schema.ToolInfo{
		Name:        FUNCTION_NAME_UPDATE_TABLE,
		Desc:        "如果已经存在相关的数据表，则使用该方法来对数据表内容进行变更，包括增、删、改",
		ParamsOneOf: paramsOneOf,
	}, nil
}

func (t *UpdateTableTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var params struct {
		TableID string `json:"tableID"`
		Data    string `json:"data"`
	}
	if err := json.Unmarshal([]byte(argumentsInJSON), &params); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	messages, _, err := t.agent.ModifyTable(params.TableID, params.Data)
	if err != nil {
		return "", fmt.Errorf("failed to update table: %w", err)
	}
	if len(messages) > 0 {
		return messages[0].Content, nil
	}
	return "Table updated successfully", nil
}

// DeleteTableTool 删除表格工具
type DeleteTableTool struct {
	core   *core.Core
	userID string
	agent  *ButlerAgent
}

// NewDeleteTableTool 创建新的删除表格工具实例
func NewDeleteTableTool(core *core.Core, userID string, agent *ButlerAgent) *DeleteTableTool {
	return &DeleteTableTool{
		core:   core,
		userID: userID,
		agent:  agent,
	}
}

var _ tool.InvokableTool = (*DeleteTableTool)(nil)

func (t *DeleteTableTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	params := map[string]*schema.ParameterInfo{
		"tableID": {
			Type:     schema.String,
			Desc:     "需要删除的数据表ID",
			Required: true,
		},
	}

	paramsOneOf := schema.NewParamsOneOfByParams(params)

	return &schema.ToolInfo{
		Name:        FUNCTION_NAME_DELETE_TABLE,
		Desc:        "用户明确指定需要删除的数据表",
		ParamsOneOf: paramsOneOf,
	}, nil
}

func (t *DeleteTableTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var params struct {
		TableID string `json:"tableID"`
	}
	if err := json.Unmarshal([]byte(argumentsInJSON), &params); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	messages, err := t.agent.DeleteTable(params.TableID)
	if err != nil {
		return "", fmt.Errorf("failed to delete table: %w", err)
	}
	if len(messages) > 0 {
		return messages[0].Content, nil
	}
	return "Table deleted successfully", nil
}
