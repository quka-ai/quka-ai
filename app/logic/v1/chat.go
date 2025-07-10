package v1

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/samber/lo"

	"github.com/quka-ai/quka-ai/app/core"
	"github.com/quka-ai/quka-ai/app/logic/v1/process"
	"github.com/quka-ai/quka-ai/pkg/ai/agents/rag"
	"github.com/quka-ai/quka-ai/pkg/errors"
	"github.com/quka-ai/quka-ai/pkg/i18n"
	"github.com/quka-ai/quka-ai/pkg/safe"
	"github.com/quka-ai/quka-ai/pkg/types"
	"github.com/quka-ai/quka-ai/pkg/types/protocol"
	"github.com/quka-ai/quka-ai/pkg/utils"
)

type ChatLogic struct {
	ctx  context.Context
	core *core.Core
	UserInfo
}

func NewChatLogic(ctx context.Context, core *core.Core) *ChatLogic {
	return &ChatLogic{
		ctx:      ctx,
		core:     core,
		UserInfo: SetupUserInfo(ctx, core),
	}
}

func GenUserTextMessage(spaceID, sessionID, userID, msgID, message string) *types.ChatMessage {
	return &types.ChatMessage{
		ID:        msgID,
		SpaceID:   spaceID,
		SessionID: sessionID,
		UserID:    userID,
		Role:      types.USER_ROLE_USER,
		Message:   message,
		MsgType:   types.MESSAGE_TYPE_TEXT,
		SendTime:  time.Now().Unix(),
		Complete:  types.MESSAGE_PROGRESS_COMPLETE,
	}
}

