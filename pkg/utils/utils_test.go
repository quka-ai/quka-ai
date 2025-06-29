package utils

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenRandomID(t *testing.T) {
	SetupIDWorker(1)

	t.Log(GenSpecIDStr(), len(GenSpecIDStr()))
}

func Test_ParseAcceptLanguage(t *testing.T) {
	res := ParseAcceptLanguage("zh-CN,zh;q=0.9,en-US;q=0.8,en;q=0.7")
	t.Log(res)
}

func Test_Crypt(t *testing.T) {
	// 密钥和明文
	key := []byte("examplekey123456")
	plaintext := []byte("Sensitive Data to be encrypted")

	// 加密
	ciphertext, err := EncryptCFB(plaintext, key)
	if err != nil {
		log.Fatalf("加密失败: %v", err)
	}
	fmt.Printf("加密后的数据: %s\n", ciphertext)

	// 解密
	decrypted, err := DecryptCFB(ciphertext, key)
	if err != nil {
		log.Fatalf("解密失败: %v", err)
	}
	fmt.Printf("解密后的数据: %s\n", string(decrypted))

	assert.Equal(t, plaintext, decrypted)
}

func TestConverSVGToPNG(t *testing.T) {
	_, err := ConvertSVGToPNG([]byte(`<?xml version="1.0" standalone="no"?>
<!DOCTYPE svg PUBLIC "-//W3C//DTD SVG 20010904//EN"
 "http://www.w3.org/TR/2001/REC-SVG-20010904/DTD/svg10.dtd">
<svg version="1.0" xmlns="http://www.w3.org/2000/svg"
 width="1280.000000pt" height="1280.000000pt" viewBox="0 0 1280.000000 1280.000000"
 preserveAspectRatio="xMidYMid meet">
<metadata>
Created by potrace 1.15, written by Peter Selinger 2001-2017
</metadata>
<g transform="translate(0.000000,1280.000000) scale(0.100000,-0.100000)"
fill="#000000" stroke="none">
<path d="M2150 12023 c-198 -31 -354 -79 -498 -152 -171 -85 -273 -158 -406
-289 -225 -222 -367 -481 -438 -799 -30 -135 -33 -483 -4 -613 140 -645 607
-1110 1246 -1241 147 -30 413 -33 555 -7 337 63 620 212 855 451 411 417 553
1029 369 1588 -186 562 -686 978 -1272 1058 -74 10 -352 13 -407 4z"/>
<path d="M6130 12010 c-869 -154 -1440 -994 -1264 -1860 60 -295 207 -564 428
-786 411 -411 988 -560 1546 -399 263 75 497 219 696 428 497 519 571 1321
178 1924 -240 371 -618 619 -1051 693 -151 26 -385 26 -533 0z"/>
<path d="M10215 12015 c-546 -86 -1011 -457 -1213 -970 -190 -481 -132 -1022
156 -1450 393 -584 1123 -834 1792 -614 471 155 847 535 998 1009 59 185 67
245 67 480 -1 183 -4 229 -23 313 -70 313 -204 562 -425 785 -240 244 -528
391 -872 447 -109 18 -367 18 -480 0z"/>
<path d="M2193 7959 c-540 -52 -1026 -390 -1258 -874 -248 -517 -197 -1130
133 -1595 85 -120 241 -276 362 -363 164 -116 387 -216 579 -257 303 -66 623
-39 912 75 458 181 818 590 939 1065 203 796 -245 1616 -1022 1875 -218 72
-425 96 -645 74z"/>
<path d="M6245 7959 c-267 -28 -533 -128 -752 -282 -113 -79 -291 -257 -370
-370 -338 -481 -383 -1097 -116 -1619 77 -151 161 -266 288 -393 271 -271 589
-419 984 -456 197 -19 441 13 640 83 117 41 283 127 388 201 113 79 291 257
370 370 384 546 384 1268 0 1814 -79 113 -257 291 -370 370 -310 218 -695 320
-1062 282z"/>
<path d="M10305 7959 c-573 -60 -1067 -424 -1290 -950 -165 -390 -165 -828 0
-1218 292 -691 1043 -1081 1775 -922 603 132 1069 602 1202 1214 19 88 22 132
22 317 0 239 -14 332 -75 507 -242 690 -920 1126 -1634 1052z"/>
<path d="M2164 3889 c-607 -71 -1128 -499 -1312 -1079 -59 -186 -67 -245 -67
-480 1 -183 4 -229 23 -313 71 -318 213 -578 438 -799 220 -216 470 -350 781
-421 85 -19 127 -22 318 -21 187 0 234 3 315 21 290 65 551 202 760 399 554
523 648 1376 222 2009 -326 485 -902 751 -1478 684z"/>
<path d="M6227 3890 c-382 -48 -701 -208 -963 -483 -497 -519 -571 -1321 -178
-1924 240 -370 619 -619 1052 -694 154 -26 370 -26 525 1 504 86 934 412 1152
873 174 368 198 777 69 1166 -180 542 -645 943 -1214 1046 -102 19 -349 27
-443 15z"/>
<path d="M10277 3890 c-592 -68 -1114 -491 -1302 -1054 -271 -812 147 -1685
945 -1974 186 -67 331 -92 542 -92 420 0 800 156 1097 452 221 220 363 481
433 795 30 136 33 483 4 613 -69 320 -211 586 -430 804 -231 231 -500 375
-813 436 -123 24 -354 34 -476 20z"/>
</g>
</svg>
`))
	if err != nil {
		t.Fatal(err)
	}
}

