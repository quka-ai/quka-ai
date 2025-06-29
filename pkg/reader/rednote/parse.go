package rednote

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/davidscottmills/goeditorjs"
	"github.com/lib/pq"
	"github.com/samber/lo"

	"github.com/quka-ai/quka-ai/app/core"
	"github.com/quka-ai/quka-ai/pkg/types"
	"github.com/quka-ai/quka-ai/pkg/utils"
)

type Knowledge struct {
	Title       string                     `json:"title" db:"title"`
	Tags        pq.StringArray             `json:"tags" db:"tags"`
	Content     types.KnowledgeContent     `json:"content" db:"content"`
	ContentType types.KnowledgeContentType `json:"content_type" db:"content_type"`
}

type FileWithContentType struct {
	Content     []byte
	ContentType string
}

func ParseRedNote(ctx context.Context, spaceID string, detail *NoteDetail, objectStorage core.FileStorage) (*Knowledge, error) {
	var knowledgeContent types.BlockContent
	// create knowledge
	images := make(map[string]*FileWithContentType)
	for _, v := range detail.Images {
		// 下载图片
		imageData, contentType, err := DownloadFile(v)
		if err != nil {
			return nil, fmt.Errorf("Failed to download rednote images, %w", err)
		}
		images[v] = &FileWithContentType{
			Content:     imageData,
			ContentType: contentType,
		}
	}

	var imageBlocks []utils.EditorImage
	for url, data := range images {
		ct := strings.Split(data.ContentType, "/")
		fileName := fmt.Sprintf("%s.%s", utils.MD5(url), ct[len(ct)-1])
		filePath := types.GenS3FilePath(spaceID, "rednote", fileName)
		if err := objectStorage.SaveFile(filePath, data.Content); err != nil {
			return nil, fmt.Errorf("Failed to save image, %w", err)
		}

		fullURL := fmt.Sprintf("https://%s/%s", objectStorage.GetStaticDomain(), strings.TrimPrefix(filepath.Join(filePath, fileName), "/"))
		imageBlocks = append(imageBlocks, utils.EditorImage{
			File: utils.EditorImageFile{
				URL: fullURL,
			},
		})
	}

	var videoBlocks []utils.EditorVideo

	videos := make(map[string]*FileWithContentType)
	for _, v := range detail.Videos {
		// 下载图片
		data, contentType, err := DownloadFile(v)
		if err != nil {
			return nil, fmt.Errorf("Failed to download rednote videos, %w", err)
		}
		videos[v] = &FileWithContentType{
			Content:     data,
			ContentType: contentType,
		}
	}
	for url, data := range videos {
		ct := strings.Split(data.ContentType, "/")
		fileName := fmt.Sprintf("%s.%s", utils.MD5(url), ct[len(ct)-1])
		filePath := types.GenS3FilePath(spaceID, "rednote", fileName)
		if err := objectStorage.SaveFile(filePath, data.Content); err != nil {
			return nil, fmt.Errorf("Failed to save video, %w", err)
		}

		fullURL := fmt.Sprintf("https://%s/%s", objectStorage.GetStaticDomain(), strings.TrimPrefix(filepath.Join(filePath, fileName), "/"))
		videoBlocks = append(videoBlocks, utils.EditorVideo{
			File: utils.EditorVideoFile{
				URL: fullURL,
			},
		})
	}
	// 创建knowledge

	knowledgeContent.Blocks = append(knowledgeContent.Blocks, goeditorjs.EditorJSBlock{
		Type: "paragraph",
		Data: func() json.RawMessage {
			raw, _ := json.Marshal(utils.EditorParagraph{
				Text: detail.Content,
			})
			return raw
		}(),
	})

	if len(imageBlocks) > 0 {
		knowledgeContent.Blocks = lo.Map(imageBlocks, func(item utils.EditorImage, index int) goeditorjs.EditorJSBlock {
			raw, _ := json.Marshal(item)
			return goeditorjs.EditorJSBlock{
				Type: "image",
				Data: raw,
			}
		})
	}

	if len(videoBlocks) > 0 {
		knowledgeContent.Blocks = lo.Map(videoBlocks, func(item utils.EditorVideo, index int) goeditorjs.EditorJSBlock {
			raw, _ := json.Marshal(item)
			return goeditorjs.EditorJSBlock{
				Type: "video",
				Data: raw,
			}
		})
	}

	contentRaw, _ := json.Marshal(knowledgeContent)
	result := &Knowledge{
		Title:       detail.Title,
		Tags:        detail.Tags,
		Content:     contentRaw,
		ContentType: types.KNOWLEDGE_CONTENT_TYPE_BLOCKS,
	}
	return result, nil
}

var downloader = &http.Client{
	Timeout: time.Minute,
}

func DownloadFile(url string) ([]byte, string, error) {
	resp, err := downloader.Get(url)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}
	return body, resp.Header.Get("content-type"), nil
}
