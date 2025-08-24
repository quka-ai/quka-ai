package utils

import (
	"net/url"
	"strings"
)

// ProcessStorageURL 处理存储URL，如果是本地存储的文件则生成预签名URL
func ProcessStorageURL(urlStr string, staticDomain string, preSignFunc func(path string) (string, error)) (string, error) {
	if urlStr == "" {
		return urlStr, nil
	}

	// 检查是否为http/https协议
	if !strings.HasPrefix(urlStr, "http://") && !strings.HasPrefix(urlStr, "https://") {
		return urlStr, nil
	}

	// 如果静态域名为空，保持原URL不变
	if staticDomain == "" {
		return urlStr, nil
	}

	// 解析URL
	parsedURL, parseErr := url.Parse(urlStr)
	if parseErr != nil {
		return urlStr, nil // 解析失败保持原URL
	}

	// 解析静态域名URL
	staticParsed, staticParseErr := url.Parse(staticDomain)
	if staticParseErr != nil {
		return urlStr, nil // 解析失败保持原URL
	}

	// 检查host是否匹配
	if parsedURL.Host != staticParsed.Host {
		return urlStr, nil // host不匹配保持原URL
	}

	// host匹配，提取路径并生成预签名URL
	path := parsedURL.Path
	if path == "" {
		return urlStr, nil
	}

	// 调用预签名函数生成签名URL
	if preSignFunc != nil {
		return preSignFunc(path)
	}

	return urlStr, nil
}

// IsLocalStorageURL 检查URL是否指向本地存储服务
func IsLocalStorageURL(urlStr string, staticDomain string) bool {
	if urlStr == "" || staticDomain == "" {
		return false
	}

	urlParsed, err := url.Parse(urlStr)
	if err != nil {
		return false
	}

	staticParsed, err := url.Parse(staticDomain)
	if err != nil {
		return false
	}

	return urlParsed.Host == staticParsed.Host
}