func TestMaskString(t *testing.T) {
	if MaskString("1234567890", 6, 4) != "123456******7890" {
		t.Fatal("mask fail")
	}
}

func TestImageResponseToBase64(t *testing.T) {
	// 测试空响应
	_, err := ImageResponseToBase64(nil)
	if err == nil {
		t.Fatal("expected error for nil response")
	}

	// 创建一个模拟的图片数据（1x1像素的PNG）
	pngData := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D,
		0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53, 0xDE, 0x00, 0x00, 0x00,
		0x0C, 0x49, 0x44, 0x41, 0x54, 0x08, 0xD7, 0x63, 0xF8, 0x0F, 0x00, 0x00,
		0x01, 0x00, 0x01, 0x5C, 0xC2, 0x8A, 0xBD, 0x00, 0x00, 0x00, 0x00, 0x49,
		0x45, 0x4E, 0x44, 0xAE, 0x42, 0x60, 0x82,
	}

	// 创建一个模拟的HTTP响应
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader(pngData)),
	}
	resp.Header.Set("Content-Type", "image/png")

	base64Result, err := ImageResponseToBase64(resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.HasPrefix(base64Result, "data:image/png;base64,") {
		t.Fatal("result should start with data:image/png;base64,")
	}

	t.Logf("Base64 result: %s", base64Result)
}

