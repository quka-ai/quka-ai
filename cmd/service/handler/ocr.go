package handler

import (
	"io"

	"github.com/gin-gonic/gin"

	"github.com/quka-ai/quka-ai/app/response"
	"github.com/quka-ai/quka-ai/pkg/ai/baidu"
	"github.com/quka-ai/quka-ai/pkg/errors"
	"github.com/quka-ai/quka-ai/pkg/utils"
)

type ProcessOCRRequest struct {
	FileURL string `json:"file_url,omitempty"`
}

type ProcessOCRFromFileRequest struct {
	// FileType is no longer needed as OCR automatically detects the file type
}

type OCRResponse struct {
	Title        string   `json:"title"`
	MarkdownText string   `json:"markdown_text"`
	Images       []string `json:"images"`
	Model        string   `json:"model"`
	TokensUsed   int      `json:"tokens_used,omitempty"`
}

// ProcessOCRFromFile 处理上传的文件进行OCR识别
func (s *HttpSrv) ProcessOCRFromFile(c *gin.Context) {
	var req ProcessOCRFromFileRequest
	if err := utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	// 获取上传的文件
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		response.APIError(c, errors.New("ProcessOCRFromFile.FormFile", "Failed to get uploaded file", err))
		return
	}
	defer file.Close()

	// 读取文件数据
	fileData, err := io.ReadAll(file)
	if err != nil {
		response.APIError(c, errors.New("ProcessOCRFromFile.ReadAll", "Failed to read file data", err))
		return
	}

	// 检查文件大小限制 (50MB)
	if len(fileData) > 50*1024*1024 {
		response.APIError(c, errors.New("ProcessOCRFromFile.FileSizeLimit", "File size exceeds limit (50MB)", nil))
		return
	}

	// 自动检测文件类型并处理
	ocrResult, err := s.Core.Srv().AI().ProcessOCR(c.Request.Context(), fileData)
	if err != nil {
		response.APIError(c, errors.New("ProcessOCRFromFile.ProcessOCR", "Failed to process OCR", err))
		return
	}

	result := convertOCRResult(ocrResult)
	response.APISuccess(c, result)
}

// ProcessOCRFromURL 从URL下载文件并进行OCR识别
func (s *HttpSrv) ProcessOCRFromURL(c *gin.Context) {
	var req ProcessOCRRequest
	if err := utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	if req.FileURL == "" {
		response.APIError(c, errors.New("ProcessOCRFromURL.EmptyURL", "File URL is required", nil))
		return
	}

	// 下载文件
	fileData, _, err := utils.DownloadFileFromURLWithContext(c.Request.Context(), req.FileURL)
	if err != nil {
		response.APIError(c, errors.New("ProcessOCRFromURL.DownloadFile", "Failed to download file", err))
		return
	}

	// 检查文件大小限制 (50MB)
	if len(fileData) > 50*1024*1024 {
		response.APIError(c, errors.New("ProcessOCRFromURL.FileSizeLimit", "File size exceeds limit (50MB)", nil))
		return
	}

	// 自动检测文件类型并处理
	ocrResult, err := s.Core.Srv().AI().ProcessOCR(c.Request.Context(), fileData)
	if err != nil {
		response.APIError(c, errors.New("ProcessOCRFromURL.ProcessOCR", "Failed to process OCR", err))
		return
	}

	result := convertOCRResult(ocrResult)
	response.APISuccess(c, result)
}

// convertOCRResult 转换OCR结果为响应格式
func convertOCRResult(ocrResult *baidu.OCRProcessResult) OCRResponse {
	response := OCRResponse{
		Title:        ocrResult.Title,
		MarkdownText: ocrResult.MarkdownText,
		Images:       ocrResult.Images,
		Model:        ocrResult.Model,
	}

	if ocrResult.Usage != nil {
		response.TokensUsed = ocrResult.Usage.TokensUsed
	}

	return response
}
