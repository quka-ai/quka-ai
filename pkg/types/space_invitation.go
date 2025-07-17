package types

import sq "github.com/Masterminds/squirrel"

type Invitation struct {
	ID           int64                 `json:"id" db:"id"`                       // 主键ID
	Appid        string                `json:"appid" db:"appid"`                 // 应用id
	InviterID    string                `json:"inviter_id" db:"inviter_id"`       // 邀请人ID
	InviteeEmail string                `json:"invitee_email" db:"invitee_email"` // 被邀请人邮箱
	SpaceID      string                `json:"space_id" db:"space_id"`           // 邀请所属的空间ID
	Role         string                `json:"role" db:"role"`                   // 被邀请人权限（1=管理员，2=成员，3=访客）
	InviteStatus SpaceInvitationStatus `json:"invite_status" db:"invite_status"` // 邀请状态（1=待接受，2=已接受，3=已拒绝，4=已过期）
	CreatedAt    int64                 `json:"created_at" db:"created_at"`       // 邀请创建时间
	ExpiredAt    int64                 `json:"expired_at" db:"expired_at"`       // 邀请过期时间
	UpdatedAt    int64                 `json:"updated_at" db:"updated_at"`       // 最后更新时间
}

type SpaceInvitationStatus int32

const (
	SPACE_INVITATION_STATUS_PENDING SpaceInvitationStatus = iota
	SPACE_INVITATION_STATUS_ACCEPTED
	SPACE_INVITATION_STATUS_REJECTED
	SPACE_INVITATION_STATUS_EXPIRED
)

type ListSpaceInvitationOptions struct {
	Status SpaceInvitationStatus
}

func (opts ListSpaceInvitationOptions) Apply(query *sq.SelectBuilder) {
	if opts.Status != 0 {
		*query = query.Where(sq.Eq{"status": opts.Status})
	}
}
