package service

import (
	"time"

	"github.com/gin-gonic/gin"

	"github.com/quka-ai/quka-ai/app/core"
	"github.com/quka-ai/quka-ai/app/core/srv"
	v1 "github.com/quka-ai/quka-ai/app/logic/v1"
	"github.com/quka-ai/quka-ai/app/response"
	"github.com/quka-ai/quka-ai/cmd/service/handler"
	"github.com/quka-ai/quka-ai/cmd/service/middleware"
	"github.com/quka-ai/quka-ai/pkg/mcp"
)

func serve(core *core.Core) {
	httpSrv := &handler.HttpSrv{
		Core:   core,
		Engine: core.HttpEngine(),
	}
	setupHttpRouter(httpSrv)

	address := core.Cfg().Addr
	if address == "" {
		address = ":33033"
	}
	core.HttpEngine().Run(address)
}

func GetIPLimitBuilder(appCore *core.Core) middleware.LimiterFunc {
	return func(key string, opts ...core.LimitOption) gin.HandlerFunc {
		return middleware.UseLimit(appCore, key, func(c *gin.Context) string {
			return key + ":" + c.ClientIP()
		}, opts...)
	}
}

func GetUserLimitBuilder(appCore *core.Core) middleware.LimiterFunc {
	return func(key string, opts ...core.LimitOption) gin.HandlerFunc {
		return middleware.UseLimit(appCore, key, func(c *gin.Context) string {
			token, _ := v1.InjectTokenClaim(c)
			return key + ":" + token.User
		}, opts...)
	}
}

func GetSpaceLimitBuilder(appCore *core.Core) middleware.LimiterFunc {
	return func(key string, opts ...core.LimitOption) gin.HandlerFunc {
		return middleware.UseLimit(appCore, key, func(c *gin.Context) string {
			spaceid, _ := c.Params.Get("spaceid")
			return key + ":" + spaceid
		}, opts...)
	}
}

func GetAILimitBuilder(appCore *core.Core) middleware.LimiterFunc {
	return func(key string, opts ...core.LimitOption) gin.HandlerFunc {
		return middleware.UseLimit(appCore, "ai", func(c *gin.Context) string {
			return key
		}, opts...)
	}
}

