package fetcher

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/fluffyriot/rpsync/internal/authhelp"
	"github.com/fluffyriot/rpsync/internal/database"
	"github.com/google/uuid"
	"golang.org/x/oauth2/google"
	analyticsdata "google.golang.org/api/analyticsdata/v1beta"
	"google.golang.org/api/option"
)

func FetchGoogleAnalyticsStats(dbQueries *database.Queries, sourceID uuid.UUID, encryptionKey []byte) error {
	ctx := context.Background()

	statsCheck, err := dbQueries.CheckCountOfAnalyticsSiteStatsForUser(ctx, sourceID)
	if err != nil {
		log.Printf("Error checking existing stats: %v", err)
	}

	startDate := "7daysAgo"
	if statsCheck == 0 {
		startDate = "730daysAgo"
	}
	endDate := "today"

	return FetchGoogleAnalyticsStatsWithRange(dbQueries, sourceID, encryptionKey, startDate, endDate)
}

func FetchGoogleAnalyticsStatsWithRange(dbQueries *database.Queries, sourceID uuid.UUID, encryptionKey []byte, startDate, endDate string) error {
	ctx := context.Background()

	source, err := dbQueries.GetSourceById(ctx, sourceID)
	if err != nil {
		return fmt.Errorf("failed to get source: %w", err)
	}

	token, propertyID, _, err := authhelp.GetSourceToken(ctx, dbQueries, encryptionKey, sourceID)
	if err != nil {
		return fmt.Errorf("failed to get source token: %w", err)
	}

	if propertyID == "" {
		propertyID = source.UserName
	}

	creds, err := google.CredentialsFromJSON(ctx, []byte(token), analyticsdata.AnalyticsReadonlyScope)
	if err != nil {
		return fmt.Errorf("failed to parse credentials: %w", err)
	}

	client, err := analyticsdata.NewService(ctx, option.WithCredentials(creds))
	if err != nil {
		return fmt.Errorf("failed to create analytics client: %w", err)
	}

	if err := fetchAndSaveSiteStats(ctx, client, dbQueries, sourceID, propertyID, startDate, endDate); err != nil {
		return fmt.Errorf("failed to fetch site stats: %w", err)
	}

	if err := fetchAndSavePageStats(ctx, client, dbQueries, sourceID, propertyID, startDate, endDate); err != nil {
		return fmt.Errorf("failed to fetch page stats: %w", err)
	}

	return nil
}

func fetchAndSaveSiteStats(ctx context.Context, svc *analyticsdata.Service, db *database.Queries, sourceID uuid.UUID, propertyID, startDate, endDate string) error {
	req := &analyticsdata.RunReportRequest{
		Property: "properties/" + propertyID,
		DateRanges: []*analyticsdata.DateRange{
			{StartDate: startDate, EndDate: endDate},
		},
		Dimensions: []*analyticsdata.Dimension{
			{Name: "date"},
		},
		Metrics: []*analyticsdata.Metric{
			{Name: "activeUsers"},
			{Name: "averageSessionDuration"},
		},
	}

	resp, err := svc.Properties.RunReport(req.Property, req).Do()
	if err != nil {
		return err
	}

	for _, row := range resp.Rows {
		if len(row.DimensionValues) < 1 || len(row.MetricValues) < 2 {
			continue
		}

		dateStr := row.DimensionValues[0].Value
		visitors := row.MetricValues[0].Value
		avgDuration := row.MetricValues[1].Value

		parsedDate, err := time.Parse("20060102", dateStr)
		if err != nil {
			log.Printf("Error parsing date %s: %v", dateStr, err)
			continue
		}

		var visitorsInt int
		fmt.Sscanf(visitors, "%d", &visitorsInt)

		var durationFloat float64
		fmt.Sscanf(avgDuration, "%f", &durationFloat)

		_, err = db.CreateAnalyticsSiteStat(ctx, database.CreateAnalyticsSiteStatParams{
			ID:                 uuid.New(),
			Date:               parsedDate,
			Visitors:           int32(visitorsInt),
			AvgSessionDuration: durationFloat,
			SourceID:           sourceID,
		})
		if err != nil {
			log.Printf("Error saving site stat for %s: %v", dateStr, err)
		}
	}
	return nil
}

func fetchAndSavePageStats(ctx context.Context, svc *analyticsdata.Service, db *database.Queries, sourceID uuid.UUID, propertyID, startDate, endDate string) error {
	req := &analyticsdata.RunReportRequest{
		Property: "properties/" + propertyID,
		DateRanges: []*analyticsdata.DateRange{
			{StartDate: startDate, EndDate: endDate},
		},
		Dimensions: []*analyticsdata.Dimension{
			{Name: "date"},
			{Name: "pagePath"},
		},
		Metrics: []*analyticsdata.Metric{
			{Name: "screenPageViews"},
		},
	}

	resp, err := svc.Properties.RunReport(req.Property, req).Do()
	if err != nil {
		return err
	}

	redirects, err := db.GetRedirectsForSource(ctx, sourceID)
	if err != nil {
		log.Printf("Warning: failed to fetch redirects for source %s: %v", sourceID, err)
	}
	redirectMap := make(map[string]string)
	for _, r := range redirects {
		redirectMap[r.FromPath] = r.ToPath
	}

	type PageStatKey struct {
		Date time.Time
		Path string
	}
	consolidatedStats := make(map[PageStatKey]int32)

	for _, row := range resp.Rows {
		if len(row.DimensionValues) < 2 || len(row.MetricValues) < 1 {
			continue
		}

		dateStr := row.DimensionValues[0].Value
		pagePath := row.DimensionValues[1].Value
		views := row.MetricValues[0].Value

		parsedDate, err := time.Parse("20060102", dateStr)
		if err != nil {
			log.Printf("Error parsing date %s: %v", dateStr, err)
			continue
		}

		var viewsInt int
		fmt.Sscanf(views, "%d", &viewsInt)

		if toPath, ok := redirectMap[pagePath]; ok {
			pagePath = toPath
		}

		key := PageStatKey{Date: parsedDate, Path: pagePath}
		consolidatedStats[key] += int32(viewsInt)
	}

	for key, views := range consolidatedStats {
		_, err = db.CreateAnalyticsPageStat(ctx, database.CreateAnalyticsPageStatParams{
			ID:       uuid.New(),
			Date:     key.Date,
			UrlPath:  key.Path,
			Views:    views,
			SourceID: sourceID,
		})
		if err != nil {
			log.Printf("Error saving page stat for %s %s: %v", key.Date.Format("2006-01-02"), key.Path, err)
		}
	}
	return nil
}
