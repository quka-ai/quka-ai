package handler

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"

	v1 "github.com/quka-ai/quka-ai/app/logic/v1"
	"github.com/quka-ai/quka-ai/app/response"
	"github.com/quka-ai/quka-ai/pkg/errors"
	"github.com/quka-ai/quka-ai/pkg/i18n"
	"github.com/quka-ai/quka-ai/pkg/utils"
)

type CreateKnowledgeShareTokenRequest struct {
	EmbeddingURL string `json:"embedding_url" binding:"required"`
	KnowledgeID  string `json:"knowledge_id" binding:"required"`
}

type CreateKnowledgeShareTokenResponse struct {
	Token string `json:"token"`
	URL   string `json:"url"`
}

func (s *HttpSrv) CreateKnowledgeShareToken(c *gin.Context) {
	var (
		err error
		req CreateKnowledgeShareTokenRequest
	)
	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	spaceID, _ := v1.InjectSpaceID(c)
	if strings.HasPrefix(req.EmbeddingURL, "wails") && s.Core.Cfg().Site.Share.Domain != "" {
		req.EmbeddingURL = strings.ReplaceAll(req.EmbeddingURL, "wails://", "https://")
	}
	res, err := v1.NewManageShareLogic(c, s.Core).CreateKnowledgeShareToken(spaceID, req.KnowledgeID, req.EmbeddingURL)
	if err != nil {
		response.APIError(c, err)
		return
	}

	var shareURL string
	if s.Core.Cfg().Site.Share.Domain != "" {
		shareURL = genKnowledgeShareURL(s.Core.Cfg().Site.Share.Domain, res.Token)
	} else {
		shareURL = strings.ReplaceAll(req.EmbeddingURL, "{token}", res.Token)
	}

	response.APISuccess(c, CreateKnowledgeShareTokenResponse{
		Token: res.Token,
		URL:   shareURL,
	})
}

type CreateSessionShareTokenRequest struct {
	EmbeddingURL string `json:"embedding_url" binding:"required"`
	SessionID    string `json:"session_id" binding:"required"`
}

type CreateSessionShareTokenResponse struct {
	Token string `json:"token"`
	URL   string `json:"url"`
}

func (s *HttpSrv) CreateSessionShareToken(c *gin.Context) {
	var (
		err error
		req CreateSessionShareTokenRequest
	)
	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	spaceID, _ := v1.InjectSpaceID(c)
	res, err := v1.NewManageShareLogic(c, s.Core).CreateSessionShareToken(spaceID, req.SessionID, req.EmbeddingURL)
	if err != nil {
		response.APIError(c, err)
		return
	}

	var shareURL string
	if s.Core.Cfg().Site.Share.Domain != "" {
		shareURL = genSessionShareURL(s.Core.Cfg().Site.Share.Domain, res.Token)
	} else {
		shareURL = strings.ReplaceAll(req.EmbeddingURL, "{token}", res.Token)
	}

	response.APISuccess(c, CreateKnowledgeShareTokenResponse{
		Token: res.Token,
		URL:   shareURL,
	})
}

func genKnowledgeShareURL(domain, token string) string {
	return fmt.Sprintf("%s/s/k/%s", domain, token)
}

func genSessionShareURL(domain, token string) string {
	return fmt.Sprintf("%s/s/s/%s", domain, token)
}

func genSpaceShareURL(domain, token string) string {
	return fmt.Sprintf("%s/s/sp/%s", domain, token)
}

func (s *HttpSrv) GetKnowledgeByShareToken(c *gin.Context) {
	token, _ := c.Params.Get("token")
	res, err := v1.NewShareLogic(c, s.Core).GetKnowledgeByShareToken(token)
	if err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, res)
}

func (s *HttpSrv) GetSessionByShareToken(c *gin.Context) {
	token, _ := c.Params.Get("token")

	res, err := v1.NewShareLogic(c, s.Core).GetSessionByShareToken(token)
	if err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, res)
}

