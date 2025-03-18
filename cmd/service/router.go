package service

import (
	"github.com/gin-gonic/gin"

	"github.com/quka-ai/quka-ai/app/core"
	"github.com/quka-ai/quka-ai/app/core/srv"
	v1 "github.com/quka-ai/quka-ai/app/logic/v1"
	"github.com/quka-ai/quka-ai/app/response"
	"github.com/quka-ai/quka-ai/cmd/service/handler"
	"github.com/quka-ai/quka-ai/cmd/service/middleware"
)

func serve(core *core.Core) {
	httpSrv := &handler.HttpSrv{
		Core:   core,
		Engine: core.HttpEngine(),
	}
	setupHttpRouter(httpSrv)

	core.HttpEngine().Run(core.Cfg().Addr)
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

	s.Engine.LoadHTMLGlob("./tpls/*")
	s.Engine.GET("/s/k/:token", s.BuildKnowledgeSharePage)
	s.Engine.GET("/s/s/:token", s.BuildSessionSharePage)
	// auth
	s.Engine.Use(middleware.I18n(), response.NewResponse())
	s.Engine.Use(middleware.Cors)
	s.Engine.Use(middleware.SetAppid(s.Core))
	apiV1 := s.Engine.Group("/api/v1")
	{
		apiV1.GET("/mode", func(c *gin.Context) {
			response.APISuccess(c, s.Core.Plugins.Name())
		})
		apiV1.GET("/connect", middleware.AuthorizationFromQuery(s.Core), handler.Websocket(s.Core))
		share := apiV1.Group("/share")
		{
			share.GET("/knowledge/:token", s.GetKnowledgeByShareToken)
			share.GET("/session/:token", s.GetSessionByShareToken)
			share.POST("/copy/knowledge", middleware.Authorization(s.Core), middleware.PaymentRequired, s.CopyKnowledge)
		}

		authed := apiV1.Group("")
		authed.Use(middleware.Authorization(s.Core))

		spaceShare := authed.Group("/space/application/:token")
		{
			spaceShare.GET("/landing", s.GetSpaceApplicationLandingDetail)
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

		space := authed.Group("/space")
		{
			space.GET("/list", s.ListUserSpaces)
			space.DELETE("/:spaceid/leave", middleware.VerifySpaceIDPermission(s.Core, srv.PermissionView), s.LeaveSpace)

			space.POST("", userLimit("modify_space"), s.CreateUserSpace)

			space.Use(middleware.VerifySpaceIDPermission(s.Core, srv.PermissionAdmin))
			space.DELETE("/:spaceid", s.DeleteUserSpace)
			space.PUT("/:spaceid", userLimit("modify_space"), s.UpdateSpace)
			space.POST("/:spaceid/application/handler")
			space.PUT("/:spaceid/user/role", userLimit("modify_space"), s.SetUserSpaceRole)
			space.GET("/:spaceid/users", s.ListSpaceUsers)
			space.DELETE("/:spaceid/user/remove", s.RemoveSpaceUser)
			// share
			space.POST("/:spaceid/knowledge/share", middleware.PaymentRequired, s.CreateKnowledgeShareToken)
			space.POST("/:spaceid/session/share", middleware.PaymentRequired, s.CreateSessionShareToken)
			space.POST("/:spaceid/share", middleware.PaymentRequired, s.CreateSpaceShareToken)

			object := space.Group("/:spaceid/object")
			{
				object.POST("/upload/key", userLimit("upload"), s.GenUploadKey)
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
			chat.Use(middleware.VerifySpaceIDPermission(s.Core, srv.PermissionView))
			chat.POST("", middleware.PaymentRequired, s.CreateChatSession)
			chat.DELETE("/:session", s.DeleteChatSession)
			chat.GET("/list", s.ListChatSession)
			chat.POST("/:session/message/id", middleware.PaymentRequired, s.GenMessageID)
			chat.PUT("/:session/named", spaceLimit("named_session"), middleware.PaymentRequired, s.RenameChatSession)
			chat.GET("/:session/message/:messageid/ext", s.GetChatMessageExt)

			history := chat.Group("/:session/history")
			{
				history.GET("/list", s.GetChatSessionHistory)
			}

			message := chat.Group("/:session/message")
			{
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
	}
}
