package middleware

import (
	"net/http"
	"strings"

	"github.com/fluffyriot/rpsync/internal/database"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func AuthMiddleware(db *database.Queries) gin.HandlerFunc {
	return func(c *gin.Context) {
		if isPublicRoute(c.Request.URL.Path) {
			c.Next()
			return
		}

		session := sessions.Default(c)
		userID := session.Get("user_id")

		if userID == nil {

			users, err := db.GetAllUsers(c.Request.Context())
			if err == nil && len(users) == 0 {
				if c.Request.URL.Path != "/user/setup" && !strings.HasPrefix(c.Request.URL.Path, "/static") {
					c.Redirect(http.StatusFound, "/user/setup")
					c.Abort()
					return
				}
				c.Next()
				return
			}

			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		username := session.Get("username")
		hasAvatar := session.Get("has_avatar")
		avatarVersion := session.Get("avatar_version")
		lastSeenVersion := session.Get("last_seen_version")

		userIdStr, ok := userID.(string)
		if !ok {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		var currentUsername string
		var currentHasAvatar bool
		var currentAvatarVersion int64
		var currentLastSeenVersion string

		if username != nil {
			currentUsername = username.(string)
			if hasAvatar != nil {
				currentHasAvatar = hasAvatar.(bool)
			}
			if avatarVersion != nil {
				switch v := avatarVersion.(type) {
				case int64:
					currentAvatarVersion = v
				case int:
					currentAvatarVersion = int64(v)
				case float64:
					currentAvatarVersion = int64(v)
				}
			}
			if lastSeenVersion != nil {
				currentLastSeenVersion = lastSeenVersion.(string)
			}
		} else {
			users, err := db.GetAllUsers(c.Request.Context())
			if err == nil {
				targetUuid, _ := uuid.Parse(userIdStr)
				for _, u := range users {
					if u.ID == targetUuid {
						currentUsername = u.Username
						if u.ProfileImage.Valid && u.ProfileImage.String != "" {
							currentHasAvatar = true
						}
						currentAvatarVersion = u.UpdatedAt.Unix()
						currentLastSeenVersion = u.LastSeenVersion

						if !u.PasswordHash.Valid || u.PasswordHash.String == "" {
							if c.Request.URL.Path != "/setup/password" {
								c.Redirect(http.StatusFound, "/setup/password")
								c.Abort()
								return
							}
						}
						break
					}
				}
			}
		}

		c.Set("user_id", userIdStr)
		c.Set("username", currentUsername)
		c.Set("has_avatar", currentHasAvatar)
		c.Set("avatar_version", currentAvatarVersion)
		if len(currentUsername) > 0 {
			c.Set("username_initial", strings.ToUpper(string(currentUsername[0])))
		}
		c.Set("last_seen_version", currentLastSeenVersion)

		c.Next()
	}
}

func isPublicRoute(path string) bool {
	publicPrefixes := []string{
		"/login",
		"/static",
		"/health",
		"/health",
		"/auth",
	}

	for _, prefix := range publicPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}
