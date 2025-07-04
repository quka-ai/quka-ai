package sqlstore

import (
	"embed"
)

//go:embed *.sql
var CreateTableFiles embed.FS
