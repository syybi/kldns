package main

import (
	"fmt"
	"log"
	"strings"

	"kldns/internal/config"
	"kldns/internal/handler"
	"kldns/internal/repository"
	migrationassets "kldns/internal/repository/migrations"
	"kldns/internal/runtime"
	_ "kldns/pkg/dns/providers"
	webassets "kldns/web"

	"github.com/gin-gonic/gin"
)

const defaultSecretKey = "change-me-before-production-kldns-secret"

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	runtime.SetConfig(cfg)
	mustValidateRuntimeSecret(cfg)
	setGinMode(cfg.App.Mode)

	db, err := repository.OpenSQLite(cfg.Database.Path, cfg.Database.BusyTimeoutMS, cfg.Database.WAL)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()

	if err := repository.RunMigrationsFS(db.SQLDB(), migrationassets.FS, migrationassets.Dir); err != nil {
		log.Fatalf("run migrations: %v", err)
	}
	runtime.SetDB(db)
	if err := handler.SetFrontendFS(webassets.FS, webassets.DistDir); err != nil {
		log.Fatalf("load embedded frontend: %v", err)
	}

	router := handler.NewRouter()
	if err := router.Run(fmt.Sprintf(":%d", cfg.App.Port)); err != nil {
		log.Fatalf("run server: %v", err)
	}
}

func mustValidateRuntimeSecret(cfg config.Config) {
	if isProductionMode(cfg.App.Mode) && strings.TrimSpace(cfg.Security.SecretKey) == defaultSecretKey {
		log.Fatal("refusing to start in production with the default secret_key")
	}
}

func setGinMode(runMode string) {
	if isProductionMode(runMode) {
		gin.SetMode(gin.ReleaseMode)
		return
	}
	gin.SetMode(gin.DebugMode)
}

func isProductionMode(runMode string) bool {
	switch strings.ToLower(strings.TrimSpace(runMode)) {
	case "prod", "production":
		return true
	default:
		return false
	}
}
