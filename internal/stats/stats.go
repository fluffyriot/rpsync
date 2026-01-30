package stats

import (
	"context"

	"github.com/fluffyriot/rpsync/internal/database"
	"github.com/google/uuid"
)

type ValidationPoint struct {
	Date    string `json:"date"`
	Likes   int64  `json:"likes"`
	Reposts int64  `json:"reposts"`
}

type SourceStats struct {
	SourceID uuid.UUID         `json:"source_id"`
	Network  string            `json:"network"`
	Username string            `json:"username"`
	Points   []ValidationPoint `json:"points"`
}

func GetStats(dbQueries *database.Queries, userID uuid.UUID) ([]SourceStats, error) {

	stats, err := dbQueries.GetMonthlyEngagementStats(context.Background(), userID)
	if err != nil {
		return nil, err
	}

	statsMap := make(map[uuid.UUID]*SourceStats)

	for _, row := range stats {

		if _, ok := statsMap[row.ID]; !ok {
			statsMap[row.ID] = &SourceStats{
				SourceID: row.ID,
				Network:  row.Network,
				Username: row.UserName,
				Points:   []ValidationPoint{},
			}
		}

		statsMap[row.ID].Points = append(statsMap[row.ID].Points, ValidationPoint{
			Date:    row.YearMonth,
			Likes:   row.TotalLikes,
			Reposts: row.TotalReposts,
		})
	}

	var result []SourceStats
	for _, s := range statsMap {
		result = append(result, *s)
	}

	return result, nil
}

type AnalyticsPoint struct {
	Date  string `json:"date"`
	Value int64  `json:"value"`
}

type AnalyticsSeries struct {
	Label  string           `json:"label"`
	Points []AnalyticsPoint `json:"points"`
}

func GetAnalyticsStats(dbQueries *database.Queries, userID uuid.UUID) ([]AnalyticsSeries, error) {
	ctx := context.Background()

	visitors, err := dbQueries.GetMonthlySiteVisitors(ctx, userID)
	if err != nil {
		return nil, err
	}

	views, err := dbQueries.GetMonthlyPageViews(ctx, userID)
	if err != nil {
		return nil, err
	}

	var visitorsPoints []AnalyticsPoint
	for _, v := range visitors {
		visitorsPoints = append(visitorsPoints, AnalyticsPoint{
			Date:  v.YearMonth,
			Value: v.TotalVisitors,
		})
	}

	var viewsPoints []AnalyticsPoint
	for _, v := range views {
		viewsPoints = append(viewsPoints, AnalyticsPoint{
			Date:  v.YearMonth,
			Value: v.TotalViews,
		})
	}

	series := []AnalyticsSeries{
		{
			Label:  "Website Visitors",
			Points: visitorsPoints,
		},
		{
			Label:  "Page Views",
			Points: viewsPoints,
		},
	}

	return series, nil
}
