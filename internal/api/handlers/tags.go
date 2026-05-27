// SPDX-License-Identifier: AGPL-3.0-only
package handlers

import (
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/fluffyriot/rpsync/internal/database"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

type CreateClassificationRequest struct {
	Name string `json:"name" binding:"required"`
}

type UpdateClassificationRequest struct {
	Name string `json:"name" binding:"required"`
}

type CreateTagRequest struct {
	Name             string  `json:"name" binding:"required"`
	ClassificationID *string `json:"classification_id"`
}

type UpdateTagRequest struct {
	Name             string  `json:"name" binding:"required"`
	ClassificationID *string `json:"classification_id"`
}

type SetPostTagsRequest struct {
	TagIDs []string `json:"tag_ids"`
}

type ClassificationResponse struct {
	ID        string `json:"id"`
	CreatedAt string `json:"created_at"`
	Name      string `json:"name"`
}

type TagResponse struct {
	ID                 string `json:"id"`
	CreatedAt          string `json:"created_at"`
	Name               string `json:"name"`
	ClassificationID   string `json:"classification_id"`
	ClassificationName string `json:"classification_name"`
}

type PostTagResponse struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	ClassificationID   string `json:"classification_id"`
	ClassificationName string `json:"classification_name"`
}

type PostTagBulkRow struct {
	PostID             string `json:"post_id"`
	TagID              string `json:"tag_id"`
	TagName            string `json:"tag_name"`
	ClassificationName string `json:"classification_name"`
}

func (h *Handler) TagsHandler(c *gin.Context) {
	if h.Config.DBInitErr != nil {
		c.HTML(http.StatusInternalServerError, "error.html", h.CommonData(c, gin.H{
			"error": h.Config.DBInitErr.Error(),
			"title": "Error",
		}))
		return
	}

	_, loggedIn := h.GetAuthenticatedUser(c)
	if !loggedIn {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	c.HTML(http.StatusOK, "tags.html", h.CommonData(c, gin.H{
		"title": "Tags",
	}))
}

func (h *Handler) HandleGetClassifications(c *gin.Context) {
	if h.Config.DBInitErr != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: h.Config.DBInitErr.Error()})
		return
	}

	user, loggedIn := h.GetAuthenticatedUser(c)
	if !loggedIn {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "Unauthorized"})
		return
	}

	classifications, err := h.DB.GetTagClassificationsForUser(c.Request.Context(), user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	response := make([]ClassificationResponse, len(classifications))
	for i, cl := range classifications {
		response[i] = ClassificationResponse{
			ID:        cl.ID.String(),
			CreatedAt: cl.CreatedAt.Format(time.RFC3339),
			Name:      cl.Name,
		}
	}

	c.JSON(http.StatusOK, response)
}

func (h *Handler) HandleCreateClassification(c *gin.Context) {
	if h.Config.DBInitErr != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: h.Config.DBInitErr.Error()})
		return
	}

	user, loggedIn := h.GetAuthenticatedUser(c)
	if !loggedIn {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "Unauthorized"})
		return
	}

	var req CreateClassificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid request: " + err.Error()})
		return
	}

	if len(req.Name) > 40 {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Classification name must be 40 characters or less"})
		return
	}

	now := time.Now()
	classification, err := h.DB.CreateTagClassification(c.Request.Context(), database.CreateTagClassificationParams{
		ID:        uuid.New(),
		CreatedAt: now,
		UpdatedAt: now,
		UserID:    user.ID,
		Name:      req.Name,
	})
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			c.JSON(http.StatusConflict, ErrorResponse{Error: "A classification with this name already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, ClassificationResponse{
		ID:        classification.ID.String(),
		CreatedAt: classification.CreatedAt.Format(time.RFC3339),
		Name:      classification.Name,
	})
}

