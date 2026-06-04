// SPDX-License-Identifier: AGPL-3.0-only
package backup

import (
	"archive/zip"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/fluffyriot/rpsync/internal/database"
	"github.com/google/uuid"
)

type ImportMode string

const (
	ImportModeReplace ImportMode = "replace"
	ImportModeNew     ImportMode = "new"
)

func ImportUserData(ctx context.Context, db *database.Queries, dbConn *sql.DB, zipReader *zip.Reader, mode ImportMode, currentUserID uuid.UUID) (*ImportResult, error) {
	files := make(map[string][]byte)
	for _, f := range zipReader.File {
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("failed to open zip entry %s: %w", f.Name, err)
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read zip entry %s: %w", f.Name, err)
		}
		files[f.Name] = data
	}

	var manifest Manifest
	if err := parseJSON(files, "manifest.json", &manifest); err != nil {
		return nil, fmt.Errorf("invalid backup: %w", err)
	}
	if manifest.Version != ManifestVersion {
		return nil, fmt.Errorf("unsupported backup version: %s (expected %s)", manifest.Version, ManifestVersion)
	}

	var backupUser BackupUser
	if err := parseJSON(files, "user.json", &backupUser); err != nil {
		return nil, fmt.Errorf("invalid user data: %w", err)
	}

	var sources []BackupSource
	if err := parseJSON(files, "sources.json", &sources); err != nil {
		return nil, fmt.Errorf("invalid sources data: %w", err)
	}

	var targets []BackupTarget
	if err := parseJSON(files, "targets.json", &targets); err != nil {
		return nil, fmt.Errorf("invalid targets data: %w", err)
	}

	var tokens []BackupToken
	if err := parseJSON(files, "tokens.json", &tokens); err != nil {
		return nil, fmt.Errorf("invalid tokens data: %w", err)
	}

	var posts []BackupPost
	if err := parseJSON(files, "posts.json", &posts); err != nil {
		return nil, fmt.Errorf("invalid posts data: %w", err)
	}

	var reactions []BackupReaction
	if err := parseJSON(files, "posts_reactions_history.json", &reactions); err != nil {
		return nil, fmt.Errorf("invalid reactions data: %w", err)
	}

	var tagClassifications []BackupTagClassification
	if err := parseJSON(files, "tag_classifications.json", &tagClassifications); err != nil {
		return nil, fmt.Errorf("invalid tag classifications data: %w", err)
	}

	var tags []BackupTag
	if err := parseJSON(files, "tags.json", &tags); err != nil {
		return nil, fmt.Errorf("invalid tags data: %w", err)
	}

	var postTags []BackupPostTag
	if err := parseJSON(files, "post_tags.json", &postTags); err != nil {
		return nil, fmt.Errorf("invalid post tags data: %w", err)
	}

	var exclusions []BackupExclusion
	if err := parseJSON(files, "exclusions.json", &exclusions); err != nil {
		return nil, fmt.Errorf("invalid exclusions data: %w", err)
	}

	var redirects []BackupRedirect
	if err := parseJSON(files, "redirects.json", &redirects); err != nil {
		return nil, fmt.Errorf("invalid redirects data: %w", err)
	}

	var sourcesStats []BackupSourcesStat
	if err := parseJSON(files, "sources_stats.json", &sourcesStats); err != nil {
		return nil, fmt.Errorf("invalid sources stats data: %w", err)
	}

	var pageStats []BackupAnalyticsPageStat
	if err := parseJSON(files, "analytics_page_stats.json", &pageStats); err != nil {
		return nil, fmt.Errorf("invalid page stats data: %w", err)
	}

	var siteStats []BackupAnalyticsSiteStat
	if err := parseJSON(files, "analytics_site_stats.json", &siteStats); err != nil {
		return nil, fmt.Errorf("invalid site stats data: %w", err)
	}

	idMap := make(map[string]uuid.UUID)
	targetUserID := currentUserID

	if mode == ImportModeNew {
		targetUserID = uuid.New()
	}

	oldUserID := backupUser.ID
	idMap[oldUserID] = targetUserID

	for _, s := range sources {
		idMap[s.ID] = uuid.New()
	}
	for _, t := range targets {
		idMap[t.ID] = uuid.New()
	}
	for _, t := range tokens {
		idMap[t.ID] = uuid.New()
	}
	for _, p := range posts {
		idMap[p.ID] = uuid.New()
	}
	for _, r := range reactions {
		idMap[r.ID] = uuid.New()
	}
	for _, tc := range tagClassifications {
		idMap[tc.ID] = uuid.New()
	}
	for _, t := range tags {
		idMap[t.ID] = uuid.New()
	}
	for _, pt := range postTags {
		idMap[pt.ID] = uuid.New()
	}
	for _, e := range exclusions {
		idMap[e.ID] = uuid.New()
	}
	for _, r := range redirects {
		idMap[r.ID] = uuid.New()
	}
	for _, s := range sourcesStats {
		idMap[s.ID] = uuid.New()
	}
	for _, s := range pageStats {
		idMap[s.ID] = uuid.New()
	}
	for _, s := range siteStats {
		idMap[s.ID] = uuid.New()
	}

	remap := func(old string) uuid.UUID {
		if v, ok := idMap[old]; ok {
			return v
		}
		return uuid.Nil
	}

	tx, err := dbConn.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	qtx := db.WithTx(tx)

	if mode == ImportModeReplace {
		if err := deleteAllUserData(ctx, qtx, currentUserID); err != nil {
			return nil, fmt.Errorf("failed to delete existing data: %w", err)
		}
	}

	result := &ImportResult{}

	if mode == ImportModeNew {
		createdAt, _ := time.Parse(timeFormat, backupUser.CreatedAt)
		finalUsername := backupUser.Username

		_, err := qtx.GetUserByUsername(ctx, backupUser.Username)
		if err == nil {
			timestamp := time.Now().Format("20060102150405")
			finalUsername = fmt.Sprintf("%s_%s", backupUser.Username, timestamp)
		}

		_, err = qtx.CreateUser(ctx, database.CreateUserParams{
			ID:         targetUserID,
			Username:   finalUsername,
			CreatedAt:  createdAt,
			UpdatedAt:  time.Now(),
			SyncPeriod: backupUser.SyncPeriod,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create user: %w", err)
		}

		result.GeneratedUsername = finalUsername
	}

	for _, s := range sources {
		createdAt, _ := time.Parse(timeFormat, s.CreatedAt)
		updatedAt, _ := time.Parse(timeFormat, s.UpdatedAt)
		var lastSynced sql.NullTime
		if s.LastSynced != nil {
			t, _ := time.Parse(timeFormat, *s.LastSynced)
			lastSynced = sql.NullTime{Time: t, Valid: true}
		}
		var statusReason sql.NullString
		if s.StatusReason != nil {
			statusReason = sql.NullString{String: *s.StatusReason, Valid: true}
		}
		_, err := qtx.CreateSource(ctx, database.CreateSourceParams{
			ID:           remap(s.ID),
			CreatedAt:    createdAt,
			UpdatedAt:    updatedAt,
			Network:      s.Network,
			UserName:     s.UserName,
			UserID:       targetUserID,
			IsActive:     s.IsActive,
			SyncStatus:   s.SyncStatus,
			StatusReason: statusReason,
			LastSynced:   lastSynced,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to import source %s: %w", s.UserName, err)
		}
		result.Sources++
	}

	for _, t := range targets {
		createdAt, _ := time.Parse(timeFormat, t.CreatedAt)
		updatedAt, _ := time.Parse(timeFormat, t.UpdatedAt)
		var lastSynced sql.NullTime
		if t.LastSynced != nil {
			ts, _ := time.Parse(timeFormat, *t.LastSynced)
			lastSynced = sql.NullTime{Time: ts, Valid: true}
		}
		var statusReason sql.NullString
		if t.StatusReason != nil {
			statusReason = sql.NullString{String: *t.StatusReason, Valid: true}
		}
		var dbID sql.NullString
		if t.DbID != nil {
			dbID = sql.NullString{String: *t.DbID, Valid: true}
		}
		var hostUrl sql.NullString
		if t.HostUrl != nil {
			hostUrl = sql.NullString{String: *t.HostUrl, Valid: true}
		}
		_, err := qtx.CreateTarget(ctx, database.CreateTargetParams{
			ID:            remap(t.ID),
			CreatedAt:     createdAt,
			UpdatedAt:     updatedAt,
			TargetType:    t.TargetType,
			UserID:        targetUserID,
			DbID:          dbID,
			IsActive:      t.IsActive,
			SyncFrequency: t.SyncFrequency,
			SyncStatus:    t.SyncStatus,
			HostUrl:       hostUrl,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to import target %s: %w", t.TargetType, err)
		}
		_ = statusReason
		_ = lastSynced
		result.Targets++
	}

	for _, tk := range tokens {
		createdAt, _ := time.Parse(timeFormat, tk.CreatedAt)
		updatedAt, _ := time.Parse(timeFormat, tk.UpdatedAt)
		encToken, err := base64.StdEncoding.DecodeString(tk.EncryptedAccessToken)
		if err != nil {
			return nil, fmt.Errorf("failed to decode token: %w", err)
		}
		nonce, err := base64.StdEncoding.DecodeString(tk.Nonce)
		if err != nil {
			return nil, fmt.Errorf("failed to decode nonce: %w", err)
		}
		var sourceID uuid.NullUUID
		if tk.SourceID != nil {
			sourceID = uuid.NullUUID{UUID: remap(*tk.SourceID), Valid: true}
		}
		var targetID uuid.NullUUID
		if tk.TargetID != nil {
			targetID = uuid.NullUUID{UUID: remap(*tk.TargetID), Valid: true}
		}
		var profileID sql.NullString
		if tk.ProfileID != nil {
			profileID = sql.NullString{String: *tk.ProfileID, Valid: true}
		}
		var appData json.RawMessage
		if tk.SourceAppData != nil {
			appData = *tk.SourceAppData
		}
		_, err = qtx.CreateToken(ctx, database.CreateTokenParams{
			ID:                   remap(tk.ID),
			EncryptedAccessToken: encToken,
			Nonce:                nonce,
			CreatedAt:            createdAt,
			UpdatedAt:            updatedAt,
			SourceID:             sourceID,
			TargetID:             targetID,
			ProfileID:            profileID,
			SourceAppData:        appData,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to import token: %w", err)
		}
		result.Tokens++
	}

	for _, p := range posts {
		createdAt, _ := time.Parse(timeFormat, p.CreatedAt)
		lastSyncedAt, _ := time.Parse(timeFormat, p.LastSyncedAt)
		var content sql.NullString
		if p.Content != nil {
			content = sql.NullString{String: *p.Content, Valid: true}
		}
		_, err := qtx.CreatePost(ctx, database.CreatePostParams{
			ID:                remap(p.ID),
			CreatedAt:         createdAt,
			LastSyncedAt:      lastSyncedAt,
			SourceID:          remap(p.SourceID),
			IsArchived:        p.IsArchived,
			NetworkInternalID: p.NetworkInternalID,
			Content:           content,
			PostType:          p.PostType,
			Author:            p.Author,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to import post: %w", err)
		}
		result.Posts++
	}

	for _, r := range reactions {
		syncedAt, _ := time.Parse(timeFormat, r.SyncedAt)
		var likes, reposts, views sql.NullInt64
		if r.Likes != nil {
			likes = sql.NullInt64{Int64: *r.Likes, Valid: true}
		}
		if r.Reposts != nil {
			reposts = sql.NullInt64{Int64: *r.Reposts, Valid: true}
		}
		if r.Views != nil {
			views = sql.NullInt64{Int64: *r.Views, Valid: true}
		}
		_, err := qtx.SyncReactions(ctx, database.SyncReactionsParams{
			ID:       remap(r.ID),
			SyncedAt: syncedAt,
			PostID:   remap(r.PostID),
			Likes:    likes,
			Reposts:  reposts,
			Views:    views,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to import reaction: %w", err)
		}
		result.Reactions++
	}

	for _, tc := range tagClassifications {
		createdAt, _ := time.Parse(timeFormat, tc.CreatedAt)
		updatedAt, _ := time.Parse(timeFormat, tc.UpdatedAt)
		_, err := qtx.CreateTagClassification(ctx, database.CreateTagClassificationParams{
			ID:        remap(tc.ID),
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
			UserID:    targetUserID,
			Name:      tc.Name,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to import tag classification %s: %w", tc.Name, err)
		}
		result.TagClassifications++
	}

	for _, t := range tags {
		createdAt, _ := time.Parse(timeFormat, t.CreatedAt)
		updatedAt, _ := time.Parse(timeFormat, t.UpdatedAt)
		var classID uuid.NullUUID
		if t.ClassificationID != nil {
			classID = uuid.NullUUID{UUID: remap(*t.ClassificationID), Valid: true}
		}
		_, err := qtx.CreateTag(ctx, database.CreateTagParams{
			ID:               remap(t.ID),
			CreatedAt:        createdAt,
			UpdatedAt:        updatedAt,
			UserID:           targetUserID,
			ClassificationID: classID,
			Name:             t.Name,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to import tag %s: %w", t.Name, err)
		}
		result.Tags++
	}

	for _, pt := range postTags {
		createdAt, _ := time.Parse(timeFormat, pt.CreatedAt)
		_, err := qtx.AddTagToPost(ctx, database.AddTagToPostParams{
			ID:        remap(pt.ID),
			CreatedAt: createdAt,
			PostID:    remap(pt.PostID),
			TagID:     remap(pt.TagID),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to import post tag: %w", err)
		}
		result.PostTags++
	}

	for _, e := range exclusions {
		createdAt, _ := time.Parse(timeFormat, e.CreatedAt)
		_, err := qtx.CreateExclusion(ctx, database.CreateExclusionParams{
			ID:                remap(e.ID),
			CreatedAt:         createdAt,
			SourceID:          remap(e.SourceID),
			NetworkInternalID: e.NetworkInternalID,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to import exclusion: %w", err)
		}
		result.Exclusions++
	}

	for _, r := range redirects {
		createdAt, _ := time.Parse(timeFormat, r.CreatedAt)
		_, err := qtx.CreateRedirect(ctx, database.CreateRedirectParams{
			ID:        remap(r.ID),
			SourceID:  remap(r.SourceID),
			FromPath:  r.FromPath,
			ToPath:    r.ToPath,
			CreatedAt: createdAt,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to import redirect: %w", err)
		}
		result.Redirects++
	}

	for _, s := range sourcesStats {
		date, _ := time.Parse(timeFormat, s.Date)
		var followers, following, postsCount sql.NullInt64
		var avgLikes, avgReposts, avgViews sql.NullFloat64
		if s.FollowersCount != nil {
			followers = sql.NullInt64{Int64: *s.FollowersCount, Valid: true}
		}
		if s.FollowingCount != nil {
			following = sql.NullInt64{Int64: *s.FollowingCount, Valid: true}
		}
		if s.PostsCount != nil {
			postsCount = sql.NullInt64{Int64: *s.PostsCount, Valid: true}
		}
		if s.AverageLikes != nil {
			avgLikes = sql.NullFloat64{Float64: *s.AverageLikes, Valid: true}
		}
		if s.AverageReposts != nil {
			avgReposts = sql.NullFloat64{Float64: *s.AverageReposts, Valid: true}
		}
		if s.AverageViews != nil {
			avgViews = sql.NullFloat64{Float64: *s.AverageViews, Valid: true}
		}
		_, err := qtx.CreateSourceStat(ctx, database.CreateSourceStatParams{
			ID:             remap(s.ID),
			Date:           date,
			SourceID:       remap(s.SourceID),
			FollowersCount: followers,
			FollowingCount: following,
			PostsCount:     postsCount,
			AverageLikes:   avgLikes,
			AverageReposts: avgReposts,
			AverageViews:   avgViews,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to import sources stat: %w", err)
		}
		result.SourcesStats++
	}

	for _, s := range pageStats {
		date, _ := time.Parse(timeFormat, s.Date)
		var impressions sql.NullInt64
		if s.Impressions != nil {
			impressions = sql.NullInt64{Int64: *s.Impressions, Valid: true}
		}
		_, err := qtx.CreateAnalyticsPageStat(ctx, database.CreateAnalyticsPageStatParams{
			ID:            remap(s.ID),
			Date:          date,
			UrlPath:       s.UrlPath,
			Views:         s.Views,
			SourceID:      remap(s.SourceID),
			AnalyticsType: s.AnalyticsType,
			Impressions:   impressions,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to import page stat: %w", err)
		}
		result.PageStats++
	}

	for _, s := range siteStats {
		date, _ := time.Parse(timeFormat, s.Date)
		var impressions sql.NullInt64
		if s.Impressions != nil {
			impressions = sql.NullInt64{Int64: *s.Impressions, Valid: true}
		}
		_, err := qtx.CreateAnalyticsSiteStat(ctx, database.CreateAnalyticsSiteStatParams{
			ID:                 remap(s.ID),
			Date:               date,
			Visitors:           s.Visitors,
			AvgSessionDuration: s.AvgSessionDuration,
			SourceID:           remap(s.SourceID),
			AnalyticsType:      s.AnalyticsType,
			Impressions:        impressions,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to import site stat: %w", err)
		}
		result.SiteStats++
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return result, nil
}

func deleteAllUserData(ctx context.Context, qtx *database.Queries, userID uuid.UUID) error {
	deletions := []struct {
		name string
		fn   func(context.Context, uuid.UUID) error
	}{
		{"post tags", qtx.BackupDeletePostTagsForUser},
		{"reactions", qtx.BackupDeleteReactionsForUser},
		{"logs", qtx.BackupDeleteLogsForUser},
		{"exclusions", qtx.BackupDeleteExclusionsForUser},
		{"redirects", qtx.BackupDeleteRedirectsForUser},
		{"posts on target", qtx.BackupDeletePostsOnTargetForUser},
		{"sources on target", qtx.BackupDeleteSourcesOnTargetForUser},
		{"sources stats on target", qtx.BackupDeleteSourcesStatsOnTargetForUser},
		{"analytics page stats on target", qtx.BackupDeleteAnalyticsPageStatsOnTargetForUser},
		{"analytics site stats on target", qtx.BackupDeleteAnalyticsSiteStatsOnTargetForUser},
		{"posts", qtx.BackupDeletePostsForUser},
		{"sources stats", qtx.BackupDeleteSourcesStatsForUser},
		{"analytics page stats", qtx.BackupDeleteAnalyticsPageStatsForUser},
		{"analytics site stats", qtx.BackupDeleteAnalyticsSiteStatsForUser},
		{"tokens", qtx.BackupDeleteTokensForUser},
		{"tags", qtx.BackupDeleteTagsForUser},
		{"tag classifications", qtx.BackupDeleteTagClassificationsForUser},
		{"table mappings", qtx.BackupDeleteTableMappingsForUser},
		{"exports", qtx.BackupDeleteExportsForUser},
		{"targets", qtx.BackupDeleteTargetsForUser},
		{"sources", qtx.BackupDeleteSourcesForUser},
	}

	for _, d := range deletions {
		if err := d.fn(ctx, userID); err != nil {
			return fmt.Errorf("failed to delete %s: %w", d.name, err)
		}
	}

	return nil
}

func parseJSON(files map[string][]byte, name string, dest any) error {
	data, ok := files[name]
	if !ok {
		return fmt.Errorf("missing file: %s", name)
	}
	return json.Unmarshal(data, dest)
}