func (s *HttpSrv) BuildKnowledgeSharePage(c *gin.Context) {
	token, _ := c.Params.Get("token")

	res, err := v1.NewShareLogic(c, s.Core).GetKnowledgeByShareToken(token)
	if err != nil {
		response.APIError(c, err)
		return
	}

	parsedURL, err := url.Parse(res.EmbeddingURL)
	if err != nil {
		response.APIError(c, errors.New("api.BuildSharePage.url.Parse", i18n.ERROR_INTERNAL, err))
		return
	}

	c.HTML(http.StatusOK, "share.html", gin.H{
		"siteTitle":       s.Core.Cfg().Site.Share.SiteTitle,
		"siteDescription": s.Core.Cfg().Site.Share.SiteDescription,
		"title":           res.Title,
		"contentURL":      res.EmbeddingURL,
		"frontendURL":     fmt.Sprintf("%s://%s", parsedURL.Scheme, parsedURL.Host),
	})
}

func (s *HttpSrv) BuildSessionSharePage(c *gin.Context) {
	token, _ := c.Params.Get("token")

	res, err := v1.NewShareLogic(c, s.Core).GetSessionByShareToken(token)
	if err != nil {
		response.APIError(c, err)
		return
	}

	parsedURL, err := url.Parse(res.EmbeddingURL)
	if err != nil {
		response.APIError(c, errors.New("api.BuildSharePage.url.Parse", i18n.ERROR_INTERNAL, err))
		return
	}

	c.HTML(http.StatusOK, "share.html", gin.H{
		"siteTitle":       s.Core.Cfg().Site.Share.SiteTitle,
		"siteDescription": s.Core.Cfg().Site.Share.SiteDescription,
		"title":           res.Session.Title,
		"contentURL":      res.EmbeddingURL,
		"frontendURL":     fmt.Sprintf("%s://%s", parsedURL.Scheme, parsedURL.Host),
	})
}

func (s *HttpSrv) BuildSpaceSharePage(c *gin.Context) {
	token, _ := c.Params.Get("token")

	res, err := v1.NewShareLogic(c, s.Core).GetSpaceByShareToken(token)
	if err != nil {
		response.APIError(c, err)
		return
	}

	parsedURL, err := url.Parse(res.EmbeddingURL)
	if err != nil {
		response.APIError(c, errors.New("api.BuildSharePage.url.Parse", i18n.ERROR_INTERNAL, err))
		return
	}

	c.HTML(http.StatusOK, "share.html", gin.H{
		"siteTitle":       s.Core.Cfg().Site.Share.SiteTitle,
		"siteDescription": s.Core.Cfg().Site.Share.SiteDescription,
		"title":           res.Space.Title,
		"contentURL":      res.EmbeddingURL,
		"frontendURL":     fmt.Sprintf("%s://%s", parsedURL.Scheme, parsedURL.Host),
	})
}

type CopyKnowledgeRequest struct {
	Token        string `json:"token" binding:"required"`
	UserSpace    string `json:"user_space" binding:"required"`
	UserResource string `json:"user_resource" binding:"required"`
}

func (s *HttpSrv) CopyKnowledge(c *gin.Context) {
	var (
		err error
		req CopyKnowledgeRequest
	)
	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	if err = v1.NewShareLogic(c, s.Core).CopyKnowledgeByShareToken(req.Token, req.UserSpace, req.UserResource); err != nil {
		response.APIError(c, err)
		return
	}
	response.APISuccess(c, nil)
}

type CreateSpaceShareTokenRequest struct {
	EmbeddingURL string `json:"embedding_url" binding:"required"`
}

type CreateSpaceShareTokenResponse struct {
	Token string `json:"token"`
	URL   string `json:"url"`
}

func (s *HttpSrv) CreateSpaceShareToken(c *gin.Context) {
	var (
		err error
		req CreateSpaceShareTokenRequest
	)
	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	spaceID, _ := v1.InjectSpaceID(c)
	res, err := v1.NewManageShareLogic(c, s.Core).CreateSpaceShareToken(spaceID, req.EmbeddingURL)
	if err != nil {
		response.APIError(c, err)
		return
	}

	var shareURL string
	if s.Core.Cfg().Site.Share.Domain != "" {
		shareURL = genSpaceShareURL(s.Core.Cfg().Site.Share.Domain, res.Token)
	} else {
		shareURL = strings.ReplaceAll(req.EmbeddingURL, "{token}", res.Token)
	}

	response.APISuccess(c, CreateSpaceShareTokenResponse{
		Token: res.Token,
		URL:   shareURL,
	})
}
