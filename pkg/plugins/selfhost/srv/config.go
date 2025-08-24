package srv

import "github.com/quka-ai/quka-ai/app/core"

type CustomConfig struct {
	Host          string                   `toml:"host"`
	ObjectStorage core.ObjectStorageDriver `toml:"object_storage"`
	EncryptKey    string                   `toml:"encrypt_key"`

	ChunkService ChunkService `toml:"chunk_service"`
}

type ChunkService struct {
	Enabled bool   `toml:"enabled"` // gRPC enabled
	Address string `toml:"address"` // gRPC server address
	Timeout int    `toml:"timeout"` // timeout in seconds
}
