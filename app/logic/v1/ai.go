package v1

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/samber/lo"
	"github.com/sashabaranov/go-openai"

	"github.com/quka-ai/quka-ai/app/core"
	"github.com/quka-ai/quka-ai/app/core/srv"
	"github.com/quka-ai/quka-ai/app/logic/v1/process"
	"github.com/quka-ai/quka-ai/pkg/ai"
	"github.com/quka-ai/quka-ai/pkg/ai/agents/butler"
	"github.com/quka-ai/quka-ai/pkg/ai/agents/journal"
	"github.com/quka-ai/quka-ai/pkg/errors"
	"github.com/quka-ai/quka-ai/pkg/i18n"
	"github.com/quka-ai/quka-ai/pkg/safe"
	"github.com/quka-ai/quka-ai/pkg/types"
	"github.com/quka-ai/quka-ai/pkg/types/protocol"
	"github.com/quka-ai/quka-ai/pkg/utils"
)

// handleAssistantMessage 通过ws通知前端开始响应用户请求
func getStreamReceiveFunc(ctx context.Context, core *core.Core, sendedCounter SendedCounter, msg *types.ChatMessage) types.ReceiveFunc {
	imTopic := protocol.GenIMTopic(msg.SessionID)
	return func(message types.MessageContent, progressStatus types.MessageProgress) error {
		assistantStatus := types.WS_EVENT_ASSISTANT_CONTINUE
		switch message.Type() {
		case types.MESSAGE_TYPE_TEXT:
			if msg.Message == "" {
				msg.Message = string(message.Bytes())
			}

			defer sendedCounter.Add(message.Bytes())

			switch progressStatus {
			case types.MESSAGE_PROGRESS_CANCELED:
				assistantStatus = types.WS_EVENT_ASSISTANT_DONE
				if err := core.Store().ChatMessageStore().UpdateMessageCompleteStatus(ctx, msg.SessionID, msg.ID, int32(progressStatus)); err != nil {
					slog.Error("failed to finished ai answer message", slog.String("session_id", msg.SessionID), slog.String("msg_id", msg.ID),
						slog.String("error", err.Error()))
				}
			case types.MESSAGE_PROGRESS_FAILED:
				assistantStatus = types.WS_EVENT_ASSISTANT_FAILED
				if err := core.Store().ChatMessageStore().RewriteMessage(ctx, msg.SpaceID, msg.SessionID, msg.ID, message.Bytes(), int32(progressStatus)); err != nil {
					slog.Error("failed to rewrite ai answer message to db", slog.String("session_id", msg.SessionID), slog.String("msg_id", msg.ID),
						slog.String("error", err.Error()))
					return err
				}
			default:
				// todo retry
				if err := core.Store().ChatMessageStore().AppendMessage(ctx, msg.SpaceID, msg.SessionID, msg.ID, message.Bytes(), int32(progressStatus)); err != nil {
					slog.Error("failed to append ai answer message to db", slog.String("session_id", msg.SessionID), slog.String("msg_id", msg.ID),
						slog.String("error", err.Error()))
					return err
				}
			}

			if err := core.Srv().Tower().PublishStreamMessage(imTopic, assistantStatus, &types.StreamMessage{
				MessageID: msg.ID,
				SessionID: msg.SessionID,
				Message:   string(message.Bytes()),
				StartAt:   sendedCounter.Get(),
				MsgType:   msg.MsgType,
				Complete:  int32(progressStatus),
			}); err != nil {
				slog.Error("failed to publish ai answer", slog.String("imtopic", imTopic), slog.String("error", err.Error()))
				return err
			}

		case types.MESSAGE_TYPE_TOOL_TIPS:
			if err := core.Srv().Tower().PublishStreamMessage(imTopic, assistantStatus, &types.StreamMessage{
				MessageID: msg.ID,
				SessionID: msg.SessionID,
				ToolTips:  message.Bytes(),
				StartAt:   sendedCounter.Get(),
				MsgType:   types.MESSAGE_TYPE_TOOL_TIPS,
				Complete:  int32(progressStatus),
			}); err != nil {
				slog.Error("failed to publish ai answer", slog.String("imtopic", imTopic), slog.String("error", err.Error()))
				return err
			}

		default:
			slog.Error("unknown message type", slog.Int("message_type", int(message.Type())))
			return errors.New("unknown message type", i18n.ERROR_INTERNAL, fmt.Errorf("unknown message type: %d", message.Type()))
		}

		return nil
	}
}

