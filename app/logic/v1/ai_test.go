package v1_test

import (
	"context"
	"log/slog"
	"os"
	"sort"
	"testing"

	"github.com/quka-ai/quka-ai/app/core"
	v1 "github.com/quka-ai/quka-ai/app/logic/v1"
	"github.com/quka-ai/quka-ai/pkg/plugins"
	_ "github.com/quka-ai/quka-ai/pkg/plugins/selfhost"
	"github.com/quka-ai/quka-ai/pkg/types"
	"github.com/sashabaranov/go-openai"
)

func NewSelfhostCore() *core.Core {
	core := core.MustSetupCore(core.MustLoadBaseConfig(os.Getenv("TEST_SELFHOST_CONFIG_PATH")))
	plugins.Setup(core.InstallPlugins, "selfhost")
	return core
}

func TestGenSummary(t *testing.T) {
	core := NewSelfhostCore()

	spaceID := "5FXRXGv2e9BDuQ6Tz0c8a7HAlfwzyfm4"
	sessionID := "1050480187923238912"
	var sequence int64 = 0

	// 获取比summary msgid更大的聊天内容组成上下文
	msgList, err := core.Store().ChatMessageStore().ListSessionMessage(context.Background(), spaceID, sessionID, sequence, types.NO_PAGINATION, types.NO_PAGINATION)
	if err != nil {
		t.Fatal(err)
	}

	// 对消息按msgid进行排序
	sort.Slice(msgList, func(i, j int) bool {
		return msgList[i].Sequence < msgList[j].Sequence
	})

	var reqMsg []*types.MessageContext

	for _, v := range msgList {
		if v.IsEncrypt == types.MESSAGE_IS_ENCRYPT {
			deData, err := core.DecryptData([]byte(v.Message))
			if err != nil {
				t.Fatal(err)
			}

			v.Message = string(deData)
		}

		// if isErrorMessage(v.Message) {
		// 	fmt.Println("", "skip error message in context:", v.Message)
		// 	continue
		// }

		if v.Complete != types.MESSAGE_PROGRESS_COMPLETE {
			continue
		}

		if len(v.Attach) > 0 {
			item := &types.MessageContext{
				Role: types.USER_ROLE_USER,
			}
			item.MultiContent = v.Attach.ToMultiContent(v.Message, core.FileStorage())
			reqMsg = append(reqMsg, item)
		} else {
			if v.Role == types.USER_ROLE_TOOL {
				ext, err := core.Store().ChatMessageExtStore().GetChatMessageExt(ctx, spaceID, sessionID, v.ID)
				if err != nil {
					slog.Error("failed to get tool call message ext", slog.Any("error", err))
					continue
				}
				reqMsg = append(reqMsg, &types.MessageContext{
					Role:    types.USER_ROLE_TOOL,
					Content: "",
					ToolCalls: []openai.ToolCall{
						{
							Type: openai.ToolTypeFunction,
							Function: openai.FunctionCall{
								Name:      ext.ToolName,
								Arguments: ext.ToolArgs.String,
							},
						},
					},
				})
			} else {
				reqMsg = append(reqMsg, &types.MessageContext{
					Role:    v.Role,
					Content: v.Message,
				})
			}
		}
	}

	err = v1.GenChatSessionContextSummary(context.Background(), core, spaceID, sessionID, sequence, reqMsg)
	if err != nil {
		t.Fatal(err)
	}
}
