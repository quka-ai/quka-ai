package s3

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3 struct {
	Endpoint string
	Region   string
	Bucket   string
	ak       string
	sk       string
	cli      *s3.Client
}

func NewS3Client(endpoint, region, bucket, ak, sk string) *S3 {
	cli := &S3{
		Endpoint: endpoint,
		Region:   region,
		Bucket:   bucket,
		ak:       ak,
		sk:       sk,
	}

	if _, err := cli.DefaultConfig(context.Background()); err != nil {
		panic(err)
	}

	return cli
}

func (s *S3) DefaultConfig(ctx context.Context) (aws.Config, error) {
	cfg, err := config.LoadDefaultConfig(
		ctx,
		config.WithCredentialsProvider(credentials.StaticCredentialsProvider{
			Value: aws.Credentials{
				AccessKeyID: s.ak, SecretAccessKey: s.sk,
			},
		}),
		config.WithRegion(s.Region),
		config.WithEndpointResolverWithOptions(aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{
				URL:           s.Endpoint,
				SigningRegion: s.Region,
			}, nil
		})))
	if err != nil {
		return aws.Config{}, err
	}

	s.cli = s3.NewFromConfig(cfg)
	return cfg, nil
}

func (s *S3) GenGetObjectPreSignURL(filePath string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	s3PresignClient := s3.NewPresignClient(s.cli)
	req, err := s3PresignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.Bucket),
		Key:    aws.String(strings.TrimPrefix(filePath, "/")),
	}, s3.WithPresignExpires(time.Minute*5))
	if err != nil {
		return "", err
	}

	return req.URL, nil
}

type GetObjectResult struct {
	File     []byte
	FileType string
}

func (s *S3) GetObject(ctx context.Context, key string) (*GetObjectResult, error) {
	// s3M :=	manager.NewDownloader(s.cli)
	// s3M.Download()
	key = strings.TrimPrefix(key, "/")

	resp, err := s.cli.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}

	fileContent, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	fr := bytes.NewReader(fileContent)
	// 读取前 512 字节
	buffer := make([]byte, 512)
	_, err = fr.Read(buffer)
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("Error reading file: %w", err)
	}

	// 检测 MIME 类型
	mimeType := http.DetectContentType(buffer)

	return &GetObjectResult{
		File:     fileContent,
		FileType: mimeType,
	}, nil
}

func (s *S3) GenClientUploadKey(filePath, file string, contentLength int64) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	filePath = strings.TrimPrefix(filePath, "/")

	s3PresignClient := s3.NewPresignClient(s.cli)
	req, err := s3PresignClient.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.Bucket),
		Key:           aws.String(filepath.Join(filePath, file)),
		ContentLength: contentLength,
	}, s3.WithPresignExpires(20*time.Second))
	if err != nil {
		return "", err
	}

	return req.URL, nil
}

func (s *S3) Upload(filePath, file string, body io.Reader) error {
	filePath = strings.TrimPrefix(filePath, "/")
	// cfg, err := config.LoadDefaultConfig(
	// 	context.Background(),
	// 	config.WithCredentialsProvider(credentials.StaticCredentialsProvider{
	// 		Value: aws.Credentials{
	// 			AccessKeyID: s.ak, SecretAccessKey: s.sk,
	// 		},
	// 	}),
	// 	config.WithRegion(s.Region),
	// 	config.WithEndpointResolverWithOptions(aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
	// 		return aws.Endpoint{
	// 			URL: s.Endpoint,
	// 		}, nil
	// 	})))
	// if err != nil {
	// 	return err
	// }
	// s3Client := s3.NewFromConfig(cfg)
	s3Manager := manager.NewUploader(s.cli)

	_, err := s3Manager.Upload(context.Background(), &s3.PutObjectInput{
		Bucket: aws.String(s.Bucket),
		Key:    aws.String(filepath.Join(filePath, file)),
		Body:   body,
	})
	if err != nil {
		return err
	}
	return nil
}

func (s *S3) Delete(fullPath string) error {
	cfg, err := config.LoadDefaultConfig(
		context.Background(),
		config.WithCredentialsProvider(credentials.StaticCredentialsProvider{
			Value: aws.Credentials{
				AccessKeyID: s.ak, SecretAccessKey: s.sk,
			},
		}),
		config.WithRegion(s.Region),
		config.WithEndpointResolverWithOptions(aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{
				URL: s.Endpoint,
			}, nil
		})))
	if err != nil {
		return err
	}
	s3Client := s3.NewFromConfig(cfg)
	_, err = s3Client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
		Bucket: aws.String(s.Bucket),
		Key:    aws.String(fullPath),
	})
	if err != nil {
		return err
	}
	return nil
}
