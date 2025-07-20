package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/pgvector/pgvector-go"

	"github.com/quka-ai/quka-ai/pkg/ai"
	"github.com/quka-ai/quka-ai/pkg/sqlstore"
	"github.com/quka-ai/quka-ai/pkg/types"
)

// KnowledgeStoreInterface 定义 KnowledgeStore 的方法集合
type KnowledgeStore interface {
	sqlstore.SqlCommons
	// Create 创建新的知识记录
	Create(ctx context.Context, data types.Knowledge) error
	BatchCreate(ctx context.Context, datas []*types.Knowledge) error
	// GetKnowledge 根据ID获取知识记录
	GetKnowledge(ctx context.Context, spaceID, id string) (*types.Knowledge, error)
	// Update 更新知识记录
	Update(ctx context.Context, spaceID, id string, data types.UpdateKnowledgeArgs) error
	// Delete 删除知识记录
	Delete(ctx context.Context, spaceID, id string) error
	DeleteAll(ctx context.Context, spaceID string) error
	BatchDelete(ctx context.Context, ids []string) error
	// ListKnowledges 分页获取知识记录列表
	ListKnowledges(ctx context.Context, opts types.GetKnowledgeOptions, page, pageSize uint64) ([]*types.Knowledge, error)
	ListKnowledgeIDs(ctx context.Context, opts types.GetKnowledgeOptions, page, pageSize uint64) ([]string, error)
	Total(ctx context.Context, opts types.GetKnowledgeOptions) (uint64, error)
	ListLiteKnowledges(ctx context.Context, opts types.GetKnowledgeOptions, page, pageSize uint64) ([]*types.KnowledgeLite, error)
	FinishedStageSummarize(ctx context.Context, spaceID, id string, summary ai.ChunkResult) error
	FinishedStageEmbedding(ctx context.Context, spaceID, id string) error
	SetRetryTimes(ctx context.Context, spaceID, id string, retryTimes int) error
	ListProcessingKnowledges(ctx context.Context, retryTimes int, page, pageSize uint64) ([]types.Knowledge, error)
	ListFailedKnowledges(ctx context.Context, stage types.KnowledgeStage, retryTimes int, page, pageSize uint64) ([]types.Knowledge, error)
}

// KnowledgeChunkStore 定义 KnowledgeChunkStore 的接口
type KnowledgeChunkStore interface {
	sqlstore.SqlCommons
	Create(ctx context.Context, data types.KnowledgeChunk) error
	BatchCreate(ctx context.Context, data []*types.KnowledgeChunk) error
	Get(ctx context.Context, spaceID, knowledgeID, id string) (*types.KnowledgeChunk, error)
	Update(ctx context.Context, spaceID, knowledgeID, id, chunk string) error
	Delete(ctx context.Context, spaceID, knowledgeID, id string) error
	DeleteAll(ctx context.Context, spaceID string) error
	BatchDelete(ctx context.Context, spaceID, knowledgeID string) error
	BatchDeleteByIDs(ctx context.Context, knowledgeIDs []string) error
	List(ctx context.Context, spaceID, knowledgeID string) ([]types.KnowledgeChunk, error)
}

// TODO support other vector db
// current only pg
// next qdrant
type VectorStore interface {
	sqlstore.SqlCommons
	Create(ctx context.Context, data types.Vector) error
	BatchCreate(ctx context.Context, datas []types.Vector) error
	GetVector(ctx context.Context, spaceID, knowledgeID string) (*types.Vector, error)
	Update(ctx context.Context, spaceID, knowledgeID, id string, vector pgvector.Vector) error
	Delete(ctx context.Context, spaceID, knowledgeID, id string) error
	BatchDelete(ctx context.Context, spaceID, knowledgeID string) error
	DeleteAll(ctx context.Context, spaceID string) error
	DeleteByResource(ctx context.Context, spaceID, resource string) error
	ListVectors(ctx context.Context, opts types.GetVectorsOptions, page, pageSize uint64) ([]types.Vector, error)
	Query(ctx context.Context, opts types.GetVectorsOptions, vectors pgvector.Vector, limit uint64) ([]types.QueryResult, error)
}