func (h *Handler) HandleUpdateClassification(c *gin.Context) {
	if h.Config.DBInitErr != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: h.Config.DBInitErr.Error()})
		return
	}

	_, loggedIn := h.GetAuthenticatedUser(c)
	if !loggedIn {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "Unauthorized"})
		return
	}

	classificationID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid classification ID format"})
		return
	}

	var req UpdateClassificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid request: " + err.Error()})
		return
	}

	if len(req.Name) > 40 {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Classification name must be 40 characters or less"})
		return
	}

	updated, err := h.DB.UpdateTagClassification(c.Request.Context(), database.UpdateTagClassificationParams{
		ID:        classificationID,
		Name:      req.Name,
		UpdatedAt: time.Now(),
	})
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			c.JSON(http.StatusConflict, ErrorResponse{Error: "A classification with this name already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, ClassificationResponse{
		ID:        updated.ID.String(),
		CreatedAt: updated.CreatedAt.Format(time.RFC3339),
		Name:      updated.Name,
	})
}

func (h *Handler) HandleDeleteClassification(c *gin.Context) {
	if h.Config.DBInitErr != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: h.Config.DBInitErr.Error()})
		return
	}

	_, loggedIn := h.GetAuthenticatedUser(c)
	if !loggedIn {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "Unauthorized"})
		return
	}

	classificationID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid classification ID format"})
		return
	}

	err = h.DB.DeleteTagClassification(c.Request.Context(), classificationID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{Message: "Classification deleted successfully"})
}

func (h *Handler) HandleGetTags(c *gin.Context) {
	if h.Config.DBInitErr != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: h.Config.DBInitErr.Error()})
		return
	}

	user, loggedIn := h.GetAuthenticatedUser(c)
	if !loggedIn {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "Unauthorized"})
		return
	}

	tags, err := h.DB.GetTagsForUser(c.Request.Context(), user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	response := make([]TagResponse, len(tags))
	for i, t := range tags {
		classID := ""
		if t.ClassificationID.Valid {
			classID = t.ClassificationID.UUID.String()
		}
		className := ""
		if t.ClassificationName.Valid {
			className = t.ClassificationName.String
		}
		response[i] = TagResponse{
			ID:                 t.ID.String(),
			CreatedAt:          t.CreatedAt.Format(time.RFC3339),
			Name:               t.Name,
			ClassificationID:   classID,
			ClassificationName: className,
		}
	}

	c.JSON(http.StatusOK, response)
}

func (h *Handler) HandleCreateTag(c *gin.Context) {
	if h.Config.DBInitErr != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: h.Config.DBInitErr.Error()})
		return
	}

	user, loggedIn := h.GetAuthenticatedUser(c)
	if !loggedIn {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "Unauthorized"})
		return
	}

	var req CreateTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid request: " + err.Error()})
		return
	}

	if len(req.Name) > 40 {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Tag name must be 40 characters or less"})
		return
	}

	var classificationID uuid.NullUUID
	if req.ClassificationID != nil && *req.ClassificationID != "" {
		parsed, err := uuid.Parse(*req.ClassificationID)
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid classification_id format"})
			return
		}
		classificationID = uuid.NullUUID{UUID: parsed, Valid: true}
	}

	now := time.Now()
	tag, err := h.DB.CreateTag(c.Request.Context(), database.CreateTagParams{
		ID:               uuid.New(),
		CreatedAt:        now,
		UpdatedAt:        now,
		UserID:           user.ID,
		ClassificationID: classificationID,
		Name:             req.Name,
	})
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			c.JSON(http.StatusConflict, ErrorResponse{Error: "A tag with this name already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	classID := ""
	if tag.ClassificationID.Valid {
		classID = tag.ClassificationID.UUID.String()
	}

	c.JSON(http.StatusCreated, TagResponse{
		ID:               tag.ID.String(),
		CreatedAt:        tag.CreatedAt.Format(time.RFC3339),
		Name:             tag.Name,
		ClassificationID: classID,
	})
}

func (h *Handler) HandleUpdateTag(c *gin.Context) {
	if h.Config.DBInitErr != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: h.Config.DBInitErr.Error()})
		return
	}

	_, loggedIn := h.GetAuthenticatedUser(c)
	if !loggedIn {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "Unauthorized"})
		return
	}

	tagID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid tag ID format"})
		return
	}

	var req UpdateTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid request: " + err.Error()})
		return
	}

	if len(req.Name) > 40 {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Tag name must be 40 characters or less"})
		return
	}

	var classificationID uuid.NullUUID
	if req.ClassificationID != nil && *req.ClassificationID != "" {
		parsed, err := uuid.Parse(*req.ClassificationID)
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid classification_id format"})
			return
		}
		classificationID = uuid.NullUUID{UUID: parsed, Valid: true}
	}

	updated, err := h.DB.UpdateTag(c.Request.Context(), database.UpdateTagParams{
		ID:               tagID,
		Name:             req.Name,
		UpdatedAt:        time.Now(),
		ClassificationID: classificationID,
	})
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			c.JSON(http.StatusConflict, ErrorResponse{Error: "A tag with this name already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	classID := ""
	if updated.ClassificationID.Valid {
		classID = updated.ClassificationID.UUID.String()
	}

	c.JSON(http.StatusOK, TagResponse{
		ID:               updated.ID.String(),
		CreatedAt:        updated.CreatedAt.Format(time.RFC3339),
		Name:             updated.Name,
		ClassificationID: classID,
	})
}

func (h *Handler) HandleDeleteTag(c *gin.Context) {
	if h.Config.DBInitErr != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: h.Config.DBInitErr.Error()})
		return
	}

	_, loggedIn := h.GetAuthenticatedUser(c)
	if !loggedIn {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "Unauthorized"})
		return
	}

	tagID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid tag ID format"})
		return
	}

	err = h.DB.DeleteTag(c.Request.Context(), tagID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{Message: "Tag deleted successfully"})
}

