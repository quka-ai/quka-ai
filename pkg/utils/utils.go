package utils

import (
	"bufio"
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	crand "crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"math"
	"math/rand"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/davidscottmills/goeditorjs"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/holdno/snowFlakeByGo"
	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"

	"github.com/quka-ai/quka-ai/pkg/errors"
	"github.com/quka-ai/quka-ai/pkg/i18n"
	"github.com/quka-ai/quka-ai/pkg/utils/editorjs"
)

const ()

var (
	// IdWorker 全局唯一id生成器实例
	idWorker *snowFlakeByGo.Worker
)

func SetupIDWorker(clusterID int64) {
	idWorker, _ = snowFlakeByGo.NewWorker(clusterID)
}

func GenUniqID() int64 {
	return idWorker.GetId()
}

func GenUniqIDStr() string {
	return strconv.FormatInt(GenUniqID(), 10)
}

func GenSpecID() int64 {
	return idWorker.GetId()
}

func GenSpecIDStr() string {
	return strconv.FormatInt(GenSpecID(), 10)
}

func GenRandomID() string {
	return RandomStr(32)
}

// RandomStr 随机字符串
func RandomStr(l int) string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	seed := "1234567890qwertyuiopasdfghjklzxcvbnmQWERTYUIOPASDFGHJKLZXCVBNM"
	str := ""
	length := len(seed)
	for i := 0; i < l; i++ {
		point := r.Intn(length)
		str = str + seed[point:point+1]
	}
	return str
}

// Random 生成随机数
func Random(min, max int) int {
	if min == max {
		return max
	}
	max = max + 1
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return min + r.Intn(max-min)
}

func MD5(s string) string {
	md5Ctx := md5.New()
	md5Ctx.Write([]byte(s))
	cipherStr := md5Ctx.Sum(nil)

	return hex.EncodeToString(cipherStr)
}

func BindArgsWithGin(c *gin.Context, req interface{}) error {
	err := c.ShouldBindWith(req, binding.Default(c.Request.Method, c.ContentType()))
	if err != nil {
		return errors.New(fmt.Sprintf("Gin.ShouldBindWith.%s.%s", c.Request.Method, c.Request.URL.Path), i18n.ERROR_INVALIDARGUMENT, err).Code(http.StatusBadRequest)
	}
	return nil
}

type Binding interface {
	Name() string
	Bind(*http.Request, any) error
}

func TextEnterToBr(s string) string {
	return strings.TrimSpace(strings.Replace(strings.Replace(s, "\r\n", "(br)", -1), "\n", "(br)", -1))
}

func IsAlphabetic(s string) bool {
	match, _ := regexp.MatchString(`^[a-zA-Z]+$`, s)
	return match
}

func GenUserPassword(salt string, pwd string) string {
	return MD5(MD5(salt) + salt + MD5(pwd))
}

// Language represents a language and its weight (priority)
type Language struct {
	Tag    string  // Language tag, e.g., "en-US"
	Weight float64 // Weight (priority), default is 1.0
}

// ParseAcceptLanguage parses the Accept-Language header and returns a sorted list of languages by weight.
func ParseAcceptLanguage(header string) []Language {
	if header == "" {
		return []Language{}
	}

	// Regular expression to match language and optional weight
	re := regexp.MustCompile(`([a-zA-Z\-]+)(?:;q=([0-9\.]+))?`)

	// Find all matches
	matches := re.FindAllStringSubmatch(header, -1)

	// Parse languages
	var languages []Language
	for _, match := range matches {
		tag := match[1]
		weight := 1.0 // Default weight
		if len(match) > 2 && match[2] != "" {
			parsedWeight, err := strconv.ParseFloat(match[2], 64)
			if err == nil {
				weight = parsedWeight
			}
		}
		languages = append(languages, Language{Tag: tag, Weight: weight})
	}

	// Sort languages by weight in descending order
	sort.Slice(languages, func(i, j int) bool {
		return languages[i].Weight > languages[j].Weight
	})

	return languages
}

// CFB 加密函数
func EncryptCFB(plaintext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// 生成随机 IV
	iv := make([]byte, aes.BlockSize)
	if _, err := io.ReadFull(crand.Reader, iv); err != nil {
		return nil, err
	}

	// 使用 CFB 模式加密
	ciphertext := make([]byte, len(plaintext))
	encrypter := cipher.NewCFBEncrypter(block, iv)
	encrypter.XORKeyStream(ciphertext, plaintext)

	// 返回 IV 和密文
	result := append(iv, ciphertext...)

	dst := make([]byte, hex.EncodedLen(len(result)))
	hex.Encode(dst, result)
	return dst, nil
}

