package rednote

import (
	"os"
	"testing"
)

func TestLoadCookie(t *testing.T) {
	reader, err := NewReader(os.Getenv("READNOTE_COOKIE_PATH"))
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
	result, err := Read(os.Getenv("READNOTE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	t.Log(result)
}
