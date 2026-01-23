package handlers

import (
	"context"
	"encoding/base64"
	"net/http"
	"strings"

	"github.com/fluffyriot/rpsync/internal/authhelp"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
)

func (h *Handler) FacebookLoginHandler(c *gin.Context) {

	sid, err := uuid.Parse(c.Query("sid"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid source_id"})
		return
	}

	pid := c.Query("pid")
	if pid == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "profile_id is required"})
		return
	}

	payload := base64.URLEncoding.EncodeToString([]byte(sid.String() + ":" + pid))

	state := h.Config.OauthEncryptionKey + "|" + payload

	url := h.Config.FBConfig.AuthCodeURL(state)
	c.Redirect(http.StatusTemporaryRedirect, url)

}

func (h *Handler) FacebookCallbackHandler(c *gin.Context) {
	rawState := c.Query("state")
	parts := strings.SplitN(rawState, "|", 2)

	if len(parts) != 2 || parts[0] != h.Config.OauthEncryptionKey {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid oauth state"})
		return
	}

	decoded, err := base64.URLEncoding.DecodeString(parts[1])
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid state payload"})
		return
	}

	values := strings.SplitN(string(decoded), ":", 2)
	if len(values) != 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid state format"})
		return
	}

	sid, err := uuid.Parse(values[0])
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid sid in state"})
		return
	}

	pid := values[1]

	code := c.Query("code")
	token, err := h.Config.FBConfig.Exchange(c, code)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token exchange failed", "details": err.Error()})
		return
	}

	longLivedToken, err := authhelp.ExchangeLongLivedToken(token.AccessToken, h.Config.FBConfig)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "long-lived token exchange failed", "details": err.Error()})
		return
	}
	token.AccessToken = longLivedToken

	client := h.Config.FBConfig.Client(c, token)
	resp, err := client.Get("https://graph.facebook.com/me?fields=id,email")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch user info"})
		return
	}
	defer resp.Body.Close()

	tokenStr, err := authhelp.OauthTokenToString(token)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to serialize token", "details": err.Error()})
		return
	}

	err = authhelp.InsertSourceToken(context.Background(), h.DB, sid, tokenStr, pid, h.Config.TokenEncryptionKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to store token", "details": err.Error()})
		return
	}

	c.Redirect(http.StatusSeeOther, "/sources")
}

func (h *Handler) FacebookRefreshTokenHandler(c *gin.Context) {
	sidStr := c.PostForm("source_id")
	if sidStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "source_id is required"})
		return
	}

	sid, err := uuid.Parse(sidStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid source_id"})
		return
	}

	currentAccessToken, profileID, _, err := authhelp.GetSourceToken(context.Background(), h.DB, h.Config.TokenEncryptionKey, sid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve existing token", "details": err.Error()})
		return
	}

	newLongLivedToken, err := authhelp.ExchangeLongLivedToken(currentAccessToken, h.Config.FBConfig)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to refresh token with Facebook", "details": err.Error()})
		return
	}

	newTokenStruct := &oauth2.Token{
		AccessToken: newLongLivedToken,
		TokenType:   "bearer",
	}

	tokenStr, err := authhelp.OauthTokenToString(newTokenStruct)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to serialize new token", "details": err.Error()})
		return
	}

	_, _, tokenID, err := authhelp.GetSourceToken(context.Background(), h.DB, h.Config.TokenEncryptionKey, sid)
	if err == nil {
		_ = h.DB.DeleteTokenById(context.Background(), tokenID)
	}

	err = authhelp.InsertSourceToken(context.Background(), h.DB, sid, tokenStr, profileID, h.Config.TokenEncryptionKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to store new token", "details": err.Error()})
		return
	}

	c.Redirect(http.StatusSeeOther, "/sources")
}
