package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	v1 "github.com/quka-ai/quka-ai/app/logic/v1"
	"github.com/quka-ai/quka-ai/app/response"
	"github.com/quka-ai/quka-ai/pkg/errors"
	"github.com/quka-ai/quka-ai/pkg/i18n"
	"github.com/quka-ai/quka-ai/pkg/types"
	"github.com/quka-ai/quka-ai/pkg/utils"
)

type UpsertJournalRequest struct {
	Date    string                 `json:"date" form:"date" binding:"required"`
	Content types.KnowledgeContent `json:"content" form:"content" binding:"required"`
}

func (s *HttpSrv) UpsertJournal(c *gin.Context) {
	var (
		err error
		req UpsertJournalRequest
	)
	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	spaceID, _ := v1.InjectSpaceID(c)
	err = v1.NewJournalLogic(c, s.Core).UpsertJournal(spaceID, req.Date, req.Content)
	if err != nil {
		response.APIError(c, err)
		return
	}
	response.APISuccess(c, nil)
}

type ListJournalRequest struct {
	StartDate string `json:"start_date" form:"start_date" binding:"required"`
	EndDate   string `json:"end_date" form:"end_date" binding:"required"`
}

func (r *ListJournalRequest) Validate() error {
	const layout = "2006-01-02"
	start, err := time.Parse(layout, r.StartDate)
	if err != nil {
		return errors.New("api.ListJournal.Validate.StartDate", i18n.ERROR_INVALIDARGUMENT, err).Code(http.StatusBadRequest)
	}

	end, err := time.Parse(layout, r.EndDate)
	if err != nil {
		return errors.New("api.ListJournal.Validate.EndDate", i18n.ERROR_INVALIDARGUMENT, err).Code(http.StatusBadRequest)
	}

	if end.Sub(start).Hours() > 24*10 {
		return errors.New("api.ListJournal.Validate.Range", i18n.ERROR_INVALIDARGUMENT, err).Code(http.StatusBadRequest)
	}

	return nil
}

type GetJournalRequest struct {
	Date string `json:"date" form:"date" binding:"required"`
}

func (s *HttpSrv) GetJournal(c *gin.Context) {
	var (
		err error
		req GetJournalRequest
	)
	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	spaceID, _ := v1.InjectSpaceID(c)
	data, err := v1.NewJournalLogic(c, s.Core).GetJournal(spaceID, req.Date)
	if err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, data)
}

func (s *HttpSrv) ListJournal(c *gin.Context) {
	var (
		err error
		req ListJournalRequest
	)
	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	if err := req.Validate(); err != nil {
		response.APIError(c, err)
		return
	}

	spaceID, _ := v1.InjectSpaceID(c)
	list, err := v1.NewJournalLogic(c, s.Core).ListJournals(spaceID, req.StartDate, req.EndDate)
	if err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, list)
}

type DeleteJournalRequest struct {
	Date string `json:"date" form:"date" binding:"required"`
}

func (s *HttpSrv) DeleteJournal(c *gin.Context) {
	var (
		err error
		req DeleteJournalRequest
	)
	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	spaceID, _ := v1.InjectSpaceID(c)
	err = v1.NewJournalLogic(c, s.Core).DeleteJournal(spaceID, req.Date)
	if err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, nil)
}
