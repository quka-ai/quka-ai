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

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"
	"github.com/samber/lo"
	"github.com/sashabaranov/go-openai"

	"github.com/quka-ai/quka-ai/app/core"
	"github.com/quka-ai/quka-ai/app/core/srv"
	"github.com/quka-ai/quka-ai/pkg/ai"
	"github.com/quka-ai/quka-ai/pkg/ai/agents/butler"
	"github.com/quka-ai/quka-ai/pkg/ai/agents/journal"
	"github.com/quka-ai/quka-ai/pkg/errors"
	"github.com/quka-ai/quka-ai/pkg/i18n"
	"github.com/quka-ai/quka-ai/pkg/mark"
	"github.com/quka-ai/quka-ai/pkg/types"
	"github.com/quka-ai/quka-ai/pkg/types/protocol"
	"github.com/quka-ai/quka-ai/pkg/utils"
)

func nopReceiveFunc(message types.MessageContent, progressStatus types.MessageProgress) error {
	return nil
}

func nopDoneFunc(_ error) error {
	return nil
}

// handleAssistantMessage 通过ws通知前端开始响应用户请求
func getStreamReceiveFunc(ctx context.Context, core *core.Core, sendedCounter SendedCounter, msg *types.ChatMessage) types.ReceiveFunc {
	if msg == nil {
		return nopReceiveFunc
	}
	imTopic := protocol.GenIMTopic(msg.SpaceID, msg.SessionID)
	return func(message types.MessageContent, progressStatus types.MessageProgress) error {
		defer sendedCounter.Add(message.Bytes())
		msg.Message += string(message.Bytes())
		switch message.Type() {
		case types.MESSAGE_TYPE_TEXT:
			assistantStatus := types.WS_EVENT_ASSISTANT_CONTINUE
			if err := core.Srv().Centrifuge().PublishStreamMessage(imTopic, assistantStatus, &types.StreamMessage{
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

			switch progressStatus {
			case types.MESSAGE_PROGRESS_CANCELED:
				assistantStatus = types.WS_EVENT_ASSISTANT_DONE
				if err := core.Store().ChatMessageStore().UpdateMessageCompleteStatus(ctx, msg.SessionID, msg.ID, progressStatus); err != nil {
					slog.Error("failed to finished ai answer message", slog.String("session_id", msg.SessionID), slog.String("msg_id", msg.ID),
						slog.String("error", err.Error()))
				}
			case types.MESSAGE_PROGRESS_FAILED:
				assistantStatus = types.WS_EVENT_ASSISTANT_FAILED
				if err := core.Store().ChatMessageStore().RewriteMessage(ctx, msg.SpaceID, msg.SessionID, msg.ID, message.Bytes(), progressStatus); err != nil {
					slog.Error("failed to rewrite ai answer message to db", slog.String("session_id", msg.SessionID), slog.String("msg_id", msg.ID),
						slog.String("error", err.Error()))
					return err
				}
			default:
				// todo retry
				raw := message.Bytes()
				if len(raw) == 0 {
					break
				}
				if err := core.Store().ChatMessageStore().AppendMessage(ctx, msg.SpaceID, msg.SessionID, msg.ID, raw, progressStatus); err != nil {
					slog.Error("failed to append ai answer message to db", slog.String("session_id", msg.SessionID), slog.String("msg_id", msg.ID),
						slog.String("error", err.Error()))
					return err
				}
			}

		case types.MESSAGE_TYPE_TOOL_TIPS:
			assistantStatus := types.WS_EVENT_TOOL_CONTINUE
			if err := core.Srv().Centrifuge().PublishStreamMessage(imTopic, assistantStatus, &types.StreamMessage{
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

			switch progressStatus {
			case types.MESSAGE_PROGRESS_COMPLETE:
				assistantStatus = types.WS_EVENT_TOOL_DONE
				if err := core.Store().ChatMessageStore().RewriteMessage(ctx, msg.SpaceID, msg.SessionID, msg.ID, message.Bytes(), progressStatus); err != nil {
					slog.Error("failed to finished ai answer message", slog.String("session_id", msg.SessionID), slog.String("msg_id", msg.ID),
						slog.String("error", err.Error()))
				}
			case types.MESSAGE_PROGRESS_CANCELED:
				assistantStatus = types.WS_EVENT_TOOL_DONE
				if err := core.Store().ChatMessageStore().UpdateMessageCompleteStatus(ctx, msg.SessionID, msg.ID, progressStatus); err != nil {
					slog.Error("failed to finished ai answer message", slog.String("session_id", msg.SessionID), slog.String("msg_id", msg.ID),
						slog.String("error", err.Error()))
				}
			case types.MESSAGE_PROGRESS_FAILED:
				assistantStatus = types.WS_EVENT_TOOL_FAILED
				if err := core.Store().ChatMessageStore().RewriteMessage(ctx, msg.SpaceID, msg.SessionID, msg.ID, message.Bytes(), progressStatus); err != nil {
					slog.Error("failed to rewrite ai answer message to db", slog.String("session_id", msg.SessionID), slog.String("msg_id", msg.ID),
						slog.String("error", err.Error()))
					return err
				}
			default:
				// todo retry
				raw := message.Bytes()
				if len(raw) == 0 {
					break
				}
				if err := core.Store().ChatMessageStore().RewriteMessage(ctx, msg.SpaceID, msg.SessionID, msg.ID, raw, progressStatus); err != nil {
					slog.Error("failed to append ai answer message to db", slog.String("session_id", msg.SessionID), slog.String("msg_id", msg.ID),
						slog.String("error", err.Error()))
					return err
				}
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
	if msg == nil {
		return nopDoneFunc
	}
	imTopic := protocol.GenIMTopic(msg.SpaceID, msg.SessionID)
	once := sync.Once{}
	handler := func(err error) error {
		// todo retry
		assistantStatus := lo.If(msg.MsgType == types.MESSAGE_TYPE_TOOL_TIPS, types.WS_EVENT_TOOL_DONE).Else(types.WS_EVENT_ASSISTANT_DONE)
		completeStatus := types.MESSAGE_PROGRESS_COMPLETE
		message := ""

		if err != nil {
			if err == context.Canceled {
				completeStatus = types.MESSAGE_PROGRESS_CANCELED
			} else {
				completeStatus = types.MESSAGE_PROGRESS_FAILED
				assistantStatus = lo.If(msg.MsgType == types.MESSAGE_TYPE_TOOL_TIPS, types.WS_EVENT_TOOL_FAILED).Else(types.WS_EVENT_ASSISTANT_FAILED)
			}

			if err := core.Store().ChatMessageStore().UpdateMessageCompleteStatus(ctx, msg.SessionID, msg.ID, completeStatus); err != nil {
				slog.Error("failed to finished assistant answer message", slog.String("session_id", msg.SessionID), slog.String("msg_id", msg.ID),
					slog.String("error", err.Error()))
				return err
			}

			if callback != nil {
				callback(msg)
			}
		} else {
			if strCounter.Get() == 0 {
				message = types.AssistantFailedMessage
				assistantStatus = lo.If(msg.MsgType == types.MESSAGE_TYPE_TOOL_TIPS, types.WS_EVENT_TOOL_FAILED).Else(types.WS_EVENT_ASSISTANT_FAILED)
				completeStatus = types.MESSAGE_PROGRESS_FAILED
				slog.Error("assistant response is empty, will delete assistant answer message", slog.String("session_id", msg.SessionID), slog.String("msg_id", msg.ID))
				// 返回了0个字符就完成的情况一般是assistant服务异常，无响应，服务端删除该消息，避免数据库存在空记录
				if err := core.Store().ChatMessageStore().DeleteMessage(ctx, msg.ID); err != nil {
					slog.Error("failed to delete assistant answer message", slog.String("session_id", msg.SessionID), slog.String("msg_id", msg.ID),
						slog.String("error", err.Error()))
					return err
				}
			} else {
				if err := core.Store().ChatMessageStore().UpdateMessageCompleteStatus(ctx, msg.SessionID, msg.ID, types.MESSAGE_PROGRESS_COMPLETE); err != nil {
					slog.Error("failed to finished assistant answer message", slog.String("session_id", msg.SessionID), slog.String("msg_id", msg.ID),
						slog.String("error", err.Error()))
					return err
				}

				if callback != nil {
					callback(msg)
				}
			}
		}

		if err := core.Srv().Centrifuge().PublishStreamMessage(imTopic, assistantStatus, &types.StreamMessage{
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

	return func(err error) error {
		var handlerResult error
		once.Do(func() {
			handlerResult = handler(err)
		})
		return handlerResult
	}
}

// protocol.GenIMTopic(msg.SessionID)
func DefaultMessager(topic string, centrifuge srv.CentrifugeManager) types.Messager {
	return &CentrifugeMessager{
		topic:      topic,
		centrifuge: centrifuge,
	}
}

type CentrifugeMessager struct {
	topic      string
	centrifuge srv.CentrifugeManager
}

func (s *CentrifugeMessager) PublishMessage(_type types.WsEventType, data any) error {
	switch _type {
	case types.WS_EVENT_ASSISTANT_INIT:
		fallthrough
	case types.WS_EVENT_TOOL_INIT:
		if err := s.centrifuge.PublishStreamMessageWithSubject(s.topic, "on_message_init", _type, data); err != nil {
			slog.Error("failed to publish ai message builded event", slog.String("im_topic", s.topic), slog.String("error", err.Error()))
			return err
		}
	case types.WS_EVENT_ASSISTANT_CONTINUE:
		fallthrough
	case types.WS_EVENT_TOOL_CONTINUE:
		if err := s.centrifuge.PublishStreamMessageWithSubject(s.topic, "on_message", _type, data); err != nil {
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
	// if err := core.Srv().Centrifuge().PublishStreamMessage(imTopic, types.WS_EVENT_ASSISTANT_FAILED, &types.StreamMessage{
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

func NewChatReceiver(ctx context.Context, core *core.Core, msger types.Messager, reqMessage *types.ChatMessage) types.Receiver {
	return &ChatReceiveHandler{
		ctx:           ctx,
		core:          core,
		Messager:      msger,
		userReqMsg:    reqMessage,
		messageID:     core.GenMessageID(),
		sendedCounter: &sendedCounter{},

		varHandler: mark.NewSensitiveWork(),
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

	messageID     string
	userReqMsg    *types.ChatMessage
	receiveMsg    *types.ChatMessage
	sendedCounter *sendedCounter

	varHandler mark.VariableHandler
}

func (s *ChatReceiveHandler) IsStream() bool {
	return true
}

func (s *ChatReceiveHandler) MessageID() string {
	return s.messageID
}

func (s *ChatReceiveHandler) VariableHandler() mark.VariableHandler {
	return s.varHandler
}

func (s *ChatReceiveHandler) RecvMessageInit(ext types.ChatMessageExt) error {
	ctx, cancel := context.WithTimeout(s.ctx, time.Second*5)
	defer cancel()
	var err error
	seqID, err := s.core.GetChatSessionSeqID(ctx, s.userReqMsg.SpaceID, s.userReqMsg.SessionID)
	if err != nil {
		return err
	}

	s.receiveMsg, err = initAssistantMessage(ctx, s.core, s.messageID, seqID, s.userReqMsg, ext)
	if err != nil {
		return err
	}

	return s.PublishMessage(lo.If(s.receiveMsg.Role == types.USER_ROLE_TOOL, types.WS_EVENT_TOOL_INIT).Else(types.WS_EVENT_ASSISTANT_INIT), chatMsgToTextMsg(s.receiveMsg))
}

func (s *ChatReceiveHandler) Copy() types.Receiver {
	c := *s
	c.receiveMsg = nil
	c.messageID = s.core.GenMessageID()
	c.sendedCounter = &sendedCounter{}
	return &c
}

func (s *ChatReceiveHandler) GetReceiveFunc() types.ReceiveFunc {
	if s.receiveMsg == nil {
		s.receiveMsg = &types.ChatMessage{
			ID:        s.messageID,
			SpaceID:   s.userReqMsg.SpaceID,
			SessionID: s.userReqMsg.SessionID,
			UserID:    s.userReqMsg.UserID,
		}
	}
	return getStreamReceiveFunc(s.ctx, s.core, s.sendedCounter, s.receiveMsg)
}

func (s *ChatReceiveHandler) GetDoneFunc(callback func(msg *types.ChatMessage)) types.DoneFunc {
	if s.receiveMsg == nil {
		s.receiveMsg = &types.ChatMessage{
			ID:        s.messageID,
			SpaceID:   s.userReqMsg.SpaceID,
			SessionID: s.userReqMsg.SessionID,
			UserID:    s.userReqMsg.UserID,
		}
	}
	return getStreamDoneFunc(s.ctx, s.core, s.sendedCounter, s.receiveMsg, callback)
}

func NewQueryReceiver(ctx context.Context, core *core.Core, responseChan chan types.MessageContent) types.Receiver {
	return &QueryReceiveHandler{
		ctx:          ctx,
		core:         core,
		resp:         responseChan,
		sendedLength: 0,

		varHandler: mark.NewSensitiveWork(),
	}
}

type QueryReceiveHandler struct {
	ctx          context.Context
	core         *core.Core
	resp         chan types.MessageContent
	sendedLength int64

	varHandler mark.VariableHandler
}

func (s *QueryReceiveHandler) VariableHandler() mark.VariableHandler {
	return s.varHandler
}

func (s *QueryReceiveHandler) MessageID() string {
	return s.MessageID()
}

func (s *QueryReceiveHandler) Copy() types.Receiver {
	c := *s
	return &c
}

func (s *QueryReceiveHandler) IsStream() bool {
	return false
}

func (s *QueryReceiveHandler) RecvMessageInit(ext types.ChatMessageExt) error {
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

func NewBulterAssistant(core *core.Core, agentType string) *ButlerAssistant {
	return &ButlerAssistant{
		core:      core,
		agentType: agentType,
		client:    butler.NewButlerAgent(core),
	}
}

type ButlerAssistant struct {
	core      *core.Core
	agentType string
	client    *butler.ButlerAgent
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
func (s *ButlerAssistant) RequestAssistant(ctx context.Context, reqMsg *types.ChatMessage, receiver types.Receiver, aiCallOptions *types.AICallOptions) error {
	// // 1. 获取空间信息
	// space, err := s.core.Store().SpaceStore().GetSpace(ctx, reqMsg.SpaceID)
	// if err != nil {
	// 	return HandleAssistantEarlyError(err, reqMsg, receiver, "获取空间信息失败")
	// }

	list, err := s.core.Store().BulterTableStore().ListButlerTables(ctx, reqMsg.UserID)
	if err != nil {
		return HandleAssistantEarlyError(err, reqMsg, receiver, "获取用户表格信息失败")
	}

	userExistsTable := &strings.Builder{}
	userExistsTable.WriteString("这是目前用户已经创建的数据表，你可以结合下列数据表简介来分析用户需求，从而决定你下一步要怎么做：\n\n")
	if len(list) == 0 {
		userExistsTable.WriteString("用户当前没有任何数据表\n\n")
	} else {
		for _, v := range list {
			userExistsTable.WriteString("表ID：")
			userExistsTable.WriteString(v.TableID)
			userExistsTable.WriteString("\n")
			userExistsTable.WriteString("表名：")
			userExistsTable.WriteString(v.TableName)
			userExistsTable.WriteString("\n表描述：")
			userExistsTable.WriteString(v.TableDescription)
			userExistsTable.WriteString("------\n\n")
		}
	}

	prompt := butler.BuildButlerPrompt("", s.core.Srv().AI(), userExistsTable.String())
	prompt = receiver.VariableHandler().Do(prompt)

	// 3. 生成会话上下文
	sessionContext, err := s.GenSessionContext(ctx, prompt, reqMsg)
	if err != nil {
		return HandleAssistantEarlyError(err, reqMsg, receiver, "生成会话上下文失败")
	}

	// 4. 创建 AgentContext - 提取思考和搜索配置
	enableThinking := aiCallOptions.EnableThinking
	enableWebSearch := aiCallOptions.EnableSearch

	agentCtx := types.NewAgentContextWithOptions(
		ctx,
		reqMsg.SpaceID,
		reqMsg.UserID,
		reqMsg.SessionID,
		reqMsg.ID,
		reqMsg.Sequence,
		enableThinking,
		enableWebSearch,
		false,
	)

	// 5. 将 MessageContext 转换为 eino 消息格式
	einoMessages := ai.ConvertMessageContextToEinoMessages(sessionContext.MessageContext)
	einoMessages = lo.Map(einoMessages, func(item *schema.Message, _ int) *schema.Message {
		item.Content = receiver.VariableHandler().Do(item.Content)
		return item
	})

	// 6. 创建工具包装器
	notifyToolWrapper := NewNotifyToolWrapper(s.core, reqMsg, receiver.Copy())

	// 7. 使用通用工厂创建Butler ReAct Agent
	factory := NewEinoAgentFactory(s.core)
	agent, modelConfig, err := factory.CreateButlerReActAgent(agentCtx, notifyToolWrapper, einoMessages, s.client)
	if err != nil {
		return HandleAssistantEarlyError(err, reqMsg, receiver, "创建Butler代理失败")
	}

	// 从这里开始，错误处理交给具体的 handler 方法
	// 8. 创建响应处理器
	responseHandler := NewEinoResponseHandler(receiver, reqMsg)
	callbackHandler := NewCallbackHandlers(s.core, modelConfig.ModelName, responseHandler)

	// 9. 执行推理
	if receiver.IsStream() {
		// 流式处理
		return s.handleStreamResponse(agentCtx, agent, einoMessages, responseHandler, callbackHandler)
	} else {
		// 非流式处理
		return s.handleDirectResponse(agentCtx, agent, einoMessages, responseHandler)
	}
}

// handleStreamResponse 处理流式响应 (复用AutoAssistant的实现)
func (s *ButlerAssistant) handleStreamResponse(ctx context.Context, reactAgent *react.Agent, messages []*schema.Message, streamHandler *EinoResponseHandler, callbacksHandler callbacks.Handler) error {
	// reqMessage := streamHandler.reqMsg
	// initFunc := func(ctx context.Context) error {
	// 	if err := streamHandler.Receiver().RecvMessageInit(types.ChatMessageExt{
	// 		SpaceID:   reqMessage.SpaceID,
	// 		SessionID: reqMessage.SessionID,
	// 		CreatedAt: time.Now().Unix(),
	// 		UpdatedAt: time.Now().Unix(),
	// 	}); err != nil {
	// 		slog.Error("failed to initialize receive message", slog.String("error", err.Error()))
	// 		return err
	// 	}

	// 	slog.Debug("AI message session created",
	// 		slog.String("msg_id", streamHandler.Receiver().MessageID()),
	// 		slog.String("session_id", reqMessage.SessionID))
	// 	return nil
	// }

	// 使用 eino agent 进行流式推理
	_, err := reactAgent.Stream(ctx, messages, agent.WithComposeOptions(
		compose.WithCallbacks(callbacksHandler, &LoggerCallback{}),
	))
	if err != nil {
		// initFunc(ctx)
		// streamHandler.GetDoneFunc(nil)(err)
		slog.Error("failed to start eino stream response", slog.Any("error", err))
		return err
	}

	// if err := streamHandler.HandleStreamResponse(ctx, result, initFunc); err != nil {
	// 	slog.Error("failed to handle stream response", slog.Any("error", err), slog.String("message_id", reqMessage.ID))
	// 	return err
	// }

	return nil
}

// handleDirectResponse 处理非流式响应 (复用AutoAssistant的实现)
func (s *ButlerAssistant) handleDirectResponse(ctx context.Context, agent *react.Agent, messages []*schema.Message, handler *EinoResponseHandler) error {
	// 使用 eino agent 进行推理
	done := handler.GetDoneFunc(nil)
	result, err := agent.Generate(ctx, messages)
	if err != nil {
		if done != nil {
			done(err)
		}
		return err
	}

	reqMessage := handler.reqMsg
	handler.Receiver().RecvMessageInit(types.ChatMessageExt{
		SpaceID:   reqMessage.SpaceID,
		SessionID: reqMessage.SessionID,
	})
	if err = handler.GetReceiveFunc()(&types.TextMessage{Text: result.Content}, types.MESSAGE_PROGRESS_GENERATING); err != nil {
		return err
	}
	return done(nil)
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

func (s *JournalAssistant) RequestAssistant(ctx context.Context, reqMsg *types.ChatMessage, receiver types.Receiver, aiCallOptions *types.AICallOptions) error {
	// 2. 准备提示词 - 使用journal专用提示词
	prompt := journal.BuildJournalPrompt("", s.core.Srv().AI())
	prompt = receiver.VariableHandler().Do(prompt)

	// 3. 生成会话上下文
	sessionContext, err := s.GenSessionContext(ctx, prompt, reqMsg)
	if err != nil {
		return HandleAssistantEarlyError(err, reqMsg, receiver, "生成会话上下文失败")
	}

	// 4. 创建 AgentContext - 提取思考和搜索配置
	enableThinking := aiCallOptions.EnableThinking
	enableWebSearch := aiCallOptions.EnableSearch

	agentCtx := types.NewAgentContextWithOptions(
		ctx,
		reqMsg.SpaceID,
		reqMsg.UserID,
		reqMsg.SessionID,
		reqMsg.ID,
		reqMsg.Sequence,
		enableThinking,
		enableWebSearch,
		false,
	)

	// 5. 将 MessageContext 转换为 eino 消息格式
	einoMessages := ai.ConvertMessageContextToEinoMessages(sessionContext.MessageContext)
	einoMessages = lo.Map(einoMessages, func(item *schema.Message, _ int) *schema.Message {
		item.Content = receiver.VariableHandler().Do(item.Content)
		return item
	})

	// 6. 创建工具包装器
	notifyToolWrapper := NewNotifyToolWrapper(s.core, reqMsg, receiver.Copy())

	// 7. 使用通用工厂创建Journal ReAct Agent
	factory := NewEinoAgentFactory(s.core)
	agent, modelConfig, err := factory.CreateJournalReActAgent(agentCtx, notifyToolWrapper, einoMessages, s.agent)
	if err != nil {
		return HandleAssistantEarlyError(err, reqMsg, receiver, "创建Journal代理失败")
	}

	// 从这里开始，错误处理交给具体的 handler 方法
	// 8. 创建响应处理器
	responseHandler := NewEinoResponseHandler(receiver, reqMsg)
	callbackHandler := NewCallbackHandlers(s.core, modelConfig.ModelName, responseHandler)

	// 9. 执行推理
	if receiver.IsStream() {
		// 流式处理
		return s.handleStreamResponse(agentCtx, agent, einoMessages, responseHandler, callbackHandler)
	} else {
		// 非流式处理
		return s.handleDirectResponse(agentCtx, agent, einoMessages, responseHandler)
	}
}

// handleStreamResponse 处理流式响应 (复用ButlerAssistant的实现)
func (s *JournalAssistant) handleStreamResponse(ctx context.Context, reactAgent *react.Agent, messages []*schema.Message, streamHandler *EinoResponseHandler, callbacksHandler callbacks.Handler) error {
	// reqMessage := streamHandler.reqMsg
	// initFunc := func(ctx context.Context) error {
	// 	if err := streamHandler.Receiver().RecvMessageInit(types.ChatMessageExt{
	// 		SpaceID:   reqMessage.SpaceID,
	// 		SessionID: reqMessage.SessionID,
	// 		CreatedAt: time.Now().Unix(),
	// 		UpdatedAt: time.Now().Unix(),
	// 	}); err != nil {
	// 		slog.Error("failed to initialize receive message", slog.String("error", err.Error()))
	// 		return err
	// 	}

	// 	slog.Debug("AI message session created",
	// 		slog.String("msg_id", streamHandler.Receiver().MessageID()),
	// 		slog.String("session_id", reqMessage.SessionID))
	// 	return nil
	// }

	// 使用 eino agent 进行流式推理
	_, err := reactAgent.Stream(ctx, messages, agent.WithComposeOptions(
		compose.WithCallbacks(callbacksHandler, &LoggerCallback{}),
	))
	if err != nil {
		// initFunc(ctx)
		// streamHandler.GetDoneFunc(nil)(err)
		slog.Error("failed to start eino stream response", slog.Any("error", err))
		return err
	}

	// if err := streamHandler.HandleStreamResponse(ctx, result, initFunc); err != nil {
	// 	slog.Error("failed to handle stream response", slog.Any("error", err), slog.String("message_id", reqMessage.ID))
	// 	return err
	// }

	return nil
}

// handleDirectResponse 处理非流式响应 (复用ButlerAssistant的实现)
func (s *JournalAssistant) handleDirectResponse(ctx context.Context, agent *react.Agent, messages []*schema.Message, handler *EinoResponseHandler) error {
	// 使用 eino agent 进行推理
	done := handler.GetDoneFunc(nil)
	result, err := agent.Generate(ctx, messages)
	if err != nil {
		if done != nil {
			done(err)
		}
		return err
	}

	reqMessage := handler.reqMsg
	handler.Receiver().RecvMessageInit(types.ChatMessageExt{
		SpaceID:   reqMessage.SpaceID,
		SessionID: reqMessage.SessionID,
	})
	if err = handler.GetReceiveFunc()(&types.TextMessage{Text: result.Content}, types.MESSAGE_PROGRESS_GENERATING); err != nil {
		return err
	}
	return done(nil)
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
	if ext.ToolName != "" {
		answerMsg.Role = types.USER_ROLE_TOOL
		answerMsg.MsgType = types.MESSAGE_TYPE_TOOL_TIPS
	}
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

type messageCondition func(historyMsgSequence, inputMsgSequence int64) bool

func normalGenMessageCondition(historyMsgSequence, inputMsgSequence int64) bool {
	return historyMsgSequence > inputMsgSequence
}

func reGenMessageCondition(historyMsgSequence, inputMsgSequence int64) bool {
	return historyMsgSequence >= inputMsgSequence
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
	msgList, err := core.Store().ChatMessageStore().ListSessionMessage(ctx, reqMsgWithDocs.SpaceID, reqMsgWithDocs.SessionID, summary.Sequence, types.NO_PAGINATION, types.NO_PAGINATION)
	if err != nil {
		return nil, errors.New("genDialogContextAndSummaryIfExceedsTokenLimit.ChatMessageStore.ListSessionMessage", i18n.ERROR_INTERNAL, err)
	}

	// 对消息按msgid进行排序
	sort.Slice(msgList, func(i, j int) bool {
		return msgList[i].Sequence < msgList[j].Sequence
	})

	var (
		summaryMessageCutRange int
		summaryMessageSeqID    int64
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

		// if isErrorMessage(v.Message) {
		// 	fmt.Println("", "skip error message in context:", v.Message)
		// 	continue
		// }

		if v.Complete != types.MESSAGE_PROGRESS_COMPLETE {
			continue
		}

		if msgCondition(v.Sequence, reqMsgWithDocs.Sequence) {
			// 当前逻辑回复的是 msgID, 所以上下文中不应该出现晚于 msgID 出现的消息，多人场景会有此情况
			break
		}

		contextIndex++

		if len(v.Attach) > 0 {
			item := &types.MessageContext{
				Role: types.USER_ROLE_USER,
			}
			item.MultiContent = v.Attach.ToMultiContent("", core.FileStorage())
			reqMsg = append(reqMsg, item)
		} else {
			if v.Role == types.USER_ROLE_TOOL {
				ext, err := core.Store().ChatMessageExtStore().GetChatMessageExt(ctx, reqMsgWithDocs.SpaceID, reqMsgWithDocs.SessionID, v.ID)
				if err != nil {
					slog.Error("failed to get tool call message ext", slog.Any("error", err))
					continue
				}
				reqMsg = append(reqMsg, &types.MessageContext{
					Role:    types.USER_ROLE_ASSISTANT,
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
			}

			reqMsg = append(reqMsg, &types.MessageContext{
				Role:    v.Role,
				Content: v.Message,
			})
		}

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
		summaryMessageSeqID = msgList[contextIndex-summaryMessageCutRange].Sequence
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
		if err = GenChatSessionContextSummary(ctx, core, reqMsgWithDocs.SpaceID, reqMsgWithDocs.SessionID, summaryMessageSeqID, summaryReq); err != nil {
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

// GenChatSessionContextSummary 生成dialog上下文总结
func GenChatSessionContextSummary(ctx context.Context, core *core.Core, spaceID, sessionID string, summaryMessageSeqID int64, reqMsg []*types.MessageContext) error {
	// slog.Debug("start generating context summary", slog.String("session_id", sessionID), slog.String("msg_id", summaryMessageID), slog.Any("request_message", reqMsg))
	prompt := core.Cfg().Prompt.ChatSummary
	if prompt == "" {
		prompt = ai.PROMPT_SUMMARY_DEFAULT_CN
	}

	cfg := core.Srv().AI().GetConfig(types.MODEL_TYPE_CHAT)
	model, err := GetToolCallingModel(&types.AgentContext{
		Context:        ctx,
		EnableThinking: false,
	}, cfg)
	if err != nil {
		return err
	}

	messages := lo.Map(reqMsg, func(item *types.MessageContext, i int) *schema.Message {
		if i == 0 {
			return schema.SystemMessage(prompt)
		}
		return &schema.Message{
			Role:    schema.RoleType(item.Role.String()),
			Content: item.Content,
		}
	})

	messages = append(messages, schema.UserMessage("请对上述对话做一个总结。"))

	resp, err := model.Generate(ctx, messages)
	if err != nil {
		return errors.New("g11enDialogContextSummary.gptSrv.Chat", i18n.ERROR_INTERNAL, err)
	}

	if err = core.Store().ChatSummaryStore().Create(ctx, types.ChatSummary{
		ID:        utils.GenSpecIDStr(),
		SpaceID:   spaceID,
		SessionID: sessionID,
		Sequence:  summaryMessageSeqID,
		Content:   resp.Content,
	}); err != nil {
		return errors.New("genDialogContextSummary.ChatSummaryStore.Create", i18n.ERROR_INTERNAL, err)
	}
	slog.Debug("succeed to generate summary", slog.String("session_id", sessionID), slog.Int64("message_sequence", summaryMessageSeqID))
	return nil
}
