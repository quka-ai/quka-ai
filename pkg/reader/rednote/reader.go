package rednote

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/playwright-community/playwright-go"
)

func init() {
	path := os.Getenv("READNOTE_COOKIE_PATH")
	if path == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			homeDir = "~"
		}
		path = filepath.Join(homeDir, "/.quka/rednote/cookies.json")
		if _, err := os.Stat(path); os.IsNotExist(err) {
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				slog.Error("failed to create directory for rednote cookies", slog.String("error", err.Error()))
			}
		}
	}
	var err error
	reader, err = NewReader(path)
	if err != nil {
		slog.Error("failed to init rednote reader", slog.String("error", err.Error()))
	}
}

var reader *Reader

// export interface NoteDetail {
//   title: string
//   content: string
//   tags: string[]
//   imgs?: string[]
//   videos?: string[]
//   url: string
//   author: string
//   likes?: number
//   collects?: number
//   comments?: number
// }

type Reader struct {
	cookies    []playwright.OptionalCookie
	cookiePath string
	browser    playwright.Browser

	latestSetCookieTime time.Time
	lock                sync.Mutex
}

func Match(endpoint string) bool {
	switch true {
	case strings.Contains(endpoint, "/www.xiaohongshu.com/"):
		fallthrough
	case strings.Contains(endpoint, "xhslink.com/"):
		fallthrough
	case strings.Contains(endpoint, "小红书"):
		return true
	default:
		return false
	}
}

// ExtractRedNoteUrl 从分享文本中提取小红书链接
func ExtractRedNoteUrl(shareText string) string {
	// 匹配 http://xhslink.com/ 开头的链接
	xhslinkRegex := regexp.MustCompile(`(?i)(https?://xhslink\.com/[a-zA-Z0-9/]+)`)
	xhslinkMatch := xhslinkRegex.FindStringSubmatch(shareText)

	if len(xhslinkMatch) > 1 {
		return xhslinkMatch[1]
	}

	// 匹配 https://www.xiaohongshu.com/ 开头的链接
	xiaohongshuRegex := regexp.MustCompile(`(?i)(https?://(?:www\.)?xiaohongshu\.com/[^，\s]+)`)
	xiaohongshuMatch := xiaohongshuRegex.FindStringSubmatch(shareText)

	if len(xiaohongshuMatch) > 1 {
		return xiaohongshuMatch[1]
	}

	return shareText
}

func NewReader(cookiePath string) (*Reader, error) {
	r := &Reader{
		cookiePath: cookiePath,
	}

	var err error
	if r.cookies, err = r.ReadCookies(); err != nil {
		return nil, fmt.Errorf("failed to read cookies, %w", err)
	}
	r.latestSetCookieTime = time.Now()

	if err := playwright.Install(); err != nil {
		return nil, fmt.Errorf("failed to install playwright, %w", err)
	}

	pw, err := playwright.Run()
	if err != nil {
		return nil, fmt.Errorf("could not start playwright: %w", err)
	}
	r.browser, err = pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(true),
	})
	if err != nil {
		return nil, fmt.Errorf("could not launch browser: %w", err)
	}

	return r, nil
}

type NoteDetail struct {
	Title   string   `json:"title"`
	Content string   `json:"content"`
	Tags    []string `json:"tags"`
	Images  []string `json:"images"`
	Videos  []string `json:"videos"`
	URL     string   `json:"url"`
	Author  string   `json:"author"`
}

func (r *Reader) ReadCookies() ([]playwright.OptionalCookie, error) {
	raw, err := os.ReadFile(r.cookiePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read cookie file, %w", err)
	}

	if len(raw) == 0 {
		return nil, nil
	}

	var cookies []playwright.OptionalCookie
	if err = json.Unmarshal(raw, &cookies); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cookie, %w", err)
	}

	return cookies, nil
}

func (r *Reader) SaveCookies(cookies []playwright.Cookie) error {
	raw, _ := json.Marshal(cookies)
	return os.WriteFile(r.cookiePath, raw, 0644)
}

func (r *Reader) Login(page playwright.Page) ([]playwright.Cookie, error) {
	l := page.Locator(".login-container")
	err := l.WaitFor(playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(5000),
	})
	if err != nil {
		return nil, fmt.Errorf("can not found login-container, %w", err)
	}

	l = page.Locator(".qrcode-img")
	err = l.WaitFor(playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(5000),
	})
	if err != nil {
		return nil, fmt.Errorf("can not found qrcode-img, %w", err)
	}

	l = page.Locator(".user.side-bar-component .channel")
	err = l.WaitFor(playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(60000),
	})

	if err != nil {
		return nil, fmt.Errorf("login failed, %w", err)
	}

	cookies, err := page.Context().Cookies()
	if err != nil {
		return nil, fmt.Errorf("failed to load page cookies, %w", err)
	}

	return cookies, nil
}

