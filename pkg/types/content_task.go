package types

const (
	LONG_INIT                          = 0
	LONG_CONTENT_STEP_CREATE_AI_OBJECT = 1
	LONG_CONTENT_STEP_CREATE_CHUNK     = 2
	LONG_CONTENT_STEP_FINISHED         = 3
)

// Task status constants for UI display
const (
	TASK_STATUS_FINISHED    = 1 // 已完成
	TASK_STATUS_IN_PROGRESS = 2 // 进行中
	TASK_STATUS_FAILED      = 3 // 失败（重试次数达到上限）
)

// ContentTask 数据表结构
// 该结构体表示与 bw_content_task 表相关的记录
type ContentTask struct {
	TaskID     string `json:"task_id" db:"task_id"`       // 任务ID，32字符字符串类型，唯一标识任务
	SpaceID    string `json:"space_id" db:"space_id"`     // 空间ID，标识任务归属的空间
	UserID     string `json:"user_id" db:"user_id"`       // 用户ID，标识发起任务的用户
	Resource   string `json:"resource" db:"resource"`     // 资源类型
	MetaInfo   string `json:"meta_info" db:"meta_info"`   // 用户自定义meta信息，空则使用文件名填充
	FileURL    string `json:"file_url" db:"file_url"`     // 文件URL，任务需要处理的文件路径
	FileName   string `json:"file_name" db:"file_name"`   // 文件名，任务需要处理的文件名称
	AIFileID   string `json:"ai_file_id" db:"ai_file_id"` // AI 服务中对应该文件的id
	Step       int    `json:"step" db:"step"`             // 任务的当前阶段，例如：1-待处理，2-处理中，3-已完成等
	TaskType   string `json:"task_type" db:"task_type"`   // 任务类型，表示任务的目的或用途，例如：'文本切割'，'数据清洗'等
	CreatedAt  int64  `json:"created_at" db:"created_at"` // 任务创建时间，时间戳格式
	UpdatedAt  int64  `json:"updated_at" db:"updated_at"` // 任务创建时间，时间戳格式
	RetryTimes int    `json:"retry_times" db:"retry_times"`
}

type TaskStatus struct {
	TaskID     string `json:"task_id" db:"task_id"`
	Status     int    `json:"status" db:"step"`
	RetryTimes int    `json:"retry_times" db:"retry_times"`
	UpdatedAt  int64  `json:"updated_at" db:"updated_at"`
}
