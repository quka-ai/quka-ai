package v1

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/sashabaranov/go-openai"

	"github.com/quka-ai/quka-ai/app/core"
	"github.com/quka-ai/quka-ai/app/core/srv"
	"github.com/quka-ai/quka-ai/app/logic/v1/process"
	"github.com/quka-ai/quka-ai/pkg/ai"
	"github.com/quka-ai/quka-ai/pkg/errors"
	"github.com/quka-ai/quka-ai/pkg/i18n"
	"github.com/quka-ai/quka-ai/pkg/reader/rednote"
	"github.com/quka-ai/quka-ai/pkg/types"
	"github.com/quka-ai/quka-ai/pkg/utils"
)

type ReaderLogic struct {
	ctx  context.Context
	core *core.Core
	UserInfo
}

func NewReaderLogic(ctx context.Context, core *core.Core) *ReaderLogic {
	l := &ReaderLogic{
		ctx:      ctx,
		core:     core,
		UserInfo: SetupUserInfo(ctx, core),
	}

	return l
}

var downloader = &http.Client{
	Timeout: time.Minute,
}

func downloadFile(url string) ([]byte, error) {
	resp, err := downloader.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func (l *ReaderLogic) Reader(endpoint string) (*ai.ReaderResult, error) {
	if rednote.Match(endpoint) {
		detail, err := rednote.Read(endpoint)
		if err != nil {
			return nil, errors.New("ReaderLogic.Reader.RedNote.Read", i18n.ERROR_INTERNAL, err)
		}

		// create knowledge
		images := make(map[string][]byte)
		for _, v := range detail.Images {
			// 下载图片
			imageData, err := downloadFile(v)
			if err != nil {
				return nil, errors.New("ReaderLogic.Reader.RedNote.DownloadFile", i18n.ERROR_INTERNAL, err)
			}
			images[v] = imageData
		}
		date := time.Now().Format("20060102")
		for url, data := range images {
			fileName := utils.MD5(url)
			filePath := fmt.Sprintf("/quka/%s/knowledge/%s", l.GetUserInfo().User, date)
			if err = l.core.FileStorage().SaveFile(filePath, fileName, data); err != nil {
				return nil, errors.New("ReaderLogic.Reader.RedNote.SaveFile", i18n.ERROR_INTERNAL, err)
			}
		}

		videos := make(map[string][]byte)
		for _, v := range detail.Videos {
			// 下载图片
			data, err := downloadFile(v)
			if err != nil {
				return nil, errors.New("ReaderLogic.Reader.RedNote.DownloadFile", i18n.ERROR_INTERNAL, err)
			}
			videos[v] = data
		}
		for url, data := range videos {
			fileName := utils.MD5(url)
			filePath := fmt.Sprintf("/quka/%s/knowledge/%s", l.GetUserInfo().User, date)
			if err = l.core.FileStorage().SaveFile(filePath, fileName, data); err != nil {
				return nil, errors.New("ReaderLogic.Reader.RedNote.SaveFile", i18n.ERROR_INTERNAL, err)
			}
		}
		// 创建knowledge
	}

	res, err := l.core.Srv().AI().Reader(l.ctx, endpoint)
	if err != nil {
		errMsg := i18n.ERROR_INTERNAL
		code := http.StatusInternalServerError

		if err == srv.ERROR_UNSUPPORTED_FEATURE {
			errMsg = i18n.ERROR_UNSUPPORTED_FEATURE
			code = http.StatusForbidden
		}
		return nil, errors.New("ReaderLogic.Reader.Srv.AI.Reader", errMsg, err).Code(code)
	}

	process.NewRecordUsageRequest("", types.USAGE_TYPE_USER, types.USAGE_SUB_TYPE_READ, "", l.GetUserInfo().User, &openai.Usage{
		CompletionTokens: res.Usage.Tokens,
	})

	return res, nil
}

func (l *ReaderLogic) DescribeImage(imageURL string) (string, error) {
	if strings.Contains(imageURL, ".svg") || strings.Contains(imageURL, ".gif") {
		return "", errors.New("KnowledgeLogic.DescribeImage.Get", i18n.ERROR_IMAGE_TYPE_UNSUPPORT, nil).Code(http.StatusBadRequest)
	}

	imageResponse, err := http.Get(imageURL)
	if err != nil {
		return "", errors.New("KnowledgeLogic.DescribeImage.Get", i18n.ERROR_IMAGE_READ_FAIL, err).Code(http.StatusBadRequest)
	}

	defer imageResponse.Body.Close()
	if imageResponse.StatusCode != http.StatusOK {
		imageURL, err = l.core.FileStorage().GenGetObjectPreSignURL(imageURL)
		if err != nil {
			return "", errors.New("KnowledgeLogic.DescribeImage.GenGetObjectPreSignURL", i18n.ERROR_IMAGE_READ_FAIL, err).Code(http.StatusBadRequest)
		}
	}

	// if strings.Contains(imageURL, ".svg") {
	// 	url, err := url.Parse(imageURL)
	// 	if err != nil {
	// 		return "", errors.New("KnowledgeLogic.DescribeImage.Parse", i18n.ERROR_IMAGE_READ_FAIL, err).Code(http.StatusBadRequest)
	// 	}

	// 	ctx, cancel := context.WithTimeout(l.ctx, time.Minute)
	// 	defer cancel()
	// 	obj, err := l.core.FileStorage().DownloadFile(ctx, url.RequestURI())
	// 	if err != nil {
	// 		return "", errors.New("KnowledgeLogic.DescribeImage.FileStorage.DownloadFile", i18n.ERROR_IMAGE_READ_FAIL, err).Code(http.StatusBadRequest)
	// 	}

	// 	pngImage, err := utils.ConvertSVGToPNG(obj.File)
	// 	if err != nil {
	// 		return "", errors.New("KnowledgeLogic.DescribeImage.SvgToPng", i18n.ERROR_IMAGE_READ_FAIL, err).Code(http.StatusBadRequest)
	// 	}

	// 	encodeImage := base64.StdEncoding.EncodeToString(pngImage)
	// 	imageURL = fmt.Sprintf("data:image/png;base64,%s", encodeImage)
	// 	// if err = l.core.FileStorage().SaveFile("/tmp/convert/", utils.MD5(url.RequestURI())+".png", pngImage); err != nil {
	// 	// 	return "", err
	// 	// }
	// 	// path := fmt.Sprintf("/tmp/convert/%s", utils.MD5(url.RequestURI())+".png")
	// 	// fmt.Println(path)
	// 	imageURL, err = l.core.FileStorage().GenGetObjectPreSignURL("/tmp/convert/Ollama (1).png")
	// 	if err != nil {
	// 		return "", errors.New("KnowledgeLogic.DescribeImage.GenGetObjectPreSignURL", i18n.ERROR_IMAGE_READ_FAIL, err).Code(http.StatusBadRequest)
	// 	}

	// 	// fmt.Println(imageURL)
	// }

	resp, err := l.core.Srv().AI().DescribeImage(l.ctx, GetContentByClientLanguage(l.ctx, "English", "中文"), imageURL)
	if err != nil {
		return "", errors.New("KnowledgeLogic.DescribeImage.Query", i18n.ERROR_INTERNAL, err)
	}

	if resp.Usage != nil {
		process.NewRecordUsageRequest(resp.Model, types.USAGE_TYPE_SYSTEM, types.USAGE_SUB_TYPE_DESCRIBE_IMAGE, "", l.GetUserInfo().User, resp.Usage)
	}

	return resp.Message(), nil
}

// func describeImage(ctx context.Context, driver srv.VisionAI, imageURL string) (ai.GenerateResponse, error) {
// 	opts := driver.NewVisionQuery(ctx, []*types.MessageContext{
// 		{
// 			Role: types.USER_ROLE_USER,
// 			MultiContent: []openai.ChatMessagePart{
// 				{
// 					Type: openai.ChatMessagePartTypeImageURL,
// 					ImageURL: &openai.ChatMessageImageURL{
// 						URL: imageURL,
// 					},
// 				},
// 			},
// 		},
// 	})

// 	opts.WithPrompt(lo.If(driver.Lang() == ai.MODEL_BASE_LANGUAGE_CN, ai.IMAGE_GENERATE_PROMPT_CN).Else(ai.IMAGE_GENERATE_PROMPT_EN))
// 	opts.WithVar("{lang}", GetContentByClientLanguage(ctx, "English", "中文"))
// 	resp, err := opts.Query()
// 	if err != nil {
// 		return resp, err
// 	}

// 	return resp, nil
// }
