package types

import (
	"github.com/davidscottmills/goeditorjs"
)

type BlockContent struct {
	Blocks  []goeditorjs.EditorJSBlock `json:"blocks"`
	Time    int64                      `json:"time"` // javascript time
	Version string                     `json:"version"`
}


