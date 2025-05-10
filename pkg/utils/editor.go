package utils

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/davidscottmills/goeditorjs"
	"github.com/samber/lo"
)

var editorJSMarkdownEngine *goeditorjs.MarkdownEngine

func init() {
	editorJSMarkdownEngine = goeditorjs.NewMarkdownEngine()
	// Register the handlers you wish to use
	editorJSMarkdownEngine.RegisterBlockHandlers(
		&goeditorjs.HeaderHandler{},
		&goeditorjs.ParagraphHandler{},
		&goeditorjs.ListHandler{},
		&goeditorjs.CodeBoxHandler{},
		&goeditorjs.CodeHandler{},
		&QuoteHandler{},
		&ImageHandler{},
		&goeditorjs.TableHandler{},
		&VideoHandler{},
		&ListV2Handler{},
		&LineHandler{},
	)
}

func ConvertEditorJSBlocksToMarkdown(blockString json.RawMessage) (string, error) {
	return editorJSMarkdownEngine.GenerateMarkdownWithUnknownBlock(string(blockString))
}

// list represents list data from EditorJS
type listv2 struct {
	Style string       `json:"style"`
	Items []listv2Item `json:"items"`
}

type listv2Item struct {
	Content string          `json:"content"`
	Items   json.RawMessage `json:"items"`
	Meta    listV2ItemMeta  `json:"meta"`
}

type listV2ItemMeta struct {
	Checked bool `json:"checked,omitempty"`
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

// image represents image data from EditorJS
type EditorVideo struct {
	File           EditorVideoFile `json:"file"`
	Caption        string          `json:"caption"`
	WithBorder     bool            `json:"withBorder"`
	WithBackground bool            `json:"withBackground"`
	Stretched      bool            `json:"stretched"`
}

type EditorVideoFile struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

type VideoHandler struct{}

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

	html := strings.Builder{}
	html.WriteString("<video controls preload=\"metadata\">")
	html.WriteString(fmt.Sprintf("<source src=\"%s\">", data.File.URL))
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

// Line
type line struct {
	Style         string `json:"style"`
	LineThickness int    `json:"lineThickness"`
	LineWidth     int    `json:"lineWidth"`
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

type EditorImage struct {
	File           EditorImageFile `json:"file"`
	Caption        string          `json:"caption"`
	WithBorder     bool            `json:"withBorder"`
	WithBackground bool            `json:"withBackground"`
	Stretched      bool            `json:"stretched"`
}

type EditorImageFile struct {
	URL string `json:"url"`
}

type ImageHandler struct {
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

	return fmt.Sprintf(`<img src="%s" alt="%s" %s/>`, image.File.URL, image.Caption, class), nil
}

type quote struct {
	Alignment string `json:"alignment"`
	Caption   string `json:"caption"`
	Text      string `json:"text"`
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

type EditorParagraph struct {
	Text      string `json:"text"`
	Alignment string `json:"alignment"`
}
