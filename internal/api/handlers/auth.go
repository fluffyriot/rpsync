package handlers

import (
	"context"
	"encoding/base64"
	"net/http"
	"strings"

	"github.com/fluffyriot/rpsync/internal/authhelp"
	"github.com/gin-contrib/sessions"
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

	session := sessions.Default(c)
	appID := session.Get("app_id_" + sid.String())
	appSecret := session.Get("app_secret_" + sid.String())

	var fbConfig *oauth2.Config
	if appID != nil && appSecret != nil {
		fbConfig = authhelp.GenerateFacebookConfig(appID.(string), appSecret.(string), "https://"+h.Config.DomainName+"/auth/facebook/callback")
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "App ID and Secret not found in session"})
		return
	}

	url := fbConfig.AuthCodeURL(state)
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

	session := sessions.Default(c)
	appID := session.Get("app_id_" + sid.String())
	appSecret := session.Get("app_secret_" + sid.String())

	if appID == nil || appSecret == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "App ID and Secret not found in session"})
		return
	}

	fbConfig := authhelp.GenerateFacebookConfig(appID.(string), appSecret.(string), "https://"+h.Config.DomainName+"/auth/facebook/callback")

	code := c.Query("code")
	token, err := fbConfig.Exchange(c, code)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token exchange failed", "details": err.Error()})
		return
	}

	longLivedToken, err := authhelp.ExchangeLongLivedToken(token.AccessToken, fbConfig)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "long-lived token exchange failed", "details": err.Error()})
		return
	}
	token.AccessToken = longLivedToken

	client := fbConfig.Client(c, token)
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

	sourceAppData := map[string]any{
		"app_id":     appID.(string),
		"app_secret": appSecret.(string),
	}

	err = authhelp.InsertSourceToken(context.Background(), h.DB, sid, tokenStr, pid, sourceAppData, h.Config.TokenEncryptionKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to store token", "details": err.Error()})
		return
	}

	session.Delete("app_id_" + sid.String())
	session.Delete("app_secret_" + sid.String())
	session.Save()

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

	currentAccessToken, profileID, sourceAppData, _, err := authhelp.GetSourceToken(context.Background(), h.DB, h.Config.TokenEncryptionKey, sid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve existing token", "details": err.Error()})
		return
	}

	appID, ok1 := sourceAppData["app_id"].(string)
	appSecret, ok2 := sourceAppData["app_secret"].(string)

	if !ok1 || !ok2 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "App ID and Secret not found in token metadata"})
		return
	}

	fbConfig := authhelp.GenerateFacebookConfig(appID, appSecret, "https://"+h.Config.DomainName+"/auth/facebook/callback")

	newLongLivedToken, err := authhelp.ExchangeLongLivedToken(currentAccessToken, fbConfig)
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

	_, _, _, tokenID, err := authhelp.GetSourceToken(context.Background(), h.DB, h.Config.TokenEncryptionKey, sid)
	if err == nil {
		_ = h.DB.DeleteTokenById(context.Background(), tokenID)
	}

	err = authhelp.InsertSourceToken(context.Background(), h.DB, sid, tokenStr, profileID, sourceAppData, h.Config.TokenEncryptionKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to store new token", "details": err.Error()})
		return
	}

	c.Redirect(http.StatusSeeOther, "/sources")
}