type AccessTokenStore interface {
	sqlstore.SqlCommons
	Create(ctx context.Context, data types.AccessToken) error
	GetAccessToken(ctx context.Context, appid, token string) (*types.AccessToken, error)
	Delete(ctx context.Context, appid, userID string, id int64) error
	Deletes(ctx context.Context, appid, userID string, ids []int64) error
	ListAccessTokens(ctx context.Context, appid, userID string, page, pageSize uint64) ([]types.AccessToken, error)
	ClearUserTokens(ctx context.Context, appid, userID string) error
	Total(ctx context.Context, appid, userID string) (int64, error)
}

type UserSpaceStore interface {
	sqlstore.SqlCommons
	Create(ctx context.Context, data types.UserSpace) error
	GetUserSpaceRole(ctx context.Context, userID, spaceID string) (*types.UserSpace, error)
	GetSpaceChief(ctx context.Context, spaceID string) (*types.UserSpace, error)
	Update(ctx context.Context, userID, spaceID, role string) error
	List(ctx context.Context, opts types.ListUserSpaceOptions, page, pageSize uint64) ([]types.UserSpace, error)
	Total(ctx context.Context, opts types.ListUserSpaceOptions) (int64, error)
	Delete(ctx context.Context, userID, spaceID string) error
	DeleteAll(ctx context.Context, spaceID string) error
	ListSpaceUsers(ctx context.Context, spaceID string) ([]string, error)
}

type SpaceStore interface {
	sqlstore.SqlCommons
	Create(ctx context.Context, data types.Space) error
	GetSpace(ctx context.Context, spaceID string) (*types.Space, error)
	Update(ctx context.Context, spaceID, title, desc, basePrompt, chatPrompt string) error
	Delete(ctx context.Context, spaceID string) error
	List(ctx context.Context, spaceIDs []string, page, pageSize uint64) ([]types.Space, error)
}

type ResourceStore interface {
	sqlstore.SqlCommons // 继承通用SQL操作
	Create(ctx context.Context, data types.Resource) error
	GetResource(ctx context.Context, spaceID, id string) (*types.Resource, error)
	Update(ctx context.Context, spaceID, id, title, desc, prompt string, cycle int) error
	Delete(ctx context.Context, spaceID, id string) error
	ListResources(ctx context.Context, spaceID string, page, pageSize uint64) ([]types.Resource, error)
	ListUserResources(ctx context.Context, userID string, page, pageSize uint64) ([]types.Resource, error)
}

type UserStore interface {
	sqlstore.SqlCommons // 继承通用SQL操作
	Create(ctx context.Context, data types.User) error
	GetUser(ctx context.Context, appid, id string) (*types.User, error)
	GetByEmail(ctx context.Context, appid, email string) (*types.User, error)
	UpdateUserProfile(ctx context.Context, appid, id, userName, email, avatar string) error
	UpdateUserPassword(ctx context.Context, appid, id, salt, password string) error
	Delete(ctx context.Context, appid, id string) error
	ListUsers(ctx context.Context, opts types.ListUserOptions, page, pageSize uint64) ([]types.User, error)
	Total(ctx context.Context, opts types.ListUserOptions) (int64, error)
	UpdateUserPlan(ctx context.Context, appid, id, planID string) error
	BatchUpdateUserPlan(ctx context.Context, appid string, ids []string, planID string) error
}

// UserGlobalRoleStore 全局用户角色存储接口
type UserGlobalRoleStore interface {
	sqlstore.SqlCommons // 继承通用SQL操作
	Create(ctx context.Context, data types.UserGlobalRole) error
	GetUserRole(ctx context.Context, appid, userID string) (*types.UserGlobalRole, error)
	UpdateUserRole(ctx context.Context, appid, userID, role string) error
	Delete(ctx context.Context, appid, userID string) error
	ListUsersByRole(ctx context.Context, opts types.ListUserGlobalRoleOptions, page, pageSize uint64) ([]types.UserGlobalRole, error)
	Total(ctx context.Context, opts types.ListUserGlobalRoleOptions) (int64, error)
}

