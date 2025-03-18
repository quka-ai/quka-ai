package utils

import (
	"bufio"
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	crand "crypto/rand"
	"encoding/hex"
	"fmt"
	"image"
	"image/png"
	"io"
	"math"
	"math/rand"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/holdno/snowFlakeByGo"
	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"

	"github.com/quka-ai/quka-ai/pkg/errors"
	"github.com/quka-ai/quka-ai/pkg/i18n"
)

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