func setupHttpRouter(s *handler.HttpSrv) {
	userLimit := GetUserLimitBuilder(s.Core)
	spaceLimit := GetSpaceLimitBuilder(s.Core)
	aiLimit := GetAILimitBuilder(s.Core)

	s.Engine.Use(middleware.Cors)
	s.Engine.LoadHTMLGlob("./tpls/*")
	s.Engine.GET("/s/k/:token", s.BuildKnowledgeSharePage)
	s.Engine.GET("/s/s/:token", s.BuildSessionSharePage)
	s.Engine.GET("/s/sp/:token", s.BuildSpaceSharePage)

	// 公共资源路由（无需认证）
	s.Engine.GET("/public/*object_path", s.ObjectHandler)

	// 图片代理路由（支持多种认证方式，用于 <img> 标签）
	s.Engine.GET("/image/*object_path", middleware.FlexibleAuth(s.Core), s.ObjectHandler)

	// auth
	s.Engine.Use(middleware.I18n(), response.NewResponse())
	s.Engine.Use(middleware.SetAppid(s.Core))
	apiV1 := s.Engine.Group("/api/v1")
	{
		// MCP 路由（独立认证，不使用 JWT middleware）
		// 使用 MCP SDK 的 StreamableHTTPHandler（推荐方式）
		apiV1.POST("/mcp", mcp.MCPStreamableHandler(s.Core))
		apiV1.GET("/mode", func(c *gin.Context) {
			response.APISuccess(c, s.Core.Plugins.Name())
		})
		apiV1.GET("/connect", handler.Websocket(s.Core))
		share := apiV1.Group("/share")
		{
			share.GET("/knowledge/:token", s.GetKnowledgeByShareToken)
			share.GET("/session/:token", s.GetSessionByShareToken)
			share.POST("/copy/knowledge", middleware.Authorization(s.Core), middleware.PaymentRequired, s.CopyKnowledge)
		}

		authed := apiV1.Group("")
		authed.Use(middleware.Authorization(s.Core))

		spaceShare := authed.Group("/space/landing/:token")
		{
			spaceShare.GET("", s.GetSpaceApplicationLandingDetail)
			spaceShare.POST("/apply", s.ApplySpace)
		}

		user := authed.Group("/user")
		{
			user.GET("/info", s.GetUser)
			user.PUT("/profile", userLimit("profile"), s.UpdateUserProfile)
			user.POST("/secret/token", s.CreateAccessToken)
			user.GET("/secret/tokens", s.GetUserAccessTokens)
			user.DELETE("/secret/tokens", s.DeleteAccessTokens)
		}

		// space 相关路由
		space := authed.Group("/space")
		{
			space.GET("/list", s.ListUserSpaces)
			space.DELETE("/:spaceid/leave", middleware.VerifySpaceIDPermission(s.Core, srv.PermissionView), s.LeaveSpace)

			space.POST("", userLimit("modify_space"), s.CreateUserSpace)

			editorSpace := space.Group("/:spaceid").Use(middleware.VerifySpaceIDPermission(s.Core, srv.PermissionEdit))
			editorSpace.POST("/task/file-chunk", aiLimit("file_chunk", core.WithLimit(10), core.WithRange(time.Hour)), s.CreateFileChunkTask)
			editorSpace.GET("/task/list", s.GetFileChunkTaskList)
			editorSpace.GET("/task/status", s.GetTaskStatus)
			editorSpace.DELETE("/task/file-chunk", s.DeleteChunkTask)
			editorSpace.POST("/invite", s.SpaceInvitation)

			space.Use(middleware.VerifySpaceIDPermission(s.Core, srv.PermissionAdmin))
			space.DELETE("/:spaceid", s.DeleteUserSpace)
			space.PUT("/:spaceid", userLimit("modify_space"), s.UpdateSpace)
			space.PUT("/:spaceid/user/role", userLimit("modify_space"), s.SetUserSpaceRole)
			space.GET("/:spaceid/users", s.ListSpaceUsers)
			space.GET("/:spaceid/application/users", s.GetSpaceApplicationWaitingList)
			space.PUT("/:spaceid/application/handler", s.HandlerSpaceApplication)
			space.DELETE("/:spaceid/user/:userid", s.RemoveSpaceUser)
			// share
			space.POST("/:spaceid/knowledge/share", middleware.PaymentRequired, s.CreateKnowledgeShareToken)
			space.POST("/:spaceid/session/share", middleware.PaymentRequired, s.CreateSessionShareToken)
			space.POST("/:spaceid/share", middleware.PaymentRequired, s.CreateSpaceShareToken)

			object := space.Group("/:spaceid/object")
			{
				object.POST("/upload/key", userLimit("upload"), s.GenUploadKey)
				object.GET("/proxy/*object_path", middleware.VerifySpaceIDPermission(s.Core, srv.PermissionView), s.ObjectHandler)
			}

			journal := space.Group("/:spaceid/journal")
			{
				journal.GET("/list", s.ListJournal)
				journal.GET("", s.GetJournal)
				journal.PUT("", s.UpsertJournal)
				journal.DELETE("", s.DeleteJournal)
			}
		}

		knowledge := authed.Group("/:spaceid/knowledge")
		{
			viewScope := knowledge.Group("")
			{
				viewScope.Use(middleware.VerifySpaceIDPermission(s.Core, srv.PermissionView))
				viewScope.GET("", s.GetKnowledge)
				viewScope.GET("/list", spaceLimit("knowledge_list"), s.ListKnowledge)
				viewScope.GET("/chunk/list", spaceLimit("knowledge_list"), s.ListContentTask)
				viewScope.GET("/chunk/knowledge", spaceLimit("knowledge_list"), s.GetTaskKnowledge)
				viewScope.POST("/query", spaceLimit("chat_message"), s.Query)
				viewScope.GET("/time/list", spaceLimit("knowledge_list"), s.GetDateCreatedKnowledge)
			}

			editScope := knowledge.Group("")
			{
				editScope.Use(middleware.VerifySpaceIDPermission(s.Core, srv.PermissionEdit), spaceLimit("knowledge_modify"))
				editScope.POST("", aiLimit("create_knowledge"), s.CreateKnowledge)
				editScope.PUT("", aiLimit("create_knowledge"), s.UpdateKnowledge)
				editScope.DELETE("", s.DeleteKnowledge)
			}
		}

		authed.GET("/resource/list", s.ListUserResources)
		resource := authed.Group("/:spaceid/resource")
		{
			resource.Use(middleware.VerifySpaceIDPermission(s.Core, srv.PermissionView))
			resource.GET("", s.GetResource)
			resource.GET("/list", s.ListResource)

			resource.Use(spaceLimit("resource"))
			resource.POST("", s.CreateResource)
			resource.PUT("", s.UpdateResource)
			resource.DELETE("/:resourceid", s.DeleteResource)
		}

		chat := authed.Group("/:spaceid/chat")
		{
			chat.Use(middleware.VerifySpaceIDPermission(s.Core, srv.PermissionMember))
			chat.POST("", middleware.PaymentRequired, s.CreateChatSession)
			chat.DELETE("/:session", s.DeleteChatSession)
			chat.GET("/list", s.ListChatSession)
			chat.POST("/:session/message/id", middleware.PaymentRequired, s.GenMessageID)
			chat.PUT("/:session/named", spaceLimit("named_session"), middleware.PaymentRequired, s.RenameChatSession)
			chat.POST("/:session/stop", s.StopChatStream)

			history := chat.Group("/:session/history")
			{
				history.GET("/list", s.GetChatSessionHistory)
			}

			message := chat.Group("/:session/message")
			{
				message.GET("/:messageid/ext", s.GetChatMessageExt)
				message.Use(spaceLimit("create_message"), middleware.PaymentRequired)
				message.POST("", aiLimit("chat_message"), s.CreateChatMessage)
			}
		}

		tools := authed.Group("/tools")
		{
			tools.Use(userLimit("tools"))
			tools.GET("/reader", s.ToolsReader)
			tools.POST("/describe/image", s.DescribeImage)
		}

		// 管理员路由（需要管理员权限）
		admin := authed.Group("/admin")
		admin.Use(middleware.VerifyAdminPermission(s.Core))
		{
			// 模型提供商管理
			providers := admin.Group("/model/providers")
			{
				providers.POST("", s.CreateModelProvider)       // 创建提供商
				providers.GET("", s.ListModelProviders)         // 获取提供商列表
				providers.GET("/:id", s.GetModelProvider)       // 获取提供商详情
				providers.PUT("/:id", s.UpdateModelProvider)    // 更新提供商
				providers.DELETE("/:id", s.DeleteModelProvider) // 删除提供商
			}

			// 模型配置管理
			configs := admin.Group("/model/configs")
			{
				configs.POST("", s.CreateModelConfig)       // 创建模型配置
				configs.GET("", s.ListModelConfigs)         // 获取模型配置列表
				configs.GET("/:id", s.GetModelConfig)       // 获取模型配置详情
				configs.PUT("/:id", s.UpdateModelConfig)    // 更新模型配置
				configs.DELETE("/:id", s.DeleteModelConfig) // 删除模型配置
			}

			// AI系统管理
			aiSystem := admin.Group("/ai/system")
			{
				aiSystem.POST("/reload", s.ReloadAIConfig) // 重新加载AI配置
				aiSystem.GET("/status", s.GetAIStatus)     // 获取AI系统状态
				aiSystem.PUT("/usage", s.UpdateAIUsage)    // 更新AI使用配置
				aiSystem.GET("/usage", s.GetAIUsage)       // 获取AI使用配置
			}

			// 用户管理
			users := admin.Group("/users")
			{
				users.POST("", s.AdminCreateUser)                  // 创建单个用户
				users.GET("", s.AdminListUsers)                    // 获取用户列表
				users.POST("/token", s.AdminRegenerateAccessToken) // 重新生成AccessToken
				users.DELETE("", s.AdminDeleteUser)                // 删除用户
			}
		}
	}
}