func (h *Handler) HandleGetPostTags(c *gin.Context) {
	if h.Config.DBInitErr != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: h.Config.DBInitErr.Error()})
		return
	}

	_, loggedIn := h.GetAuthenticatedUser(c)
	if !loggedIn {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "Unauthorized"})
		return
	}

	postID, err := uuid.Parse(c.Param("post_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid post ID format"})
		return
	}

	tags, err := h.DB.GetTagsForPost(c.Request.Context(), postID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	response := make([]PostTagResponse, len(tags))
	for i, t := range tags {
		classID := ""
		if t.ClassificationID.Valid {
			classID = t.ClassificationID.UUID.String()
		}
		className := ""
		if t.ClassificationName.Valid {
			className = t.ClassificationName.String
		}
		response[i] = PostTagResponse{
			ID:                 t.ID.String(),
			Name:               t.Name,
			ClassificationID:   classID,
			ClassificationName: className,
		}
	}

	c.JSON(http.StatusOK, response)
}

func (h *Handler) HandleSetPostTags(c *gin.Context) {
	if h.Config.DBInitErr != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: h.Config.DBInitErr.Error()})
		return
	}

	_, loggedIn := h.GetAuthenticatedUser(c)
	if !loggedIn {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "Unauthorized"})
		return
	}

	postID, err := uuid.Parse(c.Param("post_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid post ID format"})
		return
	}

	var req SetPostTagsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid request: " + err.Error()})
		return
	}

	if len(req.TagIDs) > 5 {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "A post cannot have more than 5 tags"})
		return
	}

	ctx := c.Request.Context()

	err = h.DB.ClearTagsForPost(ctx, postID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	for _, tagIDStr := range req.TagIDs {
		tagID, err := uuid.Parse(tagIDStr)
		if err != nil {
			continue
		}
		_, err = h.DB.AddTagToPost(ctx, database.AddTagToPostParams{
			ID:        uuid.New(),
			CreatedAt: time.Now(),
			PostID:    postID,
			TagID:     tagID,
		})
		if err != nil {
			log.Printf("Warning: Failed to add tag %s to post %s: %v", tagIDStr, postID, err)
		}
	}

	c.JSON(http.StatusOK, SuccessResponse{Message: "Post tags updated successfully"})
}

