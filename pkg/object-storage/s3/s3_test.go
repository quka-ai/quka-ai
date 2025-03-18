package s3

import (
	"context"
	"os"
	"testing"
	"time"
)

func Test_UploadKey(t *testing.T) {
	s3 := NewS3Client(os.Getenv("TEST_BREW_S3_ENDPOINT"), os.Getenv("TEST_BREW_S3_REGION"), os.Getenv("TEST_BREW_S3_BUCKET"), os.Getenv("TEST_BREW_S3_ACCESS_KEY"), os.Getenv("TEST_BREW_S3_SECRET_KEY"))

	resp, err := s3.GenClientUploadKey("test", "aaa.png", 1)
	if err != nil {
		t.Fatal(err)
	}

	t.Log(resp)
}

func Test_GenGetPreSignKey(t *testing.T) {
	s3 := NewS3Client(os.Getenv("TEST_BREW_S3_ENDPOINT"), os.Getenv("TEST_BREW_S3_REGION"), os.Getenv("TEST_BREW_S3_BUCKET"), os.Getenv("TEST_BREW_S3_ACCESS_KEY"), os.Getenv("TEST_BREW_S3_SECRET_KEY"))

	resp, err := s3.GenGetObjectPreSignURL("")
	if err != nil {
		t.Fatal(err)
	}

	t.Log(resp)
}

func newClient() *S3 {
	return NewS3Client(os.Getenv("TEST_BREW_S3_ENDPOINT"), os.Getenv("TEST_BREW_S3_REGION"), os.Getenv("TEST_BREW_S3_BUCKET"), os.Getenv("TEST_BREW_S3_ACCESS_KEY"), os.Getenv("TEST_BREW_S3_SECRET_KEY"))
}

func Test_GetObject(t *testing.T) {
	cli := newClient()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	res, err := cli.GetObject(ctx, "/test_tmp/test.docx")
	if err != nil {
		t.Fatal(err)
	}

	t.Log(string(res.FileType))

	file, err := os.Create("./test.docx")
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
