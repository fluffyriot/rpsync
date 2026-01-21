-- name: GetActiveSourcesCount :one
SELECT COUNT(*) FROM sources where is_active = TRUE and user_id = $1;

-- name: GetActiveTargetsCount :one
SELECT COUNT(*) FROM targets where is_active = TRUE and user_id = $1;

-- name: GetTotalPostsCount :one
SELECT COUNT(*)
FROM posts
    left join sources on posts.source_id = sources.id
where
    sources.user_id = $1;

-- name: GetTotalReactions :one
SELECT
    COALESCE(SUM(likes), 0)::BIGINT AS total_likes,
    COALESCE(SUM(reposts), 0)::BIGINT AS total_shares,
    COALESCE(SUM(views), 0)::BIGINT AS total_views
FROM (
        SELECT DISTINCT
            ON (prh.post_id) prh.likes, prh.reposts, prh.views
        FROM
            posts_reactions_history prh
            left join posts p on prh.post_id = p.id
            left join sources s on p.source_id = s.id
        where
            s.user_id = $1
        ORDER BY prh.post_id, prh.synced_at DESC
    ) AS latest_reactions;

-- name: GetTotalSiteStats :one
SELECT COALESCE(SUM(visitors), 0)::BIGINT AS total_visitors
FROM
    analytics_site_stats
    left join sources on analytics_site_stats.source_id = sources.id
where
    sources.user_id = $1;

-- name: GetTotalPageViews :one
SELECT COALESCE(SUM(views), 0)::BIGINT AS total_page_views
FROM
    analytics_page_stats
    left join sources on analytics_page_stats.source_id = sources.id
where
    sources.user_id = $1;

-- name: GetAverageWebsiteSession :one
SELECT COALESCE(AVG(avg_session_duration), 0)::BIGINT AS average_website_session
FROM
    analytics_site_stats
    left join sources on analytics_site_stats.source_id = sources.id
where
    sources.user_id = $1;