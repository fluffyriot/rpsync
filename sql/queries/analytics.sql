-- name: CreateAnalyticsSiteStat :one
INSERT INTO
    analytics_site_stats (
        id,
        date,
        visitors,
        avg_session_duration,
        source_id
    )
VALUES ($1, $2, $3, $4, $5)
RETURNING
    *;

-- name: CreateAnalyticsPageStat :one
INSERT INTO
    analytics_page_stats (
        id,
        date,
        url_path,
        views,
        source_id
    )
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (source_id, date, url_path) DO
UPDATE
SET
    views = $4
RETURNING
    *;

-- name: GetAnalyticsSiteStatsBySource :many
SELECT *
FROM analytics_site_stats
WHERE
    source_id = $1
ORDER BY date DESC;

-- name: GetAnalyticsPageStatsBySource :many
SELECT *
FROM analytics_page_stats
WHERE
    source_id = $1
ORDER BY date DESC;

-- name: CheckCountOfAnalyticsSiteStatsForUser :one
SELECT COUNT(*)
FROM
    analytics_site_stats s
    join sources src on s.source_id = src.id
where
    src.user_id = $1;

-- name: CheckCountOfAnalyticsPageStatsForUser :one
SELECT COUNT(*)
FROM
    analytics_page_stats s
    join sources src on s.source_id = src.id
where
    src.user_id = $1;

-- name: GetAnalyticsSiteStatsBySourceAndRange :many
SELECT *
FROM analytics_site_stats
WHERE
    source_id = $1
    AND date >= $2
    AND date <= $3
ORDER BY date DESC;

-- name: AddAnalyticsSiteStatToTarget :one
INSERT INTO
    analytics_site_stats_on_target (
        id,
        synced_at,
        stat_id,
        target_id,
        target_record_id
    )
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (stat_id, target_id) DO
UPDATE
SET
    synced_at = $2,
    target_record_id = $5
RETURNING
    *;

-- name: AddAnalyticsPageStatToTarget :one
INSERT INTO
    analytics_page_stats_on_target (
        id,
        synced_at,
        stat_id,
        target_id,
        target_record_id
    )
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (stat_id, target_id) DO
UPDATE
SET
    synced_at = $2,
    target_record_id = $5
RETURNING
    *;

-- name: GetUnsyncedSiteStatsForTarget :many
SELECT s.*
FROM
    analytics_site_stats s
    LEFT JOIN analytics_site_stats_on_target map ON s.id = map.stat_id
    AND map.target_id = $1
WHERE
    map.id IS NULL
    AND s.source_id = $2;

-- name: GetUnsyncedPageStatsForTarget :many
SELECT s.*
FROM
    analytics_page_stats s
    LEFT JOIN analytics_page_stats_on_target map ON s.id = map.stat_id
    AND map.target_id = $1
WHERE
    map.id IS NULL
    AND s.source_id = $2;

-- name: GetAllSiteStatsWithTargetInfo :many
SELECT s.*, map.target_record_id
FROM
    analytics_site_stats s
    LEFT JOIN analytics_site_stats_on_target map ON s.id = map.stat_id
    AND map.target_id = $1
WHERE
    s.source_id = $2;

-- name: GetAllPageStatsWithTargetInfo :many
SELECT s.*, map.target_record_id
FROM
    analytics_page_stats s
    LEFT JOIN analytics_page_stats_on_target map ON s.id = map.stat_id
    AND map.target_id = $1
WHERE
    s.source_id = $2;

-- name: GetAllAnalyticsSiteStatsForUser :many
SELECT
    s.*,
    src.network as source_network,
    src.user_name as source_user_name
FROM
    analytics_site_stats s
    JOIN sources src ON s.source_id = src.id
WHERE
    src.user_id = $1
ORDER BY s.date DESC;

-- name: GetAllAnalyticsPageStatsForUser :many
SELECT
    s.*,
    src.network as source_network,
    src.user_name as source_user_name
FROM
    analytics_page_stats s
    JOIN sources src ON s.source_id = src.id
WHERE
    src.user_id = $1
ORDER BY s.date DESC;

-- name: DeleteAnalyticsPageStat :exec
DELETE FROM analytics_page_stats WHERE id = $1;

-- name: DeleteAnalyticsPageStatsByPathAndSource :exec
DELETE FROM analytics_page_stats
WHERE
    source_id = $1
    AND url_path = $2;

-- name: UpdateAnalyticsPageStatPath :exec
UPDATE analytics_page_stats SET url_path = $2 WHERE id = $1;