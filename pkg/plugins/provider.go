package plugins

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/quka-ai/quka-ai/app/core"
	"github.com/quka-ai/quka-ai/pkg/object-storage/s3"
	"github.com/quka-ai/quka-ai/pkg/types"
)

func Setup(install func(p core.Plugins), mode string) {
	p := provider[mode]
	if p == nil {
		panic("Setup mode not found: " + mode)
	}
	install(p())
}

var provider = make(map[string]core.SetupFunc)

func RegisterProvider(key string, p core.Plugins) {
	provider[key] = func() core.Plugins {
		return p
	}
}

func SetupObjectStorage(cfg core.ObjectStorageDriver) core.FileStorage {
	var s core.FileStorage
	switch strings.ToLower(cfg.Driver) {
	case "s3":
		s3Cfg := cfg.S3
		s = &S3FileStorage{
			StaticDomain: cfg.StaticDomain,
			S3:           s3.NewS3Client(s3Cfg.Endpoint, s3Cfg.Region, s3Cfg.Bucket, s3Cfg.AccessKey, s3Cfg.SecretKey, s3.WithPathStyle(s3Cfg.UsePathStyle)),
		}
	case "local":
		s = &LocalFileStorage{
			StaticDomain: cfg.StaticDomain,
		}
	default:
		s = &NoneFileStorage{}
	}

	return s
}

type NoneFileStorage struct {
}

func (lfs *NoneFileStorage) GetStaticDomain() string {
	return ""
}

func (lfs *NoneFileStorage) GenGetObjectPreSignURL(url string) (string, error) {
	return "", fmt.Errorf("Unsupported")
}

func (lfs *NoneFileStorage) GenUploadFileMeta(fullPath string, _ int64) (core.UploadFileMeta, error) {
	return core.UploadFileMeta{}, fmt.Errorf("Unsupported")
}

func (lfs *NoneFileStorage) SaveFile(fullPath string, content []byte) error {
	return fmt.Errorf("Unsupported")
}

func (lfs *NoneFileStorage) DeleteFile(fullFilePath string) error {
	return fmt.Errorf("Unsupported")
}

func (fs *NoneFileStorage) DownloadFile(ctx context.Context, filePath string) (*s3.GetObjectResult, error) {
	return nil, fmt.Errorf("Unsupported")
}

type LocalFileStorage struct {
	StaticDomain string
}

func (lfs *LocalFileStorage) GetStaticDomain() string {
	return lfs.StaticDomain
}

func (lfs *LocalFileStorage) GenUploadFileMeta(fullPath string, _ int64) (core.UploadFileMeta, error) {
	return core.UploadFileMeta{
		FullPath: fullPath,
		Domain:   lfs.StaticDomain,
	}, nil
}

// SaveFile stores a file on the local file system.
func (lfs *LocalFileStorage) SaveFile(fullPath string, content []byte) error {
	// Check if the directory exists
	filePath := filepath.Dir(fullPath)
	_, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		// If the directory doesn't exist, create it
		err := os.MkdirAll(filePath, 0755)
		if err != nil {
			return fmt.Errorf("failed to create directory: %v", err)
		}
	} else if err != nil {
		// If there's an error other than "not exist", return it
		return fmt.Errorf("failed to check directory: %v", err)
	}

	// Save the file
	err = os.WriteFile(fullPath, content, 0644)
	if err != nil {
		return fmt.Errorf("failed to save file: %v", err)
	}

	return nil
}

func (lfs *LocalFileStorage) DownloadFile(ctx context.Context, filePath string) (*s3.GetObjectResult, error) {
	_, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to create directory: %v", err)
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("Error opening file: %v", err)
	}
	defer file.Close() // 确保文件在使用后关闭

	bytes, _ := io.ReadAll(file)
	// 读取文件的前 512 个字节
	file.Seek(0, 0)
	buffer := make([]byte, 512)
	_, err = file.Read(buffer)
	if err != nil {
		return nil, fmt.Errorf("Error reading file: %v", err)
	}

	// 检测文件类型
	mimeType := http.DetectContentType(buffer)
	return &s3.GetObjectResult{
		File:     bytes,
		FileType: mimeType,
	}, nil
}

// DeleteFile deletes a file from the local file system using the full file path.
func (lfs *LocalFileStorage) DeleteFile(fullFilePath string) error {
	err := os.Remove(fullFilePath)
	if err != nil {
		return fmt.Errorf("failed to delete file: %v", err)
	}
	return nil
}

func (lfs *LocalFileStorage) GenGetObjectPreSignURL(url string) (string, error) {
	return url, nil
}

type S3FileStorage struct {
	StaticDomain string
	*s3.S3
}

func (fs *S3FileStorage) GetStaticDomain() string {
	return fs.StaticDomain
}

func (fs *S3FileStorage) GenUploadFileMeta(fullPath string, contentLength int64) (core.UploadFileMeta, error) {
	key, err := fs.S3.GenClientUploadKey(fullPath, contentLength)
	if err != nil {
		return core.UploadFileMeta{}, err
	}
	return core.UploadFileMeta{
		FullPath:       fullPath,
		UploadEndpoint: key,
	}, nil
}

// SaveFile stores a file
func (fs *S3FileStorage) SaveFile(fullPath string, content []byte) error {
	return fs.Upload(fullPath, bytes.NewReader(content))
}

func (fs *S3FileStorage) DownloadFile(ctx context.Context, filePath string) (*s3.GetObjectResult, error) {
	return fs.GetObject(ctx, filePath)
}

// DeleteFile deletes a file
func (fs *S3FileStorage) DeleteFile(fullFilePath string) error {
	return fs.Delete(fullFilePath)
}

func (fs *S3FileStorage) GenGetObjectPreSignURL(_url string) (string, error) {
	res, err := url.Parse(_url)
	if err != nil {
		return "", err
	}

	_url, _ = url.QueryUnescape(res.RequestURI())
	return fs.S3.GenGetObjectPreSignURL(_url)
}

type Assistant interface {
	InitAssistantMessage(ctx context.Context, msgID string, seqID int64, userMessage *types.ChatMessage, ext types.ChatMessageExt) (*types.ChatMessage, error)
	RequestAssistant(ctx context.Context, reqMsgInfo *types.ChatMessage, receiver types.Receiver, aiCallOptions *types.AICallOptions) error
}