// handleAssistantMessage 通过ws通知前端智能助理完成用户请求
func getStreamDoneFunc(ctx context.Context, core *core.Core, strCounter SendedCounter, msg *types.ChatMessage, callback func(msg *types.ChatMessage)) types.DoneFunc {
	imTopic := protocol.GenIMTopic(msg.SessionID)
	return func(err error) error {
		// todo retry
		assistantStatus := types.WS_EVENT_ASSISTANT_DONE
		completeStatus := types.MESSAGE_PROGRESS_COMPLETE
		message := ""

		if err != nil {
			if err == context.Canceled {
				assistantStatus = types.WS_EVENT_ASSISTANT_DONE
				completeStatus = types.MESSAGE_PROGRESS_CANCELED

				if err := core.Store().ChatMessageStore().UpdateMessageCompleteStatus(ctx, msg.SessionID, msg.ID, int32(completeStatus)); err != nil {
					slog.Error("failed to finished assistant answer message", slog.String("session_id", msg.SessionID), slog.String("msg_id", msg.ID),
						slog.String("error", err.Error()))
					return err
				}

				if callback != nil {
					callback(msg)
				}
			}
		} else {
			if strCounter.Get() == 0 {
				message = types.AssistantFailedMessage
				assistantStatus = types.WS_EVENT_ASSISTANT_FAILED
				completeStatus = types.MESSAGE_PROGRESS_FAILED
				slog.Error("assistant response is empty, will delete assistant answer message", slog.String("session_id", msg.SessionID), slog.String("msg_id", msg.ID))
				// 返回了0个字符就完成的情况一般是assistant服务异常，无响应，服务端删除该消息，避免数据库存在空记录
				// if err := core.Store().ChatMessageStore().DeleteMessage(ctx, msg.ID); err != nil {
				// 	slog.Error("failed to delete assistant answer message", slog.String("session_id", msg.SessionID), slog.String("msg_id", msg.ID),
				// 		slog.String("error", err.Error()))
				// 	return err
				// }
			} else {
				if err := core.Store().ChatMessageStore().UpdateMessageCompleteStatus(ctx, msg.SessionID, msg.ID, int32(types.MESSAGE_PROGRESS_COMPLETE)); err != nil {
					slog.Error("failed to finished assistant answer message", slog.String("session_id", msg.SessionID), slog.String("msg_id", msg.ID),
						slog.String("error", err.Error()))
					return err
				}

				if callback != nil {
					callback(msg)
				}
			}
		}

		if err := core.Srv().Tower().PublishStreamMessage(imTopic, assistantStatus, &types.StreamMessage{
			MessageID: msg.ID,
			SessionID: msg.SessionID,
			Complete:  int32(completeStatus),
			MsgType:   msg.MsgType,
			Message:   message,
			StartAt:   strCounter.Get(),
		}); err != nil {
			slog.Error("failed to publish AI answer", slog.String("imtopic", imTopic), slog.String("error", err.Error()))
			return err
		}
		return nil
	}
}

// protocol.GenIMTopic(msg.SessionID)
func DefaultMessager(topic string, tower *srv.Tower) types.Messager {
	return &FireTowerMessager{
		topic: topic,
		tower: tower,
	}
}

type FireTowerMessager struct {
	topic string
	tower *srv.Tower
}

func (s *FireTowerMessager) PublishMessage(_type types.WsEventType, data any) error {
	switch _type {
	case types.WS_EVENT_ASSISTANT_INIT:
		if err := s.tower.PublishStreamMessageWithSubject(s.topic, "on_message_init", types.WS_EVENT_ASSISTANT_INIT, data); err != nil {
			slog.Error("failed to publish ai message builded event", slog.String("im_topic", s.topic), slog.String("error", err.Error()))
			return err
		}
	case types.WS_EVENT_ASSISTANT_CONTINUE:
		if err := s.tower.PublishStreamMessageWithSubject(s.topic, "on_message", types.WS_EVENT_ASSISTANT_INIT, data); err != nil {
			slog.Error("failed to publish ai message stream", slog.String("im_topic", s.topic), slog.String("error", err.Error()))
			return err
		}
	default:
	}
	return nil
}

func handleAndNotifyAssistantFailed(core *core.Core, receiver types.Receiver, reqMessage *types.ChatMessage, err error) error {
	slog.Error("Failed to request AI", slog.String("error", err.Error()), slog.String("message_id", reqMessage.ID))
	content := types.AssistantFailedMessage
	completeStatus := types.MESSAGE_PROGRESS_FAILED
	if err == context.Canceled { // 用户手动终止 会关闭上下文
		completeStatus = types.MESSAGE_PROGRESS_CANCELED
		content = ""
	}

	receiveFunc := receiver.GetReceiveFunc()
	return receiveFunc(&types.TextMessage{Text: content}, completeStatus)
	// if err := core.Srv().Tower().PublishStreamMessage(imTopic, types.WS_EVENT_ASSISTANT_FAILED, &types.StreamMessage{
	// 	MessageID: aiMessage.ID,
	// 	SessionID: aiMessage.SessionID,
	// 	Complete:  int32(completeStatus),
	// 	MsgType:   aiMessage.MsgType,
	// 	Message:   content,
	// }); err != nil {
	// 	slog.Error("failed to publish gpt answer", slog.String("imtopic", imTopic), slog.String("error", err.Error()))
	// 	return err
	// }
	// return nil
}