func TestImageBytesToBase64(t *testing.T) {
	// 测试空数据
	_, err := ImageBytesToBase64([]byte{}, "image/png")
	if err == nil {
		t.Fatal("expected error for empty data")
	}

	// 测试不支持的类型
	_, err = ImageBytesToBase64([]byte{1, 2, 3}, "application/pdf")
	if err == nil {
		t.Fatal("expected error for unsupported type")
	}

	// 测试正常情况
	testData := []byte{1, 2, 3, 4, 5}
	result, err := ImageBytesToBase64(testData, "image/jpeg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "data:image/jpeg;base64,AQIDBAU="
	if result != expected {
		t.Fatalf("expected %s, got %s", expected, result)
	}
}

func TestIsValidImageType(t *testing.T) {
	validTypes := []string{
		"image/jpeg",
		"image/jpg",
		"image/png",
		"image/webp",
		"image/bmp",
		"image/gif",
	}

	for _, validType := range validTypes {
		if !isValidImageType(validType) {
			t.Fatalf("expected %s to be valid", validType)
		}
	}

	invalidTypes := []string{
		"application/pdf",
		"text/html",
		"image/svg+xml",
		"",
	}

	for _, invalidType := range invalidTypes {
		if isValidImageType(invalidType) {
			t.Fatalf("expected %s to be invalid", invalidType)
		}
	}
}

func TestFileBytesToBase64(t *testing.T) {
	// 测试空数据
	_, err := FileBytesToBase64([]byte{}, "text/plain")
	if err == nil {
		t.Fatal("expected error for empty data")
	}

	// 测试正常情况
	testData := []byte("Hello, World!")
	result, err := FileBytesToBase64(testData, "text/plain")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "data:text/plain;base64,SGVsbG8sIFdvcmxkIQ=="
	if result != expected {
		t.Fatalf("expected %s, got %s", expected, result)
	}

	// 测试空Content-Type
	result2, err := FileBytesToBase64(testData, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected2 := "data:application/octet-stream;base64,SGVsbG8sIFdvcmxkIQ=="
	if result2 != expected2 {
		t.Fatalf("expected %s, got %s", expected2, result2)
	}
}

func TestIsValidFileTypeForLLM(t *testing.T) {
	// 测试支持的类型
	supportedTypes := []string{
		"image/jpeg",
		"image/png",
		"text/plain",
		"application/json",
		"audio/mpeg",
		"video/mp4",
		"application/pdf",
	}

	for _, supportedType := range supportedTypes {
		if !IsValidFileTypeForLLM(supportedType) {
			t.Fatalf("expected %s to be supported by LLM", supportedType)
		}
	}

	// 测试不支持的类型
	unsupportedTypes := []string{
		"application/zip",
		"application/x-executable",
		"application/vnd.ms-excel",
		"",
	}

	for _, unsupportedType := range unsupportedTypes {
		if IsValidFileTypeForLLM(unsupportedType) {
			t.Fatalf("expected %s to be unsupported by LLM", unsupportedType)
		}
	}
}

func TestFileResponseToBase64(t *testing.T) {
	// 测试空响应
	_, err := FileResponseToBase64(nil)
	if err == nil {
		t.Fatal("expected error for nil response")
	}

	// 创建模拟的文本文件数据
	textData := []byte("Hello, World!")

	// 创建模拟的HTTP响应
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader(textData)),
	}
	resp.Header.Set("Content-Type", "text/plain")

	base64Result, err := FileResponseToBase64(resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "data:text/plain;base64,SGVsbG8sIFdvcmxkIQ=="
	if base64Result != expected {
		t.Fatalf("expected %s, got %s", expected, base64Result)
	}
}

func TestGetFileInfo(t *testing.T) {
	// 测试空路径
	_, err := GetFileInfo("")
	if err == nil {
		t.Fatal("expected error for empty path")
	}

	// 测试不存在的本地文件
	_, err = GetFileInfo("/non/existent/path")
	if err == nil {
		t.Fatal("expected error for non-existent file")
	}

	// 注意：这里不测试实际的URL或文件，因为测试环境可能不稳定
	// 实际项目中可以创建临时文件进行测试
}

// 创建一个简单的测试，创建临时文件进行测试
func TestGetFileBase64FromPath(t *testing.T) {
	// 创建临时文件
	tempFile, err := os.CreateTemp("", "test_*.txt")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name()) // 清理临时文件

	// 写入测试数据
	testData := "Hello, World!"
	_, err = tempFile.WriteString(testData)
	if err != nil {
		t.Fatalf("failed to write to temp file: %v", err)
	}
	tempFile.Close()

	// 测试读取并转换为base64
	base64Result, err := GetFileBase64FromPath(tempFile.Name())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "data:text/plain;base64,SGVsbG8sIFdvcmxkIQ=="
	if base64Result != expected {
		t.Fatalf("expected %s, got %s", expected, base64Result)
	}
}

func TestFileToBase64(t *testing.T) {
	// 测试空路径
	_, err := FileToBase64("")
	if err == nil {
		t.Fatal("expected error for empty path")
	}

	// 创建临时文件进行测试
	tempFile, err := os.CreateTemp("", "test_*.json")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	// 写入JSON数据
	jsonData := `{"message": "Hello, World!"}`
	_, err = tempFile.WriteString(jsonData)
	if err != nil {
		t.Fatalf("failed to write to temp file: %v", err)
	}
	tempFile.Close()

	// 测试
	base64Result, err := FileToBase64(tempFile.Name())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.HasPrefix(base64Result, "data:application/json;base64,") {
		t.Fatal("result should start with data:application/json;base64,")
	}

	t.Logf("JSON Base64 result: %s", base64Result)
}

func TestCleanContentType(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"text/plain", "text/plain"},
		{"text/plain; charset=utf-8", "text/plain"},
		{"application/json; charset=utf-8", "application/json"},
		{"image/png", "image/png"},
		{"text/html; charset=utf-8; boundary=something", "text/html"},
		{"", ""},
		{"text/plain;", "text/plain"},
		{"text/plain; ", "text/plain"},
	}

	for _, test := range tests {
		result := cleanContentType(test.input)
		if result != test.expected {
			t.Fatalf("cleanContentType(%q) = %q, expected %q", test.input, result, test.expected)
		}
	}
}
