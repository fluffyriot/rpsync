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

-- name: GetTotalDailyEngagementStats :many
SELECT
    calendar.date::date as period_date,
    COALESCE(
        (
            SELECT SUM(
                    COALESCE(likes, 0) + COALESCE(reposts, 0)
                )
            FROM (
                    SELECT DISTINCT
                        ON (prh.post_id) prh.likes, prh.reposts
                    FROM
                        posts_reactions_history prh
                        JOIN posts p ON prh.post_id = p.id
                        JOIN sources s ON p.source_id = s.id
                    WHERE
                        s.user_id = $1
                        AND prh.synced_at < calendar.date + INTERVAL '1 day'
                    ORDER BY prh.post_id, prh.synced_at DESC
                ) as distinct_posts
        ),
        0
    )::BIGINT as total_engagement
FROM generate_series(
        date_trunc('day', $2::timestamp), date_trunc('day', $3::timestamp), '1 day'::interval
    ) as calendar (date)
ORDER BY calendar.date ASC;

-- name: GetTotalDailyFollowerStats :many
SELECT
    calendar.date::date as period_date,
    COALESCE(
        (
            SELECT SUM(COALESCE(followers_count, 0))
            FROM (
                    SELECT DISTINCT
                        ON (ss.source_id) ss.followers_count
                    FROM sources_stats ss
                        JOIN sources s ON ss.source_id = s.id
                    WHERE
                        s.user_id = $1
                        AND ss.date < calendar.date + INTERVAL '1 day'
                    ORDER BY ss.source_id, ss.date DESC
                ) as distinct_sources
        ),
        0
    )::BIGINT as total_followers
FROM generate_series(
        date_trunc('day', $2::timestamp), date_trunc('day', $3::timestamp), '1 day'::interval
    ) as calendar (date)
ORDER BY calendar.date ASC;

-- name: GetTopSources :many
SELECT
    s.id,
    s.user_name,
    s.network,
    SUM(
        COALESCE(prh.likes, 0) + COALESCE(prh.reposts, 0)
    )::BIGINT AS total_interactions,
    COALESCE(
        (
            SELECT ss.followers_count
            FROM sources_stats ss
            WHERE
                ss.source_id = s.id
            ORDER BY ss.date DESC
            LIMIT 1
        ),
        0
    )::BIGINT AS followers_count
FROM sources s
    LEFT JOIN posts p ON s.id = p.source_id
    LEFT JOIN (
        SELECT DISTINCT
            ON (post_id) post_id, likes, reposts
        FROM posts_reactions_history
        ORDER BY post_id, synced_at DESC
    ) prh ON p.id = prh.post_id
WHERE
    s.user_id = $1
    AND s.is_active = TRUE
    AND NOT s.network in ('Google Analytics')
GROUP BY
    s.id
ORDER BY total_interactions DESC
LIMIT 3;