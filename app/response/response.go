package response

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/quka-ai/quka-ai/pkg/errors"
	"github.com/quka-ai/quka-ai/pkg/i18n"
	"github.com/quka-ai/quka-ai/pkg/utils"
)

func ProvideResponseLocalizer(l i18n.Localizer) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("i18n", l)
	}
}

func InjectResponseLocalizer(c *gin.Context) i18n.Localizer {
	return c.MustGet("i18n").(i18n.Localizer)
}

// 常量定义
const (
	RequestIDKey = "request_id"
	ResponseKey  = "response_key"
)

// EmptyStruct 空结构体
type EmptyStruct struct {
}

// Response 响应结构体定义
type Response struct {
	Meta Meta        `json:"meta"`
	Data interface{} `json:"data"`
}

// Meta 响应meta定义
type Meta struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
}

// BodyPaging 分页参数结构
type BodyPaging struct {
	Cursors BodyCursors `json:"cursors"`
}

// BodyCursors 分页参数
type BodyCursors struct {
	Before string `json:"before"`
	After  string `json:"after"`
}

func GetLangFromRequestOrDefault(c *gin.Context) string {
	lang := c.Request.Header.Get("Accept-Language")
	if lang == "zh" {
		lang = "zh-CN"
	}
	if i18n.ALLOW_LANG[lang] {
		return lang
	}
	return i18n.DEFAULT_LANG
}

// APIError api响应失败
func APIError(c *gin.Context, err error) {
	c.Abort()
	l := InjectResponseLocalizer(c)

	res := c.MustGet(ResponseKey).(*Response)
	var httpStatus int
	if cerrptr, ok := err.(*errors.CustomizedError); !ok {
		res.Meta.Code = http.StatusInternalServerError
		res.Meta.Message = err.Error()
		httpStatus = res.Meta.Code
	} else {
		res.Meta.Code = cerrptr.GetCode()
		lang := GetLangFromRequestOrDefault(c)
		if lang == "" {
			lang = "en"
		}
		res.Meta.Message = l.Get(lang, cerrptr.Message())
		httpStatus = cerrptr.GetCode()
	}

	c.JSON(httpStatus, res)
	printErrorLog(c, res, err)
}

func printErrorLog(c *gin.Context, res *Response, err error) {
	endTime := time.Now().Unix()
	// 统一打印日志
	var logFields = map[string]any{
		"request_uri": c.Request.URL.Path,
		"end_time":    endTime,
		"code":        res.Meta.Code,
		"error":       err.Error(),
		"platform":    c.Request.Header.Get("Platform"),
		"version":     c.Request.Header.Get("Version"),
	}

	// 如果有uid打印uid
	uid := c.GetInt64("user")
	if uid > 0 {
		logFields["uid"] = uid
	}
	slog.Error("response error", slog.Any("fileds", logFields))
}

func printSuccessLog(c *gin.Context, res *Response) {
	endTime := time.Now().Unix()
	// 统一打印日志
	var logFields = map[string]any{
		"request_uri": c.Request.URL.Path,
		"end_time":    endTime,
		"platform":    c.Request.Header.Get("Platform"),
		"version":     c.Request.Header.Get("Version"),
	}

	if c.Request.Method == "POST" {
		c.Request.ParseForm()
		logFields["params"] = c.Request.Form.Encode()
	} else {
		logFields["params"] = c.Request.URL.Query().Encode()
	}

	// 如果有uid打印uid
	uid := c.GetInt64("user")
	if uid > 0 {
		logFields["uid"] = uid
	}
	slog.Info("request success", slog.Any("fileds", logFields))
}

// APISuccess api响应成功
func APISuccess(c *gin.Context, response interface{}) {
	c.Abort()
	res := c.MustGet(ResponseKey).(*Response)
	if response != nil {
		res.Data = response
	}
	c.JSON(http.StatusOK, res)
	printSuccessLog(c, res)
}

// GetRequestID 获取请求ID
func NewResponse() gin.HandlerFunc {
	return func(c *gin.Context) {
		resp := &Response{
			Meta: Meta{
				RequestID: utils.GenRandomID(),
			},
		}
		c.Set(ResponseKey, resp)
	}
}
