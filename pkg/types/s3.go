package types

import (
	"path/filepath"
	"time"
)

func GenS3FilePath(spaceID, _type, fileName string) string {
	return filepath.Join(FIXED_S3_UPLOAD_PATH_PREFIX, spaceID, _type, time.Now().Format("20060102"), fileName)
}
