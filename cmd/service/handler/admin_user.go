package handler

import (
	"github.com/gin-gonic/gin"
	v1 "github.com/quka-ai/quka-ai/app/logic/v1"
	"github.com/quka-ai/quka-ai/app/response"
	"github.com/quka-ai/quka-ai/pkg/types"
	"github.com/quka-ai/quka-ai/pkg/utils"
)

// AdminCreateUserRequest 管理员创建用户请求
// @Description 管理员创建新用户的请求参数
// @Description name: 用户昵称，必填
// @Description email: 用户邮箱，必填，需符合邮箱格式
// @Example {"name": "张三", "email": "zhangsan@example.com"}
type AdminCreateUserRequest struct {
	Name  string `json:"name" binding:"required,min=1,max=50"`
	Email string `json:"email" binding:"required,email,max=100"`
}

// AdminCreateUserResponse 管理员创建用户响应
// @Description 创建成功后的响应数据
// @Description user_id: 新创建用户的ID
// @Description name: 用户昵称
// @Description email: 用户邮箱
// @Description access_token: 可直接使用的访问令牌
// @Description created_at: 创建时间戳
// @Example {"user_id": "usr_1234567890abcdef", "name": "张三", "email": "zhangsan@example.com", "access_token": "tkn_abcdef1234567890", "created_at": 1699123456}
type AdminCreateUserResponse struct {
	UserID      string `json:"user_id"`
	Name        string `json:"name"`
	Email       string `json:"email"`
	AccessToken string `json:"access_token"`
	CreatedAt   int64  `json:"created_at"`
}

// AdminCreateUser 管理员创建用户
// @Summary 管理员创建新用户
// @Description 管理员可以通过此接口创建新用户，系统会自动生成AccessToken
// @Tags 管理员
// @Accept json
// @Produce json
// @Param request body AdminCreateUserRequest true "创建用户请求参数"
// @Success 200 {object} response.APIResponse{data=AdminCreateUserResponse}
// @Failure 400 {object} response.APIResponse
// @Failure 403 {object} response.APIResponse
// @Failure 409 {object} response.APIResponse
// @Failure 500 {object} response.APIResponse
// @Router /api/v1/admin/users [post]
func (s *HttpSrv) AdminCreateUser(c *gin.Context) {
	var req AdminCreateUserRequest
	if err := utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	// 创建管理员用户逻辑实例（复用）
	logic := v1.NewAdminUserLogic(c, s.Core)
	result, err := logic.CreateUser(v1.CreateUserRequest{
		Name:  req.Name,
		Email: req.Email,
	})
	if err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, AdminCreateUserResponse{
		UserID:      result.UserID,
		Name:        result.Name,
		Email:       result.Email,
		AccessToken: result.AccessToken,
		CreatedAt:   result.CreatedAt,
	})
}

// AdminListCreatedUsersRequest 获取管理员创建用户列表请求
// @Description 分页参数
// @Description page: 页码，从1开始
// @Description pagesize: 每页数量，最大50
type AdminListCreatedUsersRequest struct {
	Page     uint64 `json:"page" form:"page" binding:"required,min=1"`
	PageSize uint64 `json:"pagesize" form:"pagesize" binding:"required,min=1,max=50"`
}

// AdminListCreatedUsersResponse 获取管理员创建用户列表响应
// @Description 用户列表和总数
type AdminListCreatedUsersResponse struct {
	List  []types.User `json:"list"`
	Total int64        `json:"total"`
}

// AdminListCreatedUsers 获取管理员创建的用户列表
// @Summary 获取管理员创建的用户列表
// @Description 管理员可以查看自己创建的用户列表，支持分页
// @Tags 管理员
// @Accept json
// @Produce json
// @Param page query int true "页码" minimum(1)
// @Param pagesize query int true "每页数量" minimum(1) maximum(50)
// @Success 200 {object} response.APIResponse{data=AdminListCreatedUsersResponse}
// @Failure 403 {object} response.APIResponse
// @Failure 500 {object} response.APIResponse
// @Router /api/v1/admin/users [get]
func (s *HttpSrv) AdminListCreatedUsers(c *gin.Context) {
	var req AdminListCreatedUsersRequest
	if err := utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	// 复用logic实例
	logic := v1.NewAdminUserLogic(c, s.Core)
	users, total, err := logic.GetCreatedUsers(req.Page, req.PageSize)
	if err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, AdminListCreatedUsersResponse{
		List:  users,
		Total: total,
	})
}

