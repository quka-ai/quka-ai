package types

import (
	"fmt"

	sq "github.com/Masterminds/squirrel"
)

type SpaceApplication struct {
	ID          string               `json:"id" db:"id"`
	SpaceID     string               `json:"space_id" db:"space_id"`
	UserID      string               `json:"user_id" db:"user_id"`
	Description string               `json:"description" db:"description"`
	Status      SpaceApplicationType `json:"status" db:"status"`
	UpdatedAt   int64                `json:"updated_at" db:"updated_at"`
	CreatedAt   int64                `json:"created_at" db:"created_at"`
}

type ListSpaceApplicationOptions struct {
	IDs      []string
	Keywords string
	Status   SpaceApplicationType
}

func (opts ListSpaceApplicationOptions) Apply(query *sq.SelectBuilder) {
	if len(opts.IDs) > 0 {
		*query = query.Where(sq.Eq{"id": opts.IDs})
	}
	if opts.Status != "" {
		*query = query.Where(sq.Eq{"status": opts.Status})
	}
	if opts.Keywords != "" {
		*query = query.InnerJoin(fmt.Sprintf("%s as u ON u.id = %s.user_id", TABLE_USER.Name(), TABLE_SPACE_APPLICATION.Name())).Where(sq.Or{sq.Eq{"u.id": opts.Keywords}, sq.Like{"u.name": "%" + opts.Keywords + "%"}, sq.Eq{"email": opts.Keywords}})
	}
}

type SpaceApplicationType string

const (
	SPACE_APPLICATION_NONE     SpaceApplicationType = "none"
	SPACE_APPLICATION_APPROVED SpaceApplicationType = "approved"
	SPACE_APPLICATION_WAITING  SpaceApplicationType = "waiting"
	SPACE_APPLICATION_REFUSE   SpaceApplicationType = "refused"
)
