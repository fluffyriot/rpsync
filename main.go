package main

import (
	"context"
	"encoding/json"
	"log"
	"time"

	_ "embed"

	"github.com/fluffyriot/rpsync/internal/api/handlers"
	"github.com/fluffyriot/rpsync/internal/config"
	"github.com/fluffyriot/rpsync/internal/fetcher"
	"github.com/fluffyriot/rpsync/internal/pusher/common"
	"github.com/fluffyriot/rpsync/internal/updater"
	"github.com/fluffyriot/rpsync/internal/worker"
	"github.com/gin-gonic/gin"
)

//go:embed version.json
var versionFile []byte

type projectVersion struct {
	Latest string `json:"latest"`
}

func main() {

	var pv projectVersion
	if err := json.Unmarshal(versionFile, &pv); err != nil {
		log.Printf("Error unmarshalling version.json: %v", err)
		config.AppVersion = "unknown"
	} else {
		config.AppVersion = pv.Latest
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatal(err)
	}

	clientFetch := fetcher.NewClient(600 * time.Second)
	clientPull := common.NewClient(600 * time.Second)

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

	upd := updater.NewUpdater(config.AppVersion)
	upd.Start()

	ctx := context.Background()
	users, err := dbQueries.GetAllUsers(ctx)
	shouldStart := true
	startInterval := 30 * time.Minute

	if err == nil && len(users) > 0 {
		user := users[0]
		if !user.EnabledOnStartup {
			shouldStart = false
		} else {
			parsedDuration, err := time.ParseDuration(user.SyncPeriod)
			if err == nil {
				startInterval = parsedDuration
			} else {
				log.Printf("Invalid sync period '%s', defaulting to 30m", user.SyncPeriod)
			}
		}
	}

	if shouldStart {
		w.Start(startInterval)
	} else {
		log.Println("Worker disabled on startup by user settings")
	}

	h := handlers.NewHandler(
		dbQueries,
		clientFetch,
		clientPull,
		cfg,
		w,
		upd,
	)

	r.GET("/", h.RootHandler)

	r.GET("/settings/sync", h.SyncSettingsHandler)
	r.POST("/settings/sync", h.UpdateSyncSettingsHandler)
	r.POST("/settings/sync/reset", h.ResetSyncSettingsHandler)
	r.POST("/settings/sync/start", h.StartWorkerHandler)
	r.POST("/settings/sync/stop", h.StopWorkerHandler)

	r.GET("/auth/facebook/login", h.FacebookLoginHandler)
	r.GET("/auth/facebook/callback", h.FacebookCallbackHandler)
	r.POST("/auth/facebook/refresh", h.FacebookRefreshTokenHandler)

	r.GET("/auth/tiktok/login", h.TikTokLoginHandler)
	r.GET("/auth/tiktok/check", h.TikTokCheckHandler)

	r.GET("/exports", h.ExportsHandler)
	r.POST("/exports/deleteAll", h.ExportDeleteAllHandler)

	r.GET("/outputs/*filepath", h.DownloadExportHandler)

	r.POST("/user/setup", h.UserSetupHandler)
	r.GET("/sources", h.SourcesHandler)
	r.POST("/sources/setup", h.SourcesSetupHandler)
	r.POST("/sources/deactivate", h.DeactivateSourceHandler)
	r.POST("/sources/activate", h.ActivateSourceHandler)
	r.POST("/sources/delete", h.DeleteSourceHandler)
	r.POST("/sources/sync", h.SyncSourceHandler)
	r.POST("/syncAll", h.TriggerSyncHandler)

	r.GET("/targets", h.TargetsHandler)
	r.POST("/targets/setup", h.TargetsSetupHandler)
	r.POST("/targets/deactivate", h.DeactivateTargetHandler)
	r.POST("/targets/activate", h.ActivateTargetHandler)
	r.POST("/targets/delete", h.DeleteTargetHandler)
	r.POST("/targets/sync", h.SyncTargetHandler)

	r.GET("/stats", h.StatsHandler)

	r.GET("/api/sources", h.HandleGetSourcesAPI)
	r.GET("/api/exclusions", h.HandleGetExclusions)
	r.POST("/api/exclusions", h.HandleCreateExclusion)
	r.DELETE("/api/exclusions/:id", h.HandleDeleteExclusion)

	log.Printf("Server started on http://localhost:%s", cfg.AppPort)
	if err := r.Run(":" + cfg.AppPort); err != nil {
		log.Fatal(err)
	}
}
