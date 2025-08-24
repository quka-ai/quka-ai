package types

import sq "github.com/Masterminds/squirrel"

// User 数据表结构，请注意，该结构应该定义在 "your/path/types" 中
type User struct {
	ID        string `json:"id" db:"id"`                 // 用户ID，主键
	Appid     string `json:"appid" db:"appid"`           // 租户id
	Name      string `json:"name" db:"name"`             // 用户名
	Avatar    string `json:"avatar" db:"avatar"`         // 用户头像URL
	Email     string `json:"email" db:"email"`           // 用户邮箱，唯一约束
	Password  string `json:"-" db:"password"`            // 用户密码
	Salt      string `json:"-" db:"salt"`                // 用户密码盐值
	Source    string `json:"-" db:"source"`              // 用户注册来源
	PlanID    string `json:"plan_id" db:"plan_id"`       // 会员方案ID
	UpdatedAt int64  `json:"updated_at" db:"updated_at"` // 更新时间，Unix时间戳
	CreatedAt int64  `json:"created_at" db:"created_at"` // 创建时间，Unix时间戳
}

// UserWithRole 用户信息（包含全局角色）
type UserWithRole struct {
	User
	GlobalRole string `json:"global_role"` // 用户全局角色
}

type ListUserOptions struct {
	Appid     string
	IDs       []string
	Email     string
	Source    string // 用户来源过滤
	Name      string // 用户名模糊搜索
	EmailLike string // 邮箱模糊搜索
}

func (opt ListUserOptions) Apply(query *sq.SelectBuilder) {
	if opt.Appid != "" {
		*query = query.Where(sq.Eq{"appid": opt.Appid})
	}
	if len(opt.IDs) > 0 {
		*query = query.Where(sq.Eq{"id": opt.IDs})
	}
	if opt.Email != "" {
		*query = query.Where(sq.Eq{"email": opt.Email})
	}
	if opt.Source != "" {
		*query = query.Where(sq.Eq{"source": opt.Source})
	}
	if opt.Name != "" {
		*query = query.Where(sq.Like{"name": "%" + opt.Name + "%"})
	}
	if opt.EmailLike != "" {
		*query = query.Where(sq.Like{"email": "%" + opt.EmailLike + "%"})
	}
}

type UserTokenMeta struct {
	UserID   string `json:"user_id"`
	Appid    string `json:"appid"`
	ExpireAt int64  `json:"expire_at"`
}
