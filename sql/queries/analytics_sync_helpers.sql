-- name: GetSiteStatsOnTarget :many
SELECT * FROM analytics_site_stats_on_target WHERE target_id = $1;

-- name: GetPageStatsOnTarget :many
SELECT * FROM analytics_page_stats_on_target WHERE target_id = $1;

-- name: DeleteAnalyticsSiteStatOnTarget :exec
DELETE FROM analytics_site_stats_on_target WHERE id = $1;

-- name: DeleteAnalyticsPageStatOnTarget :exec
DELETE FROM analytics_page_stats_on_target WHERE id = $1;