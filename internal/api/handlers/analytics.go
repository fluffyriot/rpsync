// SPDX-License-Identifier: AGPL-3.0-only
package handlers

import (
	"database/sql"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/fluffyriot/rpsync/internal/database"
	"github.com/fluffyriot/rpsync/internal/helpers"
	"github.com/fluffyriot/rpsync/internal/stats"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type analyticsFilters struct {
	UserID    uuid.UUID
	StartDate sql.NullTime
	EndDate   sql.NullTime
	PostTypes []string
	TagIDs    []uuid.UUID
	HasFilter bool
}

func parseAnalyticsFilters(c *gin.Context, userID uuid.UUID) analyticsFilters {
	f := analyticsFilters{UserID: userID}

	if sd := c.Query("start_date"); sd != "" {
		if t, err := time.Parse("2006-01-02", sd); err == nil {
			f.StartDate = sql.NullTime{Time: t, Valid: true}
			f.HasFilter = true
		}
	}
	if ed := c.Query("end_date"); ed != "" {
		if t, err := time.Parse("2006-01-02", ed); err == nil {
			f.EndDate = sql.NullTime{Time: t, Valid: true}
			f.HasFilter = true
		}
	}
	if pt := c.Query("post_types"); pt != "" {
		f.PostTypes = strings.Split(pt, ",")
		f.HasFilter = true
	}
	if ti := c.Query("tag_ids"); ti != "" {
		for _, id := range strings.Split(ti, ",") {
			if uid, err := uuid.Parse(strings.TrimSpace(id)); err == nil {
				f.TagIDs = append(f.TagIDs, uid)
			}
		}
		if len(f.TagIDs) > 0 {
			f.HasFilter = true
		}
	}

	return f
}

func (h *Handler) AnalyticsEngagementHandler(c *gin.Context) {

	if h.Config.DBInitErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": h.Config.DBInitErr.Error()})
		return
	}

	user, loggedIn := h.GetAuthenticatedUser(c)
	if !loggedIn {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	statsData, err := stats.GetStats(h.DB, user.ID)
	if err != nil {
		log.Printf("Error getting stats: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, statsData)
}

func (h *Handler) AnalyticsWebsiteHandler(c *gin.Context) {
	if h.Config.DBInitErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": h.Config.DBInitErr.Error()})
		return
	}

	user, loggedIn := h.GetAuthenticatedUser(c)
	if !loggedIn {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	statsData, err := stats.GetAnalyticsStats(h.DB, user.ID)
	if err != nil {
		log.Printf("Error getting analytics stats: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, statsData)
}

func (h *Handler) AnalyticsDashboardSummaryHandler(c *gin.Context) {
	if h.Config.DBInitErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": h.Config.DBInitErr.Error()})
		return
	}

	user, loggedIn := h.GetAuthenticatedUser(c)
	if !loggedIn {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	summary, err := stats.GetDashboardSummary(h.DB, user.ID)
	if err != nil {
		log.Printf("Error getting dashboard summary: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, summary)
}

func (h *Handler) AnalyticsHandler(c *gin.Context) {
	user, loggedIn := h.GetAuthenticatedUser(c)
	if !loggedIn {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	c.HTML(http.StatusOK, "analytics.html", h.CommonData(c, gin.H{
		"title":   "Advanced Analytics",
		"user_id": user.ID,
	}))
}

func (h *Handler) AnalyticsWordCloudHandler(c *gin.Context) {
	user, loggedIn := h.GetAuthenticatedUser(c)
	if !loggedIn {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	f := parseAnalyticsFilters(c, user.ID)
	var data interface{}
	var err error
	if f.HasFilter {
		data, err = h.DB.GetWordCloudDataFiltered(c.Request.Context(), database.GetWordCloudDataFilteredParams{
			UserID:    f.UserID,
			StartDate: f.StartDate,
			EndDate:   f.EndDate,
			PostTypes: f.PostTypes, TagIds: f.TagIDs,
		})
	} else {
		data, err = h.DB.GetWordCloudData(c.Request.Context(), user.ID)
	}
	if err != nil {
		log.Printf("Error getting word cloud data: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, data)
}

func (h *Handler) AnalyticsHashtagsHandler(c *gin.Context) {
	user, loggedIn := h.GetAuthenticatedUser(c)
	if !loggedIn {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	f := parseAnalyticsFilters(c, user.ID)
	viewsMode := c.Query("mode") == "views"
	var data interface{}
	var err error
	if f.HasFilter {
		if viewsMode {
			data, err = h.DB.GetHashtagAnalyticsViewsFiltered(c.Request.Context(), database.GetHashtagAnalyticsViewsFilteredParams{
				UserID:    f.UserID,
				StartDate: f.StartDate,
				EndDate:   f.EndDate,
				PostTypes: f.PostTypes, TagIds: f.TagIDs,
			})
		} else {
			data, err = h.DB.GetHashtagAnalyticsFiltered(c.Request.Context(), database.GetHashtagAnalyticsFilteredParams{
				UserID:    f.UserID,
				StartDate: f.StartDate,
				EndDate:   f.EndDate,
				PostTypes: f.PostTypes, TagIds: f.TagIDs,
			})
		}
	} else {
		if viewsMode {
			data, err = h.DB.GetHashtagAnalyticsViews(c.Request.Context(), user.ID)
		} else {
			data, err = h.DB.GetHashtagAnalytics(c.Request.Context(), user.ID)
		}
	}
	if err != nil {
		log.Printf("Error getting hashtags data: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, data)
}

func (h *Handler) AnalyticsMentionsHandler(c *gin.Context) {
	user, loggedIn := h.GetAuthenticatedUser(c)
	if !loggedIn {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	f := parseAnalyticsFilters(c, user.ID)
	viewsMode := c.Query("mode") == "views"
	var data interface{}
	var err error
	if f.HasFilter {
		if viewsMode {
			data, err = h.DB.GetMentionsAnalyticsViewsFiltered(c.Request.Context(), database.GetMentionsAnalyticsViewsFilteredParams{
				UserID:    f.UserID,
				StartDate: f.StartDate,
				EndDate:   f.EndDate,
				PostTypes: f.PostTypes, TagIds: f.TagIDs,
			})
		} else {
			data, err = h.DB.GetMentionsAnalyticsFiltered(c.Request.Context(), database.GetMentionsAnalyticsFilteredParams{
				UserID:    f.UserID,
				StartDate: f.StartDate,
				EndDate:   f.EndDate,
				PostTypes: f.PostTypes, TagIds: f.TagIDs,
			})
		}
	} else {
		if viewsMode {
			data, err = h.DB.GetMentionsAnalyticsViews(c.Request.Context(), user.ID)
		} else {
			data, err = h.DB.GetMentionsAnalytics(c.Request.Context(), user.ID)
		}
	}
	if err != nil {
		log.Printf("Error getting mentions data: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, data)
}

func (h *Handler) AnalyticsTimeHandler(c *gin.Context) {
	user, loggedIn := h.GetAuthenticatedUser(c)
	if !loggedIn {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	f := parseAnalyticsFilters(c, user.ID)
	var data interface{}
	var err error
	if f.HasFilter {
		data, err = h.DB.GetTimePerformanceFiltered(c.Request.Context(), database.GetTimePerformanceFilteredParams{
			UserID:    f.UserID,
			StartDate: f.StartDate,
			EndDate:   f.EndDate,
			PostTypes: f.PostTypes, TagIds: f.TagIDs,
		})
	} else {
		data, err = h.DB.GetTimePerformance(c.Request.Context(), user.ID)
	}
	if err != nil {
		log.Printf("Error getting time performance data: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, data)
}

func (h *Handler) AnalyticsPostTypesHandler(c *gin.Context) {
	user, loggedIn := h.GetAuthenticatedUser(c)
	if !loggedIn {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	f := parseAnalyticsFilters(c, user.ID)
	var data interface{}
	var err error
	if f.HasFilter {
		data, err = h.DB.GetGlobalPostTypeAnalyticsFiltered(c.Request.Context(), database.GetGlobalPostTypeAnalyticsFilteredParams{
			UserID:    f.UserID,
			StartDate: f.StartDate,
			EndDate:   f.EndDate,
			PostTypes: f.PostTypes, TagIds: f.TagIDs,
		})
	} else {
		data, err = h.DB.GetGlobalPostTypeAnalytics(c.Request.Context(), user.ID)
	}
	if err != nil {
		log.Printf("Error getting post type analytics: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, data)
}

func (h *Handler) AnalyticsNetworkEfficiencyHandler(c *gin.Context) {
	user, loggedIn := h.GetAuthenticatedUser(c)
	if !loggedIn {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	f := parseAnalyticsFilters(c, user.ID)
	var data interface{}
	var err error
	if f.HasFilter {
		data, err = h.DB.GetNetworkEfficiencyFiltered(c.Request.Context(), database.GetNetworkEfficiencyFilteredParams{
			UserID:    f.UserID,
			StartDate: f.StartDate,
			EndDate:   f.EndDate,
			PostTypes: f.PostTypes, TagIds: f.TagIDs,
		})
	} else {
		data, err = h.DB.GetNetworkEfficiency(c.Request.Context(), user.ID)
	}
	if err != nil {
		log.Printf("Error getting network efficiency: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, data)
}

func (h *Handler) AnalyticsTopPagesHandler(c *gin.Context) {
	user, loggedIn := h.GetAuthenticatedUser(c)
	if !loggedIn {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	f := parseAnalyticsFilters(c, user.ID)
	var data interface{}
	var err error
	if f.HasFilter && (f.StartDate.Valid || f.EndDate.Valid) {
		data, err = h.DB.GetTopPagesByViewsFiltered(c.Request.Context(), database.GetTopPagesByViewsFilteredParams{
			UserID:    f.UserID,
			StartDate: f.StartDate,
			EndDate:   f.EndDate,
		})
	} else {
		data, err = h.DB.GetTopPagesByViews(c.Request.Context(), user.ID)
	}
	if err != nil {
		log.Printf("Error getting top pages: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, data)
}

func (h *Handler) AnalyticsSiteStatsHandler(c *gin.Context) {
	user, loggedIn := h.GetAuthenticatedUser(c)
	if !loggedIn {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	f := parseAnalyticsFilters(c, user.ID)
	var data interface{}
	var err error
	if f.HasFilter && (f.StartDate.Valid || f.EndDate.Valid) {
		data, err = h.DB.GetSiteStatsOverTimeFiltered(c.Request.Context(), database.GetSiteStatsOverTimeFilteredParams{
			UserID:    f.UserID,
			StartDate: f.StartDate,
			EndDate:   f.EndDate,
		})
	} else {
		data, err = h.DB.GetSiteStatsOverTime(c.Request.Context(), user.ID)
	}
	if err != nil {
		log.Printf("Error getting site stats: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, data)
}

func (h *Handler) AnalyticsGSCSiteStatsHandler(c *gin.Context) {
	user, loggedIn := h.GetAuthenticatedUser(c)
	if !loggedIn {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	f := parseAnalyticsFilters(c, user.ID)
	var data interface{}
	var err error
	if f.HasFilter && (f.StartDate.Valid || f.EndDate.Valid) {
		data, err = h.DB.GetGSCSiteStatsOverTimeFiltered(c.Request.Context(), database.GetGSCSiteStatsOverTimeFilteredParams{
			UserID:    f.UserID,
			StartDate: f.StartDate,
			EndDate:   f.EndDate,
		})
	} else {
		data, err = h.DB.GetGSCSiteStatsOverTime(c.Request.Context(), user.ID)
	}
	if err != nil {
		log.Printf("Error getting GSC site stats: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, data)
}

func (h *Handler) AnalyticsGSCTopPagesHandler(c *gin.Context) {
	user, loggedIn := h.GetAuthenticatedUser(c)
	if !loggedIn {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	f := parseAnalyticsFilters(c, user.ID)
	var data interface{}
	var err error
	if f.HasFilter && (f.StartDate.Valid || f.EndDate.Valid) {
		data, err = h.DB.GetGSCTopPagesByClicksFiltered(c.Request.Context(), database.GetGSCTopPagesByClicksFilteredParams{
			UserID:    f.UserID,
			StartDate: f.StartDate,
			EndDate:   f.EndDate,
		})
	} else {
		data, err = h.DB.GetGSCTopPagesByClicks(c.Request.Context(), user.ID)
	}
	if err != nil {
		log.Printf("Error getting GSC top pages: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, data)
}

func (h *Handler) AnalyticsPostingConsistencyHandler(c *gin.Context) {
	user, loggedIn := h.GetAuthenticatedUser(c)
	if !loggedIn {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	f := parseAnalyticsFilters(c, user.ID)
	var data interface{}
	var err error
	if f.HasFilter {
		data, err = h.DB.GetPostingConsistencyFiltered(c.Request.Context(), database.GetPostingConsistencyFilteredParams{
			UserID:    f.UserID,
			StartDate: f.StartDate,
			EndDate:   f.EndDate,
			PostTypes: f.PostTypes, TagIds: f.TagIDs,
		})
	} else {
		data, err = h.DB.GetPostingConsistency(c.Request.Context(), user.ID)
	}
	if err != nil {
		log.Printf("Error getting posting consistency: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, data)
}

func (h *Handler) AnalyticsEngagementRateHandler(c *gin.Context) {
	user, loggedIn := h.GetAuthenticatedUser(c)
	if !loggedIn {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	f := parseAnalyticsFilters(c, user.ID)
	var data interface{}
	var err error
	if f.HasFilter {
		data, err = h.DB.GetEngagementRateDataFiltered(c.Request.Context(), database.GetEngagementRateDataFilteredParams{
			UserID:    f.UserID,
			StartDate: f.StartDate,
			EndDate:   f.EndDate,
			PostTypes: f.PostTypes, TagIds: f.TagIDs,
		})
	} else {
		data, err = h.DB.GetEngagementRateData(c.Request.Context(), user.ID)
	}
	if err != nil {
		log.Printf("Error getting engagement rate data: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, data)
}

func (h *Handler) AnalyticsFollowRatioHandler(c *gin.Context) {
	user, loggedIn := h.GetAuthenticatedUser(c)
	if !loggedIn {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	data, err := h.DB.GetFollowRatioData(c.Request.Context(), user.ID)
	if err != nil {
		log.Printf("Error getting follow ratio data: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if data == nil {
		data = []database.GetFollowRatioDataRow{}
	}
	c.JSON(http.StatusOK, data)
}

func (h *Handler) AnalyticsPerformanceDeviationHandler(c *gin.Context) {
	user, loggedIn := h.GetAuthenticatedUser(c)
	if !loggedIn {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	f := parseAnalyticsFilters(c, user.ID)
	viewsMode := c.Query("mode") == "views"

	type DeviationItem struct {
		ID                 interface{} `json:"id"`
		NetworkInternalID  string      `json:"network_internal_id"`
		Content            string      `json:"content"`
		CreatedAt          interface{} `json:"created_at"`
		Author             string      `json:"author"`
		Network            string      `json:"network"`
		Likes              int64       `json:"likes"`
		Reposts            int64       `json:"reposts"`
		Views              int64       `json:"views"`
		ExpectedEngagement float64     `json:"expected_engagement"`
		URL                string      `json:"url"`
		Deviation          float64     `json:"deviation"`
	}

	buildURL := func(network, author, networkInternalID string) string {
		if network == "" || author == "" {
			return ""
		}
		u, _ := helpers.ConvPostToURL(network, author, networkInternalID)
		return u
	}

	var positive, negative []DeviationItem

	if viewsMode {
		if f.HasFilter {
			params := database.GetPerformanceDeviationPositiveViewsFilteredParams{
				UserID: f.UserID, StartDate: f.StartDate, EndDate: f.EndDate, PostTypes: f.PostTypes, TagIds: f.TagIDs,
			}
			rows, e := h.DB.GetPerformanceDeviationPositiveViewsFiltered(c.Request.Context(), params)
			if e != nil {
				log.Printf("Error getting performance deviation positive views data: %v", e)
				c.JSON(http.StatusInternalServerError, gin.H{"error": e.Error()})
				return
			}
			for _, r := range rows {
				positive = append(positive, DeviationItem{
					ID: r.ID, NetworkInternalID: r.NetworkInternalID, Content: r.Content,
					CreatedAt: r.CreatedAt, Author: r.Author, Network: r.Network,
					Views: r.Views, ExpectedEngagement: r.ExpectedEngagement,
					URL:       buildURL(r.Network, r.Author, r.NetworkInternalID),
					Deviation: float64(r.Views) - r.ExpectedEngagement,
				})
			}
			paramN := database.GetPerformanceDeviationNegativeViewsFilteredParams{
				UserID: f.UserID, StartDate: f.StartDate, EndDate: f.EndDate, PostTypes: f.PostTypes, TagIds: f.TagIDs,
			}
			rowsN, e := h.DB.GetPerformanceDeviationNegativeViewsFiltered(c.Request.Context(), paramN)
			if e != nil {
				log.Printf("Error getting performance deviation negative views data: %v", e)
				c.JSON(http.StatusInternalServerError, gin.H{"error": e.Error()})
				return
			}
			for _, r := range rowsN {
				negative = append(negative, DeviationItem{
					ID: r.ID, NetworkInternalID: r.NetworkInternalID, Content: r.Content,
					CreatedAt: r.CreatedAt, Author: r.Author, Network: r.Network,
					Views: r.Views, ExpectedEngagement: r.ExpectedEngagement,
					URL:       buildURL(r.Network, r.Author, r.NetworkInternalID),
					Deviation: float64(r.Views) - r.ExpectedEngagement,
				})
			}
		} else {
			rows, e := h.DB.GetPerformanceDeviationPositiveViews(c.Request.Context(), user.ID)
			if e != nil {
				log.Printf("Error getting performance deviation positive views data: %v", e)
				c.JSON(http.StatusInternalServerError, gin.H{"error": e.Error()})
				return
			}
			for _, r := range rows {
				positive = append(positive, DeviationItem{
					ID: r.ID, NetworkInternalID: r.NetworkInternalID, Content: r.Content,
					CreatedAt: r.CreatedAt, Author: r.Author, Network: r.Network,
					Views: r.Views, ExpectedEngagement: r.ExpectedEngagement,
					URL:       buildURL(r.Network, r.Author, r.NetworkInternalID),
					Deviation: float64(r.Views) - r.ExpectedEngagement,
				})
			}
			rowsN, e := h.DB.GetPerformanceDeviationNegativeViews(c.Request.Context(), user.ID)
			if e != nil {
				log.Printf("Error getting performance deviation negative views data: %v", e)
				c.JSON(http.StatusInternalServerError, gin.H{"error": e.Error()})
				return
			}
			for _, r := range rowsN {
				negative = append(negative, DeviationItem{
					ID: r.ID, NetworkInternalID: r.NetworkInternalID, Content: r.Content,
					CreatedAt: r.CreatedAt, Author: r.Author, Network: r.Network,
					Views: r.Views, ExpectedEngagement: r.ExpectedEngagement,
					URL:       buildURL(r.Network, r.Author, r.NetworkInternalID),
					Deviation: float64(r.Views) - r.ExpectedEngagement,
				})
			}
		}
	} else {
		if f.HasFilter {
			rawPos, e := h.DB.GetPerformanceDeviationPositiveFiltered(c.Request.Context(), database.GetPerformanceDeviationPositiveFilteredParams{
				UserID: f.UserID, StartDate: f.StartDate, EndDate: f.EndDate, PostTypes: f.PostTypes, TagIds: f.TagIDs,
			})
			if e != nil {
				log.Printf("Error getting performance deviation positive data: %v", e)
				c.JSON(http.StatusInternalServerError, gin.H{"error": e.Error()})
				return
			}
			for _, r := range rawPos {
				positive = append(positive, DeviationItem{
					ID: r.ID, NetworkInternalID: r.NetworkInternalID, Content: r.Content,
					CreatedAt: r.CreatedAt, Author: r.Author, Network: r.Network,
					Likes: r.Likes, Reposts: r.Reposts, ExpectedEngagement: r.ExpectedEngagement,
					URL:       buildURL(r.Network, r.Author, r.NetworkInternalID),
					Deviation: float64(r.Likes+r.Reposts) - r.ExpectedEngagement,
				})
			}
			rawNeg, e := h.DB.GetPerformanceDeviationNegativeFiltered(c.Request.Context(), database.GetPerformanceDeviationNegativeFilteredParams{
				UserID: f.UserID, StartDate: f.StartDate, EndDate: f.EndDate, PostTypes: f.PostTypes, TagIds: f.TagIDs,
			})
			if e != nil {
				log.Printf("Error getting performance deviation negative data: %v", e)
				c.JSON(http.StatusInternalServerError, gin.H{"error": e.Error()})
				return
			}
			for _, r := range rawNeg {
				negative = append(negative, DeviationItem{
					ID: r.ID, NetworkInternalID: r.NetworkInternalID, Content: r.Content,
					CreatedAt: r.CreatedAt, Author: r.Author, Network: r.Network,
					Likes: r.Likes, Reposts: r.Reposts, ExpectedEngagement: r.ExpectedEngagement,
					URL:       buildURL(r.Network, r.Author, r.NetworkInternalID),
					Deviation: float64(r.Likes+r.Reposts) - r.ExpectedEngagement,
				})
			}
		} else {
			rows, e := h.DB.GetPerformanceDeviationPositive(c.Request.Context(), user.ID)
			if e != nil {
				log.Printf("Error getting performance deviation positive data: %v", e)
				c.JSON(http.StatusInternalServerError, gin.H{"error": e.Error()})
				return
			}
			for _, r := range rows {
				positive = append(positive, DeviationItem{
					ID: r.ID, NetworkInternalID: r.NetworkInternalID, Content: r.Content,
					CreatedAt: r.CreatedAt, Author: r.Author, Network: r.Network,
					Likes: r.Likes, Reposts: r.Reposts, ExpectedEngagement: r.ExpectedEngagement,
					URL:       buildURL(r.Network, r.Author, r.NetworkInternalID),
					Deviation: float64(r.Likes+r.Reposts) - r.ExpectedEngagement,
				})
			}
			rowsN, e := h.DB.GetPerformanceDeviationNegative(c.Request.Context(), user.ID)
			if e != nil {
				log.Printf("Error getting performance deviation negative data: %v", e)
				c.JSON(http.StatusInternalServerError, gin.H{"error": e.Error()})
				return
			}
			for _, r := range rowsN {
				negative = append(negative, DeviationItem{
					ID: r.ID, NetworkInternalID: r.NetworkInternalID, Content: r.Content,
					CreatedAt: r.CreatedAt, Author: r.Author, Network: r.Network,
					Likes: r.Likes, Reposts: r.Reposts, ExpectedEngagement: r.ExpectedEngagement,
					URL:       buildURL(r.Network, r.Author, r.NetworkInternalID),
					Deviation: float64(r.Likes+r.Reposts) - r.ExpectedEngagement,
				})
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"positive": positive,
		"negative": negative,
	})
}

func (h *Handler) AnalyticsVelocityHandler(c *gin.Context) {
	user, loggedIn := h.GetAuthenticatedUser(c)
	if !loggedIn {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	f := parseAnalyticsFilters(c, user.ID)

	type VelocityItem struct {
		PostID            interface{} `json:"post_id"`
		HistorySyncedAt   interface{} `json:"history_synced_at"`
		Likes             int64       `json:"likes"`
		Reposts           int64       `json:"reposts"`
		Views             int64       `json:"views"`
		PostCreatedAt     interface{} `json:"post_created_at"`
		Content           string      `json:"content"`
		Author            string      `json:"author"`
		NetworkInternalID string      `json:"network_internal_id"`
		Network           string      `json:"network"`
		URL               string      `json:"url"`
	}

	var items []VelocityItem

	if f.HasFilter {
		data, err := h.DB.GetEngagementVelocityDataFiltered(c.Request.Context(), database.GetEngagementVelocityDataFilteredParams{
			UserID:    f.UserID,
			StartDate: f.StartDate,
			EndDate:   f.EndDate,
			PostTypes: f.PostTypes, TagIds: f.TagIDs,
		})
		if err != nil {
			log.Printf("Error getting engagement velocity data: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		items = make([]VelocityItem, len(data))
		for i, d := range data {
			url := ""
			if d.Network != "" && d.Author != "" {
				url, _ = helpers.ConvPostToURL(d.Network, d.Author, d.NetworkInternalID)
			}
			items[i] = VelocityItem{d.PostID, d.HistorySyncedAt, d.Likes, d.Reposts, d.Views, d.PostCreatedAt, d.Content, d.Author, d.NetworkInternalID, d.Network, url}
		}
	} else {
		data, err := h.DB.GetEngagementVelocityData(c.Request.Context(), user.ID)
		if err != nil {
			log.Printf("Error getting engagement velocity data: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		items = make([]VelocityItem, len(data))
		for i, d := range data {
			url := ""
			if d.Network != "" && d.Author != "" {
				url, _ = helpers.ConvPostToURL(d.Network, d.Author, d.NetworkInternalID)
			}
			items[i] = VelocityItem{d.PostID, d.HistorySyncedAt, d.Likes, d.Reposts, d.Views, d.PostCreatedAt, d.Content, d.Author, d.NetworkInternalID, d.Network, url}
		}
	}

	c.JSON(http.StatusOK, items)
}

func (h *Handler) AnalyticsCollaborationsHandler(c *gin.Context) {
	user, loggedIn := h.GetAuthenticatedUser(c)
	if !loggedIn {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	f := parseAnalyticsFilters(c, user.ID)
	viewsMode := c.Query("mode") == "views"
	var data interface{}
	var err error
	if f.HasFilter {
		if viewsMode {
			data, err = h.DB.GetCollaborationsDataViewsFiltered(c.Request.Context(), database.GetCollaborationsDataViewsFilteredParams{
				UserID:    f.UserID,
				StartDate: f.StartDate,
				EndDate:   f.EndDate,
				TagIds:    f.TagIDs,
			})
		} else {
			data, err = h.DB.GetCollaborationsDataFiltered(c.Request.Context(), database.GetCollaborationsDataFilteredParams{
				UserID:    f.UserID,
				StartDate: f.StartDate,
				EndDate:   f.EndDate,
				TagIds:    f.TagIDs,
			})
		}
	} else {
		if viewsMode {
			data, err = h.DB.GetCollaborationsDataViews(c.Request.Context(), user.ID)
		} else {
			data, err = h.DB.GetCollaborationsData(c.Request.Context(), user.ID)
		}
	}
	if err != nil {
		log.Printf("Error getting collaborations data: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, data)
}

func (h *Handler) AnalyticsWordCloudEngagementHandler(c *gin.Context) {
	user, loggedIn := h.GetAuthenticatedUser(c)
	if !loggedIn {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	f := parseAnalyticsFilters(c, user.ID)
	viewsMode := c.Query("mode") == "views"
	var data interface{}
	var err error
	if f.HasFilter {
		if viewsMode {
			d, e := h.DB.GetWordCloudEngagementDataViewsFiltered(c.Request.Context(), database.GetWordCloudEngagementDataViewsFilteredParams{
				UserID:    f.UserID,
				StartDate: f.StartDate,
				EndDate:   f.EndDate,
				PostTypes: f.PostTypes, TagIds: f.TagIDs,
			})
			if d == nil {
				d = []database.GetWordCloudEngagementDataViewsFilteredRow{}
			}
			data, err = d, e
		} else {
			d, e := h.DB.GetWordCloudEngagementDataFiltered(c.Request.Context(), database.GetWordCloudEngagementDataFilteredParams{
				UserID:    f.UserID,
				StartDate: f.StartDate,
				EndDate:   f.EndDate,
				PostTypes: f.PostTypes, TagIds: f.TagIDs,
			})
			if d == nil {
				d = []database.GetWordCloudEngagementDataFilteredRow{}
			}
			data, err = d, e
		}
	} else {
		if viewsMode {
			d, e := h.DB.GetWordCloudEngagementDataViews(c.Request.Context(), user.ID)
			if d == nil {
				d = []database.GetWordCloudEngagementDataViewsRow{}
			}
			data, err = d, e
		} else {
			d, e := h.DB.GetWordCloudEngagementData(c.Request.Context(), user.ID)
			if d == nil {
				d = []database.GetWordCloudEngagementDataRow{}
			}
			data, err = d, e
		}
	}
	if err != nil {
		log.Printf("Error getting word cloud engagement data: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, data)
}

func (h *Handler) AnalyticsTopInteractedHandler(c *gin.Context) {
	user, loggedIn := h.GetAuthenticatedUser(c)
	if !loggedIn {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	viewsMode := c.Query("mode") == "views"

	type InteractedItem struct {
		ID                interface{} `json:"id"`
		NetworkInternalID string      `json:"network_internal_id"`
		Content           string      `json:"content"`
		CreatedAt         interface{} `json:"created_at"`
		Author            string      `json:"author"`
		Network           string      `json:"network"`
		EngagementDelta   int64       `json:"engagement_delta"`
		EngagementTotal   int64       `json:"engagement_total"`
		URL               string      `json:"url"`
	}

	buildURL := func(network, author, networkInternalID string) string {
		if network == "" || author == "" {
			return ""
		}
		u, _ := helpers.ConvPostToURL(network, author, networkInternalID)
		return u
	}

	var since1Day, since14Days []InteractedItem

	if viewsMode {
		rows1, err := h.DB.GetTopInteractedSince1DayViews(c.Request.Context(), user.ID)
		if err != nil {
			log.Printf("Error getting top interacted since 1 day views: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		for _, r := range rows1 {
			since1Day = append(since1Day, InteractedItem{
				ID: r.ID, NetworkInternalID: r.NetworkInternalID, Content: r.Content,
				CreatedAt: r.CreatedAt, Author: r.Author, Network: r.Network,
				EngagementDelta: r.EngagementDelta, EngagementTotal: r.EngagementTotal,
				URL: buildURL(r.Network, r.Author, r.NetworkInternalID),
			})
		}
		rows14, err := h.DB.GetTopInteractedSince14DaysViews(c.Request.Context(), user.ID)
		if err != nil {
			log.Printf("Error getting top interacted since 14 days views: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		for _, r := range rows14 {
			since14Days = append(since14Days, InteractedItem{
				ID: r.ID, NetworkInternalID: r.NetworkInternalID, Content: r.Content,
				CreatedAt: r.CreatedAt, Author: r.Author, Network: r.Network,
				EngagementDelta: r.EngagementDelta, EngagementTotal: r.EngagementTotal,
				URL: buildURL(r.Network, r.Author, r.NetworkInternalID),
			})
		}
	} else {
		rows1, err := h.DB.GetTopInteractedSince1Day(c.Request.Context(), user.ID)
		if err != nil {
			log.Printf("Error getting top interacted since 1 day: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		for _, r := range rows1 {
			since1Day = append(since1Day, InteractedItem{
				ID: r.ID, NetworkInternalID: r.NetworkInternalID, Content: r.Content,
				CreatedAt: r.CreatedAt, Author: r.Author, Network: r.Network,
				EngagementDelta: r.EngagementDelta, EngagementTotal: r.EngagementTotal,
				URL: buildURL(r.Network, r.Author, r.NetworkInternalID),
			})
		}
		rows14, err := h.DB.GetTopInteractedSince14Days(c.Request.Context(), user.ID)
		if err != nil {
			log.Printf("Error getting top interacted since 14 days: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		for _, r := range rows14 {
			since14Days = append(since14Days, InteractedItem{
				ID: r.ID, NetworkInternalID: r.NetworkInternalID, Content: r.Content,
				CreatedAt: r.CreatedAt, Author: r.Author, Network: r.Network,
				EngagementDelta: r.EngagementDelta, EngagementTotal: r.EngagementTotal,
				URL: buildURL(r.Network, r.Author, r.NetworkInternalID),
			})
		}
	}

	if since1Day == nil {
		since1Day = []InteractedItem{}
	}
	if since14Days == nil {
		since14Days = []InteractedItem{}
	}

	c.JSON(http.StatusOK, gin.H{
		"since_1day":   since1Day,
		"since_14days": since14Days,
	})
}