type ChatSessionStore interface {
	sqlstore.SqlCommons // 继承通用SQL操作
	Create(ctx context.Context, data types.ChatSession) error
	UpdateSessionStatus(ctx context.Context, sessionID string, status types.ChatSessionStatus) error
	UpdateSessionTitle(ctx context.Context, sessionID string, title string) error
	GetByUserID(ctx context.Context, userID string) ([]*types.ChatSession, error)
	GetChatSession(ctx context.Context, spaceID, sessionID string) (*types.ChatSession, error)
	UpdateChatSessionLatestAccessTime(ctx context.Context, spaceID, sessionID string) error
	Delete(ctx context.Context, spaceID, sessionID string) error
	DeleteAll(ctx context.Context, spaceID string) error
	List(ctx context.Context, spaceID, userID string, page, pageSize uint64) ([]types.ChatSession, error)
	ListBeforeTime(ctx context.Context, t time.Time, page, pageSize uint64) ([]types.ChatSession, error)
	Total(ctx context.Context, spaceID, userID string) (int64, error)
}

type ChatMessageStore interface {
	sqlstore.SqlCommons // 继承通用SQL操作
	Create(ctx context.Context, data *types.ChatMessage) error
	GetOne(ctx context.Context, id string) (*types.ChatMessage, error)
	RewriteMessage(ctx context.Context, spaceID, sessionID, id string, message json.RawMessage, complete int32) error
	AppendMessage(ctx context.Context, spaceID, sessionID, id string, message json.RawMessage, complete int32) error
	UpdateMessageCompleteStatus(ctx context.Context, sessionID, id string, complete int32) error
	UpdateMessageAttach(ctx context.Context, sessionID, id string, attach types.ChatMessageAttach) error
	DeleteMessage(ctx context.Context, id string) error
	DeleteAll(ctx context.Context, spaceID string) error
	DeleteSessionMessage(ctx context.Context, spaceID, sessionID string) error
	ListSessionMessageUpToGivenID(ctx context.Context, spaceID, sessionID, msgID string, page, pageSize uint64) ([]*types.ChatMessage, error)
	ListSessionMessage(ctx context.Context, spaceID, sessionID, afterMsgID string, page, pageSize uint64) ([]*types.ChatMessage, error)
	TotalSessionMessage(ctx context.Context, spaceID, sessionID, afterMsgID string) (int64, error)
	Exist(ctx context.Context, spaceID, sessionID, msgID string) (bool, error)
	GetMessagesByIDs(ctx context.Context, msgIDs []string) ([]*types.ChatMessage, error)
	GetSessionLatestMessage(ctx context.Context, spaceID, sessionID string) (*types.ChatMessage, error)
	GetSessionLatestUserMessage(ctx context.Context, spaceID, sessionID string) (*types.ChatMessage, error)
	GetSessionLatestUserMsgIDBeforeGivenID(ctx context.Context, spaceID, sessionID, msgID string) (string, error)
	ListUnEncryptMessage(ctx context.Context, page, pageSize uint64) ([]*types.ChatMessage, error)
	SaveEncrypt(ctx context.Context, id string, message json.RawMessage) error
}

type ChatSummaryStore interface {
	sqlstore.SqlCommons
	Create(ctx context.Context, data types.ChatSummary) error
	GetChatSessionLatestSummary(ctx context.Context, sessionID string) (*types.ChatSummary, error)
	DeleteSessionSummary(ctx context.Context, sessionID string) error
	DeleteAll(ctx context.Context, spaceID string) error
}

type ChatMessageExtStore interface {
	sqlstore.SqlCommons // 假设你有通用的 SQL 操作接口
	Create(ctx context.Context, data types.ChatMessageExt) error
	GetChatMessageExt(ctx context.Context, spaceID, sessionID, messageID string) (*types.ChatMessageExt, error)
	ListChatMessageExts(ctx context.Context, messageIDs []string) ([]types.ChatMessageExt, error)
	Update(ctx context.Context, id string, data types.ChatMessageExt) error
	Delete(ctx context.Context, id string) error
	DeleteAll(ctx context.Context, spaceID string) error
	DeleteSessionMessageExt(ctx context.Context, spaceID, sessionID string) error
}

// 定义接口
type FileManagementStore interface {
	Create(ctx context.Context, data types.FileManagement) error
	GetByID(ctx context.Context, spaceID, file string) (*types.FileManagement, error)
	UpdateStatus(ctx context.Context, spaceID string, files []string, status int) error
	Delete(ctx context.Context, spaceID, file string) error
}