// requestAI
func requestAI(ctx context.Context, core *core.Core, isStream bool, sessionContext *SessionContext, marks map[string]string, receiveFunc types.ReceiveFunc, done types.DoneFunc) error {
	// slog.Debug("request to ai", slog.Any("context", sessionContext.MessageContext), slog.String("prompt", sessionContext.Prompt))
	requestCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	tool := core.Srv().AI().NewQuery(requestCtx, sessionContext.MessageContext)

	if sessionContext.Prompt == "" {
		sessionContext.Prompt = core.Cfg().Prompt.Base
	}
	tool.WithPrompt(sessionContext.Prompt)

	if !isStream {
		msg, err := tool.Query()
		if err != nil {
			return err
		}

		if msg.Usage != nil {
			process.NewRecordChatUsageRequest(msg.Model, types.USAGE_SUB_TYPE_CHAT, sessionContext.MessageID, msg.Usage)
		}
		content := msg.Message()

		defer done(nil)
		return receiveFunc(&types.TextMessage{Text: content}, types.MESSAGE_PROGRESS_COMPLETE)
	}

	resp, err := tool.QueryStream()
	if err != nil {
		return err
	}

	respChan, err := ai.HandleAIStream(requestCtx, resp, marks)
	if err != nil {
		return errors.New("requestAI.HandleAIStream", i18n.ERROR_INTERNAL, err)
	}

	// 3. handle response
	for {
		select {
		case <-ctx.Done():
			if done != nil {
				done(ctx.Err())
			}
			return ctx.Err()
		case msg, ok := <-respChan:
			if !ok {
				return nil
			}
			if msg.Error != nil {
				return err
			}

			if msg.Message != "" {
				if err := receiveFunc(&types.TextMessage{Text: msg.Message}, types.MESSAGE_PROGRESS_GENERATING); err != nil {
					return errors.New("ChatGPTLogic.RequestChatGPT.for.respChan.receive", i18n.ERROR_INTERNAL, err)
				}
			}

			// slog.Debug("got ai response", slog.Any("msg", msg), slog.Bool("status", ok))
			if msg.FinishReason != "" {
				if err = done(nil); err != nil {
					slog.Error("Failed to set message done", slog.String("error", err.Error()), slog.String("msg_id", msg.ID))
				}
				if msg.FinishReason != "" && msg.FinishReason != "stop" {
					slog.Error("AI srv unexpected exit", slog.String("error", msg.FinishReason), slog.String("id", msg.ID))
					return errors.New("requestAI.Srv.AI.Query", i18n.ERROR_INTERNAL, fmt.Errorf("%s", msg.FinishReason))
				}
			}

			if msg.Usage != nil {
				process.NewRecordChatUsageRequest(msg.Model, types.USAGE_SUB_TYPE_CHAT, sessionContext.MessageID, msg.Usage)
				return nil
			}
		}
	}
}

func NewNormalAssistant(core *core.Core, agentType string) *NormalAssistant {
	return &NormalAssistant{
		core:      core,
		agentType: agentType,
	}
}

type NormalAssistant struct {
	core      *core.Core
	agentType string
}

func (s *NormalAssistant) InitAssistantMessage(ctx context.Context, msgID string, seqID int64, userReqMessage *types.ChatMessage, ext types.ChatMessageExt) (*types.ChatMessage, error) {
	// 生成ai响应消息载体的同时，写入关联的内容列表(ext)
	return initAssistantMessage(ctx, s.core, msgID, seqID, userReqMessage, ext)
}

// GenSessionContext 生成session上下文
func (s *NormalAssistant) GenSessionContext(ctx context.Context, prompt string, reqMsgWithDocs *types.ChatMessage) (*SessionContext, error) {
	// latency := s.core.Metrics().GenContextTimer("GenChatSessionContext")
	// defer latency.ObserveDuration()
	return GenChatSessionContextAndSummaryIfExceedsTokenLimit(ctx, s.core, prompt, reqMsgWithDocs, normalGenMessageCondition, types.GEN_CONTEXT)
}

func NewChatReceiver(ctx context.Context, core *core.Core, msger types.Messager) types.Receiver {
	return &ChatReceiveHandler{
		ctx:           ctx,
		core:          core,
		Messager:      msger,
		sendedCounter: &sendedCounter{},
	}
}

type SendedCounter interface {
	Add(n []byte)
	Get() int
}

type sendedCounter struct {
	counter int
}

func (s *sendedCounter) Add(n []byte) {
	s.counter += len([]rune(string(n)))
}

func (s *sendedCounter) Get() int {
	return s.counter
}

type ChatReceiveHandler struct {
	ctx  context.Context
	core *core.Core
	types.Messager
	receiveMsg    *types.ChatMessage
	sendedCounter *sendedCounter
}

func (s *ChatReceiveHandler) IsStream() bool {
	return true
}

func (s *ChatReceiveHandler) RecvMessageInit(userReqMsg *types.ChatMessage, msgID string, seqID int64, ext types.ChatMessageExt) error {
	ctx, cancel := context.WithTimeout(s.ctx, time.Second*5)
	defer cancel()
	var err error
	s.receiveMsg, err = initAssistantMessage(ctx, s.core, msgID, seqID, userReqMsg, ext)
	if err != nil {
		return err
	}
	return s.PublishMessage(types.WS_EVENT_ASSISTANT_INIT, chatMsgToTextMsg(s.receiveMsg))
}

func (s *ChatReceiveHandler) GetReceiveFunc() types.ReceiveFunc {
	return getStreamReceiveFunc(s.ctx, s.core, s.sendedCounter, s.receiveMsg)
}

func (s *ChatReceiveHandler) GetDoneFunc(callback func(msg *types.ChatMessage)) types.DoneFunc {
	return getStreamDoneFunc(s.ctx, s.core, s.sendedCounter, s.receiveMsg, callback)
}

func NewQueryReceiver(ctx context.Context, core *core.Core, responseChan chan types.MessageContent) types.Receiver {
	return &QueryReceiveHandler{
		ctx:          ctx,
		core:         core,
		resp:         responseChan,
		sendedLength: 0,
	}
}

