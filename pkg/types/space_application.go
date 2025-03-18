package types

import (
	"fmt"

	sq "github.com/Masterminds/squirrel"
)

type SpaceApplication struct {
	ID        string `json:"id" db:"id"`
	SpaceID   string `json:"space_id" db:"space_id"`
	UserID    string `json:"user_id" db:"user_id"`
	UserName  string `json:"user_name" db:"user_name"`
	UserEmail string `json:"user_email" db:"user_email"`
	Desc      string `json:"desc" db:"desc"`
	Status    string `json:"status" db:"status"`
	UpdatedAt int64  `json:"updated_at" db:"updated_at"`
	CreatedAt int64  `json:"created_at" db:"created_at"`
}

type ListSpaceApplicationOptions struct {
	UserIDs   []string
	UserName  string
	UserEmail string
}

func (opts ListSpaceApplicationOptions) Apply(query *sq.SelectBuilder) {
	if len(opts.UserIDs) > 0 {
		*query = query.Where(sq.Eq{"user_id": opts.UserIDs})
	}
	if opts.UserName != "" {
		*query = query.Where(sq.Like{"user_name": fmt.Sprintf("%%%s%%", opts.UserName)})
	}
	if opts.UserName != "" {
		*query = query.Where(sq.Like{"user_email": fmt.Sprintf("%%%s%%", opts.UserEmail)})
	}
}

const (
	SPACE_APPLICATION_NONE    = "none"
	SPACE_APPLICATION_ACCESS  = "access"
	SPACE_APPLICATION_WAITING = "waiting"
	SPACE_APPLICATION_REFUSE  = "refuse"
)
