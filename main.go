package main

import (
	"log"
	"time"

	"github.com/fluffyriot/commission-tracker/internal/api/handlers"
	"github.com/fluffyriot/commission-tracker/internal/config"
	"github.com/fluffyriot/commission-tracker/internal/fetcher"
	"github.com/fluffyriot/commission-tracker/internal/puller"
	"github.com/fluffyriot/commission-tracker/internal/worker"
	"github.com/gin-gonic/gin"
)

func main() {

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatal(err)
	}

	clientFetch := fetcher.NewClient(600 * time.Second)
	clientPull := puller.NewClient(600 * time.Second)

	r := gin.Default()

	r.SetTrustedProxies(nil)

	r.Static("/static", "./static")

	r.LoadHTMLGlob("templates/*.html")

	dbQueries, dbInitErr := config.LoadDatabase()
	if dbInitErr != nil {
		log.Printf("database init failed: %v", dbInitErr)
	}

	cfg.DBInitErr = dbInitErr

	w := worker.NewWorker(dbQueries, clientFetch, clientPull, cfg)
	w.Start(30 * time.Minute)

	h := handlers.NewHandler(
		dbQueries,
		clientFetch,
		clientPull,
		cfg,
	)

	r.GET("/", h.RootHandler)

	r.GET("/auth/facebook/login", h.FacebookLoginHandler)
	r.GET("/auth/facebook/callback", h.FacebookCallbackHandler)

	r.GET("/auth/tiktok/login", h.TikTokLoginHandler)
	r.GET("/auth/tiktok/check", h.TikTokCheckHandler)

	r.GET("/exports", h.ExportsHandler)
	r.POST("/exports/deleteAll", h.ExportDeleteAllHandler)

	r.GET("/outputs/*filepath", h.DownloadExportHandler)

	r.POST("/user/setup", h.UserSetupHandler)
	r.POST("/sources/setup", h.SourcesSetupHandler)
	r.POST("/sources/deactivate", h.DeactivateSourceHandler)
	r.POST("/sources/activate", h.ActivateSourceHandler)
	r.POST("/sources/delete", h.DeleteSourceHandler)
	r.POST("/sources/sync", h.SyncSourceHandler)
	r.POST("/sources/syncAll", h.SyncAllHandler)

	r.GET("/targets", h.TargetsHandler)
	r.POST("/targets/setup", h.TargetsSetupHandler)
	r.POST("/targets/deactivate", h.DeactivateTargetHandler)
	r.POST("/targets/activate", h.ActivateTargetHandler)
	r.POST("/targets/delete", h.DeleteTargetHandler)
	r.POST("/targets/sync", h.SyncTargetHandler)

	r.POST("/reset", h.ResetHandler)

	r.GET("/stats", h.StatsHandler)
	r.GET("/analytics", h.AnalyticsPageHandler)

	log.Printf("Server started on http://localhost:%s", cfg.AppPort)
	if err := r.Run(":" + cfg.AppPort); err != nil {
		log.Fatal(err)
	}
}