// CFB 解密函数
func DecryptCFB(ciphertext, key []byte) ([]byte, error) {
	dst := make([]byte, hex.DecodedLen(len(ciphertext)))
	n, err := hex.Decode(dst, ciphertext)
	ciphertext = dst[:n]

	if len(ciphertext) < aes.BlockSize {
		return nil, fmt.Errorf("wrong ciphertext")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// 提取 IV 和密文
	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]

	// 使用 CFB 模式解密
	plaintext := make([]byte, len(ciphertext))
	decrypter := cipher.NewCFBDecrypter(block, iv)
	decrypter.XORKeyStream(plaintext, ciphertext)

	return plaintext, nil
}

func Cosine(a []float64, b []float64) float64 {
	var (
		aLen  = len(a)
		bLen  = len(b)
		s     = 0.0
		sa    = 0.0
		sb    = 0.0
		count = 0
	)
	if aLen > bLen {
		count = aLen
	} else {
		count = bLen
	}
	for i := 0; i < count; i++ {
		if i >= bLen {
			sa += math.Pow(a[i], 2)
			continue
		}
		if i >= aLen {
			sb += math.Pow(b[i], 2)
			continue
		}
		s += a[i] * b[i]
		sa += math.Pow(a[i], 2)
		sb += math.Pow(b[i], 2)
	}
	return s / (math.Sqrt(sa) * math.Sqrt(sb))
}

func MaskString(s string, preLen, postLen int) string {
	runes := []rune(s)

	var pre, post string

	// 获取前6位
	if len(runes) >= preLen {
		pre = string(runes[:preLen])
	} else {
		pre = string(runes)
	}

	// 获取后4位
	if len(runes) >= postLen {
		post = string(runes[len(runes)-postLen:])
	} else {
		post = string(runes)
	}

	return pre + "******" + post
}

func ConvertSVGToPNG(in []byte) ([]byte, error) {
	// 解析SVG文件
	icon, err := oksvg.ReadIconStream(bytes.NewReader(in))
	if err != nil {
		return nil, fmt.Errorf("SVG format error, %w", err)
	}

	width, height := int(icon.ViewBox.W), int(icon.ViewBox.H)

	// 创建目标图像
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// 初始化扫描器和绘制器
	scanner := rasterx.NewScannerGV(width, height, img, img.Bounds())
	rasterizer := rasterx.NewDasher(width, height, scanner)

	// 设置SVG渲染的目标区域
	icon.SetTarget(0, 0, float64(width), float64(height))

	// 将SVG绘制到图像上
	icon.Draw(rasterizer, 1.0)

	var f bytes.Buffer

	if err = png.Encode(bufio.NewWriter(&f), img); err != nil {
		return nil, err
	}

	return f.Bytes(), nil
}

// cleanContentType 清理Content-Type，去除参数部分
func cleanContentType(contentType string) string {
	if contentType == "" {
		return ""
	}

	// 分离主要的MIME类型和参数（如charset）
	parts := strings.Split(contentType, ";")
	if len(parts) > 0 {
		return strings.TrimSpace(parts[0])
	}

	return contentType
}

// ImageResponseToBase64 将HTTP图片响应转换为base64格式
// 支持常见的图片格式：jpg, jpeg, png, webp, bmp, gif
func ImageResponseToBase64(imageResponse *http.Response) (string, error) {
	if imageResponse == nil {
		return "", fmt.Errorf("imageResponse is nil")
	}

	defer imageResponse.Body.Close()

	// 检查HTTP状态码
	if imageResponse.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP request failed with status: %d", imageResponse.StatusCode)
	}

	// 读取响应体
	imageData, err := io.ReadAll(imageResponse.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read image data: %w", err)
	}

	// 获取Content-Type
	contentType := cleanContentType(imageResponse.Header.Get("Content-Type"))

	// 如果Content-Type为空，尝试从URL路径推断
	if contentType == "" {
		if imageResponse.Request != nil && imageResponse.Request.URL != nil {
			ext := filepath.Ext(imageResponse.Request.URL.Path)
			contentType = cleanContentType(mime.TypeByExtension(ext))
		}
	}

	// 如果仍然无法确定Content-Type，使用默认值
	if contentType == "" {
		contentType = "image/jpeg" // 默认为jpeg
	}

	// 验证是否为支持的图片类型
	if !isValidImageType(contentType) {
		return "", fmt.Errorf("unsupported image type: %s", contentType)
	}

	// 编码为base64
	base64Data := base64.StdEncoding.EncodeToString(imageData)

	// 返回完整的data URL格式
	return fmt.Sprintf("data:%s;base64,%s", contentType, base64Data), nil
}

