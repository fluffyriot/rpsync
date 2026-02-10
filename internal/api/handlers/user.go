// SPDX-License-Identifier: AGPL-3.0-only
package handlers

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/fluffyriot/rpsync/internal/authhelp"
	"github.com/fluffyriot/rpsync/internal/database"
	"github.com/gen2brain/webp"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/image/draw"
)

func (h *Handler) AvatarHandler(c *gin.Context) {
	userIDStr := c.Param("id")
	if userIDStr == "" {
		c.Status(http.StatusNotFound)
		return
	}

	session := sessions.Default(c)
	sessionUserID := session.Get("user_id")

	if sessionUserID == nil || sessionUserID.(string) != userIDStr {
		c.Status(http.StatusForbidden)
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	user, err := h.DB.GetUserByID(c.Request.Context(), userID)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}

	if !user.ProfileImage.Valid || user.ProfileImage.String == "" {
		c.Status(http.StatusNotFound)
		return
	}

	imageData, err := base64.StdEncoding.DecodeString(user.ProfileImage.String)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}

	c.Header("Cache-Control", "public, max-age=172800")
	c.Header("Content-Type", "image/webp")

	contentType := http.DetectContentType(imageData)
	c.Header("Content-Type", contentType)

	c.Writer.Write(imageData)
}

func (h *Handler) UpdateUserProfileHandler(c *gin.Context) {
	username := c.PostForm("username")
	if username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Username is required"})
		return
	}

	user, loggedIn := h.GetAuthenticatedUser(c)
	if !loggedIn {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not logged in"})
		return
	}

	_, err := h.DB.UpdateUserUsername(c, database.UpdateUserUsernameParams{
		ID:       user.ID,
		Username: username,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update username: " + err.Error()})
		return
	}

	session := sessions.Default(c)
	session.Set("username", username)
	session.Save()

	c.JSON(http.StatusOK, gin.H{"message": "Profile updated successfully"})
}

func (h *Handler) UploadAvatarHandler(c *gin.Context) {
	file, header, err := c.Request.FormFile("avatar")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Bad request: " + err.Error()})
		return
	}
	defer file.Close()

	if header.Size > 25*1024*1024 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File size too large (max 25MB)"})
		return
	}

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".webp" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file type (allowed: jpg, jpeg, png, webp)"})
		return
	}

	img, _, err := image.Decode(file)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to decode image"})
		return
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	minDim := width
	if height < width {
		minDim = height
	}

	x0 := (width - minDim) / 2
	y0 := (height - minDim) / 2
	cropRect := image.Rect(x0, y0, x0+minDim, y0+minDim)
	dst := image.NewRGBA(image.Rect(0, 0, 512, 512))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, cropRect, draw.Over, nil)

	var buf bytes.Buffer
	if err := webp.Encode(&buf, dst, webp.Options{Lossless: false, Quality: 85}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to encode image to WebP"})
		return
	}

	base64Str := base64.StdEncoding.EncodeToString(buf.Bytes())

	user, loggedIn := h.GetAuthenticatedUser(c)
	if !loggedIn {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not logged in"})
		return
	}

	_, err = h.DB.UpdateUserProfileImage(c, database.UpdateUserProfileImageParams{
		ID:           user.ID,
		ProfileImage: sql.NullString{String: base64Str, Valid: true},
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update avatar: " + err.Error()})
		return
	}

	session := sessions.Default(c)
	session.Set("has_avatar", true)
	session.Set("avatar_version", time.Now().Unix())
	session.Save()

	c.JSON(http.StatusOK, gin.H{"message": "Avatar uploaded successfully"})
}

func (h *Handler) RemoveAvatarHandler(c *gin.Context) {
	user, loggedIn := h.GetAuthenticatedUser(c)
	if !loggedIn {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not logged in"})
		return
	}

	_, err := h.DB.UpdateUserProfileImage(c, database.UpdateUserProfileImageParams{
		ID:           user.ID,
		ProfileImage: sql.NullString{String: "", Valid: false},
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove avatar: " + err.Error()})
		return
	}

	session := sessions.Default(c)
	session.Set("has_avatar", false)
	session.Set("avatar_version", time.Now().Unix())
	session.Save()

	c.JSON(http.StatusOK, gin.H{"message": "Avatar removed successfully"})
}

func (h *Handler) UpdateUserPasswordHandler(c *gin.Context) {
	currentPassword := c.PostForm("current_password")
	newPassword := c.PostForm("new_password")
	confirmPassword := c.PostForm("confirm_password")

	user, loggedIn := h.GetAuthenticatedUser(c)
	if !loggedIn {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not logged in"})
		return
	}

	if !user.PasswordHash.Valid || user.PasswordHash.String == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Account has no password set"})
		return
	}

	if !authhelp.CheckPasswordHash(user.PasswordHash.String, currentPassword) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Incorrect current password"})
		return
	}

	if newPassword != confirmPassword {
		c.JSON(http.StatusBadRequest, gin.H{"error": "New passwords do not match"})
		return
	}

	if err := authhelp.ValidatePasswordStrength(newPassword); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	hashedPassword, err := authhelp.HashPassword(newPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	_, err = h.DB.UpdateUserPassword(c.Request.Context(), database.UpdateUserPasswordParams{
		ID:           user.ID,
		PasswordHash: sql.NullString{String: hashedPassword, Valid: true},
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update password: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password updated successfully"})
}

func (h *Handler) UpdateUserIntroCompletedHandler(c *gin.Context) {
	user, loggedIn := h.GetAuthenticatedUser(c)
	if !loggedIn {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not logged in"})
		return
	}

	_, err := h.DB.UpdateUserIntroCompleted(c.Request.Context(), database.UpdateUserIntroCompletedParams{
		ID:             user.ID,
		IntroCompleted: true,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update intro status: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Intro status updated successfully"})
}
