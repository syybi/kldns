package handler

import (
	"kldns/internal/middleware"

	"github.com/gin-gonic/gin"
)

func NewRouter() *gin.Engine {
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())

	router.GET("/", spaHandler((*SPAController).Index))
	router.GET("/favicon.svg", spaHandler((*SPAController).Favicon))
	router.GET("/assets/*filepath", spaHandler((*SPAController).Asset))

	api := router.Group("/api", middleware.NoSniff())
	{
		api.GET("/health", apiHandler((*HealthController).Get))
		api.POST("/install/admin", apiHandler((*InstallController).CreateAdmin))
		api.POST("/auth/register", apiHandler((*AuthController).Register))
		api.POST("/auth/login", apiHandler((*AuthController).Login))
		api.POST("/admin/auth/login", apiHandler((*AuthController).AdminLogin))
		api.GET("/public/domains", apiHandler((*DomainAPIController).Public))
		api.GET("/settings/turnstile", apiHandler((*SettingsAPIController).Turnstile))

		auth := api.Group("", middleware.APIBearerAuth(), middleware.OpenAPIAccessOnly())
		{
			auth.GET("/auth/me", apiHandler((*AuthController).Me))
			auth.PUT("/auth/password", apiHandler((*AuthController).ChangePassword))
			auth.GET("/domains", apiHandler((*DomainAPIController).Get))
			auth.GET("/settings/dns-policy", apiHandler((*SettingsAPIController).DNSPolicy))
			auth.GET("/subdomains", apiHandler((*SubdomainAPIController).Get))
			auth.POST("/subdomains", apiHandler((*SubdomainAPIController).Post))
			auth.DELETE("/subdomains/:id", apiHandler((*SubdomainAPIController).Delete))
			auth.GET("/records", apiHandler((*RecordAPIController).Get))
			auth.POST("/records", apiHandler((*RecordAPIController).Post))
			auth.PUT("/records/:id", apiHandler((*RecordAPIController).Put))
			auth.DELETE("/records/:id", apiHandler((*RecordAPIController).Delete))
			auth.GET("/points", apiHandler((*PointsAPIController).Get))
			auth.GET("/tokens", apiHandler((*TokenAPIController).Get))
			auth.POST("/tokens", apiHandler((*TokenAPIController).Post))
			auth.DELETE("/tokens/:id", apiHandler((*TokenAPIController).Delete))
		}

		admin := api.Group("/admin", middleware.APIBearerAuth(), middleware.OpenAPIAccessOnly(), middleware.AdminOnly())
		{
			admin.GET("/users", apiHandler((*AdminListController).Users))
			admin.PUT("/users/:id", apiHandler((*AdminListController).SaveUser))
			admin.DELETE("/users/:id", apiHandler((*AdminListController).DeleteUser))
			admin.POST("/users/:id/points", apiHandler((*AdminListController).AdjustUserPoints))
			admin.GET("/points", apiHandler((*AdminListController).Points))
			admin.GET("/groups", apiHandler((*AdminListController).Groups))
			admin.POST("/groups", apiHandler((*AdminListController).SaveGroup))
			admin.DELETE("/groups/:id", apiHandler((*AdminListController).DeleteGroup))
			admin.GET("/domains", apiHandler((*AdminListController).Domains))
			admin.POST("/domains", apiHandler((*AdminListController).SaveDomain))
			admin.POST("/domains/:id/sync-records", apiHandler((*AdminListController).SyncDomainRecords))
			admin.PUT("/domains/:id", apiHandler((*AdminListController).SaveDomain))
			admin.DELETE("/domains/:id", apiHandler((*AdminListController).DeleteDomain))
			admin.GET("/dns-providers", apiHandler((*AdminListController).Providers))
			admin.POST("/dns-providers/zones", apiHandler((*AdminListController).ProviderZones))
			admin.GET("/provider-configs", apiHandler((*AdminListController).ProviderConfigs))
			admin.POST("/provider-configs", apiHandler((*AdminListController).SaveProviderConfig))
			admin.PUT("/provider-configs/:id", apiHandler((*AdminListController).SaveProviderConfig))
			admin.DELETE("/provider-configs/:id", apiHandler((*AdminListController).DeleteProviderConfig))
			admin.GET("/records", apiHandler((*AdminListController).Records))
			admin.POST("/records", apiHandler((*AdminListController).SaveRecord))
			admin.PUT("/records/:id", apiHandler((*AdminListController).SaveRecord))
			admin.DELETE("/records/:id", apiHandler((*AdminListController).DeleteRecord))
			admin.GET("/subdomains", apiHandler((*AdminListController).Subdomains))
			admin.POST("/subdomains/:id/approve", apiHandler((*AdminListController).ApproveSubdomain))
			admin.POST("/subdomains/:id/reject", apiHandler((*AdminListController).RejectSubdomain))
			admin.DELETE("/subdomains/:id", apiHandler((*AdminListController).DeleteSubdomain))
			admin.GET("/logs", apiHandler((*AdminListController).Logs))
			admin.GET("/settings", apiHandler((*AdminListController).Settings))
			admin.PUT("/settings", apiHandler((*AdminListController).SaveSettings))
		}
	}

	router.NoRoute(spaHandler((*SPAController).Index))
	return router
}

func apiHandler[T any](action func(*T)) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		controller := new(T)
		if setter, ok := any(controller).(interface{ SetContext(*gin.Context) }); ok {
			setter.SetContext(ctx)
		}
		action(controller)
	}
}

func spaHandler(action func(*SPAController)) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		controller := &SPAController{}
		controller.SetContext(ctx)
		action(controller)
	}
}
