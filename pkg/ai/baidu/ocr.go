package baidu

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/quka-ai/quka-ai/pkg/ai"
)

const (
	NAME = "百度"
)

type Driver struct {
	apiURL string
	token  string
	client *http.Client
}

type Config struct {
	APIURL string `toml:"api_url"`
	Token  string `toml:"token"`
}

type OCRRequest struct {
	File                      string `json:"file"`
	FileType                  int    `json:"fileType"`
	UseDocOrientationClassify bool   `json:"useDocOrientationClassify,omitempty"`
	UseDocUnwarping           bool   `json:"useDocUnwarping,omitempty"`
	UseChartRecognition       bool   `json:"useChartRecognition,omitempty"`
}

type OCRResponse struct {
	Result *OCRResult `json:"result"`
	Error  *OCRError  `json:"error,omitempty"`
}

type OCRResult struct {
	LayoutParsingResults []LayoutParsingResult `json:"layoutParsingResults"`
}

type LayoutParsingResult struct {
	Markdown     MarkdownResult    `json:"markdown"`
	OutputImages map[string]string `json:"outputImages"`
}

type MarkdownResult struct {
	Text   string            `json:"text"`
	Images map[string]string `json:"images"`
}

type OCRError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type OCRProcessResult struct {
	Title        string    `json:"title"`
	MarkdownText string    `json:"markdown_text"`
	Images       []string  `json:"images"`
	Usage        *OCRUsage `json:"usage,omitempty"`
	Model        string    `json:"model"`
}

type OCRUsage struct {
	TokensUsed int `json:"tokens_used"`
}

func New(config Config) *Driver {
	return &Driver{
		apiURL: config.APIURL,
		token:  config.Token,
		client: &http.Client{},
	}
}

func (d *Driver) Lang() string {
	return ai.MODEL_BASE_LANGUAGE_CN
}

// ProcessOCR 处理OCR识别，自动检测文件类型（PDF或图片）
func (d *Driver) ProcessOCR(ctx context.Context, fileData []byte) (*OCRProcessResult, error) {
	detectedType := detectFileType(fileData)

	var fileType int
	switch detectedType {
	case "pdf":
		fileType = 0 // 0 for PDF files
	case "image":
		fileType = 1 // 1 for image files
	default:
		return nil, fmt.Errorf("unsupported file type: %s", detectedType)
	}

	return d.processOCRInternal(ctx, fileData, fileType)
}

func (d *Driver) processOCRInternal(ctx context.Context, fileData []byte, fileType int) (*OCRProcessResult, error) {
	slog.Debug("Processing OCR", slog.String("driver", NAME))

	encodedFile := base64.StdEncoding.EncodeToString(fileData)

	ocrReq := OCRRequest{
		File:                      encodedFile,
		FileType:                  fileType,
		UseDocOrientationClassify: false,
		UseDocUnwarping:           false,
		UseChartRecognition:       false,
	}

	reqBody, err := json.Marshal(ocrReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal OCR request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", d.apiURL, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("token %s", d.token))
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var ocrResp OCRResponse
	if err := json.Unmarshal(body, &ocrResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if ocrResp.Error != nil {
		return nil, fmt.Errorf("OCR API error: %s (code: %d)", ocrResp.Error.Message, ocrResp.Error.Code)
	}

	if ocrResp.Result == nil || len(ocrResp.Result.LayoutParsingResults) == 0 {
		return nil, fmt.Errorf("no OCR results returned")
	}

	result := &OCRProcessResult{
		Title:        extractTitle(ocrResp.Result.LayoutParsingResults),
		MarkdownText: combineMarkdownText(ocrResp.Result.LayoutParsingResults),
		Images:       extractImageURLs(ocrResp.Result.LayoutParsingResults),
		Usage: &OCRUsage{
			TokensUsed: len(reqBody), // Simplified token calculation
		},
		Model: NAME,
	}

	return result, nil
}

func extractTitle(results []LayoutParsingResult) string {
	if len(results) == 0 {
		return "OCR Document"
	}

	markdownText := results[0].Markdown.Text
	lines := []rune(markdownText)
	if len(lines) > 50 {
		return string(lines[:50]) + "..."
	}
	return markdownText
}

func combineMarkdownText(results []LayoutParsingResult) string {
	var combinedText string
	for i, result := range results {
		if i > 0 {
			combinedText += "\n\n---\n\n"
		}
		combinedText += result.Markdown.Text
	}
	return combinedText
}

func extractImageURLs(results []LayoutParsingResult) []string {
	var imageURLs []string
	for _, result := range results {
		for _, imageURL := range result.Markdown.Images {
			imageURLs = append(imageURLs, imageURL)
		}
		for _, imageURL := range result.OutputImages {
			imageURLs = append(imageURLs, imageURL)
		}
	}
	return imageURLs
}

func detectFileType(data []byte) string {
	if len(data) < 2 {
		return "unknown"
	}

	// Check PDF signature
	if len(data) >= 4 && string(data[:4]) == "%PDF" {
		return "pdf"
	}

	// Check common image signatures
	switch {
	case len(data) >= 8 &&
		data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 &&
		data[4] == 0x0D && data[5] == 0x0A && data[6] == 0x1A && data[7] == 0x0A:
		return "image" // PNG
	case len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF:
		return "image" // JPEG
	case len(data) >= 6 && (string(data[:6]) == "GIF87a" || string(data[:6]) == "GIF89a"):
		return "image" // GIF
	case len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP":
		return "image" // WEBP
	case len(data) >= 2 && data[0] == 0x42 && data[1] == 0x4D:
		return "image" // BMP
	default:
		return "unknown"
	}
}

type OCRProvider interface {
	ProcessOCR(ctx context.Context, fileData []byte) (*OCRProcessResult, error)
	Lang() string
}

var _ OCRProvider = (*Driver)(nil)
