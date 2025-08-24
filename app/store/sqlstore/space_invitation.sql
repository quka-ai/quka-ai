CREATE TABLE IF NOT EXISTS quka_space_invitation (
    id BIGSERIAL PRIMARY KEY, -- 主键ID
    inviter_id VARCHAR(32) NOT NULL, -- 邀请人ID
    invitee_email VARCHAR(255) NOT NULL, -- 被邀请人邮箱
    space_id VARCHAR(32) NOT NULL, -- 邀请所属的空间ID
    role VARCHAR(50) NOT NULL, -- 被邀请人权限（如：admin, member, guest）
    invite_status SMALLINT NOT NULL, -- 邀请状态（1=待接受，2=已接受，3=已拒绝，4=已过期）
    created_at BIGINT NOT NULL, -- 邀请创建时间
    expired_at BIGINT NOT NULL, -- 邀请过期时间
    updated_at BIGINT NOT NULL -- 最后更新时间
);

-- 添加字段注释
COMMENT ON COLUMN quka_space_invitation.id IS '主键ID';
COMMENT ON COLUMN quka_space_invitation.inviter_id IS '邀请人ID';
COMMENT ON COLUMN quka_space_invitation.invitee_email IS '被邀请人邮箱';
COMMENT ON COLUMN quka_space_invitation.space_id IS '邀请所属的空间ID';
COMMENT ON COLUMN quka_space_invitation.role IS '被邀请人权限（如：admin, member, guest）';
COMMENT ON COLUMN quka_space_invitation.invite_status IS '邀请状态（1=待接受，2=已接受，3=已拒绝，4=已过期）';
COMMENT ON COLUMN quka_space_invitation.created_at IS '邀请创建时间';
COMMENT ON COLUMN quka_space_invitation.expired_at IS '邀请过期时间';
COMMENT ON COLUMN quka_space_invitation.updated_at IS '最后更新时间';

CREATE INDEX IF NOT EXISTS idx_inviter_id ON quka_space_invitation (inviter_id);
CREATE INDEX IF NOT EXISTS idx_space_status ON quka_space_invitation (space_id, invite_status);