type QueryReceiveHandler struct {
	ctx          context.Context
	core         *core.Core
	resp         chan types.MessageContent
	sendedLength int64
}

func (s *QueryReceiveHandler) IsStream() bool {
	return false
}

func (s *QueryReceiveHandler) RecvMessageInit(userReqMsg *types.ChatMessage, msgID string, seqID int64, ext types.ChatMessageExt) error {
	return nil
}

func (s *QueryReceiveHandler) GetReceiveFunc() types.ReceiveFunc {
	return func(message types.MessageContent, _ types.MessageProgress) error {
		select {
		case <-s.ctx.Done():
			return s.ctx.Err()
		case s.resp <- message:
		}
		return nil
	}
}

func (s *QueryReceiveHandler) GetDoneFunc(callback func(receiveMsg *types.ChatMessage)) types.DoneFunc {
	return func(err error) error {
		if callback != nil {
			callback(nil)
		}
		return nil
	}
}

// RequestAssistant 向智能助理发起请求
// reqMsgInfo 用户请求的内容
// recvMsgInfo 用于承载ai回复的内容，会预先在数据库中为ai响应的数据创建出对应的记录
func (s *NormalAssistant) RequestAssistant(ctx context.Context, docs types.RAGDocs, reqMsg *types.ChatMessage, receiver types.Receiver) error {
	space, err := s.core.Store().SpaceStore().GetSpace(ctx, reqMsg.SpaceID)
	if err != nil {
		return err
	}

	var prompt string
	if len(docs.Refs) == 0 {
		prompt = lo.If(space.BasePrompt != "", space.BasePrompt).Else(s.core.Prompt().Base)
	} else {
		prompt = lo.If(space.ChatPrompt != "", space.ChatPrompt).Else(s.core.Prompt().Query)
	}
	prompt = ai.BuildRAGPrompt(prompt, ai.NewDocs(docs.Docs), s.core.Srv().AI())

	var (
		sessionContext *SessionContext
	)
	if reqMsg.SessionID != "" {
		sessionContext, err = s.GenSessionContext(ctx, prompt, reqMsg)
		if err != nil {
			return err
		}
	} else {
		var userChatMessage []*types.MessageContext

		if len(reqMsg.Attach) > 0 {
			item := &types.MessageContext{
				Role: types.USER_ROLE_USER,
			}
			item.MultiContent = reqMsg.Attach.ToMultiContent("")
			userChatMessage = append(userChatMessage, item)
		}

		userChatMessage = append(userChatMessage, &types.MessageContext{
			Role:    types.USER_ROLE_USER,
			Content: reqMsg.Message,
		})
		sessionContext = &SessionContext{
			Prompt:         prompt,
			MessageID:      reqMsg.ID,
			MessageContext: userChatMessage,
		}
	}

	for _, v := range sessionContext.MessageContext {
		if len(v.MultiContent) > 0 {
			for i, vv := range v.MultiContent {
				if vv.ImageURL != nil {
					url, err := s.core.FileStorage().GenGetObjectPreSignURL(vv.ImageURL.URL)
					if err != nil {
						return err
					}
					v.MultiContent[i].ImageURL.URL = url
				}
			}
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*3)
	defer cancel()

	receiveFunc := receiver.GetReceiveFunc()
	// receiveFunc := getStreamReceiveFunc(ctx, s.core, recvMsgInfo)
	doneFunc := receiver.GetDoneFunc(func(recvMsgInfo *types.ChatMessage) {
		if recvMsgInfo == nil {
			return
		}
		// set chat session pin
		go safe.Run(func() {
			switch s.agentType {
			case types.AGENT_TYPE_NORMAL:
				if len(docs.Refs) == 0 {
					return
				}
				if err := createChatSessionKnowledgePin(s.core, recvMsgInfo, &docs); err != nil {
					slog.Error("Failed to create chat session knowledge pins", slog.String("session_id", recvMsgInfo.SessionID), slog.String("error", err.Error()))
				}
			default:
			}
		})
	})

	marks := make(map[string]string)
	for _, v := range docs.Docs {
		if v.SW == nil {
			continue
		}
		for fake, real := range v.SW.Map() {
			marks[fake] = real
		}
	}

	if err = requestAI(ctx, s.core, receiver.IsStream(), sessionContext, marks, receiveFunc, doneFunc); err != nil {
		slog.Error("NormalAssistant: failed to request AI", slog.String("error", err.Error()))
		return handleAndNotifyAssistantFailed(s.core, receiver, reqMsg, err)
	}
	return nil
}

func NewBulterAssistant(core *core.Core, agentType string) *ButlerAssistant {
	cfg := openai.DefaultConfig(core.Cfg().AI.Agent.Token)
	cfg.BaseURL = core.Cfg().AI.Agent.Endpoint

	cli := openai.NewClientWithConfig(cfg)
	return &ButlerAssistant{
		core:      core,
		agentType: agentType,
		client:    butler.NewButlerAgent(core, cli, core.Cfg().AI.Agent.Model, core.Cfg().AI.Agent.VlModel),
	}
}

type ButlerAssistant struct {
	core      *core.Core
	agentType string
	client    *butler.ButlerAgent
}

func (s *ButlerAssistant) InitAssistantMessage(ctx context.Context, msgID string, seqID int64, userReqMessage *types.ChatMessage, ext types.ChatMessageExt) (*types.ChatMessage, error) {
	// 生成ai响应消息载体的同时，写入关联的内容列表(ext)
	return initAssistantMessage(ctx, s.core, msgID, seqID, userReqMessage, ext)
}

// GenSessionContext 生成session上下文
func (s *ButlerAssistant) GenSessionContext(ctx context.Context, prompt string, reqMsgWithDocs *types.ChatMessage) (*SessionContext, error) {
	// latency := s.core.Metrics().GenContextTimer("GenChatSessionContext")
	// defer latency.ObserveDuration()
	return GenChatSessionContextAndSummaryIfExceedsTokenLimit(ctx, s.core, prompt, reqMsgWithDocs, normalGenMessageCondition, types.GEN_CONTEXT)
}

// RequestAssistant 向智能助理发起请求
// reqMsgInfo 用户请求的内容
// recvMsgInfo 用于承载ai回复的内容，会预先在数据库中为ai响应的数据创建出对应的记录
func (s *ButlerAssistant) RequestAssistant(ctx context.Context, docs types.RAGDocs, reqMsg *types.ChatMessage, receiver types.Receiver) error {
	nextReq, usages, err := s.client.Query(reqMsg.UserID, reqMsg)
	if err != nil {
		return handleAndNotifyAssistantFailed(s.core, receiver, reqMsg, err)
	}

	for _, v := range usages {
		process.NewRecordUsageRequest(s.client.Model, "Agents", "Butler", reqMsg.SpaceID, reqMsg.UserID, v)
	}

	// receiveFunc := getStreamReceiveFunc(ctx, s.core, recvMsgInfo)
	receiveFunc := receiver.GetReceiveFunc()
	doneFunc := receiver.GetDoneFunc(nil)

	if len(nextReq) == 1 {
		defer doneFunc(nil)
		return receiveFunc(&types.TextMessage{Text: nextReq[0].Content}, types.MESSAGE_PROGRESS_COMPLETE)
	}

	chatSessionContext := &SessionContext{
		MessageID: reqMsg.ID,
		SessionID: reqMsg.SessionID,
		MessageContext: lo.Map(nextReq, func(item openai.ChatCompletionMessage, _ int) *types.MessageContext {
			return &types.MessageContext{
				Role:         types.GetMessageUserRole(item.Role),
				Content:      item.Content,
				MultiContent: item.MultiContent,
			}
		}),
		Prompt: butler.BuildButlerPrompt("", s.core.Srv().AI()),
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*3)
	defer cancel()

	// doneFunc := getStreamDoneFunc(ctx, s.core, recvMsgInfo, nil)

	marks := make(map[string]string)
	for _, v := range docs.Docs {
		for fake, real := range v.SW.Map() {
			marks[fake] = real
		}
	}

	if err = requestAI(ctx, s.core, receiver.IsStream(), chatSessionContext, marks, receiveFunc, doneFunc); err != nil {
		slog.Error("failed to request AI", slog.String("error", err.Error()))
		return handleAndNotifyAssistantFailed(s.core, receiver, reqMsg, err)
	}
	return nil
}

func NewJournalAssistant(core *core.Core, agentType string) *JournalAssistant {
	cfg := openai.DefaultConfig(core.Cfg().AI.Agent.Token)
	cfg.BaseURL = core.Cfg().AI.Agent.Endpoint

	cli := openai.NewClientWithConfig(cfg)
	return &JournalAssistant{
		core:      core,
		agentType: agentType,
		agent:     journal.NewJournalAgent(core, cli, core.Cfg().AI.Agent.Model),
	}
}

type JournalAssistant struct {
	core      *core.Core
	agentType string
	agent     *journal.JournalAgent
}

func (s *JournalAssistant) InitAssistantMessage(ctx context.Context, msgID string, seqID int64, userReqMessage *types.ChatMessage, ext types.ChatMessageExt) (*types.ChatMessage, error) {
	// 生成ai响应消息载体的同时，写入关联的内容列表(ext)
	return initAssistantMessage(ctx, s.core, msgID, seqID, userReqMessage, ext)
}

// GenSessionContext 生成session上下文
func (s *JournalAssistant) GenSessionContext(ctx context.Context, prompt string, reqMsgWithDocs *types.ChatMessage) (*SessionContext, error) {
	// latency := s.core.Metrics().GenContextTimer("GenChatSessionContext")
	// defer latency.ObserveDuration()
	return GenChatSessionContextAndSummaryIfExceedsTokenLimit(ctx, s.core, prompt, reqMsgWithDocs, normalGenMessageCondition, types.GEN_CONTEXT)
}

// RequestAssistant 向智能助理发起请求
// reqMsgInfo 用户请求的内容
// recvMsgInfo 用于承载ai回复的内容，会预先在数据库中为ai响应的数据创建出对应的记录
// 添加全局递归深度计数器
var (
	recursionCounter  = make(map[string]int)
	maxRecursionDepth = 5 // 最大递归深度
)

func (s *JournalAssistant) RequestAssistant(ctx context.Context, docs types.RAGDocs, reqMsg *types.ChatMessage, receiver types.Receiver) error {
	userChatMessage := openai.ChatCompletionMessage{
		Role: types.USER_ROLE_USER.String(),
	}
	if len(reqMsg.Attach) > 0 {
		userChatMessage.MultiContent = reqMsg.Attach.ToMultiContent(reqMsg.Message)
	} else {
		userChatMessage.Content = reqMsg.Message
	}
	baseReq := []openai.ChatCompletionMessage{
		{
			Role:    types.USER_ROLE_SYSTEM.String(),
			Content: journal.BuildJournalPrompt("", s.core.Srv().AI()),
		},
		userChatMessage,
	}

	receiveFunc := receiver.GetReceiveFunc()
	// receiveFunc := getStreamReceiveFunc(ctx, s.core, recvMsgInfo)
	doneFunc := receiver.GetDoneFunc(nil)
	// doneFunc := getStreamDoneFunc(ctx, s.core, recvMsgInfo, nil)

	toolID := utils.GenUniqIDStr()
	receiveFunc(&types.ToolTips{
		ID:       toolID,
		ToolName: "SearchJournal",
		Status:   types.TOOL_STATUS_RUNNING,
		Content:  "Searching your journals...",
	}, types.MESSAGE_PROGRESS_GENERATING)

	once := sync.Once{}
	actualReceiveFunc := func(message types.MessageContent, progressStatus types.MessageProgress) error {
		once.Do(func() {
			receiveFunc(&types.ToolTips{
				ID:       toolID,
				ToolName: "SearchJournal",
				Status:   types.TOOL_STATUS_SUCCESS,
				Content:  "Searching your journals done",
			}, types.MESSAGE_PROGRESS_GENERATING)
		})
		return receiveFunc(message, progressStatus)
	}

	nextReq, usage, err := s.agent.HandleUserRequest(ctx, reqMsg.SpaceID, reqMsg.UserID, baseReq, receiveFunc)
	if err != nil {
		return handleAndNotifyAssistantFailed(s.core, receiver, reqMsg, err)
	}

	if len(nextReq) == 0 {
		nextReq = baseReq
	}

	if usage != nil {
		process.NewRecordUsageRequest(s.agent.Model, "Agents", "Journal", reqMsg.SpaceID, reqMsg.UserID, usage)
	}

	chatSessionContext := &SessionContext{
		MessageID: reqMsg.ID,
		SessionID: reqMsg.SessionID,
		MessageContext: lo.Map(nextReq, func(item openai.ChatCompletionMessage, _ int) *types.MessageContext {
			return &types.MessageContext{
				Role:         types.GetMessageUserRole(item.Role),
				Content:      item.Content,
				MultiContent: item.MultiContent,
			}
		}),
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*3)
	defer cancel()

	marks := make(map[string]string)
	for _, v := range docs.Docs {
		for fake, real := range v.SW.Map() {
			marks[fake] = real
		}
	}

	if err = requestAI(ctx, s.core, receiver.IsStream(), chatSessionContext, marks, actualReceiveFunc, doneFunc); err != nil {
		slog.Error("Journal: failed to request AI", slog.String("error", err.Error()))
		return handleAndNotifyAssistantFailed(s.core, receiver, reqMsg, err)
	}
	return nil
}

// createChatSessionKnowledgePin Create this chat session prompt pin docs
func createChatSessionKnowledgePin(core *core.Core, recvMsgInfo *types.ChatMessage, docs *types.RAGDocs) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*6)
	defer cancel()
	msg, err := core.Store().ChatMessageStore().GetOne(ctx, recvMsgInfo.ID)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if msg == nil {
		return nil
	}

	var pinDocs []string
	for _, v := range docs.Refs {
		if strings.Contains(msg.Message, v.KnowledgeID) {
			pinDocs = append(pinDocs, v.KnowledgeID)
		}
	}

	if len(pinDocs) == 0 {
		return nil
	}

	var p types.ContentPinV1

	pin, err := core.Store().ChatSessionPinStore().GetBySessionID(ctx, recvMsgInfo.SessionID)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	if pin == nil {
		pin = &types.ChatSessionPin{
			SessionID: recvMsgInfo.SessionID,
			SpaceID:   recvMsgInfo.SpaceID,
			UserID:    recvMsgInfo.UserID,
			CreatedAt: time.Now().Unix(),
			UpdatedAt: time.Now().Unix(),
			Version:   types.CHAT_SESSION_PIN_VERSION_V1,
		}

		p.Knowledges = append(p.Knowledges, pinDocs...)
		pin.Content, _ = json.Marshal(p)

		if err = core.Store().ChatSessionPinStore().Create(ctx, *pin); err != nil {
			return err
		}
		return nil
	}

	if pin.Version == types.CHAT_SESSION_PIN_VERSION_V1 {
		if err = json.Unmarshal(pin.Content, &p); err != nil {
			return err
		}
	}

	p.Knowledges = append(p.Knowledges, pinDocs...)
	pin.Content, _ = json.Marshal(p)

	if err = core.Store().ChatSessionPinStore().Update(ctx, pin.SessionID, pin.SpaceID, pin.Content, types.CHAT_SESSION_PIN_VERSION_V1); err != nil {
		return err
	}
	return nil

}

