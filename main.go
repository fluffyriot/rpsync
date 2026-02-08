package main

import (
	"context"
	"encoding/json"
	"flag"
	"html/template"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	_ "embed"

	"github.com/fluffyriot/rpsync/internal/api/handlers"
	"github.com/fluffyriot/rpsync/internal/cli"
	"github.com/fluffyriot/rpsync/internal/config"
	fetcher_common "github.com/fluffyriot/rpsync/internal/fetcher/common"
	"github.com/fluffyriot/rpsync/internal/middleware"
	"github.com/fluffyriot/rpsync/internal/pusher/common"
	"github.com/fluffyriot/rpsync/internal/updater"
	"github.com/fluffyriot/rpsync/internal/worker"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

//go:embed version.json
var versionFile []byte

type projectVersion struct {
	Latest string `json:"latest"`
}

func main() {
	resetPwdFlag := flag.Bool("reset-password", false, "Reset user password")
	reset2FAFlag := flag.Bool("reset-2fa", false, "Reset 2FA (TOTP) for a user")
	resetUserFlag := flag.String("username", "", "Username (required for --reset-password and --reset-2fa)")
	flag.Parse()

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

	clientFetch := fetcher_common.NewClient(600 * time.Second)
	clientPull := common.NewClient(600 * time.Second)

	if cfg.GinMode != "" {
		gin.SetMode(cfg.GinMode)
	}
	r := gin.Default()

	r.SetTrustedProxies(nil)
	r.Use(middleware.SecurityHeadersMiddleware())

	r.Static("/static", "./static")
	r.StaticFile("/apple-touch-icon.png", "./static/images/apple-touch-icon.png")

	r.SetFuncMap(template.FuncMap{
		"lower": strings.ToLower,
		"upper": strings.ToUpper,
		"json": func(v interface{}) template.JS {
			a, _ := json.Marshal(v)
			return template.JS(a)
		},
		"stripAt": func(s string) string {
			return strings.TrimPrefix(s, "@")
		},
		"replace": strings.ReplaceAll,
		"truncate": func(s string, length int) string {
			runes := []rune(s)
			if len(runes) <= length {
				return s
			}
			return string(runes[:length]) + "..."
		},
		"add": func(a, b int32) int32 {
			return a + b
		},
	})

	r.LoadHTMLGlob("templates/*.html")

	store := cookie.NewStore(cfg.SessionKey)
	store.Options(sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 7,
		HttpOnly: true,
		Secure:   cfg.HttpsPort != "",
		SameSite: http.SameSiteLaxMode,
	})
	r.Use(sessions.Sessions("mysession", store))

	dbQueries, dbConn, dbInitErr := config.LoadDatabase()
	if dbInitErr != nil {
		log.Printf("database init failed: %v", dbInitErr)
	}

	cfg.DBInitErr = dbInitErr

	if *resetPwdFlag {
		if dbConn == nil {
			log.Fatal("Database connection failed, cannot reset password")
		}
		cli.HandleResetPassword(dbQueries, *resetUserFlag)
		return
	}

	if *reset2FAFlag {
		if dbConn == nil {
			log.Fatal("Database connection failed, cannot reset 2FA")
		}
		cli.HandleReset2FA(dbQueries, *resetUserFlag)
		return
	}

	w := worker.NewWorker(dbQueries, clientFetch, clientPull, cfg)

	upd := updater.NewUpdater(config.AppVersion)
	upd.Start()

	ctx := context.Background()

	shouldStart := true
	enableWorkerConfig, _ := dbQueries.GetAppConfig(ctx, "enable_worker_on_startup")
	if enableWorkerConfig != "true" {
		shouldStart = false
	}

	if shouldStart {
		w.Start()
	} else {
		log.Println("Worker disabled on startup by global settings")
	}

	h := handlers.NewHandler(
		dbQueries,
		dbConn,
		clientFetch,
		clientPull,
		cfg,
		w,
		upd,
	)

	r.GET("/health", h.HealthCheckHandler)
	r.GET("/login", h.LoginViewHandler)
	r.POST("/login", h.LoginSubmitHandler)
	r.POST("/logout", h.LogoutHandler)

	r.GET("/register", h.UserSetupViewHandler)
	r.POST("/register", h.UserSetupHandler)

	authorized := r.Group("/")
	authorized.Use(middleware.AuthMiddleware(dbQueries))

	authorized.GET("/setup/password", h.PasswordSetupViewHandler)
	authorized.POST("/setup/password", h.PasswordSetupSubmitHandler)

	authorized.GET("/", h.RootHandler)
	authorized.POST("/logs/dismiss", h.DismissLogHandler)

	authorized.GET("/settings/2fa/setup", h.TwoFASetupViewHandler)
	authorized.POST("/settings/2fa/verify", h.TwoFASetupSubmitHandler)

	r.GET("/login/2fa", h.TwoFALoginViewHandler)
	r.POST("/login/2fa", h.TwoFALoginSubmitHandler)

	authorized.POST("/settings/passkey/register/begin", h.PasskeyRegisterBegin)
	authorized.POST("/settings/passkey/register/finish", h.PasskeyRegisterFinish)

	r.GET("/login/passkey/begin", h.PasskeyLoginBegin)
	r.POST("/login/passkey/finish", h.PasskeyLoginFinish)

	authorized.GET("/settings/sync", h.SyncSettingsHandler)
	authorized.POST("/settings/sync", h.UpdateSyncSettingsHandler)
	authorized.POST("/settings/server", h.UpdateServerSettingsHandler)
	authorized.POST("/settings/sync/reset", h.ResetSyncSettingsHandler)
	authorized.POST("/settings/sync/start", h.StartWorkerHandler)
	authorized.POST("/settings/sync/stop", h.StopWorkerHandler)

	authorized.POST("/settings/user/update", h.UpdateUserProfileHandler)
	authorized.POST("/settings/user/avatar/upload", h.UploadAvatarHandler)
	authorized.POST("/settings/user/avatar/remove", h.RemoveAvatarHandler)
	authorized.POST("/settings/user/password", h.UpdateUserPasswordHandler)

	authorized.GET("/auth/facebook/login", h.FacebookLoginHandler)
	authorized.GET("/auth/facebook/callback", h.FacebookCallbackHandler)
	authorized.POST("/auth/facebook/refresh", h.FacebookRefreshTokenHandler)

	authorized.GET("/auth/tiktok/login", h.TikTokLoginHandler)
	authorized.GET("/auth/tiktok/check", h.TikTokCheckHandler)

	authorized.GET("/exports", h.ExportsHandler)
	authorized.POST("/exports/deleteAll", h.ExportDeleteAllHandler)

	authorized.GET("/outputs/*filepath", h.DownloadExportHandler)

	authorized.GET("/user/setup", h.UserSetupViewHandler)
	authorized.POST("/user/setup", h.UserSetupHandler)
	authorized.GET("/user/:id/avatar", h.AvatarHandler)
	authorized.GET("/sources", h.SourcesHandler)
	authorized.POST("/sources/setup", h.SourcesSetupHandler)
	authorized.POST("/sources/deactivate", h.DeactivateSourceHandler)
	authorized.POST("/sources/activate", h.ActivateSourceHandler)
	authorized.POST("/sources/delete", h.DeleteSourceHandler)
	authorized.POST("/sources/sync", h.SyncSourceHandler)
	authorized.GET("/sources/cookies/export", h.HandleExportCookies)
	authorized.POST("/sources/cookies/import", h.HandleImportCookies)
	authorized.PUT("/sources/:source_id/channels", h.UpdateSourceChannelsHandler)
	authorized.GET("/sources/:source_id/channels", h.GetSourceChannelsHandler)

	authorized.POST("/syncAll", h.TriggerSyncHandler)

	authorized.GET("/targets", h.TargetsHandler)
	authorized.POST("/targets/setup", h.TargetsSetupHandler)
	authorized.POST("/targets/deactivate", h.DeactivateTargetHandler)
	authorized.POST("/targets/activate", h.ActivateTargetHandler)
	authorized.POST("/targets/delete", h.DeleteTargetHandler)
	authorized.POST("/targets/sync", h.SyncTargetHandler)

	authorized.GET("/analytics/engagement", h.AnalyticsEngagementHandler)
	authorized.GET("/analytics/website", h.AnalyticsWebsiteHandler)
	authorized.GET("/analytics/summary", h.AnalyticsDashboardSummaryHandler)

	authorized.GET("/posts", h.PostsHandler)

	authorized.GET("/api/sources", h.HandleGetSourcesAPI)
	authorized.GET("/api/exclusions", h.HandleGetExclusions)
	authorized.POST("/api/exclusions", h.HandleCreateExclusion)
	authorized.DELETE("/api/exclusions/:id", h.HandleDeleteExclusion)

	authorized.GET("/api/redirects", h.HandleGetRedirects)
	authorized.POST("/api/redirects", h.HandleCreateRedirect)
	authorized.DELETE("/api/redirects/:id", h.HandleDeleteRedirect)

	authorized.GET("/api/updates/notes", h.GetReleaseNotesHandler)
	authorized.POST("/api/user/ack-version", h.UpdateLastSeenVersionHandler)

	srv := &http.Server{
		Addr:    ":" + cfg.AppPort,
		Handler: r,
	}

	go func() {
		slog.Info("Server started", "port", cfg.AppPort, "url", "http://localhost:"+cfg.AppPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server listen failed", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("Server forced to shutdown", "error", err)
	}

	slog.Info("Server exiting")
}