type AITokenUsageStore interface {
	Create(ctx context.Context, data types.AITokenUsage) error
	Get(ctx context.Context, _type, subType, objectID, userID string) (*types.AITokenUsage, error)
	List(ctx context.Context, spaceID, userID string, page, pageSize uint64) ([]types.AITokenUsage, error)
	ListUserEachModelUsage(ctx context.Context, userID string, st, et time.Time) ([]types.AITokenSummary, error)
	SumUserUsageByType(ctx context.Context, userID string, st, et time.Time) ([]types.UserTokenUsageWithType, error)
	SumUserUsage(ctx context.Context, userID string, st, et time.Time) (types.UserTokenUsage, error)
	Delete(ctx context.Context, spaceID, userID string, st, et time.Time) error
}

type ShareTokenStore interface {
	sqlstore.SqlCommons // 继承通用SQL操作
	Create(ctx context.Context, link *types.ShareToken) error
	Get(ctx context.Context, _type, spaceID, objectID string) (*types.ShareToken, error)
	GetByToken(ctx context.Context, token string) (*types.ShareToken, error)
	UpdateExpireTime(ctx context.Context, id, expireAt int64) error
	Delete(ctx context.Context, token string) error
}

type JournalStore interface {
	sqlstore.SqlCommons
	Create(ctx context.Context, data types.Journal) error
	Get(ctx context.Context, spaceID, userID, date string) (*types.Journal, error)
	Exist(ctx context.Context, spaceID, userID, date string) (bool, error)
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context, spaceID, userID string, page, pageSize uint64) ([]types.Journal, error)
	ListWithDate(ctx context.Context, spaceID, userID, startDate, endDate string) ([]types.Journal, error)
	Update(ctx context.Context, id int64, content types.KnowledgeContent) error
	DeleteByDate(ctx context.Context, date string) error
}

type ChatSessionPinStore interface {
	sqlstore.SqlCommons
	Create(ctx context.Context, data types.ChatSessionPin) error
	GetBySessionID(ctx context.Context, sessionID string) (*types.ChatSessionPin, error)
	Update(ctx context.Context, spaceID, sessionID string, content types.RawMessage, version string) error
	Delete(ctx context.Context, spaceID, sessionID string) error
	DeleteAll(ctx context.Context, spaceID string) error
	List(ctx context.Context, page, pageSize uint64) ([]types.ChatSessionPin, error)
}

type ButlerTableStore interface {
	sqlstore.SqlCommons
	Create(ctx context.Context, data types.ButlerTable) error
	GetTableData(ctx context.Context, id string) (*types.ButlerTable, error)
	Update(ctx context.Context, id string, data string) error
	Delete(ctx context.Context, id string) error
	ListButlerTables(ctx context.Context, userID string) ([]types.ButlerTable, error)
}

type SpaceApplicationStore interface {
	sqlstore.SqlCommons
	Create(ctx context.Context, data *types.SpaceApplication) error
	Get(ctx context.Context, spaceID, userID string) (*types.SpaceApplication, error)
	GetByID(ctx context.Context, id string) (*types.SpaceApplication, error)
	UpdateStatus(ctx context.Context, ids []string, status types.SpaceApplicationType) error
	UpdateAllWaittingStatus(ctx context.Context, spaceID string, status types.SpaceApplicationType) error
	Delete(ctx context.Context, spaceID, userID string) error
	Total(ctx context.Context, spaceID string, opts types.ListSpaceApplicationOptions) (int64, error)
	List(ctx context.Context, spaceID string, opts types.ListSpaceApplicationOptions, page, pagesize uint64) ([]types.SpaceApplication, error)
}

type ModelProviderStore interface {
	sqlstore.SqlCommons
	Create(ctx context.Context, data types.ModelProvider) error
	Get(ctx context.Context, id string) (*types.ModelProvider, error)
	Update(ctx context.Context, id string, data types.ModelProvider) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, opts types.ListModelProviderOptions, page, pageSize uint64) ([]types.ModelProvider, error)
	Total(ctx context.Context, opts types.ListModelProviderOptions) (int64, error)
}

