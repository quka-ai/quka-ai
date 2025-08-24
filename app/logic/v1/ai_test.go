package v1_test

import (
	"context"
	"os"
	"sort"
	"testing"

	"github.com/quka-ai/quka-ai/app/core"
	v1 "github.com/quka-ai/quka-ai/app/logic/v1"
	"github.com/quka-ai/quka-ai/pkg/plugins"
	_ "github.com/quka-ai/quka-ai/pkg/plugins/selfhost"
	"github.com/quka-ai/quka-ai/pkg/types"
)

func NewSelfhostCore() *core.Core {
	core := core.MustSetupCore(core.MustLoadBaseConfig(os.Getenv("TEST_SELFHOST_CONFIG_PATH")))
	plugins.Setup(core.InstallPlugins, "selfhost")
	return core
}

func TestGenSummary(t *testing.T) {
	core := NewSelfhostCore()

	spaceID := "gPyofSEORU0ZskWmPh9CLUfv5PWjmXBZ"
	sessionID := "959520525577621504"
	var sequence int64 = 24

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

		if v.Complete != types.MESSAGE_PROGRESS_COMPLETE {
			continue
		}

		if len(v.Attach) > 0 {
			item := &types.MessageContext{
				Role: types.USER_ROLE_USER,
			}
			item.MultiContent = v.Attach.ToMultiContent("", core.FileStorage())
			reqMsg = append(reqMsg, item)
		}

		reqMsg = append(reqMsg, &types.MessageContext{
			Role:    v.Role,
			Content: v.Message,
		})
	}

	err = v1.GenChatSessionContextSummary(context.Background(), core, spaceID, sessionID, sequence, reqMsg)
	if err != nil {
		t.Fatal(err)
	}
}
