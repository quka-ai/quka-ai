package rednote_test

import (
	"context"
	"os"
	"testing"

	"github.com/quka-ai/quka-ai/pkg/plugins"
	"github.com/quka-ai/quka-ai/pkg/reader/rednote"
)

func TestLoadCookie(t *testing.T) {
	reader, err := rednote.NewReader(os.Getenv("READNOTE_COOKIE_PATH"))
	if err != nil {
		t.Fatal(err)
	}
	cookies, err := reader.ReadCookies()
	if err != nil {
		t.Fatal(err)
	}
	t.Log(cookies)
}

func TestReader(t *testing.T) {
	url := os.Getenv("READNOTE_URL")
	t.Log("URL:", url)
	if url == "" {
		t.Fatal("URL is empty")
	}
	result, err := rednote.Read(url)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(result)

	objectDriver := plugins.SetupObjectStorage(plugins.ObjectStorageDriver{
		StaticDomain: "test.com",
		Driver:       "s3",
		S3: &plugins.S3Config{
			Bucket:    os.Getenv("TEST_BREW_S3_BUCKET"),
			Region:    os.Getenv("TEST_BREW_S3_REGION"),
			Endpoint:  os.Getenv("TEST_BREW_S3_ENDPOINT"),
			AccessKey: os.Getenv("TEST_BREW_S3_ACCESS_KEY"),
			SecretKey: os.Getenv("TEST_BREW_S3_SECRET_KEY"),
		},
	})
	res, err := rednote.ParseRedNote(context.Background(), "123", result, objectDriver)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(res)
}