type ModelConfigStore interface {
	sqlstore.SqlCommons
	Create(ctx context.Context, data types.ModelConfig) error
	Get(ctx context.Context, id string) (*types.ModelConfig, error)
	Update(ctx context.Context, id string, data types.ModelConfig) error
	Delete(ctx context.Context, id string) error
	DeleteByProviderID(ctx context.Context, providerID string) error
	List(ctx context.Context, opts types.ListModelConfigOptions) ([]types.ModelConfig, error)
	ListWithProvider(ctx context.Context, opts types.ListModelConfigOptions) ([]*types.ModelConfig, error)
	Total(ctx context.Context, opts types.ListModelConfigOptions) (int64, error)
}

type CustomConfigStore interface {
	sqlstore.SqlCommons
	Upsert(ctx context.Context, data types.CustomConfig) error
	BatchUpsert(ctx context.Context, configs []types.CustomConfig) error
	Get(ctx context.Context, name string) (*types.CustomConfig, error)
	Delete(ctx context.Context, name string) error
	List(ctx context.Context, opts types.ListCustomConfigOptions, page, pageSize uint64) ([]types.CustomConfig, error)
	Total(ctx context.Context, opts types.ListCustomConfigOptions) (int64, error)
}

type SpaceInvitationStore interface {
	sqlstore.SqlCommons
	Create(ctx context.Context, data types.Invitation) error
	Get(ctx context.Context, appid, inviterID, inviteeEmail string) (*types.Invitation, error)
	GetByID(ctx context.Context, appid string, id int64) (*types.Invitation, error)
	UpdateStatus(ctx context.Context, appid string, id int64, status types.SpaceInvitationStatus) error
	Delete(ctx context.Context, appid string, id int64) error
	List(ctx context.Context, appid, spaceID string, opts types.ListSpaceInvitationOptions, page, pageSize uint64) ([]types.Invitation, error)
	Total(ctx context.Context, appid, spaceID string, opts types.ListSpaceInvitationOptions) (int64, error)
}

type ContentTaskStore interface {
	sqlstore.SqlCommons
	Create(ctx context.Context, data types.ContentTask) error
	Update(ctx context.Context, taskID string, data types.ContentTask) error
	GetTask(ctx context.Context, taskID string) (*types.ContentTask, error)
	UpdateStep(ctx context.Context, taskID string, step int) error
	ListTasks(ctx context.Context, spaceID string, page, pageSize uint64) ([]types.ContentTask, error)
	Delete(ctx context.Context, taskID string) error
	DeleteAll(ctx context.Context, spaceID string) error
	UpdateAIFileID(ctx context.Context, taskID, aiFileID string) error
	ListUnprocessedTasks(ctx context.Context, page, pageSize uint64) ([]*types.ContentTask, error)
	SetRetryTimes(ctx context.Context, id string, retryTimes int) error
	ListTasksStatus(ctx context.Context, taskIDs []string) ([]types.TaskStatus, error)
	Total(ctx context.Context, spaceID string) (int64, error)
}

type KnowledgeMetaStore interface {
	Create(ctx context.Context, data types.KnowledgeMeta) error
	GetKnowledgeMeta(ctx context.Context, id string) (*types.KnowledgeMeta, error)
	Update(ctx context.Context, id string, data types.KnowledgeMeta) error
	Delete(ctx context.Context, id string) error
	DeleteAll(ctx context.Context, spaceID string) error
	ListKnowledgeMetas(ctx context.Context, ids []string) ([]*types.KnowledgeMeta, error)
}

type KnowledgeRelMetaStore interface {
	Create(ctx context.Context, data types.KnowledgeRelMeta) error
	BatchCreate(ctx context.Context, datas []types.KnowledgeRelMeta) error
	Get(ctx context.Context, id string) (*types.KnowledgeRelMeta, error)
	Update(ctx context.Context, id string, data types.KnowledgeRelMeta) error
	Delete(ctx context.Context, id string) error
	DeleteAll(ctx context.Context, spaceID string) error
	ListKnowledgesMeta(ctx context.Context, knowledgeIDs []string) ([]*types.KnowledgeRelMeta, error)
	ListRelMetaWithKnowledgeContent(ctx context.Context, opts []types.MergeDataQuery) ([]*types.RelMetaWithKnowledge, error)
}
