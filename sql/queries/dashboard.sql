-- name: GetActiveSourcesCount :one
SELECT COUNT(*) FROM sources;

-- name: GetActiveTargetsCount :one
SELECT COUNT(*) FROM targets;

-- name: GetTotalPostsCount :one
SELECT COUNT(*) FROM posts;

-- name: GetTotalReactions :one
SELECT
    COALESCE(SUM(likes), 0)::BIGINT AS total_likes,
    COALESCE(SUM(reposts), 0)::BIGINT AS total_shares,
    COALESCE(SUM(views), 0)::BIGINT AS total_views
FROM (
        SELECT DISTINCT
            ON (post_id) likes, reposts, views
        FROM posts_reactions_history
        ORDER BY post_id, synced_at DESC
    ) AS latest_reactions;

-- name: GetTotalSiteStats :one
SELECT COALESCE(SUM(visitors), 0)::BIGINT AS total_visitors
FROM analytics_site_stats;

-- name: GetTotalPageViews :one
SELECT COALESCE(SUM(views), 0)::BIGINT AS total_page_views
FROM analytics_page_stats;

-- name: GetAverageWebsiteSession :one
SELECT COALESCE(AVG(avg_session_duration), 0)::BIGINT AS average_website_session
FROM analytics_site_stats;