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
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		userIdStr, ok := userID.(string)
		if ok {
			users, err := db.GetAllUsers(c.Request.Context())
			if err == nil {
				var currentUser *database.User
				targetUuid, _ := uuid.Parse(userIdStr)

				for _, u := range users {
					if u.ID == targetUuid {
						currentUser = &u
						break
					}
				}

				if currentUser != nil {
					if !currentUser.PasswordHash.Valid || currentUser.PasswordHash.String == "" {
						if c.Request.URL.Path != "/setup/password" {
							c.Redirect(http.StatusFound, "/setup/password")
							c.Abort()
							return
						}
					}
				}
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
		"/setup",
		"/auth",
	}

	for _, prefix := range publicPrefixes {
		if strings.HasPrefix(path, prefix) {

			if prefix == "/setup" {
				if path == "/setup/password" {
					return false
				}
				return true
			}

			return true
		}
	}
	return false
}
