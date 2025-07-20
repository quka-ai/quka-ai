package editorjs

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/davidscottmills/goeditorjs"
	"github.com/samber/lo"
)

var editorJSMarkdownEngine *goeditorjs.MarkdownEngine

// SetupGlobalEditorJS 初始化全局EditorJS引擎
func SetupGlobalEditorJS(staticDomain string) {
	editorJSMarkdownEngine = goeditorjs.NewMarkdownEngine()
	// Register the handlers you wish to use
	editorJSMarkdownEngine.RegisterBlockHandlers(
		&goeditorjs.HeaderHandler{},
		&goeditorjs.ParagraphHandler{},
		&goeditorjs.ListHandler{},
		&goeditorjs.CodeBoxHandler{},
		&goeditorjs.CodeHandler{},
		&QuoteHandler{},
		&ImageHandler{StaticDomain: staticDomain},
		&goeditorjs.TableHandler{},
		&VideoHandler{StaticDomain: staticDomain},
		&ListV2Handler{},
		&LineHandler{},
	)
}

// ConvertEditorJSRawToMarkdown 将EditorJS原始数据转换为Markdown
func ConvertEditorJSRawToMarkdown(blockString json.RawMessage) (string, error) {
	return editorJSMarkdownEngine.GenerateMarkdownWithUnknownBlock(string(blockString))
}

// RemoveFileBlockHost 移除块中文件的主机名
func RemoveFileBlockHost(blocks []goeditorjs.EditorJSBlock, bucketName string) []goeditorjs.EditorJSBlock {
	for i, block := range blocks {
		switch block.Type {
		case "image":
			image := &EditorImage{}
			if err := json.Unmarshal(block.Data, image); err != nil {
				continue
			}
			u, _ := url.Parse(image.File.URL)
			image.File.URL = lo.If(bucketName != "" && strings.HasPrefix(u.Path, "/"+bucketName), u.Path[len(bucketName)+1:]).Else(u.Path)
			blocks[i].Data, _ = json.Marshal(image)
		case "video":
			video := &EditorVideo{}
			if err := json.Unmarshal(block.Data, video); err != nil {
				continue
			}
			u, _ := url.Parse(video.File.URL)
			video.File.URL = lo.If(bucketName != "" && strings.HasPrefix(u.Path, "/"+bucketName), u.Path[len(bucketName)+1:]).Else(u.Path)
			blocks[i].Data, _ = json.Marshal(video)
		default:
			continue
		}
	}
	return blocks
}

// ConvertEditorJSBlocksToMarkdown 将EditorJS块转换为Markdown
func ConvertEditorJSBlocksToMarkdown(blocks []goeditorjs.EditorJSBlock) (string, error) {
	results := []string{}
	for _, block := range blocks {
		if generator, ok := editorJSMarkdownEngine.BlockHandlers[block.Type]; ok {
			md, err := generator.GenerateMarkdown(block)
			if err != nil {
				continue
			}
			results = append(results, md)
		}
	}
	return strings.Join(results, "\n\n"), nil
}

// ListV2Handler is the default ListV2Handler for EditorJS HTML generation
type ListV2Handler struct{}

func (*ListV2Handler) parse(editorJSBlock goeditorjs.EditorJSBlock) (*listv2, error) {
	list := &listv2{}
	return list, json.Unmarshal(editorJSBlock.Data, list)
}

// Type "listv2"
func (*ListV2Handler) Type() string {
	return "listv2"
}

func renderListv2Html(style string, list []listv2Item) (string, error) {
	result := ""
	if style == "ordered" {
		result = "<ol>%s</ol>"
	} else {
		result = "<ul>%s</ul>"
	}
	innerData := strings.Builder{}
	for _, s := range list {
		if len(s.Items) > 0 {

			var inner []listv2Item
			if err := json.Unmarshal(s.Items, &inner); err != nil {
				return "", err
			}
			innerHtml, err := renderListv2Html(style, inner)
			if err != nil {
				return "", err
			}

			s.Content = fmt.Sprintf("<span>%s</span>%s", s.Content, innerHtml)
		}
		innerData.WriteString("<li>")
		innerData.WriteString(s.Content)
		innerData.WriteString("</li>")
	}

	if innerData.Len() > 0 {
		return fmt.Sprintf(result, innerData.String()), nil
	}
	return "", nil
}

