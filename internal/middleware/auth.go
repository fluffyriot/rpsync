// SPDX-License-Identifier: AGPL-3.0-only
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

		userIdStr, ok := userID.(string)
		if !ok {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		uid, err := uuid.Parse(userIdStr)
		if err != nil {
			session.Clear()
			session.Save()
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		user, err := db.GetUserByID(c.Request.Context(), uid)
		if err != nil {
			session.Clear()
			session.Save()
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		c.Set("user_id", userIdStr)
		c.Set("username", user.Username)

		hasAvatar := false
		if user.ProfileImage.Valid && user.ProfileImage.String != "" {
			hasAvatar = true
		}
		c.Set("has_avatar", hasAvatar)
		c.Set("avatar_version", user.UpdatedAt.Unix())

		if len(user.Username) > 0 {
			c.Set("username_initial", strings.ToUpper(string(user.Username[0])))
		}

		c.Set("last_seen_version", user.LastSeenVersion)
		c.Set("intro_completed", user.IntroCompleted)

		if !user.PasswordHash.Valid || user.PasswordHash.String == "" {
			if c.Request.URL.Path != "/setup/password" {
				c.Redirect(http.StatusFound, "/setup/password")
				c.Abort()
				return
			}
		}

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
