// SPDX-License-Identifier: AGPL-3.0-only
package handlers

import (
	"net/http"

	"github.com/fluffyriot/rpsync/internal/database"
	"github.com/fluffyriot/rpsync/internal/helpers"
	"github.com/gin-gonic/gin"
)

func (h *Handler) PostsHandler(c *gin.Context) {
	if h.Config.DBInitErr != nil {
		c.HTML(http.StatusInternalServerError, "error.html", h.CommonData(c, gin.H{
			"error": h.Config.DBInitErr.Error(),
			"title": "Error",
		}))
		return
	}

	user, loggedIn := h.GetAuthenticatedUser(c)
	if !loggedIn {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	ctx := c.Request.Context()

	posts, err := h.DB.GetRecentPostsForUser(ctx, user.ID)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", h.CommonData(c, gin.H{
			"error": err.Error(),
			"title": "Error",
		}))
		return
	}

	type PostWithURL struct {
		Post database.GetRecentPostsForUserRow
		URL  string
	}

	postsWithURL := make([]PostWithURL, 0, len(posts))
	for _, post := range posts {
		url := ""
		if post.Network.Valid && post.Author != "" {
			url, _ = helpers.ConvPostToURL(post.Network.String, post.Author, post.NetworkInternalID)
		}
		postsWithURL = append(postsWithURL, PostWithURL{
			Post: post,
			URL:  url,
		})
	}

	c.HTML(http.StatusOK, "posts.html", h.CommonData(c, gin.H{
		"posts": postsWithURL,
		"title": "Posts",
	}))
}