// GenerateHTML generates html for ListBlocks
func (h *ListV2Handler) GenerateHTML(editorJSBlock goeditorjs.EditorJSBlock) (string, error) {
	list, err := h.parse(editorJSBlock)
	if err != nil {
		return "", err
	}

	return renderListv2Html(list.Style, list.Items)
}

func renderListv2Markdown(style string, index int, list []listv2Item) (string, error) {
	positionPrefix := strings.Repeat("  ", index)
	listItemPrefix := positionPrefix + "- "
	results := []string{}
	for i, s := range list {
		switch style {
		case "ordered":
			listItemPrefix = fmt.Sprintf("%d. ", i+1)
		case "checklist":
			listItemPrefix = fmt.Sprintf("- [%s] ", lo.If(s.Meta.Checked, "x").Else(" "))
		default:
		}

		results = append(results, fmt.Sprintf("%s%s  ", listItemPrefix, s.Content))
		if len(s.Items) > 0 {
			var inner []listv2Item
			if err := json.Unmarshal(s.Items, &inner); err != nil {
				return "", err
			}
			innerMarkdown, err := renderListv2Markdown(style, index+1, inner)
			if err != nil {
				return "", err
			}
			if innerMarkdown != "" {
				results = append(results, innerMarkdown)
			}
		}
	}

	if len(results) > 0 {
		return strings.Join(results, "\n"), nil
	}
	return "", nil
}

// GenerateMarkdown generates markdown for ListBlocks
func (h *ListV2Handler) GenerateMarkdown(editorJSBlock goeditorjs.EditorJSBlock) (string, error) {
	list, err := h.parse(editorJSBlock)
	if err != nil {
		return "", err
	}

	return renderListv2Markdown(list.Style, 0, list.Items)
}

type VideoHandler struct {
	StaticDomain string
}

func (*VideoHandler) parse(editorJSBlock goeditorjs.EditorJSBlock) (*EditorVideo, error) {
	data := &EditorVideo{}
	return data, json.Unmarshal(editorJSBlock.Data, data)
}

// Type "video"
func (*VideoHandler) Type() string {
	return "video"
}

// GenerateHTML generates html for ListBlocks
func (h *VideoHandler) GenerateHTML(editorJSBlock goeditorjs.EditorJSBlock) (string, error) {
	data, err := h.parse(editorJSBlock)
	if err != nil {
		return "", err
	}

	res, _ := url.Parse(data.File.URL)
	if res.Host == "" {
		res.Host = h.StaticDomain
	}

	html := strings.Builder{}
	html.WriteString("<video controls preload=\"metadata\">")
	html.WriteString(fmt.Sprintf("<source src=\"%s\">", res.RawPath))
	html.WriteString("</video>")
	if data.Caption != "" {
		html.WriteString("\n")
		html.WriteString(data.Caption)
	}

	return html.String(), nil
}

// GenerateMarkdown generates markdown for ListBlocks
func (h *VideoHandler) GenerateMarkdown(editorJSBlock goeditorjs.EditorJSBlock) (string, error) {
	return h.GenerateHTML(editorJSBlock)
}

type LineHandler struct{}

func (*LineHandler) parse(editorJSBlock goeditorjs.EditorJSBlock) (*line, error) {
	line := &line{}
	return line, json.Unmarshal(editorJSBlock.Data, line)
}

// Type "delimiter"
func (*LineHandler) Type() string {
	return "delimiter"
}

func renderLineHtml(line *line) (string, error) {
	return fmt.Sprintf("<div class=\"ce-delimiter cdx-block ce-delimiter-line\"><hr class=\"ce-delimiter-thickness-%d\" style=\"width: %d%%;\"></div>", line.LineThickness, line.LineWidth), nil
}

// GenerateHTML generates html for ListBlocks
func (h *LineHandler) GenerateHTML(editorJSBlock goeditorjs.EditorJSBlock) (string, error) {
	line, err := h.parse(editorJSBlock)
	if err != nil {
		return "", err
	}

	return renderLineHtml(line)
}

func renderLineMarkdown(line *line) (string, error) {
	return "---", nil
}