func initAssistantMessage(ctx context.Context, core *core.Core, msgID string, seqID int64, userReqMsg *types.ChatMessage, ext types.ChatMessageExt) (*types.ChatMessage, error) {
	// pre-generate response messages
	answerMsg := genUncompleteAIMessage(userReqMsg.SpaceID, userReqMsg.SessionID, msgID, seqID)

	answerMsg.MsgBlock = userReqMsg.MsgBlock
	answerMsg.UserID = userReqMsg.UserID // ai answer message is also belong to user

	var err error
	err = core.Store().Transaction(ctx, func(ctx context.Context) error {
		if err = core.Store().ChatMessageStore().Create(ctx, answerMsg); err != nil {
			slog.Error("failed to insert ai answer message to db", slog.String("msg_id", answerMsg.ID), slog.String("session_id", answerMsg.SessionID), slog.String("error", err.Error()))
			return err
		}

		ext.MessageID = answerMsg.ID

		if err = core.Store().ChatMessageExtStore().Create(ctx, ext); err != nil {
			slog.Error("failed to insert ai answer ext to db", slog.String("msg_id", answerMsg.ID), slog.String("session_id", answerMsg.SessionID), slog.String("error", err.Error()))
			return err
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return answerMsg, nil
}

// generate uncomplete ai response message meta
func genUncompleteAIMessage(spaceID, sessionID, msgID string, seqID int64) *types.ChatMessage {
	return &types.ChatMessage{
		ID:        msgID,
		SpaceID:   spaceID,
		Sequence:  seqID,
		Role:      types.USER_ROLE_ASSISTANT,
		SendTime:  time.Now().Unix(),
		MsgType:   types.MESSAGE_TYPE_TEXT,
		Complete:  types.MESSAGE_PROGRESS_UNCOMPLETE,
		SessionID: sessionID,
	}
}

type messageCondition func(historyMsgID, inputMsgID string) bool

func normalGenMessageCondition(historyMsgID, inputMsgID string) bool {
	return historyMsgID > inputMsgID
}

func reGenMessageCondition(historyMsgID, inputMsgID string) bool {
	return historyMsgID >= inputMsgID
}

func appendSummaryToPromptMsg(msg *types.MessageContext, summary *types.ChatSummary) {
	// Sprintf 是个比较低效的字符串拼接方法，当前量级可以暂且这么做，量级上来以后可以优化到 strings.Builder
	msg.Content = fmt.Sprintf("%s, You will continue the conversation with understanding the context. The following is the context for conversation：{ %s }", msg.Content, summary.Content)
}

func isErrorMessage(msg string) bool {
	msg = strings.TrimSpace(msg)
	if strings.HasPrefix(msg, "Sorry，") || strings.HasPrefix(msg, "抱歉，") || msg == "" {
		return true
	}
	return false
}

// genChatSessionContextAndSummaryIfExceedsTokenLimit 生成gpt请求上下文
func GenChatSessionContextAndSummaryIfExceedsTokenLimit(ctx context.Context, core *core.Core, basePrompt string, reqMsgWithDocs *types.ChatMessage, msgCondition messageCondition, justGenSummary types.SystemContextGenConditionType) (*SessionContext, error) {
	reGen := false

ReGen:
	var reqMsg []*types.MessageContext
	summary, err := core.Store().ChatSummaryStore().GetChatSessionLatestSummary(ctx, reqMsgWithDocs.SessionID)
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.New("genDialogContextAndSummaryIfExceedsTokenLimit.ChatSummaryStore.GetChatSessionLatestSummary", i18n.ERROR_INTERNAL, err)
	}

	if basePrompt != "" {
		reqMsg = append(reqMsg, &types.MessageContext{
			Role:    types.USER_ROLE_SYSTEM,
			Content: basePrompt,
		})

		if summary != nil {
			appendSummaryToPromptMsg(reqMsg[0], summary)
		}
	}

	if summary == nil {
		summary = &types.ChatSummary{}
	}

	// 获取比summary msgid更大的聊天内容组成上下文
	msgList, err := core.Store().ChatMessageStore().ListSessionMessage(ctx, reqMsgWithDocs.SpaceID, reqMsgWithDocs.SessionID, summary.MessageID, types.NO_PAGING, types.NO_PAGING)
	if err != nil {
		return nil, errors.New("genDialogContextAndSummaryIfExceedsTokenLimit.ChatMessageStore.ListSessionMessage", i18n.ERROR_INTERNAL, err)
	}

	// 对消息按msgid进行排序
	sort.Slice(msgList, func(i, j int) bool {
		return msgList[i].ID < msgList[j].ID
	})

	var (
		summaryMessageCutRange int
		summaryMessageID       string
		contextIndex           int
	)

	for _, v := range msgList {
		if v.IsEncrypt == types.MESSAGE_IS_ENCRYPT {
			deData, err := core.DecryptData([]byte(v.Message))
			if err != nil {
				return nil, errors.New("ShareLogenDialogContextAndSummaryIfExceedsTokenLimitgic.ChatMessageStore.DecryptData", i18n.ERROR_INTERNAL, err)
			}

			v.Message = string(deData)
		}

		if isErrorMessage(v.Message) {
			continue
		}

		if v.Complete != types.MESSAGE_PROGRESS_COMPLETE {
			continue
		}

		if msgCondition(v.ID, reqMsgWithDocs.ID) {
			// 当前逻辑回复的是 msgID, 所以上下文中不应该出现晚于 msgID 出现的消息，多人场景会有此情况
			break
		}

		contextIndex++

		if len(v.Attach) > 0 {
			item := &types.MessageContext{
				Role: types.USER_ROLE_USER,
			}
			item.MultiContent = v.Attach.ToMultiContent("")
			reqMsg = append(reqMsg, item)
		}

		reqMsg = append(reqMsg, &types.MessageContext{
			Role:    types.USER_ROLE_USER,
			Content: v.Message,
		})

		// if v.ID == reqMsgWithDocs.ID {
		// 	userChatMessage := &types.MessageContext{
		// 		Role: v.Role,
		// 	}
		// 	if len(reqMsgWithDocs.Attach) > 0 {
		// 		userChatMessage.MultiContent = reqMsgWithDocs.Attach.ToMultiContent(reqMsgWithDocs.Message)
		// 	} else {
		// 		userChatMessage.Content = reqMsgWithDocs.Message
		// 	}

		// 	reqMsg = append(reqMsg, userChatMessage)
		// } else {
		// 	reqMsg = append(reqMsg, &types.MessageContext{
		// 		Role:    v.Role,
		// 		Content: v.Message,
		// 	})
		// }

	}

	if contextIndex > 0 {
		if contextIndex >= 3 { // 如果聊天记录追加超过3条，则在总结前保留最新的三条消息，否则保留最后一条
			summaryMessageCutRange = 3
		} else {
			summaryMessageCutRange = 1
		}
		summaryMessageID = msgList[contextIndex-summaryMessageCutRange].ID
	}

	// 计算token是否超出限额，超出20条记录自动做一次总结
	if len(msgList) > 20 || core.Srv().AI().MsgIsOverLimit(reqMsg) {
		if len(reqMsg) <= 3 || reGen {
			// 表明当前prompt + 总结 + 用户一段对话已经超出 max token
			slog.Warn("the current context token is insufficient", slog.String("session_id", reqMsgWithDocs.SessionID), slog.String("msg_id", reqMsgWithDocs.ID))
			return nil, errors.New("genDialogContextAndSummaryIfExceedsTokenLimit.MessageStore.ListDialogMessage", "the current dialog token is insufficient", err)
		}

		summaryReq := reqMsg[:len(reqMsg)-summaryMessageCutRange]
		if core.Srv().AI().MsgIsOverLimit(summaryReq) {
			// 历史数据迁移可能导致某些用户的历史聊天记录过大，无法生成总结，若超出limit，则每次删除第一条消息(prompt后的第一条消息，故索引为1)
			for {
				summaryReq = lo.Drop(summaryReq, 1)
				if !core.Srv().AI().MsgIsOverLimit(summaryReq) {
					break
				}
			}
		}

		reGen = true
		// 生成新的总结
		if err = genChatSessionContextSummary(ctx, core, reqMsgWithDocs.SessionID, summaryMessageID, summaryReq); err != nil {
			return nil, errors.Trace("genDialogContextAndSummaryIfExceedsTokenLimit.genDialogContextSummary", err)
		}
		if justGenSummary == types.GEN_SUMMARY_ONLY {
			return nil, nil
		}
		goto ReGen
	}
	return &SessionContext{
		Prompt:         basePrompt,
		MessageID:      reqMsgWithDocs.ID,
		SessionID:      reqMsgWithDocs.SessionID,
		MessageContext: reqMsg,
	}, nil
}

type SessionContext struct {
	MessageID      string
	SessionID      string
	MessageContext []*types.MessageContext
	Prompt         string
	Tempature      *float32
}

// genChatSessionContextSummary 生成dialog上下文总结
func genChatSessionContextSummary(ctx context.Context, core *core.Core, sessionID, summaryMessageID string, reqMsg []*types.MessageContext) error {
	slog.Debug("start generating context summary", slog.String("session_id", sessionID), slog.String("msg_id", summaryMessageID), slog.Any("request_message", reqMsg))
	prompt := core.Cfg().Prompt.ChatSummary
	if prompt == "" {
		prompt = ai.PROMPT_SUMMARY_DEFAULT_EN
	}

	queryOpts := core.Srv().AI().NewQuery(ctx, reqMsg)
	queryOpts.WithPrompt(prompt)

	// 总结仍然使用v3来生成
	resp, err := queryOpts.Query()
	if err != nil || len(resp.Received) == 0 {
		slog.Error("failed to generate dialog context summary", slog.String("error", err.Error()), slog.Any("response", resp))
		return errors.New("genDialogContextSummary.gptSrv.Chat", i18n.ERROR_INTERNAL, err)
	}

	if len(resp.Received) > 1 {
		slog.Warn("chat method response multi line content", slog.Any("response", resp))
	}

	if err = core.Store().ChatSummaryStore().Create(ctx, types.ChatSummary{
		ID:        utils.GenSpecIDStr(),
		SessionID: sessionID,
		MessageID: summaryMessageID,
		Content:   resp.Received[0],
	}); err != nil {
		return errors.New("genDialogContextSummary.ChatSummaryStore.Create", i18n.ERROR_INTERNAL, err)
	}
	slog.Debug("succeed to generate summary", slog.String("session_id", sessionID), slog.String("msg_id", summaryMessageID))
	return nil
}
