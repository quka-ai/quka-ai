package types

// ShareToken 表示文章分享链接的数据结构
type ShareToken struct {
	ID           int64  `json:"id" db:"id"`                       // 自增ID
	Appid        string `json:"appid" db:"appid"`                 // 应用ID
	SpaceID      string `json:"space_id" db:"space_id"`           // 关联空间ID
	ObjectID     string `json:"object_id" db:"object_id"`         // 文章的唯一标识符
	ShareUserID  string `json:"share_user_id" db:"share_user_id"` // 发起分享的用户id
	EmbeddingURL string `json:"embedding_url" db:"embedding_url"` // iframe 嵌入的url
	Type         string `json:"type" db:"type"`                   // 共享数据的类型 文章、消息
	Token        string `json:"token" db:"token"`                 // 分享链接的 Token
	ExpireAt     int64  `json:"expire_at" db:"expire_at"`         // 分享链接的过期时间戳
	CreatedAt    int64  `json:"created_at" db:"created_at"`       // 创建时间戳
}

const (
	SHARE_TYPE_KNOWLEDGE    = "knowledge"
	SHARE_TYPE_SESSION      = "session"
	SHARE_TYPE_MESSAGE      = "message"
	SHARE_TYPE_SPACE_INVITE = "space_invite"
)
