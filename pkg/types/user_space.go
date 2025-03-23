package types

import (
	"fmt"

	sq "github.com/Masterminds/squirrel"
)

// UserSpace 数据表结构
type UserSpace struct {
	ID        int64  `json:"id" db:"id"`                 // 自增主键
	UserID    string `json:"user_id" db:"user_id"`       // 用户ID
	SpaceID   string `json:"space_id" db:"space_id"`     // 空间ID
	Role      string `json:"role" db:"role"`             // 用户在空间中的角色
	CreatedAt int64  `json:"created_at" db:"created_at"` // 创建时间，存储为时间戳
}

type ListUserSpaceOptions struct {
	UserID   string
	SpaceID  string
	Keywords string
}

func (opts ListUserSpaceOptions) Apply(query *sq.SelectBuilder) {
	if opts.UserID != "" {
		*query = query.Where(sq.Eq{"user_id": opts.UserID})
	}
	if opts.SpaceID != "" {
		*query = query.Where(sq.Eq{"space_id": opts.SpaceID})
	}
	if opts.Keywords != "" {
		*query = query.InnerJoin(fmt.Sprintf("%s as u ON u.id = %s.user_id", TABLE_USER.Name(), TABLE_USER_SPACE.Name())).Where(sq.Or{sq.Eq{"u.id": opts.Keywords}, sq.Like{"u.name": "%" + opts.Keywords + "%"}, sq.Eq{"email": opts.Keywords}})
	}
}

type Space struct {
	SpaceID     string `json:"space_id" db:"space_id"` // 空间ID
	Title       string `json:"title" db:"title"`
	Description string `json:"description" db:"description"`
	BasePrompt  string `json:"base_prompt" db:"base_prompt"`
	ChatPrompt  string `json:"chat_prompt" db:"chat_prompt"`
	JoinLeaf    int64  `json:"join_leaf" db:"join_leaf"`
	CreatedAt   int64  `json:"created_at" db:"created_at"` // 创建时间，存储为时间戳
}

type UserSpaceDetail struct {
	UserID      string `json:"user_id"`
	SpaceID     string `json:"space_id"`
	Role        string `json:"role"`
	Title       string `json:"title"`
	Description string `json:"description"`
	BasePrompt  string `json:"base_prompt"`
	ChatPrompt  string `json:"chat_prompt"`
	CreatedAt   int64  `json:"created_at"`
}