// ImageBytesToBase64 将图片字节数据转换为base64格式
func ImageBytesToBase64(imageData []byte, contentType string) (string, error) {
	if len(imageData) == 0 {
		return "", fmt.Errorf("image data is empty")
	}

	// 验证是否为支持的图片类型
	if !isValidImageType(contentType) {
		return "", fmt.Errorf("unsupported image type: %s", contentType)
	}

	// 编码为base64
	base64Data := base64.StdEncoding.EncodeToString(imageData)

	// 返回完整的data URL格式
	return fmt.Sprintf("data:%s;base64,%s", contentType, base64Data), nil
}

// isValidImageType 检查是否为支持的图片类型
func isValidImageType(contentType string) bool {
	supportedTypes := []string{
		"image/jpeg",
		"image/jpg",
		"image/png",
		"image/webp",
		"image/bmp",
		"image/gif",
	}

	for _, t := range supportedTypes {
		if strings.EqualFold(contentType, t) {
			return true
		}
	}
	return false
}

// GetImageBase64FromURL 从URL直接获取图片并转换为base64
func GetImageBase64FromURL(imageURL string) (string, error) {
	if imageURL == "" {
		return "", fmt.Errorf("image URL is empty")
	}

	// 发起HTTP请求
	resp, err := http.Get(imageURL)
	if err != nil {
		return "", fmt.Errorf("failed to fetch image from URL: %w", err)
	}

	// 转换为base64
	return ImageResponseToBase64(resp)
}

// FileToBase64 将任意文件转换为base64格式
// 支持本地文件路径或URL
func FileToBase64(filePath string) (string, error) {
	if filePath == "" {
		return "", fmt.Errorf("file path is empty")
	}

	// 判断是本地文件还是网络文件
	if strings.HasPrefix(filePath, "http://") || strings.HasPrefix(filePath, "https://") {
		return GetFileBase64FromURL(filePath)
	}

	// 处理本地文件
	return GetFileBase64FromPath(filePath)
}

// GetFileBase64FromPath 从本地文件路径读取文件并转换为base64
func GetFileBase64FromPath(filePath string) (string, error) {
	if filePath == "" {
		return "", fmt.Errorf("file path is empty")
	}

	// 读取文件内容
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	// 根据文件扩展名确定MIME类型
	contentType := cleanContentType(mime.TypeByExtension(filepath.Ext(filePath)))
	if contentType == "" {
		// 如果无法确定类型，使用通用的二进制类型
		contentType = "application/octet-stream"
	}

	// 转换为base64
	return FileBytesToBase64(fileData, contentType)
}

// GetFileBase64FromURL 从URL获取任意文件并转换为base64
func GetFileBase64FromURL(fileURL string) (string, error) {
	if fileURL == "" {
		return "", fmt.Errorf("file URL is empty")
	}

	// 发起HTTP请求
	resp, err := http.Get(fileURL)
	if err != nil {
		return "", fmt.Errorf("failed to fetch file from URL: %w", err)
	}

	// 转换为base64
	return FileResponseToBase64(resp)
}

// FileResponseToBase64 将HTTP文件响应转换为base64格式
func FileResponseToBase64(fileResponse *http.Response) (string, error) {
	if fileResponse == nil {
		return "", fmt.Errorf("fileResponse is nil")
	}

	defer fileResponse.Body.Close()

	// 检查HTTP状态码
	if fileResponse.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP request failed with status: %d", fileResponse.StatusCode)
	}

	// 读取响应体
	fileData, err := io.ReadAll(fileResponse.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read file data: %w", err)
	}

	// 获取Content-Type
	contentType := cleanContentType(fileResponse.Header.Get("Content-Type"))

	// 如果Content-Type为空，尝试从URL路径推断
	if contentType == "" {
		if fileResponse.Request != nil && fileResponse.Request.URL != nil {
			ext := filepath.Ext(fileResponse.Request.URL.Path)
			contentType = cleanContentType(mime.TypeByExtension(ext))
		}
	}

	// 如果仍然无法确定Content-Type，使用默认值
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// 转换为base64
	return FileBytesToBase64(fileData, contentType)
}

// FileBytesToBase64 将任意文件字节数据转换为base64格式
func FileBytesToBase64(fileData []byte, contentType string) (string, error) {
	if len(fileData) == 0 {
		return "", fmt.Errorf("file data is empty")
	}

	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// 编码为base64
	base64Data := base64.StdEncoding.EncodeToString(fileData)

	// 返回完整的data URL格式
	return fmt.Sprintf("data:%s;base64,%s", contentType, base64Data), nil
}