func (l *ChatLogic) NewUserMessage(chatSession *types.ChatSession, msgArgs types.CreateChatMessageArgs, resourceQuery *types.ResourceQuery) (seqid int64, err error) {
	slog.Debug("new message", slog.String("msg_id", msgArgs.ID), slog.String("user_id", l.GetUserInfo().User), slog.String("session_id", chatSession.ID))

	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("🚨 RequestAssistant发生panic: %v\n", r)
			// 详细堆栈写入日志文件，不打印到命令行
			slog.Error("RequestAssistant panic详细堆栈", slog.String("panic", fmt.Sprintf("%v", r)), slog.String("stack", string(debug.Stack())))
		}
	}()
	// 如果dialog为非正式状态，则转换为正式状态
	if chatSession == nil {
		return 0, errors.New("ChatLogic.NewUserMessageSend.dialog", i18n.ERROR_INTERNAL, nil)
	}

	if chatSession.Status != types.CHAT_SESSION_STATUS_OFFICIAL {
		go safe.Run(func() {
			if err = l.core.Store().ChatSessionStore().UpdateSessionStatus(l.ctx, chatSession.ID, types.CHAT_SESSION_STATUS_OFFICIAL); err != nil {
				slog.Error("send message failure, failed to update dialog status", slog.String("session_id", chatSession.ID), slog.String("error", err.Error()), slog.String("msg_id", msgArgs.ID))
				//		return 0, errors.New("ChatLogic.NewUserMessageSend.UpdateDialogStatus", i18n.ERROR_INTERNAL, err)
			}
		})
	}
	{
		ctx, cancel := context.WithCancel(l.ctx)
		defer cancel()
		if ok, err := l.core.TryLock(ctx, protocol.GenChatSessionAIRequestKey(chatSession.ID)); err != nil {
			return 0, errors.New("ChatLogic.NewUserMessageSend.TryLock", i18n.ERROR_INTERNAL, err)
		} else if !ok {
			slog.Debug("duplicate ai request", slog.String("msg_id", msgArgs.ID), slog.String("session_id", chatSession.ID))
			return 0, errors.New("ChatLogic.NewUserMessageSend.TryLock", i18n.ERROR_FORBIDDEN, nil).Code(http.StatusForbidden)
		}

		exist, err := l.core.Store().ChatMessageStore().Exist(l.ctx, chatSession.SpaceID, chatSession.ID, msgArgs.ID)
		if err != nil && err != sql.ErrNoRows {
			return 0, errors.New("ChatLogic.NewUserMessageSend.MessageStore.Exist", i18n.ERROR_INTERNAL, err)
		}

		if exist {
			return 0, errors.New("ChatLogic.NewUserMessageSend.MessageStore.DuplicateMessage", i18n.ERROR_EXIST, nil).Code(http.StatusForbidden)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*5)
	defer func() {
		if err != nil {
			cancel()
		}
	}()

	// session 消息分块逻辑(session block)
	latestMessage, err := l.core.Store().ChatMessageStore().GetSessionLatestUserMessage(ctx, chatSession.SpaceID, chatSession.ID)
	if err != nil && err != sql.ErrNoRows { // 获取dialog中最后一条消息的目的是为了做消息分块，如果失败，暂时先不影响用户的正常沟通，记录日志，方便从日志恢复(需要的话)
		slog.Error("failed to get chat session latest message", slog.String("session_id", chatSession.ID),
			slog.String("error", err.Error()),
			slog.String("relevance_msg_id", msgArgs.ID))
	}

	var msgBlockID int64
	if latestMessage != nil {
		msgBlockID = latestMessage.MsgBlock
		// 如果当前时间已经晚于dialog中最后一条消息发送时间20分钟
		if time.Now().After(time.Unix(latestMessage.SendTime, 0).Add(20 * time.Minute)) {
			msgBlockID++
		}
	}

	msg := &types.ChatMessage{
		ID:        msgArgs.ID,
		UserID:    l.GetUserInfo().User,
		SpaceID:   chatSession.SpaceID,
		SessionID: chatSession.ID,
		Message:   msgArgs.Message,
		MsgType:   msgArgs.MsgType,
		SendTime:  msgArgs.SendTime,
		MsgBlock:  msgBlockID,
		Role:      types.USER_ROLE_USER,
		Complete:  types.MESSAGE_PROGRESS_COMPLETE,
		Attach:    msgArgs.ChatAttach,
	}

	if msg.Sequence == 0 {
		seqid, err = l.core.Plugins.AIChatLogic("").GetChatSessionSeqID(l.ctx, chatSession.SpaceID, chatSession.ID)
		if err != nil {
			err = errors.Trace("ChatLogic.NewUserMessageSend.GetDialogSeqID", err)
			return
		}

		msg.Sequence = seqid
	}

	// if len([]rune(queryMsg)) < 20 && latestMessage != nil {
	// 	queryMsg = fmt.Sprintf("%s. %s", latestMessage.Message, queryMsg)
	// }

	messager := DefaultMessager(protocol.GenIMTopic(msg.SessionID), l.core.Srv().Tower())
	receiver := NewChatReceiver(ctx, l.core, messager)

	err = l.core.Store().Transaction(ctx, func(ctx context.Context) error {
		if err = l.core.Store().ChatMessageStore().Create(l.ctx, msg); err != nil {
			return errors.New("ChatLogic.NewUserMessageSend.ChatMessageStore.Create", i18n.ERROR_INTERNAL, err)
		}

		err = messager.PublishMessage(types.WS_EVENT_MESSAGE_PUBLISH, chatMsgToTextMsg(msg))
		// err = l.core.Srv().Tower().PublishMessageMeta(protocol.GenIMTopic(chatSession.ID), types.WS_EVENT_MESSAGE_PUBLISH, chatMsgToTextMsg(msg))
		if err != nil {
			slog.Error("failed to publish user message", slog.String("imtopic", protocol.GenIMTopic(chatSession.ID)),
				slog.String("msg_id", msgArgs.ID),
				slog.String("session_id", chatSession.ID),
				slog.String("error", err.Error()))
			return errors.New("ChatLogic.Srv.Tower.PublishMessageDetail", i18n.ERROR_INTERNAL, err)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}

	go safe.Run(func() {
		// update session latest access time
		if err := l.core.Store().ChatSessionStore().UpdateChatSessionLatestAccessTime(l.ctx, chatSession.SpaceID, chatSession.ID); err != nil {
			slog.Error("Failed to update chat session latest access time", slog.String("error", err.Error()),
				slog.String("space_id", chatSession.SpaceID), slog.String("session_id", chatSession.ID))
		}
	})

	containsAgent := types.FilterAgent(msgArgs.Message)
	if containsAgent == types.AGENT_TYPE_NONE {
		containsAgent = msgArgs.Agent
	}

	if len(msg.Attach) > 0 {
		for i := range msg.Attach {
			if msg.Attach[i].URL == "" {
				return 0, errors.New("ChatLogic.NewUserMessageSend.FileStorage.EmptyURL", i18n.ERROR_INVALIDARGUMENT, nil).Code(http.StatusBadRequest)
			}
			url, err := l.core.FileStorage().GenGetObjectPreSignURL(msg.Attach[i].URL)
			if err != nil {
				return 0, errors.New("ChatLogic.NewUserMessageSend.FileStorage.GenGetObjectPreSignURL", i18n.ERROR_INTERNAL, err)
			}
			msg.Attach[i].SignURL = url
		}
	}

	// check agents call
	switch containsAgent {
	case types.AGENT_TYPE_BUTLER:
		go safe.Run(func() {
			if err := ButlerSessionHandle(l.core, receiver, msg, &types.AICallOptions{
				GenMode:        types.GEN_MODE_NORMAL,
				GetDocsFunc:    nil,
				EnableThinking: msgArgs.EnableThinking,
			}); err != nil {
				slog.Error("Failed to handle butler message", slog.String("msg_id", msg.ID), slog.String("error", err.Error()))
			}
		})
	case types.AGENT_TYPE_JOURNAL:
		go safe.Run(func() {
			if err := JournalSessionHandle(l.core, receiver, msg, &types.AICallOptions{
				GenMode:        types.GEN_MODE_NORMAL,
				GetDocsFunc:    nil,
				EnableThinking: msgArgs.EnableThinking,
			}); err != nil {
				slog.Error("Failed to handle journal message", slog.String("msg_id", msg.ID), slog.String("error", err.Error()))
			}
		})
	case types.AGENT_TYPE_NORMAL:
		// else rag handler
		go safe.Run(func() {
			getDocsFunc := func() (types.RAGDocs, error) {
				enhanceResult, _ := rag.EnhanceChatQuery(l.ctx, l.core, msg.Message, msg.SpaceID, msg.SessionID, msg.ID)

				if enhanceResult.Usage != nil {
					process.NewRecordChatUsageRequest(enhanceResult.Model, types.USAGE_SUB_TYPE_QUERY_ENHANCE, msg.ID, enhanceResult.Usage)
				}

				docs, usages, err := rag.GetQueryRelevanceKnowledges(l.core, chatSession.SpaceID, l.GetUserInfo().User, enhanceResult.ResultQuery(), resourceQuery)
				if len(usages) > 0 {
					for _, v := range usages {
						process.NewRecordChatUsageRequest(v.Usage.Model, v.Subject, msgArgs.ID, v.Usage.Usage)
					}
				}
				if err != nil {
					err = errors.Trace("ChatLogic.getRelevanceKnowledges", err)
					return types.RAGDocs{}, err
				}

				// Supplement associated document content.
				rag.SupplementSessionChatDocs(l.core, chatSession.SpaceID, chatSession.ID, docs)
				return docs, nil
			}

			if err := RAGSessionHandle(l.core, receiver, msg, getDocsFunc, &types.AICallOptions{
				GenMode:        types.GEN_MODE_NORMAL,
				GetDocsFunc:    getDocsFunc,
				EnableThinking: msgArgs.EnableThinking,
			}); err != nil {
				slog.Error("Failed to handle rag message", slog.String("msg_id", msg.ID), slog.String("error", err.Error()))
			}
		})
	default:
		// else rag handler
		go safe.Run(func() {
			if err := ChatSessionHandle(l.core, receiver, msg, &types.AICallOptions{
				GenMode:        types.GEN_MODE_NORMAL,
				GetDocsFunc:    nil,
				EnableThinking: msgArgs.EnableThinking,
			}); err != nil {
				slog.Error("Failed to handle message", slog.String("msg_id", msg.ID), slog.String("error", err.Error()))
			}
		})
	}
	return msg.Sequence, nil
}

// 补充 session pin docs to docs
func SupplementSessionChatDocs(core *core.Core, chatSession *types.ChatSession, docs types.RAGDocs) {
	if chatSession == nil || len(docs.Refs) == 0 {
		return
	}

	pin, err := core.Store().ChatSessionPinStore().GetBySessionID(context.Background(), chatSession.ID)
	if err != nil && err != sql.ErrNoRows {
		slog.Error("Failed to get chat session pin", slog.String("session_id", chatSession.ID), slog.String("error", err.Error()))
		return
	}

	if pin == nil || len(pin.Content) == 0 || pin.Version != types.CHAT_SESSION_PIN_VERSION_V1 {
		return
	}

	var p types.ContentPinV1
	if err = json.Unmarshal(pin.Content, &p); err != nil {
		slog.Error("Failed to unmarshal chat session pin content", slog.String("session_id", chatSession.ID), slog.String("error", err.Error()))
		return
	}

	if len(p.Knowledges) == 0 {
		return
	}

	differenceItems, _ := lo.Difference(p.Knowledges, lo.Map(docs.Refs, func(item types.QueryResult, _ int) string { return item.KnowledgeID }))
	if len(differenceItems) == 0 {
		return
	}

	// Get the knowledge content of the difference item
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	knowledges, err := core.Store().KnowledgeStore().ListKnowledges(ctx, types.GetKnowledgeOptions{
		SpaceID: chatSession.SpaceID,
		IDs:     differenceItems,
	}, types.NO_PAGINATION, types.NO_PAGINATION)
	if err != nil {
		slog.Error("Failed to get knowledge content", slog.String("session_id", chatSession.ID), slog.String("error", err.Error()), slog.Any("knowledge_ids", differenceItems))
		return
	}

	for _, v := range knowledges {
		if v.Content, err = core.DecryptData(v.Content); err != nil {
			slog.Error("Failed to decrypt knowledge data", slog.String("session_id", chatSession.ID), slog.String("error", err.Error()))
			return
		}
	}

	if docs.Docs, err = core.AppendKnowledgeContentToDocs(docs.Docs, knowledges); err != nil {
		slog.Error("Failed to append knowledge content to docs", slog.String("session_id", chatSession.ID), slog.String("error", err.Error()))
		return
	}
}

func JournalHandle(core *core.Core, receiver types.Receiver, userMessage *types.ChatMessage, aiCallOptions *types.AICallOptions) error {
	logic := core.AIChatLogic(types.AGENT_TYPE_JOURNAL)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	return logic.RequestAssistant(ctx,
		userMessage, receiver, aiCallOptions)
}

func JournalSessionHandle(core *core.Core, receiver types.Receiver, userMessage *types.ChatMessage, aiCallOptions *types.AICallOptions) error {
	logic := core.AIChatLogic(types.AGENT_TYPE_JOURNAL)

	ext := types.ChatMessageExt{
		SpaceID:   userMessage.SpaceID,
		SessionID: userMessage.SessionID,
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()
	seqID, err := logic.GetChatSessionSeqID(ctx, userMessage.SpaceID, userMessage.SessionID)
	if err != nil {
		return err
	}

	answerMessageID := logic.GenMessageID()
	if err := receiver.RecvMessageInit(userMessage, answerMessageID, seqID, ext); err != nil {
		slog.Error("Failed to notify chat message inited event", slog.String("session_id", userMessage.SessionID),
			slog.String("message_id", userMessage.ID), slog.String("error", err.Error()))
	}

	// 使用新的变量名避免资源泄露
	reqCtx, reqCancel := context.WithTimeout(context.Background(), time.Minute)
	defer reqCancel()

	// listen to stop chat stream
	removeSignalFunc := core.Srv().Tower().RegisterStreamSignal(userMessage.SessionID, func() {
		slog.Debug("close chat stream", slog.String("session_id", userMessage.SessionID))
		reqCancel()
		receiver.GetDoneFunc(nil)(context.Canceled)
	})
	defer removeSignalFunc()

	return logic.RequestAssistant(reqCtx,
		userMessage, receiver, aiCallOptions)
}

func ButlerHandle(core *core.Core, receiver types.Receiver, userMessage *types.ChatMessage, aiCallOptions *types.AICallOptions) error {
	logic := core.AIChatLogic(types.AGENT_TYPE_BUTLER)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	return logic.RequestAssistant(ctx,
		userMessage, receiver, aiCallOptions)
}

func ButlerSessionHandle(core *core.Core, receiver types.Receiver, userMessage *types.ChatMessage, aiCallOptions *types.AICallOptions) error {
	logic := core.AIChatLogic(types.AGENT_TYPE_BUTLER)

	ext := types.ChatMessageExt{
		SpaceID:   userMessage.SpaceID,
		SessionID: userMessage.SessionID,
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()
	seqID, err := logic.GetChatSessionSeqID(ctx, userMessage.SpaceID, userMessage.SessionID)
	if err != nil {
		return err
	}

	answerMessageID := logic.GenMessageID()
	if err := receiver.RecvMessageInit(userMessage, answerMessageID, seqID, ext); err != nil {
		slog.Error("Failed to notify chat message inited event", slog.String("session_id", userMessage.SessionID),
			slog.String("message_id", userMessage.ID), slog.String("error", err.Error()))
	}

	// 使用新的变量名避免资源泄露
	reqCtx, reqCancel := context.WithTimeout(context.Background(), time.Minute)
	defer reqCancel()

	// listen to stop chat stream
	removeSignalFunc := core.Srv().Tower().RegisterStreamSignal(userMessage.SessionID, func() {
		slog.Debug("close chat stream", slog.String("session_id", userMessage.SessionID))
		reqCancel()
		receiver.GetDoneFunc(nil)(context.Canceled)
	})
	defer removeSignalFunc()

	return logic.RequestAssistant(reqCtx,
		userMessage, receiver, aiCallOptions)
}

func RAGHandle(core *core.Core, receiver types.Receiver, userMessage *types.ChatMessage, aiCallOptions *types.AICallOptions) error {
	logic := core.AIChatLogic(types.AGENT_TYPE_NORMAL)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	return logic.RequestAssistant(ctx,
		userMessage, receiver, aiCallOptions)
}

// genMode new request or re-request
func RAGSessionHandle(core *core.Core, receiver types.Receiver, userMessage *types.ChatMessage, getDocsFunc func() (types.RAGDocs, error), aiCallOptions *types.AICallOptions) error {

	logic := core.AIChatLogic(types.AGENT_TYPE_NORMAL)

	answerMessageID := logic.GenMessageID()
	ext := types.ChatMessageExt{
		MessageID: answerMessageID,
		SpaceID:   userMessage.SpaceID,
		SessionID: userMessage.SessionID,
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()
	seqID, err := logic.GetChatSessionSeqID(ctx, userMessage.SpaceID, userMessage.SessionID)
	if err != nil {
		return err
	}

	if err := receiver.RecvMessageInit(userMessage, answerMessageID, seqID, ext); err != nil {
		slog.Error("Failed to notify chat message inited event", slog.String("session_id", userMessage.SessionID),
			slog.String("message_id", userMessage.ID), slog.String("error", err.Error()))
		return err
	}
	// rag docs merge to user request message

	toolID := utils.GenUniqIDStr()

	receiveFunc := receiver.GetReceiveFunc()
	receiveFunc(&types.ToolTips{
		ID:       toolID,
		ToolName: "ReviewingKnowledges",
		Status:   types.TOOL_STATUS_RUNNING,
		Content:  "Reviewing your knowledges...",
	}, types.MESSAGE_PROGRESS_GENERATING)

	docs, err := getDocsFunc()
	if err != nil {
		return err
	}

	aiCallOptions.Docs = &docs
	var relDocs []string
	if len(docs.Docs) > 0 {
		relDocs = lo.Map(docs.Docs, func(item *types.PassageInfo, _ int) string {
			return item.ID
		})
	}

	// update message ext rel docs
	ext.RelDocs = relDocs
	func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
		defer cancel()
		if err := core.Store().ChatMessageExtStore().Update(ctx, ext.MessageID, ext); err != nil {
			slog.Error("Failed to update message ext", slog.String("message_id", userMessage.ID), slog.String("error", err.Error()))
		}
	}()

	go func() {
		time.Sleep(time.Second)
		receiveFunc(&types.ToolTips{
			ID:       toolID,
			ToolName: "ReviewingKnowledges",
			Status:   types.TOOL_STATUS_SUCCESS,
			Content:  fmt.Sprintf("%d knowledges reviewed", len(relDocs)),
		}, types.MESSAGE_PROGRESS_GENERATING)
	}()

	// 使用新的变量名避免资源泄露
	reqCtx, reqCancel := context.WithTimeout(context.Background(), time.Minute*5)
	defer reqCancel()

	// listen to stop chat stream
	removeSignalFunc := core.Srv().Tower().RegisterStreamSignal(userMessage.SessionID, func() {
		slog.Debug("close chat stream", slog.String("session_id", userMessage.SessionID))
		reqCancel()
		receiver.GetDoneFunc(nil)(context.Canceled)
	})
	defer removeSignalFunc()

	return logic.RequestAssistant(reqCtx,
		userMessage, receiver, aiCallOptions)
}

func ChatHandle(core *core.Core, receiver types.Receiver, userMessage *types.ChatMessage, aiCallOptions *types.AICallOptions) error {
	logic := core.AIChatLogic(types.AGENT_TYPE_NORMAL)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	return logic.RequestAssistant(ctx,
		userMessage, receiver, aiCallOptions)
}

// genMode new request or re-request
func ChatSessionHandle(core *core.Core, receiver types.Receiver, userMessage *types.ChatMessage, aiCallOptions *types.AICallOptions) error {
	logic := core.AIChatLogic(types.AGENT_TYPE_NORMAL)

	ext := types.ChatMessageExt{
		SpaceID:   userMessage.SpaceID,
		SessionID: userMessage.SessionID,
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()
	seqID, err := logic.GetChatSessionSeqID(ctx, userMessage.SpaceID, userMessage.SessionID)
	if err != nil {
		return err
	}

	answerMessageID := logic.GenMessageID()
	if err := receiver.RecvMessageInit(userMessage, answerMessageID, seqID, ext); err != nil {
		slog.Error("Failed to notify chat message inited event", slog.String("session_id", userMessage.SessionID),
			slog.String("message_id", userMessage.ID), slog.String("error", err.Error()))
		return err
	}
	// rag docs merge to user request message

	// 使用新的变量名避免资源泄露
	reqCtx, reqCancel := context.WithTimeout(context.Background(), time.Minute*5)
	defer reqCancel()

	// listen to stop chat stream
	removeSignalFunc := core.Srv().Tower().RegisterStreamSignal(userMessage.SessionID, func() {
		slog.Debug("close chat stream", slog.String("session_id", userMessage.SessionID))
		reqCancel()
		receiver.GetDoneFunc(nil)(context.Canceled)
	})
	defer removeSignalFunc()

	return logic.RequestAssistant(reqCtx,
		userMessage, receiver, aiCallOptions)
}

func chatMsgToTextMsg(msg *types.ChatMessage) *types.MessageMeta {
	return &types.MessageMeta{
		MsgID:       msg.ID,
		SeqID:       msg.Sequence,
		SendTime:    msg.SendTime,
		Role:        msg.Role,
		UserID:      msg.UserID,
		SpaceID:     msg.SpaceID,
		SessionID:   msg.SessionID,
		MessageType: msg.MsgType,
		Message: types.MessageTypeImpl{
			Text: msg.Message,
		},
		Attach:   msg.Attach,
		Complete: msg.Complete,
	}
}

func (l *ChatLogic) StopStream(sessionID string) error {
	err := l.core.Srv().Tower().NewCloseChatStreamSignal(sessionID)
	if err != nil {
		return errors.New("ChatLogic.StopStream.Srv.Tower.NewCloseChatStreamSignal", i18n.ERROR_INTERNAL, err)
	}
	return nil
}