func (r *Reader) Close() error {
	return r.browser.Close()
}

func (r *Reader) UpdateCookies(page playwright.Page) {
	r.lock.Lock()
	if !r.latestSetCookieTime.Before(time.Now().Add(-time.Hour * 3)) {
		return
	}

	r.latestSetCookieTime = time.Now()
	newCookies, _ := page.Context().Cookies()
	if len(newCookies) > 0 {
		r.cookies = cookiesToOptionalCookies(newCookies)
		r.SaveCookies(newCookies)
	}
}

func cookiesToOptionalCookies(cookies []playwright.Cookie) []playwright.OptionalCookie {
	var optionalCookies []playwright.OptionalCookie
	raw, _ := json.Marshal(cookies)
	json.Unmarshal(raw, &optionalCookies)
	return optionalCookies
}

func Read(endpoint string) (*NoteDetail, error) {
	if reader == nil {
		return nil, fmt.Errorf("rednote reader is not initialized")
	}
	endpoint = ExtractRedNoteUrl(endpoint)
	ctx, err := reader.browser.NewContext()
	if err != nil {
		return nil, fmt.Errorf("failed to create new browser context, %w", err)
	}

	if len(reader.cookies) > 0 {
		if err = ctx.AddCookies(reader.cookies); err != nil {
			return nil, fmt.Errorf("failed to add cookies, %w", err)
		}
	}

	page, err := ctx.NewPage()
	if err != nil {
		return nil, fmt.Errorf("failed to create new page, %w", err)
	}

RELOAD:
	_, err = page.Goto(endpoint, playwright.PageGotoOptions{
		Timeout: playwright.Float(10000),
	})
	if err != nil {
		return nil, fmt.Errorf("could not goto: %w", err)
	}

	userChannel, err := page.Locator(".user.side-bar-component .channel").TextContent(playwright.LocatorTextContentOptions{
		Timeout: playwright.Float(1000),
	})
	if errors.Is(err, playwright.ErrTimeout) || userChannel != "我" {
		newCookies, err := reader.Login(page)
		if err != nil {
			return nil, err
		}

		ctx.ClearCookies()
		ctx.AddCookies(cookiesToOptionalCookies(newCookies))
		page, _ = ctx.NewPage()

		reader.SaveCookies(newCookies)
		goto RELOAD
	}

	l := page.Locator(".note-container")
	err = l.WaitFor(playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(5000),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to find note-container, %w", err)
	}

	// article.querySelector('#detail-title')?.textContent?.trim()
	title, err := page.Locator(".note-container #detail-title").TextContent(playwright.LocatorTextContentOptions{
		Timeout: playwright.Float(1000),
	})
	if err != nil && !errors.Is(err, playwright.ErrTimeout) {
		return nil, fmt.Errorf("failed to get title, %w", err)
	}

	content, err := page.Locator(".note-content .note-text span").First().TextContent(playwright.LocatorTextContentOptions{
		Timeout: playwright.Float(1000),
	})
	if err != nil && !errors.Is(err, playwright.ErrTimeout) {
		return nil, fmt.Errorf("failed to get content, %w", err)
	}

	tagsSelector, err := l.Locator(".note-content .note-text a").All()
	if err != nil {
		return nil, fmt.Errorf("failed to get tags, %w", err)
	}
	var tags []string
	for _, v := range tagsSelector {
		tag, err := v.TextContent(playwright.LocatorTextContentOptions{
			Timeout: playwright.Float(500),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to get tag text, %w", err)
		}
		tags = append(tags, strings.Replace(tag, "#", "", 1))
	}

	author, err := l.Locator(".author-container .info .username").First().TextContent(playwright.LocatorTextContentOptions{
		Timeout: playwright.Float(1000),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get username, %w", err)
	}

	imagesSelector, err := page.Locator(".media-container img").All()
	if err != nil {
		return nil, fmt.Errorf("failed to get imgs, %w", err)
	}

	var imgs []string
	for _, v := range imagesSelector {
		url, err := v.GetAttribute("src", playwright.LocatorGetAttributeOptions{
			Timeout: playwright.Float(500),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to get img src, %w", err)
		}
		if url != "" {
			imgs = append(imgs, url)
		}
	}

	videosSelector, err := page.Locator(".media-container video").All()
	if err != nil {
		return nil, fmt.Errorf("failed to get videos, %w", err)
	}

	var videos []string
	for _, v := range videosSelector {
		url, err := v.GetAttribute("src", playwright.LocatorGetAttributeOptions{
			Timeout: playwright.Float(500),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to get video src, %w", err)
		}
		if url != "" {
			videos = append(videos, url)
		}
	}

	reader.UpdateCookies(page)
	page.Close()

	return &NoteDetail{
		URL:     endpoint,
		Title:   title,
		Author:  author,
		Content: content,
		Tags:    tags,
		Images:  imgs,
		Videos:  videos,
	}, nil
}