// IsValidFileTypeForLLM 检查文件类型是否适合发送给LLM
func IsValidFileTypeForLLM(contentType string) bool {
	// 图片类型
	imageTypes := []string{
		"image/jpeg", "image/jpg", "image/png", "image/webp",
		"image/bmp", "image/gif", "image/svg+xml",
	}

	// 文档类型
	documentTypes := []string{
		"text/plain", "text/html", "text/markdown", "text/csv",
		"application/json", "application/xml", "text/xml",
		"application/pdf", // 部分LLM支持
	}

	// 音频类型（某些LLM支持）
	audioTypes := []string{
		"audio/mpeg", "audio/wav", "audio/ogg", "audio/mp4",
	}

	// 视频类型（某些LLM支持）
	videoTypes := []string{
		"video/mp4", "video/webm", "video/ogg",
	}

	allSupportedTypes := append(append(append(imageTypes, documentTypes...), audioTypes...), videoTypes...)

	for _, t := range allSupportedTypes {
		if strings.EqualFold(contentType, t) {
			return true
		}
	}
	return false
}

// ParseRawToBlocks 已迁移到 editorjs 包
// Deprecated: 请使用 editorjs.ParseRawToBlocks
func ParseRawToBlocks(blockString json.RawMessage) (*editorjs.BlockContent, error) {
	return editorjs.ParseRawToBlocks(blockString)
}

// GetFileInfo 获取文件的基本信息
func GetFileInfo(filePath string) (FileInfo, error) {
	info := FileInfo{}

	// 判断是本地文件还是网络文件
	if strings.HasPrefix(filePath, "http://") || strings.HasPrefix(filePath, "https://") {
		info.IsURL = true
		info.Path = filePath

		// 从URL获取文件信息
		resp, err := http.Head(filePath)
		if err != nil {
			return info, fmt.Errorf("failed to get file info from URL: %w", err)
		}
		defer resp.Body.Close()

		info.ContentType = cleanContentType(resp.Header.Get("Content-Type"))
		info.Size = resp.ContentLength
		info.Name = filepath.Base(resp.Request.URL.Path)
	} else {
		info.IsURL = false
		info.Path = filePath

		// 获取本地文件信息
		stat, err := os.Stat(filePath)
		if err != nil {
			return info, fmt.Errorf("failed to get file info: %w", err)
		}

		info.Name = stat.Name()
		info.Size = stat.Size()
		info.ContentType = cleanContentType(mime.TypeByExtension(filepath.Ext(filePath)))
		info.ModTime = stat.ModTime()
	}

	info.SupportedByLLM = IsValidFileTypeForLLM(info.ContentType)

	return info, nil
}

// GetMimeTypeByExtension 根据文件扩展名获取MIME类型
func GetMimeTypeByExtension(ext string) string {
	contentType := mime.TypeByExtension(ext)
	if contentType == "" {
		return "application/octet-stream"
	}
	return cleanContentType(contentType)
}

type FileInfo struct {
	Name           string    `json:"name"`
	Path           string    `json:"path"`
	Size           int64     `json:"size"`
	ContentType    string    `json:"content_type"`
	IsURL          bool      `json:"is_url"`
	SupportedByLLM bool      `json:"supported_by_llm"`
	ModTime        time.Time `json:"mod_time,omitempty"`
}

// FileStorageInterface 文件存储接口定义
// Deprecated: 请使用 editorjs.FileStorageInterface
type FileStorageInterface = editorjs.FileStorageInterface

// ReplaceMarkdownStaticResourcesWithPresignedURL 替换markdown中的静态资源URL为预签名URL
// Deprecated: 请使用 editorjs.ReplaceMarkdownStaticResourcesWithPresignedURL
func ReplaceMarkdownStaticResourcesWithPresignedURL(content string, fileStorage FileStorageInterface) string {
	return editorjs.ReplaceMarkdownStaticResourcesWithPresignedURL(content, fileStorage)
}

// ReplaceEditorJSBlocksStaticResourcesWithPresignedURL 替换EditorJS blocks中的静态资源URL为预签名URL
// Deprecated: 请使用 editorjs.ReplaceEditorJSBlocksStaticResourcesWithPresignedURL
func ReplaceEditorJSBlocksStaticResourcesWithPresignedURL(blocks []goeditorjs.EditorJSBlock, fileStorage FileStorageInterface) []goeditorjs.EditorJSBlock {
	return editorjs.ReplaceEditorJSBlocksStaticResourcesWithPresignedURL(blocks, fileStorage)
}

// ReplaceEditorJSBlocksJsonStaticResourcesWithPresignedURL 替换EditorJS blocks中的静态资源URL为预签名URL
// Deprecated: 请使用 editorjs.ReplaceEditorJSBlocksJsonStaticResourcesWithPresignedURL
func ReplaceEditorJSBlocksJsonStaticResourcesWithPresignedURL(blocksJSON string, fileStorage FileStorageInterface) string {
	return editorjs.ReplaceEditorJSBlocksJsonStaticResourcesWithPresignedURL(blocksJSON, fileStorage)
}
