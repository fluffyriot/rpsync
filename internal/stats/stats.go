package stats

import (
	"context"
	"time"

	"github.com/fluffyriot/rpsync/internal/database"
	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
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
			Likes:   int64(row.TotalLikes),
			Reposts: int64(row.TotalReposts),
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
			Value: int64(v.TotalVisitors),
		})
	}

	var viewsPoints []AnalyticsPoint
	for _, v := range views {
		viewsPoints = append(viewsPoints, AnalyticsPoint{
			Date:  v.YearMonth,
			Value: int64(v.TotalViews),
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

type ChartPoint struct {
	Date  string `json:"date"`
	Value int64  `json:"value"`
}

type SummaryChart struct {
	CurrentPeriod  []ChartPoint `json:"current_period"`
	PreviousPeriod []ChartPoint `json:"previous_period"`
}

type DashboardSummary struct {
	Engagement SummaryChart `json:"engagement"`
	Followers  SummaryChart `json:"followers"`
}

func GetDashboardSummary(dbQueries *database.Queries, userID uuid.UUID) (*DashboardSummary, error) {
	ctx := context.Background()
	now := time.Now()
	startDate := now.AddDate(0, 0, -13)

	var (
		engStats      []database.GetTotalDailyEngagementStatsRow
		followerStats []database.GetTotalDailyFollowerStatsRow
	)

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		var err error
		engStats, err = dbQueries.GetTotalDailyEngagementStats(ctx, database.GetTotalDailyEngagementStatsParams{
			UserID:  userID,
			Column2: startDate,
			Column3: now,
		})
		return err
	})

	g.Go(func() error {
		var err error
		followerStats, err = dbQueries.GetTotalDailyFollowerStats(ctx, database.GetTotalDailyFollowerStatsParams{
			UserID:  userID,
			Column2: startDate,
			Column3: now,
		})
		return err
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	summary := &DashboardSummary{
		Engagement: SummaryChart{
			CurrentPeriod:  make([]ChartPoint, 0),
			PreviousPeriod: make([]ChartPoint, 0),
		},
		Followers: SummaryChart{
			CurrentPeriod:  make([]ChartPoint, 0),
			PreviousPeriod: make([]ChartPoint, 0),
		},
	}

	for i, stat := range engStats {
		point := ChartPoint{
			Date:  stat.PeriodDate.Format("2006-01-02"),
			Value: int64(stat.TotalEngagement),
		}
		if i < 7 {
			summary.Engagement.PreviousPeriod = append(summary.Engagement.PreviousPeriod, point)
		} else {
			summary.Engagement.CurrentPeriod = append(summary.Engagement.CurrentPeriod, point)
		}
	}

	for i, stat := range followerStats {
		point := ChartPoint{
			Date:  stat.PeriodDate.Format("2006-01-02"),
			Value: int64(stat.TotalFollowers),
		}
		if i < 7 {
			summary.Followers.PreviousPeriod = append(summary.Followers.PreviousPeriod, point)
		} else {
			summary.Followers.CurrentPeriod = append(summary.Followers.CurrentPeriod, point)
		}
	}

	return summary, nil
}
