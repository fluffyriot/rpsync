// SPDX-License-Identifier: AGPL-3.0-only
package sources

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/fluffyriot/rpsync/internal/authhelp"
	"github.com/fluffyriot/rpsync/internal/database"
	"github.com/fluffyriot/rpsync/internal/fetcher/common"
	"github.com/google/uuid"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	webmasters "google.golang.org/api/webmasters/v3"
)

func FetchGoogleSearchConsoleStats(dbQueries *database.Queries, sourceID uuid.UUID, encryptionKey []byte) error {
	ctx := context.Background()

	statsCheck, err := dbQueries.CountAnalyticsSiteStatsBySource(ctx, sourceID)
	if err != nil {
		log.Printf("GSC: Error checking existing stats: %v", err)
	}

	now := time.Now()
	startDate := now.AddDate(0, 0, -7).Format("2006-01-02")
	if statsCheck == 0 {
		startDate = now.AddDate(0, 0, -730).Format("2006-01-02")
	}
	endDate := now.Format("2006-01-02")

	return FetchGoogleSearchConsoleStatsWithRange(dbQueries, sourceID, encryptionKey, startDate, endDate)
}

func FetchGoogleSearchConsoleStatsWithRange(dbQueries *database.Queries, sourceID uuid.UUID, encryptionKey []byte, startDate, endDate string) error {
	ctx := context.Background()

	source, err := dbQueries.GetSourceById(ctx, sourceID)
	if err != nil {
		return fmt.Errorf("GSC: failed to get source: %w", err)
	}

	serviceAccountJSON, siteURL, _, _, err := authhelp.GetSourceToken(ctx, dbQueries, encryptionKey, sourceID)
	if err != nil {
		return fmt.Errorf("GSC: failed to get source token: %w", err)
	}

	if siteURL == "" {
		siteURL = source.UserName
	}

	domain := strings.TrimPrefix(siteURL, "sc-domain:")
	domain = strings.TrimPrefix(domain, "https://")
	domain = strings.TrimPrefix(domain, "http://")
	domain = strings.TrimPrefix(domain, "www.")
	domain = strings.TrimRight(domain, "/")
	siteURL = "sc-domain:" + domain

	log.Printf("GSC: querying site URL: %q", siteURL)

	jwtCfg, err := google.JWTConfigFromJSON([]byte(serviceAccountJSON), webmasters.WebmastersReadonlyScope)
	if err != nil {
		return fmt.Errorf("GSC: failed to parse service account credentials: %w", err)
	}

	svc, err := webmasters.NewService(ctx, option.WithTokenSource(jwtCfg.TokenSource(ctx)))
	if err != nil {
		return fmt.Errorf("GSC: failed to create Search Console client: %w", err)
	}

	if err := fetchGSCSiteStats(ctx, svc, dbQueries, sourceID, siteURL, startDate, endDate); err != nil {
		return fmt.Errorf("GSC: failed to fetch site stats: %w", err)
	}

	if err := fetchGSCPageStats(ctx, svc, dbQueries, sourceID, siteURL, startDate, endDate); err != nil {
		return fmt.Errorf("GSC: failed to fetch page stats: %w", err)
	}

	return nil
}

func fetchGSCSiteStats(ctx context.Context, svc *webmasters.Service, db *database.Queries, sourceID uuid.UUID, siteURL, startDate, endDate string) error {
	req := &webmasters.SearchAnalyticsQueryRequest{
		StartDate:  startDate,
		EndDate:    endDate,
		Dimensions: []string{"date"},
		RowLimit:   5000,
	}

	resp, err := svc.Searchanalytics.Query(siteURL, req).Do()
	if err != nil {
		return err
	}

	for _, row := range resp.Rows {
		if len(row.Keys) < 1 {
			continue
		}

		dateStr := row.Keys[0]
		parsedDate, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			log.Printf("GSC: Error parsing date %s: %v", dateStr, err)
			continue
		}

		clicks := int64(row.Clicks)
		impressions := int64(row.Impressions)

		_, err = db.CreateAnalyticsSiteStat(ctx, database.CreateAnalyticsSiteStatParams{
			ID:            uuid.New(),
			Date:          parsedDate,
			Visitors:      clicks,
			SourceID:      sourceID,
			AnalyticsType: "gsc",
			Impressions:   sql.NullInt64{Int64: impressions, Valid: true},
		})
		if err != nil {
			log.Printf("GSC: Error saving site stat for %s: %v", dateStr, err)
		}
	}

	return nil
}

func fetchGSCPageStats(ctx context.Context, svc *webmasters.Service, db *database.Queries, sourceID uuid.UUID, siteURL, startDate, endDate string) error {
	req := &webmasters.SearchAnalyticsQueryRequest{
		StartDate:  startDate,
		EndDate:    endDate,
		Dimensions: []string{"date", "page"},
		RowLimit:   5000,
	}

	resp, err := svc.Searchanalytics.Query(siteURL, req).Do()
	if err != nil {
		return err
	}

	redirects, err := db.GetRedirectsForSource(ctx, sourceID)
	if err != nil {
		log.Printf("GSC: Warning: failed to fetch redirects: %v", err)
	}
	redirectMap := make(map[string]string)
	for _, r := range redirects {
		redirectMap[common.NormalizePagePath(r.FromPath)] = common.NormalizePagePath(r.ToPath)
	}

	type PageStatKey struct {
		Date time.Time
		Path string
	}

	type PageStat struct {
		Clicks      int64
		Impressions int64
	}

	consolidated := make(map[PageStatKey]PageStat)

	for _, row := range resp.Rows {
		if len(row.Keys) < 2 {
			continue
		}

		dateStr := row.Keys[0]
		rawPath := row.Keys[1]

		parsedDate, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			log.Printf("GSC: Error parsing date %s: %v", dateStr, err)
			continue
		}

		pagePath := rawPath
		if u, err := url.Parse(rawPath); err == nil && u.Path != "" {
			pagePath = u.Path
		}
		if toPath, ok := redirectMap[pagePath]; ok {
			pagePath = toPath
		}

		pagePath = common.NormalizePagePath(pagePath)

		clicks := int64(row.Clicks)
		impressions := int64(row.Impressions)

		key := PageStatKey{
			Date: parsedDate,
			Path: pagePath,
		}

		stat := consolidated[key]
		stat.Clicks += clicks
		stat.Impressions += impressions
		consolidated[key] = stat

	}

	for key, stat := range consolidated {
		_, err = db.CreateAnalyticsPageStat(ctx, database.CreateAnalyticsPageStatParams{
			ID:            uuid.New(),
			Date:          key.Date,
			UrlPath:       key.Path,
			Views:         stat.Clicks,
			SourceID:      sourceID,
			AnalyticsType: "gsc",
			Impressions:   sql.NullInt64{Int64: stat.Impressions, Valid: true},
		})
		if err != nil {
			log.Printf("GSC: Error saving page stat for %s %s: %v", key.Date.Format("2006-01-02"), key.Path, err)
		}
	}

	return nil
}
