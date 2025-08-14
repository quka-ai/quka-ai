package s3_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/quka-ai/quka-ai/pkg/object-storage/s3"
	"github.com/quka-ai/quka-ai/pkg/testutils"
	"github.com/quka-ai/quka-ai/pkg/types"
)

func newClient() *s3.S3 {
	testutils.LoadEnvOrPanic()
	return s3.NewS3Client(
		os.Getenv("TEST_QUKA_SELFHOST_S3_ENDPOINT"),
		os.Getenv("TEST_QUKA_SELFHOST_S3_REGION"),
		os.Getenv("TEST_QUKA_SELFHOST_S3_BUCKET"),
		os.Getenv("TEST_QUKA_SELFHOST_S3_ACCESS_KEY"),
		os.Getenv("TEST_QUKA_SELFHOST_S3_SECRET_KEY"),
		s3.WithPathStyle(os.Getenv("TEST_QUKA_SELFHOST_S3_PATH_STYLE") == "true"), // MinIO需要路径样式URL
	)
}

func Test_UploadKey(t *testing.T) {
	s3 := newClient()

	resp, err := s3.GenClientUploadKey("test/aaa.png", 1)
	if err != nil {
		t.Fatal(err)
	}

	t.Log(resp)
}

func Test_GenGetPreSignKey(t *testing.T) {
	s3 := newClient()

	resp, err := s3.GenGetObjectPreSignURL("/assets/s3/gPyofSEORU0ZskWmPh9CLUfv5PWjmXBZ/knowledge/20250723/a0f91d468c918b98892aed5947e42423.pdf")
	if err != nil {
		t.Fatal(err)
	}

	t.Log(resp)
}

func Test_GetObject(t *testing.T) {
	cli := newClient()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	res, err := cli.GetObject(ctx, "/assets/s3/gPyofSEORU0ZskWmPh9CLUfv5PWjmXBZ/knowledge/20250723/a0f91d468c918b98892aed5947e42423.pdf")
	if err != nil {
		t.Fatal(err)
	}

	t.Log(string(res.FileType))

	file, err := os.Create("./test.pdf")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	n, err := file.Write(res.File)
	if err != nil {
		t.Fatal(err)
	}

	t.Log(n)
}

func Test_Upload(t *testing.T) {
	s3 := newClient()

	// 创建测试文件内容
	testContent := "这是一个测试文件的内容\n用于测试S3文件上传功能\nTest file content for S3 upload111"
	reader := bytes.NewReader([]byte(testContent))

	// 定义上传路径和文件名
	fileName := "test-upload-file1.txt"
	fullPath := types.GenS3FilePath("testspace", "image", fileName)

	// 执行上传
	err := s3.Upload(fullPath, reader)
	if err != nil {
		t.Fatal("上传文件失败:", err)
	}

	t.Log("文件上传成功")

	// 验证上传成功 - 尝试下载刚上传的文件
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	res, err := s3.GetObject(ctx, fullPath)
	if err != nil {
		t.Fatal("下载上传的文件失败:", err)
	}

	// 验证文件内容是否一致
	downloadedContent := string(res.File)
	if downloadedContent != testContent {
		t.Fatalf("上传的文件内容不匹配。期望: %s, 实际: %s", testContent, downloadedContent)
	}

	t.Log("文件内容验证成功")
	t.Log("文件类型:", res.FileType)

	// 清理测试数据 - 删除上传的文件
	err = s3.Delete(fullPath)
	if err != nil {
		t.Log("删除测试文件失败 (这不会导致测试失败):", err)
	} else {
		t.Log("测试文件清理成功")
	}
}

func Test_UploadOnly(t *testing.T) {
	s3 := newClient()

	// 创建测试文件内容
	testContent := "简化测试：仅测试上传和下载功能\nSimplified test: upload and download only"
	reader := bytes.NewReader([]byte(testContent))

	// 定义上传路径和文件名
	fileName := "test-upload-only.txt"
	fullPath := types.GenS3FilePath("testspace1", "document", fileName)

	t.Log("生成的文件路径:", fullPath)

	// 执行上传
	err := s3.Upload(fullPath, reader)
	if err != nil {
		t.Fatal("上传文件失败:", err)
	}

	t.Log("文件上传成功")

	// 验证上传成功 - 尝试下载刚上传的文件
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	res, err := s3.GetObject(ctx, fullPath)
	if err != nil {
		t.Fatal("下载上传的文件失败:", err)
	}

	fmt.Println(res)

	// 验证文件内容是否一致
	downloadedContent := string(res.File)
	if downloadedContent != testContent {
		t.Fatalf("上传的文件内容不匹配。期望: %s, 实际: %s", testContent, downloadedContent)
	}

	err = s3.Delete(fullPath)
	if err != nil {
		t.Fatal("删除文件失败:", err)
	}

	t.Log("文件内容验证成功")
	t.Log("文件类型:", res.FileType)
	t.Log("注意：此测试不会自动删除文件，请手动清理:", fullPath)
}
