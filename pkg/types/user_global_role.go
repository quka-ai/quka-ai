package types

import sq "github.com/Masterminds/squirrel"

// UserGlobalRole 全局用户角色表结构
type UserGlobalRole struct {
	ID        int64  `json:"id" db:"id"`                 // 自增主键
	UserID    string `json:"user_id" db:"user_id"`       // 用户ID
	Appid     string `json:"appid" db:"appid"`           // 租户ID
	Role      string `json:"role" db:"role"`             // 全局角色
	CreatedAt int64  `json:"created_at" db:"created_at"` // 创建时间
	UpdatedAt int64  `json:"updated_at" db:"updated_at"` // 更新时间
}

// ListUserGlobalRoleOptions 查询选项
type ListUserGlobalRoleOptions struct {
	UserID string
	Appid  string
	Role   string
}

func (opts ListUserGlobalRoleOptions) Apply(query *sq.SelectBuilder) {
	if opts.UserID != "" {
		*query = query.Where(sq.Eq{"user_id": opts.UserID})
	}
	if opts.Appid != "" {
		*query = query.Where(sq.Eq{"appid": opts.Appid})
	}
	if opts.Role != "" {
		*query = query.Where(sq.Eq{"role": opts.Role})
	}
}

// 全局角色常量
const (
	GlobalRoleChief  = "role-chief"  // 超级管理员
	GlobalRoleAdmin  = "role-admin"  // 管理员
	GlobalRoleMember = "role-member" // 普通用户
)

// DefaultGlobalRole 默认全局角色
const DefaultGlobalRole = GlobalRoleMember