// AdminRegenerateTokenRequest 重新生成AccessToken请求
type AdminRegenerateTokenRequest struct {
	UserID string `json:"user_id" binding:"required"`
}

// AdminRegenerateTokenResponse 重新生成AccessToken响应
type AdminRegenerateTokenResponse struct {
	UserID      string `json:"user_id"`
	AccessToken string `json:"access_token"`
}

// AdminRegenerateAccessToken 重新生成用户AccessToken
// @Summary 重新生成用户AccessToken
// @Description 管理员可以为之前创建的用户重新生成AccessToken
// @Tags 管理员
// @Accept json
// @Produce json
// @Param request body AdminRegenerateTokenRequest true "重新生成Token请求"
// @Success 200 {object} response.APIResponse{data=AdminRegenerateTokenResponse}
// @Failure 400 {object} response.APIResponse
// @Failure 403 {object} response.APIResponse
// @Failure 404 {object} response.APIResponse
// @Failure 500 {object} response.APIResponse
// @Router /api/v1/admin/users/token [post]
func (s *HttpSrv) AdminRegenerateAccessToken(c *gin.Context) {
	var req AdminRegenerateTokenRequest
	if err := utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	// 复用logic实例
	logic := v1.NewAdminUserLogic(c, s.Core)
	token, err := logic.RegenerateAccessToken(req.UserID)
	if err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, AdminRegenerateTokenResponse{
		UserID:      req.UserID,
		AccessToken: token,
	})
}

// AdminBatchCreateUsersRequest 批量创建用户请求
type AdminBatchCreateUsersRequest struct {
	Users []v1.CreateUserRequest `json:"users" binding:"required,min=1,max=100"`
}

// AdminBatchCreateUsersResponse 批量创建用户响应
type AdminBatchCreateUsersResponse struct {
	SuccessCount int                          `json:"success_count"`
	FailedCount  int                          `json:"failed_count"`
	Results      []AdminBatchCreateUserResult `json:"results"`
	Errors       []AdminBatchCreateUserError  `json:"errors"`
}

type AdminBatchCreateUserResult struct {
	Name        string `json:"name"`
	Email       string `json:"email"`
	UserID      string `json:"user_id"`
	AccessToken string `json:"access_token"`
	Status      string `json:"status"`
}

type AdminBatchCreateUserError struct {
	Name   string `json:"name"`
	Email  string `json:"email"`
	Error  string `json:"error"`
	Status string `json:"status"`
}

// AdminBatchCreateUsers 批量创建用户
// @Summary 批量创建用户
// @Description 管理员可以批量创建用户，最多100个。使用优化的批量处理逻辑
// @Tags 管理员
// @Accept json
// @Produce json
// @Param request body AdminBatchCreateUsersRequest true "批量创建用户请求"
// @Success 200 {object} response.APIResponse{data=AdminBatchCreateUsersResponse}
// @Failure 400 {object} response.APIResponse
// @Failure 403 {object} response.APIResponse
// @Failure 500 {object} response.APIResponse
// @Router /api/v1/admin/users/batch [post]
func (s *HttpSrv) AdminBatchCreateUsers(c *gin.Context) {
	var req AdminBatchCreateUsersRequest
	if err := utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	logic := v1.NewAdminUserLogic(c, s.Core)

	// 使用优化的批量创建逻辑
	result, err := logic.BatchCreateUsers(req.Users)
	if err != nil {
		response.APIError(c, err)
		return
	}

	// 转换结果格式以匹配API响应结构
	apiResults := make([]AdminBatchCreateUserResult, len(result.Results))
	for i, r := range result.Results {
		apiResults[i] = AdminBatchCreateUserResult{
			Name:        r.Name,
			Email:       r.Email,
			UserID:      r.UserID,
			AccessToken: r.AccessToken,
			Status:      "success",
		}
	}

	apiErrors := make([]AdminBatchCreateUserError, len(result.Errors))
	for i, e := range result.Errors {
		apiErrors[i] = AdminBatchCreateUserError{
			Name:   e.Name,
			Email:  e.Email,
			Error:  e.Error,
			Status: e.Status,
		}
	}

	response.APISuccess(c, AdminBatchCreateUsersResponse{
		SuccessCount: result.SuccessCount,
		FailedCount:  result.FailedCount,
		Results:      apiResults,
		Errors:       apiErrors,
	})
}