func (h *Handler) HandleAddTagToPost(c *gin.Context) {
	if h.Config.DBInitErr != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: h.Config.DBInitErr.Error()})
		return
	}

	_, loggedIn := h.GetAuthenticatedUser(c)
	if !loggedIn {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "Unauthorized"})
		return
	}

	postID, err := uuid.Parse(c.Param("post_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid post ID format"})
		return
	}

	var req struct {
		TagID string `json:"tag_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid request: " + err.Error()})
		return
	}

	tagID, err := uuid.Parse(req.TagID)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid tag_id format"})
		return
	}

	currentTags, err := h.DB.GetTagsForPost(c.Request.Context(), postID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	if len(currentTags) >= 5 {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "A post cannot have more than 5 tags"})
		return
	}

	_, err = h.DB.AddTagToPost(c.Request.Context(), database.AddTagToPostParams{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		PostID:    postID,
		TagID:     tagID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, SuccessResponse{Message: "Tag added to post"})
}

func (h *Handler) HandleRemoveTagFromPost(c *gin.Context) {
	if h.Config.DBInitErr != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: h.Config.DBInitErr.Error()})
		return
	}

	_, loggedIn := h.GetAuthenticatedUser(c)
	if !loggedIn {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "Unauthorized"})
		return
	}

	postID, err := uuid.Parse(c.Param("post_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid post ID format"})
		return
	}

	tagID, err := uuid.Parse(c.Param("tag_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid tag ID format"})
		return
	}

	err = h.DB.RemoveTagFromPost(c.Request.Context(), database.RemoveTagFromPostParams{
		PostID: postID,
		TagID:  tagID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{Message: "Tag removed from post"})
}

func (h *Handler) HandleGetAllPostTagsBulk(c *gin.Context) {
	if h.Config.DBInitErr != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: h.Config.DBInitErr.Error()})
		return
	}

	user, loggedIn := h.GetAuthenticatedUser(c)
	if !loggedIn {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "Unauthorized"})
		return
	}

	rows, err := h.DB.GetAllPostTagsForUser(c.Request.Context(), user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	response := make([]PostTagBulkRow, len(rows))
	for i, r := range rows {
		className := ""
		if r.ClassificationName.Valid {
			className = r.ClassificationName.String
		}
		response[i] = PostTagBulkRow{
			PostID:             r.PostID.String(),
			TagID:              r.TagID.String(),
			TagName:            r.TagName,
			ClassificationName: className,
		}
	}

	c.JSON(http.StatusOK, response)
}

func (h *Handler) AnalyticsTagsHandler(c *gin.Context) {
	user, loggedIn := h.GetAuthenticatedUser(c)
	if !loggedIn {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	f := parseAnalyticsFilters(c, user.ID)
	var data interface{}
	var err error
	if f.HasFilter {
		data, err = h.DB.GetTagAnalyticsFiltered(c.Request.Context(), database.GetTagAnalyticsFilteredParams{
			UserID:    f.UserID,
			StartDate: f.StartDate,
			EndDate:   f.EndDate,
			PostTypes: f.PostTypes,
			TagIds:    f.TagIDs,
		})
	} else {
		data, err = h.DB.GetTagAnalytics(c.Request.Context(), user.ID)
	}
	if err != nil {
		log.Printf("Error getting tag analytics: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, data)
}

func (h *Handler) AnalyticsClassificationsHandler(c *gin.Context) {
	user, loggedIn := h.GetAuthenticatedUser(c)
	if !loggedIn {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	f := parseAnalyticsFilters(c, user.ID)
	var data interface{}
	var err error
	if f.HasFilter {
		data, err = h.DB.GetClassificationAnalyticsFiltered(c.Request.Context(), database.GetClassificationAnalyticsFilteredParams{
			UserID:    f.UserID,
			StartDate: f.StartDate,
			EndDate:   f.EndDate,
			PostTypes: f.PostTypes,
			TagIds:    f.TagIDs,
		})
	} else {
		data, err = h.DB.GetClassificationAnalytics(c.Request.Context(), user.ID)
	}
	if err != nil {
		log.Printf("Error getting classification analytics: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, data)
}
