package types

type FileManagement struct {
	ID         int64  `json:"id" db:"id"`                   // 文件记录的唯一标识
	SpaceID    string `json:"space_id" db:"space_id"`       // 空间id
	UserID     string `json:"user_id" db:"user_id"`         // 关联的用户ID，用于区分每个用户的文件数据
	File       string `json:"file" db:"file"`               // 存储文件的路径，用于定位文件内容
	FileSize   int64  `json:"file_size" db:"file_size"`     // 文件大小，单位为字节
	ObjectType string `json:"object_type" db:"object_type"` // 文件所属的功能模块，例如“用户头像”
	Kind       string `json:"kind" db:"kind"`               // 文件类型，例如“image”、“file”
	Status     int    `json:"status" db:"status"`           // 文件的状态，1表示可用，2表示已删除
	CreatedAt  int64  `json:"created_at" db:"created_at"`   // 记录文件的上传时间
}

const (
	FILE_UPLOAD_STATUS_UNKNOWN        int = 0
	FILE_UPLOAD_STATUS_UPLOADED       int = 1
	FILE_UPLOAD_STATUS_NEED_TO_DELETE int = 2
)
