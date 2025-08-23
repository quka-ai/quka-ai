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

// AdminListUsersRequest 获取用户列表请求
// @Description 分页和搜索参数
// @Description page: 页码，从1开始
// @Description pagesize: 每页数量，最大50
// @Description name: 用户名模糊搜索（可选）
// @Description email: 邮箱模糊搜索（可选）
// @Description global_role: 全局角色过滤（可选），如 "role-chief", "role-admin", "role-member" 等
type AdminListUsersRequest struct {
	Page       uint64 `json:"page" form:"page" binding:"required,min=1"`
	PageSize   uint64 `json:"pagesize" form:"pagesize" binding:"required,min=1,max=50"`
	Name       string `json:"name" form:"name"`               // 用户名模糊搜索
	Email      string `json:"email" form:"email"`             // 邮箱模糊搜索
	GlobalRole string `json:"global_role" form:"global_role"` // 全局角色过滤
}

// AdminListUsersResponse 获取用户列表响应
// @Description 用户列表和总数，包含每个用户的全局角色
type AdminListUsersResponse struct {
	List  []types.UserWithRole `json:"list"`
	Total int64                `json:"total"`
}

// AdminListUsers 管理员获取用户列表
// @Summary 管理员获取用户列表
// @Description 管理员可以查看所有用户列表，支持分页和搜索，可按全局角色过滤
// @Tags 管理员
// @Accept json
// @Produce json
// @Param page query int true "页码" minimum(1)
// @Param pagesize query int true "每页数量" minimum(1) maximum(50)
// @Param name query string false "用户名模糊搜索"
// @Param email query string false "邮箱模糊搜索"
// @Param global_role query string false "全局角色过滤" Enums(role-chief,role-admin,role-member)
// @Success 200 {object} response.APIResponse{data=AdminListUsersResponse}
// @Failure 403 {object} response.APIResponse
// @Failure 500 {object} response.APIResponse
// @Router /api/v1/admin/users [get]
func (s *HttpSrv) AdminListUsers(c *gin.Context) {
	var req AdminListUsersRequest
	if err := utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	// 构建搜索条件
	opts := types.ListUserOptions{
		Name:      req.Name,  // 用户名模糊搜索
		EmailLike: req.Email, // 邮箱模糊搜索
	}

	// 复用logic实例
	logic := v1.NewAdminUserLogic(c, s.Core)
	users, total, err := logic.GetUsers(opts, req.GlobalRole, req.Page, req.PageSize)
	if err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, AdminListUsersResponse{
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

// AdminDeleteUserRequest 删除用户请求
type AdminDeleteUserRequest struct {
	UserID string `json:"user_id" binding:"required"`
}

// AdminDeleteUserResponse 删除用户响应
type AdminDeleteUserResponse struct {
	UserID  string `json:"user_id"`
	Message string `json:"message"`
}

// AdminDeleteUser 管理员删除用户
// @Summary 管理员删除用户
// @Description 管理员可以删除用户及其所有相关数据，包括空间、知识库、聊天记录等
// @Tags 管理员
// @Accept json
// @Produce json
// @Param request body AdminDeleteUserRequest true "删除用户请求"
// @Success 200 {object} response.APIResponse{data=AdminDeleteUserResponse}
// @Failure 400 {object} response.APIResponse
// @Failure 403 {object} response.APIResponse
// @Failure 404 {object} response.APIResponse
// @Failure 500 {object} response.APIResponse
// @Router /api/v1/admin/users [delete]
func (s *HttpSrv) AdminDeleteUser(c *gin.Context) {
	var req AdminDeleteUserRequest
	if err := utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	// 复用logic实例
	logic := v1.NewAdminUserLogic(c, s.Core)
	err := logic.DeleteUser(req.UserID)
	if err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, AdminDeleteUserResponse{
		UserID:  req.UserID,
		Message: "User deleted successfully",
	})
}