// GenerateMarkdown generates markdown for ListBlocks
func (h *LineHandler) GenerateMarkdown(editorJSBlock goeditorjs.EditorJSBlock) (string, error) {
	list, err := h.parse(editorJSBlock)
	if err != nil {
		return "", err
	}

	return renderLineMarkdown(list)
}

type ImageHandler struct {
	StaticDomain string
	// Options are made available to the GenerateHTML and GenerateMarkdown functions.
	// If not provided, DefaultImageHandlerOptions will be used.
	Options *ImageHandlerOptions
}

// ImageHandlerOptions are the options available to the ImageHandler
type ImageHandlerOptions struct {
	BorderClass     string
	StretchClass    string
	BackgroundClass string
}

// DefaultImageHandlerOptions are the default options available to the ImageHandler
var DefaultImageHandlerOptions = &ImageHandlerOptions{
	StretchClass:    "image-tool--stretched",
	BorderClass:     "image-tool--withBorder",
	BackgroundClass: "image-tool--withBackground"}

func (*ImageHandler) parse(editorJSBlock goeditorjs.EditorJSBlock) (*EditorImage, error) {
	image := &EditorImage{}
	return image, json.Unmarshal(editorJSBlock.Data, image)
}

// Type "image"
func (*ImageHandler) Type() string {
	return "image"
}

// GenerateHTML generates html for ImageBlocks
func (h *ImageHandler) GenerateHTML(editorJSBlock goeditorjs.EditorJSBlock) (string, error) {
	image, err := h.parse(editorJSBlock)
	if err != nil {
		return "", err
	}

	return h.generateHTML(image)
}

// GenerateMarkdown generates markdown for ImageBlocks
func (h *ImageHandler) GenerateMarkdown(editorJSBlock goeditorjs.EditorJSBlock) (string, error) {
	image, err := h.parse(editorJSBlock)
	if err != nil {
		return "", err
	}

	if image.Stretched || image.WithBackground || image.WithBorder {
		return h.generateHTML(image)
	}

	return fmt.Sprintf("![alt text](%s)  \n%s", image.File.URL, lo.If(image.Caption != "", image.Caption+"  \n").Else("")), nil

}

func (h *ImageHandler) generateHTML(image *EditorImage) (string, error) {
	if h.Options == nil {
		h.Options = DefaultImageHandlerOptions
	}

	classes := []string{}
	if image.Stretched {
		classes = append(classes, h.Options.StretchClass)
	}

	if image.WithBorder {
		classes = append(classes, h.Options.BorderClass)
	}

	if image.WithBackground {
		classes = append(classes, h.Options.BackgroundClass)
	}

	class := ""
	if len(classes) > 0 {
		class = fmt.Sprintf(`class="%s"`, strings.Join(classes, " "))
	}

	res, _ := url.Parse(image.File.URL)
	if res.Host == "" {
		res.Host = h.StaticDomain
	}
	url := res.RawPath

	return fmt.Sprintf(`<img src="%s" alt="%s" %s/>`, url, image.Caption, class), nil
}

type QuoteHandler struct{}

func (*QuoteHandler) parse(editorJSBlock goeditorjs.EditorJSBlock) (*quote, error) {
	data := &quote{}
	return data, json.Unmarshal(editorJSBlock.Data, data)
}

// Type "delimiter"
func (*QuoteHandler) Type() string {
	return "quote"
}

func renderQuoteHtml(data *quote) (string, error) {
	return fmt.Sprintf("<blockquote class=\"ce-quota cdx-block\">%s%s</blockquote>", data.Text, lo.If(data.Caption != "", fmt.Sprintf("<p><cite>%s</cite></p>", data.Caption)).Else("")), nil
}

// GenerateHTML generates html for ListBlocks
func (h *QuoteHandler) GenerateHTML(editorJSBlock goeditorjs.EditorJSBlock) (string, error) {
	line, err := h.parse(editorJSBlock)
	if err != nil {
		return "", err
	}

	return renderQuoteHtml(line)
}

func renderQuoteMarkdown(data *quote) (string, error) {
	return fmt.Sprintf("> %s", data.Text), nil
}

// GenerateMarkdown generates markdown for ListBlocks
func (h *QuoteHandler) GenerateMarkdown(editorJSBlock goeditorjs.EditorJSBlock) (string, error) {
	data, err := h.parse(editorJSBlock)
	if err != nil {
		return "", err
	}

	return renderQuoteMarkdown(data)
}
