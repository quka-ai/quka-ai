package v1

import (
	"context"
	"net/http"
	"strings"

	"github.com/sashabaranov/go-openai"

	"github.com/quka-ai/quka-ai/app/core"
	"github.com/quka-ai/quka-ai/app/logic/v1/process"
	"github.com/quka-ai/quka-ai/pkg/ai"
	"github.com/quka-ai/quka-ai/pkg/errors"
	"github.com/quka-ai/quka-ai/pkg/i18n"
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

type ReaderResult struct {
	Type    string           `json:"type"`
	AIResut *ai.ReaderResult `json:"ai_result,omitempty"`
	// RednoteResult *rednote.Knowledge `json:"rednote_result,omitempty"`
}

func (l *ReaderLogic) Reader(endpoint string) (*ReaderResult, error) {
	// switch true {
	// case rednote.Match(endpoint):
	// 	detail, err := rednote.Read(endpoint)
	// 	if err != nil {
	// 		return nil, errors.New("ReaderLogic.Reader.RedNote.Read", i18n.ERROR_INTERNAL, err)
	// 	}

	// 	knowledge, err := rednote.ParseRedNote(l.ctx, "unknown", detail, l.core.FileStorage())
	// 	if err != nil {
	// 		return nil, errors.New("ReaderLogic.Reader.RedNote.Parse", i18n.ERROR_INTERNAL, err)
	// 	}

	// 	return &ReaderResult{
	// 		Type:          "rednote",
	// 		RednoteResult: knowledge,
	// 	}, nil
	// default:
	// }

	res, err := l.core.Srv().AI().Reader(l.ctx, endpoint)
	if err != nil {
		errMsg := i18n.ERROR_INTERNAL
		code := http.StatusInternalServerError

		if errors.Is(err, errors.ERROR_UNSUPPORTED_FEATURE) {
			errMsg = i18n.ERROR_UNSUPPORTED_FEATURE
			code = http.StatusForbidden
		}
		return nil, errors.New("ReaderLogic.Reader.Srv.AI.Reader", errMsg, err).Code(code)
	}

	process.NewRecordUsageRequest("", types.USAGE_TYPE_USER, types.USAGE_SUB_TYPE_READ, "", l.GetUserInfo().User, &openai.Usage{
		CompletionTokens: res.Usage.Tokens,
	})

	return &ReaderResult{
		Type:    "ai",
		AIResut: res,
	}, nil
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
	} else {
		// 示例：将imageResponse转换为base64格式
		// 这在某些情况下可能会用到，比如需要将图片嵌入到消息中
		base64Image, err := utils.FileResponseToBase64(imageResponse)
		if err != nil {
			return "", errors.New("KnowledgeLogic.DescribeImage.FileResponseToBase64", i18n.ERROR_IMAGE_READ_FAIL, err).Code(http.StatusBadRequest)
		}

		// 在某些AI模型中，可以选择使用base64格式的图片
		imageURL = base64Image // 如果需要使用base64格式，可以取消注释这行
	}

	resp, err := l.core.Srv().AI().DescribeImage(l.ctx, GetContentByClientLanguage(l.ctx, "English", "中文"), imageURL)
	if err != nil {
		return "", errors.New("KnowledgeLogic.DescribeImage.Query", i18n.ERROR_INTERNAL, err)
	}

	if resp.Usage.CompletionTokens > 0 {
		process.NewRecordUsageRequest(resp.Model, types.USAGE_TYPE_SYSTEM, types.USAGE_SUB_TYPE_DESCRIBE_IMAGE, "", l.GetUserInfo().User, &resp.Usage)
	}

	return resp.Choices[0].Message.Content, nil
}
