package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"syscall"
	"time"

	"golang.org/x/term"

	_ "embed"

	"github.com/fluffyriot/rpsync/internal/api/handlers"
	"github.com/fluffyriot/rpsync/internal/authhelp"
	"github.com/fluffyriot/rpsync/internal/config"
	"github.com/fluffyriot/rpsync/internal/database"
	"github.com/fluffyriot/rpsync/internal/fetcher"
	"github.com/fluffyriot/rpsync/internal/middleware"
	"github.com/fluffyriot/rpsync/internal/pusher/common"
	"github.com/fluffyriot/rpsync/internal/updater"
	"github.com/fluffyriot/rpsync/internal/worker"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

//go:embed version.json
var versionFile []byte

type projectVersion struct {
	Latest string `json:"latest"`
}

func main() {
	resetPwdFlag := flag.Bool("reset-password", false, "Reset user password")
	reset2FAFlag := flag.Bool("reset-2fa", false, "Reset 2FA (TOTP) for a user")
	resetUserFlag := flag.String("username", "", "Username to reset (optional if only one user)")
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

	clientFetch := fetcher.NewClient(600 * time.Second)
	clientPull := common.NewClient(600 * time.Second)

	r := gin.Default()

	r.SetTrustedProxies(nil)

	r.Static("/static", "./static")
	r.StaticFile("/apple-touch-icon.png", "./static/images/apple-touch-icon.png")

	r.LoadHTMLGlob("templates/*.html")

	store := cookie.NewStore(cfg.SessionKey)
	store.Options(sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 30,
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

		ctx := context.Background()
		targetUsername := *resetUserFlag

		if targetUsername == "" {
			users, err := dbQueries.GetAllUsers(ctx)
			if err != nil {
				log.Fatalf("Failed to fetch users: %v", err)
			}
			if len(users) == 0 {
				log.Fatal("No users found in database.")
			}
			if len(users) > 1 {
				log.Fatal("Multiple users found. Please specify --username <name>")
			}
			targetUsername = users[0].Username
			fmt.Printf("Found single user: %s\n", targetUsername)
		}

		fmt.Printf("Enter new password for '%s': ", targetUsername)
		bytePassword, err := term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			log.Fatalf("\nFailed to read password: %v", err)
		}
		fmt.Println()

		password := string(bytePassword)
		if err := authhelp.ValidatePasswordStrength(password); err != nil {
			log.Fatalf("Password is too weak: %v", err)
		}

		hash, err := authhelp.HashPassword(password)
		if err != nil {
			log.Fatalf("Failed to hash password: %v", err)
		}

		users, err := dbQueries.GetAllUsers(ctx)
		if err != nil {
			log.Fatalf("Failed to query users: %v", err)
		}
		var targetID string
		for _, u := range users {
			if u.Username == targetUsername {
				targetID = u.ID.String()
				break
			}
		}

		if targetID == "" {
			log.Fatalf("User '%s' not found", targetUsername)
		}

		uid, _ := uuid.Parse(targetID)
		_, err = dbQueries.UpdateUserPassword(ctx, database.UpdateUserPasswordParams{
			ID:           uid,
			PasswordHash: sql.NullString{String: hash, Valid: true},
		})
		if err != nil {
			log.Fatalf("Failed to update password: %v", err)
		}

		fmt.Println("Password updated successfully.")
		return
	}

	if *reset2FAFlag {
		if dbConn == nil {
			log.Fatal("Database connection failed, cannot reset 2FA")
		}

		ctx := context.Background()
		targetUsername := *resetUserFlag

		if targetUsername == "" {
			users, err := dbQueries.GetAllUsers(ctx)
			if err != nil {
				log.Fatalf("Failed to fetch users: %v", err)
			}
			if len(users) == 0 {
				log.Fatal("No users found in database.")
			}
			if len(users) > 1 {
				log.Fatal("Multiple users found. Please specify --username <name>")
			}
			targetUsername = users[0].Username
			fmt.Printf("Found single user: %s\n", targetUsername)
		}

		fmt.Printf("Resetting 2FA for user '%s'...\n", targetUsername)

		users, err := dbQueries.GetAllUsers(ctx)
		if err != nil {
			log.Fatalf("Failed to query users: %v", err)
		}
		var targetID string
		for _, u := range users {
			if u.Username == targetUsername {
				targetID = u.ID.String()
				break
			}
		}

		if targetID == "" {
			log.Fatalf("User '%s' not found", targetUsername)
		}

		uid, _ := uuid.Parse(targetID)
		_, err = dbQueries.UpdateUserTOTP(ctx, database.UpdateUserTOTPParams{
			ID:          uid,
			TotpSecret:  sql.NullString{Valid: false},
			TotpEnabled: sql.NullBool{Valid: true, Bool: false},
		})
		if err != nil {
			log.Fatalf("Failed to reset 2FA: %v", err)
		}

		fmt.Println("2FA (TOTP) has been disabled for this user.")
		return
	}

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
	authorized.POST("/settings/sync/reset", h.ResetSyncSettingsHandler)
	authorized.POST("/settings/sync/start", h.StartWorkerHandler)
	authorized.POST("/settings/sync/stop", h.StopWorkerHandler)

	authorized.GET("/auth/facebook/login", h.FacebookLoginHandler)
	authorized.GET("/auth/facebook/callback", h.FacebookCallbackHandler)
	authorized.POST("/auth/facebook/refresh", h.FacebookRefreshTokenHandler)

	authorized.GET("/auth/tiktok/login", h.TikTokLoginHandler)
	authorized.GET("/auth/tiktok/check", h.TikTokCheckHandler)

	authorized.GET("/exports", h.ExportsHandler)
	authorized.POST("/exports/deleteAll", h.ExportDeleteAllHandler)

	authorized.GET("/outputs/*filepath", h.DownloadExportHandler)

	authorized.POST("/user/setup", h.UserSetupHandler)
	authorized.GET("/sources", h.SourcesHandler)
	authorized.POST("/sources/setup", h.SourcesSetupHandler)
	authorized.POST("/sources/deactivate", h.DeactivateSourceHandler)
	authorized.POST("/sources/activate", h.ActivateSourceHandler)
	authorized.POST("/sources/delete", h.DeleteSourceHandler)
	authorized.POST("/sources/sync", h.SyncSourceHandler)
	authorized.POST("/syncAll", h.TriggerSyncHandler)

	authorized.GET("/targets", h.TargetsHandler)
	authorized.POST("/targets/setup", h.TargetsSetupHandler)
	authorized.POST("/targets/deactivate", h.DeactivateTargetHandler)
	authorized.POST("/targets/activate", h.ActivateTargetHandler)
	authorized.POST("/targets/delete", h.DeleteTargetHandler)
	authorized.POST("/targets/sync", h.SyncTargetHandler)

	authorized.GET("/stats", h.StatsHandler)

	authorized.GET("/api/sources", h.HandleGetSourcesAPI)
	authorized.GET("/api/exclusions", h.HandleGetExclusions)
	authorized.POST("/api/exclusions", h.HandleCreateExclusion)
	authorized.DELETE("/api/exclusions/:id", h.HandleDeleteExclusion)

	authorized.GET("/api/redirects", h.HandleGetRedirects)
	authorized.POST("/api/redirects", h.HandleCreateRedirect)
	authorized.DELETE("/api/redirects/:id", h.HandleDeleteRedirect)

	log.Printf("Server started on http://localhost:%s", cfg.AppPort)
	if err := r.Run(":" + cfg.AppPort); err != nil {
		log.Fatal(err)
	}
}
