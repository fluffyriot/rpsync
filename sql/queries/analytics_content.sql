-- name: GetHashtagAnalytics :many
SELECT lower(matches [1]) as tag,
    count(*) as usage_count,
    COALESCE(AVG(prh.likes), 0)::BIGINT as avg_likes
FROM (
        SELECT regexp_matches(content, '#([[:alnum:]_]+)', 'g') as matches,
            posts.id as post_id
        FROM posts
            JOIN sources s ON posts.source_id = s.id
        WHERE s.user_id = $1
            AND content IS NOT NULL
    ) t
    LEFT JOIN (
        SELECT DISTINCT ON (post_id) post_id,
            likes
        FROM posts_reactions_history
        ORDER BY post_id,
            synced_at DESC
    ) prh ON t.post_id = prh.post_id
GROUP BY tag
ORDER BY avg_likes DESC
LIMIT 20;
-- name: GetHashtagAnalyticsViews :many
SELECT lower(matches [1]) as tag,
    count(*) as usage_count,
    COALESCE(AVG(prh.views), 0)::BIGINT as avg_views
FROM (
        SELECT regexp_matches(content, '#([[:alnum:]_]+)', 'g') as matches,
            posts.id as post_id
        FROM posts
            JOIN sources s ON posts.source_id = s.id
        WHERE s.user_id = $1
            AND content IS NOT NULL
    ) t
    LEFT JOIN (
        SELECT DISTINCT ON (post_id) post_id,
            views
        FROM posts_reactions_history
        ORDER BY post_id,
            synced_at DESC
    ) prh ON t.post_id = prh.post_id
GROUP BY tag
ORDER BY avg_views DESC
LIMIT 20;
-- name: GetPerformanceDeviationPositive :many
WITH SourceAverages AS (
    SELECT p.source_id,
        AVG(
            COALESCE(prh.likes, 0) + COALESCE(prh.reposts, 0)
        ) as avg_engagement
    FROM posts p
        LEFT JOIN (
            SELECT DISTINCT ON (post_id) post_id,
                likes,
                reposts
            FROM posts_reactions_history
            ORDER BY post_id,
                synced_at DESC
        ) prh ON p.id = prh.post_id
    GROUP BY p.source_id
)
SELECT p.id,
    p.network_internal_id,
    COALESCE(p.content, '')::TEXT as content,
    p.created_at,
    p.author,
    s.network,
    COALESCE(prh.likes, 0)::BIGINT as likes,
    COALESCE(prh.reposts, 0)::BIGINT as reposts,
    (
        sa.avg_engagement * LEAST(
            1.0,
            EXTRACT(
                EPOCH
                FROM (NOW() - p.created_at)
            ) / 86400.0
        )
    )::FLOAT as expected_engagement
FROM posts p
    JOIN sources s ON p.source_id = s.id
    JOIN SourceAverages sa ON p.source_id = sa.source_id
    LEFT JOIN (
        SELECT DISTINCT ON (post_id) post_id,
            likes,
            reposts
        FROM posts_reactions_history
        ORDER BY post_id,
            synced_at DESC
    ) prh ON p.id = prh.post_id
WHERE s.user_id = $1
    AND p.post_type NOT IN ('tag', 'repost', 'quote')
    AND (COALESCE(prh.likes, 0) + COALESCE(prh.reposts, 0)) > (
        sa.avg_engagement * LEAST(
            1.0,
            EXTRACT(EPOCH FROM (NOW() - p.created_at)) / 86400.0
        )
    )
    AND p.created_at > NOW() - INTERVAL '90 days'
ORDER BY (
        (COALESCE(prh.likes, 0) + COALESCE(prh.reposts, 0)) - (
            sa.avg_engagement * LEAST(
                1.0,
                EXTRACT(EPOCH FROM (NOW() - p.created_at)) / 86400.0
            )
        )
    ) DESC
LIMIT 7;
-- name: GetPerformanceDeviationPositiveViews :many
WITH SourceAverages AS (
    SELECT p.source_id,
        AVG(COALESCE(prh.views, 0)) as avg_engagement
    FROM posts p
        LEFT JOIN (
            SELECT DISTINCT ON (post_id) post_id,
                views
            FROM posts_reactions_history
            ORDER BY post_id,
                synced_at DESC
        ) prh ON p.id = prh.post_id
    GROUP BY p.source_id
)
SELECT p.id,
    p.network_internal_id,
    COALESCE(p.content, '')::TEXT as content,
    p.created_at,
    p.author,
    s.network,
    COALESCE(prh.views, 0)::BIGINT as views,
    (
        sa.avg_engagement * LEAST(
            1.0,
            EXTRACT(
                EPOCH
                FROM (NOW() - p.created_at)
            ) / 86400.0
        )
    )::FLOAT as expected_engagement
FROM posts p
    JOIN sources s ON p.source_id = s.id
    JOIN SourceAverages sa ON p.source_id = sa.source_id
    LEFT JOIN (
        SELECT DISTINCT ON (post_id) post_id,
            views
        FROM posts_reactions_history
        ORDER BY post_id,
            synced_at DESC
    ) prh ON p.id = prh.post_id
WHERE s.user_id = $1
    AND p.post_type NOT IN ('tag', 'repost', 'quote')
    AND COALESCE(prh.views, 0) > (
        sa.avg_engagement * LEAST(
            1.0,
            EXTRACT(EPOCH FROM (NOW() - p.created_at)) / 86400.0
        )
    )
    AND p.created_at > NOW() - INTERVAL '90 days'
ORDER BY (
        COALESCE(prh.views, 0) - (
            sa.avg_engagement * LEAST(
                1.0,
                EXTRACT(EPOCH FROM (NOW() - p.created_at)) / 86400.0
            )
        )
    ) DESC
LIMIT 7;
-- name: GetPerformanceDeviationNegative :many
WITH SourceAverages AS (
    SELECT p.source_id,
        AVG(
            COALESCE(prh.likes, 0) + COALESCE(prh.reposts, 0)
        ) as avg_engagement
    FROM posts p
        LEFT JOIN (
            SELECT DISTINCT ON (post_id) post_id,
                likes,
                reposts
            FROM posts_reactions_history
            ORDER BY post_id,
                synced_at DESC
        ) prh ON p.id = prh.post_id
    GROUP BY p.source_id
)
SELECT p.id,
    p.network_internal_id,
    COALESCE(p.content, '')::TEXT as content,
    p.created_at,
    p.author,
    s.network,
    COALESCE(prh.likes, 0)::BIGINT as likes,
    COALESCE(prh.reposts, 0)::BIGINT as reposts,
    (
        sa.avg_engagement * LEAST(
            1.0,
            EXTRACT(
                EPOCH
                FROM (NOW() - p.created_at)
            ) / 86400.0
        )
    )::FLOAT as expected_engagement
FROM posts p
    JOIN sources s ON p.source_id = s.id
    JOIN SourceAverages sa ON p.source_id = sa.source_id
    LEFT JOIN (
        SELECT DISTINCT ON (post_id) post_id,
            likes,
            reposts
        FROM posts_reactions_history
        ORDER BY post_id,
            synced_at DESC
    ) prh ON p.id = prh.post_id
WHERE s.user_id = $1
    AND p.post_type NOT IN ('tag', 'repost', 'quote')
    AND (COALESCE(prh.likes, 0) + COALESCE(prh.reposts, 0)) < (
        sa.avg_engagement * LEAST(
            1.0,
            EXTRACT(EPOCH FROM (NOW() - p.created_at)) / 86400.0
        )
    )
    AND p.created_at > NOW() - INTERVAL '90 days'
ORDER BY (
        (COALESCE(prh.likes, 0) + COALESCE(prh.reposts, 0)) - (
            sa.avg_engagement * LEAST(
                1.0,
                EXTRACT(EPOCH FROM (NOW() - p.created_at)) / 86400.0
            )
        )
    ) ASC
LIMIT 7;
-- name: GetPerformanceDeviationNegativeViews :many
WITH SourceAverages AS (
    SELECT p.source_id,
        AVG(COALESCE(prh.views, 0)) as avg_engagement
    FROM posts p
        LEFT JOIN (
            SELECT DISTINCT ON (post_id) post_id,
                views
            FROM posts_reactions_history
            ORDER BY post_id,
                synced_at DESC
        ) prh ON p.id = prh.post_id
    GROUP BY p.source_id
)
SELECT p.id,
    p.network_internal_id,
    COALESCE(p.content, '')::TEXT as content,
    p.created_at,
    p.author,
    s.network,
    COALESCE(prh.views, 0)::BIGINT as views,
    (
        sa.avg_engagement * LEAST(
            1.0,
            EXTRACT(
                EPOCH
                FROM (NOW() - p.created_at)
            ) / 86400.0
        )
    )::FLOAT as expected_engagement
FROM posts p
    JOIN sources s ON p.source_id = s.id
    JOIN SourceAverages sa ON p.source_id = sa.source_id
    LEFT JOIN (
        SELECT DISTINCT ON (post_id) post_id,
            views
        FROM posts_reactions_history
        ORDER BY post_id,
            synced_at DESC
    ) prh ON p.id = prh.post_id
WHERE s.user_id = $1
    AND p.post_type NOT IN ('tag', 'repost', 'quote')
    AND COALESCE(prh.views, 0) < (
        sa.avg_engagement * LEAST(
            1.0,
            EXTRACT(EPOCH FROM (NOW() - p.created_at)) / 86400.0
        )
    )
    AND p.created_at > NOW() - INTERVAL '90 days'
ORDER BY (
        COALESCE(prh.views, 0) - (
            sa.avg_engagement * LEAST(
                1.0,
                EXTRACT(EPOCH FROM (NOW() - p.created_at)) / 86400.0
            )
        )
    ) ASC
LIMIT 7;
-- name: GetEngagementVelocityData :many
SELECT prh.post_id,
    prh.synced_at as history_synced_at,
    COALESCE(prh.likes, 0)::BIGINT as likes,
    COALESCE(prh.reposts, 0)::BIGINT as reposts,
    COALESCE(prh.views, 0)::BIGINT as views,
    p.created_at as post_created_at,
    COALESCE(p.content, '')::TEXT as content,
    p.author,
    p.network_internal_id,
    s.network
FROM posts_reactions_history prh
    JOIN posts p ON prh.post_id = p.id
    JOIN sources s ON p.source_id = s.id
    JOIN (
        SELECT post_id,
            MIN(synced_at) as first_synced
        FROM posts_reactions_history
        GROUP BY post_id
    ) first_sync ON p.id = first_sync.post_id
WHERE s.user_id = $1
    AND p.created_at > NOW() - INTERVAL '30 days'
    AND p.post_type NOT IN ('tag', 'repost', 'quote')
    AND DATE(p.created_at) = DATE(first_sync.first_synced)
ORDER BY prh.post_id,
    prh.synced_at ASC;
-- name: GetHashtagAnalyticsFiltered :many
SELECT lower(matches [1]) as tag,
    count(*) as usage_count,
    COALESCE(AVG(prh.likes), 0)::BIGINT as avg_likes
FROM (
        SELECT regexp_matches(content, '#([[:alnum:]_]+)', 'g') as matches,
            posts.id as post_id
        FROM posts
            JOIN sources s ON posts.source_id = s.id
        WHERE s.user_id = @user_id
            AND content IS NOT NULL
            AND (sqlc.narg('start_date')::date IS NULL OR posts.created_at >= sqlc.narg('start_date')::date)
            AND (sqlc.narg('end_date')::date IS NULL OR posts.created_at < sqlc.narg('end_date')::date + INTERVAL '1 day')
            AND (array_length(@post_types::text[], 1) IS NULL OR posts.post_type = ANY(@post_types::text[]))
            AND (array_length(@tag_ids::uuid[], 1) IS NULL OR posts.id IN (SELECT post_id FROM post_tags WHERE tag_id = ANY(@tag_ids::uuid[])))
    ) t
    LEFT JOIN (
        SELECT DISTINCT ON (post_id) post_id,
            likes
        FROM posts_reactions_history
        ORDER BY post_id,
            synced_at DESC
    ) prh ON t.post_id = prh.post_id
GROUP BY tag
ORDER BY avg_likes DESC
LIMIT 20;
-- name: GetHashtagAnalyticsViewsFiltered :many
SELECT lower(matches [1]) as tag,
    count(*) as usage_count,
    COALESCE(AVG(prh.views), 0)::BIGINT as avg_views
FROM (
        SELECT regexp_matches(content, '#([[:alnum:]_]+)', 'g') as matches,
            posts.id as post_id
        FROM posts
            JOIN sources s ON posts.source_id = s.id
        WHERE s.user_id = @user_id
            AND content IS NOT NULL
            AND (sqlc.narg('start_date')::date IS NULL OR posts.created_at >= sqlc.narg('start_date')::date)
            AND (sqlc.narg('end_date')::date IS NULL OR posts.created_at < sqlc.narg('end_date')::date + INTERVAL '1 day')
            AND (array_length(@post_types::text[], 1) IS NULL OR posts.post_type = ANY(@post_types::text[]))
            AND (array_length(@tag_ids::uuid[], 1) IS NULL OR posts.id IN (SELECT post_id FROM post_tags WHERE tag_id = ANY(@tag_ids::uuid[])))
    ) t
    LEFT JOIN (
        SELECT DISTINCT ON (post_id) post_id,
            views
        FROM posts_reactions_history
        ORDER BY post_id,
            synced_at DESC
    ) prh ON t.post_id = prh.post_id
GROUP BY tag
ORDER BY avg_views DESC
LIMIT 20;
-- name: GetPerformanceDeviationPositiveFiltered :many
WITH SourceAverages AS (
    SELECT p.source_id,
        AVG(
            COALESCE(prh.likes, 0) + COALESCE(prh.reposts, 0)
        ) as avg_engagement
    FROM posts p
        LEFT JOIN (
            SELECT DISTINCT ON (post_id) post_id,
                likes,
                reposts
            FROM posts_reactions_history
            ORDER BY post_id,
                synced_at DESC
        ) prh ON p.id = prh.post_id
    GROUP BY p.source_id
)
SELECT p.id,
    p.network_internal_id,
    COALESCE(p.content, '')::TEXT as content,
    p.created_at,
    p.author,
    s.network,
    COALESCE(prh.likes, 0)::BIGINT as likes,
    COALESCE(prh.reposts, 0)::BIGINT as reposts,
    (
        sa.avg_engagement * LEAST(
            1.0,
            EXTRACT(
                EPOCH
                FROM (NOW() - p.created_at)
            ) / 86400.0
        )
    )::FLOAT as expected_engagement
FROM posts p
    JOIN sources s ON p.source_id = s.id
    JOIN SourceAverages sa ON p.source_id = sa.source_id
    LEFT JOIN (
        SELECT DISTINCT ON (post_id) post_id,
            likes,
            reposts
        FROM posts_reactions_history
        ORDER BY post_id,
            synced_at DESC
    ) prh ON p.id = prh.post_id
WHERE s.user_id = @user_id
    AND p.post_type NOT IN ('tag', 'repost', 'quote')
    AND (COALESCE(prh.likes, 0) + COALESCE(prh.reposts, 0)) > (
        sa.avg_engagement * LEAST(
            1.0,
            EXTRACT(EPOCH FROM (NOW() - p.created_at)) / 86400.0
        )
    )
    AND p.created_at > NOW() - INTERVAL '90 days'
    AND (sqlc.narg('start_date')::date IS NULL OR p.created_at >= sqlc.narg('start_date')::date)
    AND (sqlc.narg('end_date')::date IS NULL OR p.created_at < sqlc.narg('end_date')::date + INTERVAL '1 day')
    AND (array_length(@post_types::text[], 1) IS NULL OR p.post_type = ANY(@post_types::text[]))
    AND (array_length(@tag_ids::uuid[], 1) IS NULL OR p.id IN (SELECT post_id FROM post_tags WHERE tag_id = ANY(@tag_ids::uuid[])))
ORDER BY (
        (COALESCE(prh.likes, 0) + COALESCE(prh.reposts, 0)) - (
            sa.avg_engagement * LEAST(
                1.0,
                EXTRACT(EPOCH FROM (NOW() - p.created_at)) / 86400.0
            )
        )
    ) DESC
LIMIT 7;
-- name: GetPerformanceDeviationPositiveViewsFiltered :many
WITH SourceAverages AS (
    SELECT p.source_id,
        AVG(COALESCE(prh.views, 0)) as avg_engagement
    FROM posts p
        LEFT JOIN (
            SELECT DISTINCT ON (post_id) post_id,
                views
            FROM posts_reactions_history
            ORDER BY post_id,
                synced_at DESC
        ) prh ON p.id = prh.post_id
    GROUP BY p.source_id
)
SELECT p.id,
    p.network_internal_id,
    COALESCE(p.content, '')::TEXT as content,
    p.created_at,
    p.author,
    s.network,
    COALESCE(prh.views, 0)::BIGINT as views,
    (
        sa.avg_engagement * LEAST(
            1.0,
            EXTRACT(
                EPOCH
                FROM (NOW() - p.created_at)
            ) / 86400.0
        )
    )::FLOAT as expected_engagement
FROM posts p
    JOIN sources s ON p.source_id = s.id
    JOIN SourceAverages sa ON p.source_id = sa.source_id
    LEFT JOIN (
        SELECT DISTINCT ON (post_id) post_id,
            views
        FROM posts_reactions_history
        ORDER BY post_id,
            synced_at DESC
    ) prh ON p.id = prh.post_id
WHERE s.user_id = @user_id
    AND p.post_type NOT IN ('tag', 'repost', 'quote')
    AND COALESCE(prh.views, 0) > (
        sa.avg_engagement * LEAST(
            1.0,
            EXTRACT(EPOCH FROM (NOW() - p.created_at)) / 86400.0
        )
    )
    AND p.created_at > NOW() - INTERVAL '90 days'
    AND (sqlc.narg('start_date')::date IS NULL OR p.created_at >= sqlc.narg('start_date')::date)
    AND (sqlc.narg('end_date')::date IS NULL OR p.created_at < sqlc.narg('end_date')::date + INTERVAL '1 day')
    AND (array_length(@post_types::text[], 1) IS NULL OR p.post_type = ANY(@post_types::text[]))
    AND (array_length(@tag_ids::uuid[], 1) IS NULL OR p.id IN (SELECT post_id FROM post_tags WHERE tag_id = ANY(@tag_ids::uuid[])))
ORDER BY (
        COALESCE(prh.views, 0) - (
            sa.avg_engagement * LEAST(
                1.0,
                EXTRACT(EPOCH FROM (NOW() - p.created_at)) / 86400.0
            )
        )
    ) DESC
LIMIT 7;
-- name: GetPerformanceDeviationNegativeFiltered :many
WITH SourceAverages AS (
    SELECT p.source_id,
        AVG(
            COALESCE(prh.likes, 0) + COALESCE(prh.reposts, 0)
        ) as avg_engagement
    FROM posts p
        LEFT JOIN (
            SELECT DISTINCT ON (post_id) post_id,
                likes,
                reposts
            FROM posts_reactions_history
            ORDER BY post_id,
                synced_at DESC
        ) prh ON p.id = prh.post_id
    GROUP BY p.source_id
)
SELECT p.id,
    p.network_internal_id,
    COALESCE(p.content, '')::TEXT as content,
    p.created_at,
    p.author,
    s.network,
    COALESCE(prh.likes, 0)::BIGINT as likes,
    COALESCE(prh.reposts, 0)::BIGINT as reposts,
    (
        sa.avg_engagement * LEAST(
            1.0,
            EXTRACT(
                EPOCH
                FROM (NOW() - p.created_at)
            ) / 86400.0
        )
    )::FLOAT as expected_engagement
FROM posts p
    JOIN sources s ON p.source_id = s.id
    JOIN SourceAverages sa ON p.source_id = sa.source_id
    LEFT JOIN (
        SELECT DISTINCT ON (post_id) post_id,
            likes,
            reposts
        FROM posts_reactions_history
        ORDER BY post_id,
            synced_at DESC
    ) prh ON p.id = prh.post_id
WHERE s.user_id = @user_id
    AND p.post_type NOT IN ('tag', 'repost', 'quote')
    AND (COALESCE(prh.likes, 0) + COALESCE(prh.reposts, 0)) < (
        sa.avg_engagement * LEAST(
            1.0,
            EXTRACT(EPOCH FROM (NOW() - p.created_at)) / 86400.0
        )
    )
    AND p.created_at > NOW() - INTERVAL '90 days'
    AND (sqlc.narg('start_date')::date IS NULL OR p.created_at >= sqlc.narg('start_date')::date)
    AND (sqlc.narg('end_date')::date IS NULL OR p.created_at < sqlc.narg('end_date')::date + INTERVAL '1 day')
    AND (array_length(@post_types::text[], 1) IS NULL OR p.post_type = ANY(@post_types::text[]))
    AND (array_length(@tag_ids::uuid[], 1) IS NULL OR p.id IN (SELECT post_id FROM post_tags WHERE tag_id = ANY(@tag_ids::uuid[])))
ORDER BY (
        (COALESCE(prh.likes, 0) + COALESCE(prh.reposts, 0)) - (
            sa.avg_engagement * LEAST(
                1.0,
                EXTRACT(EPOCH FROM (NOW() - p.created_at)) / 86400.0
            )
        )
    ) ASC
LIMIT 7;
-- name: GetPerformanceDeviationNegativeViewsFiltered :many
WITH SourceAverages AS (
    SELECT p.source_id,
        AVG(COALESCE(prh.views, 0)) as avg_engagement
    FROM posts p
        LEFT JOIN (
            SELECT DISTINCT ON (post_id) post_id,
                views
            FROM posts_reactions_history
            ORDER BY post_id,
                synced_at DESC
        ) prh ON p.id = prh.post_id
    GROUP BY p.source_id
)
SELECT p.id,
    p.network_internal_id,
    COALESCE(p.content, '')::TEXT as content,
    p.created_at,
    p.author,
    s.network,
    COALESCE(prh.views, 0)::BIGINT as views,
    (
        sa.avg_engagement * LEAST(
            1.0,
            EXTRACT(
                EPOCH
                FROM (NOW() - p.created_at)
            ) / 86400.0
        )
    )::FLOAT as expected_engagement
FROM posts p
    JOIN sources s ON p.source_id = s.id
    JOIN SourceAverages sa ON p.source_id = sa.source_id
    LEFT JOIN (
        SELECT DISTINCT ON (post_id) post_id,
            views
        FROM posts_reactions_history
        ORDER BY post_id,
            synced_at DESC
    ) prh ON p.id = prh.post_id
WHERE s.user_id = @user_id
    AND p.post_type NOT IN ('tag', 'repost', 'quote')
    AND COALESCE(prh.views, 0) < (
        sa.avg_engagement * LEAST(
            1.0,
            EXTRACT(EPOCH FROM (NOW() - p.created_at)) / 86400.0
        )
    )
    AND p.created_at > NOW() - INTERVAL '90 days'
    AND (sqlc.narg('start_date')::date IS NULL OR p.created_at >= sqlc.narg('start_date')::date)
    AND (sqlc.narg('end_date')::date IS NULL OR p.created_at < sqlc.narg('end_date')::date + INTERVAL '1 day')
    AND (array_length(@post_types::text[], 1) IS NULL OR p.post_type = ANY(@post_types::text[]))
    AND (array_length(@tag_ids::uuid[], 1) IS NULL OR p.id IN (SELECT post_id FROM post_tags WHERE tag_id = ANY(@tag_ids::uuid[])))
ORDER BY (
        COALESCE(prh.views, 0) - (
            sa.avg_engagement * LEAST(
                1.0,
                EXTRACT(EPOCH FROM (NOW() - p.created_at)) / 86400.0
            )
        )
    ) ASC
LIMIT 7;
-- name: GetEngagementVelocityDataFiltered :many
SELECT prh.post_id,
    prh.synced_at as history_synced_at,
    COALESCE(prh.likes, 0)::BIGINT as likes,
    COALESCE(prh.reposts, 0)::BIGINT as reposts,
    COALESCE(prh.views, 0)::BIGINT as views,
    p.created_at as post_created_at,
    COALESCE(p.content, '')::TEXT as content,
    p.author,
    p.network_internal_id,
    s.network
FROM posts_reactions_history prh
    JOIN posts p ON prh.post_id = p.id
    JOIN sources s ON p.source_id = s.id
    JOIN (
        SELECT post_id,
            MIN(synced_at) as first_synced
        FROM posts_reactions_history
        GROUP BY post_id
    ) first_sync ON p.id = first_sync.post_id
WHERE s.user_id = @user_id
    AND p.created_at > NOW() - INTERVAL '30 days'
    AND p.post_type NOT IN ('tag', 'repost', 'quote')
    AND DATE(p.created_at) = DATE(first_sync.first_synced)
    AND (sqlc.narg('start_date')::date IS NULL OR p.created_at >= sqlc.narg('start_date')::date)
    AND (sqlc.narg('end_date')::date IS NULL OR p.created_at < sqlc.narg('end_date')::date + INTERVAL '1 day')
    AND (array_length(@post_types::text[], 1) IS NULL OR p.post_type = ANY(@post_types::text[]))
    AND (array_length(@tag_ids::uuid[], 1) IS NULL OR p.id IN (SELECT post_id FROM post_tags WHERE tag_id = ANY(@tag_ids::uuid[])))
ORDER BY prh.post_id,
    prh.synced_at ASC;
-- name: GetTopInteractedSince1Day :many
WITH latest AS (
    SELECT DISTINCT ON (post_id) post_id,
        COALESCE(likes, 0) AS likes,
        COALESCE(reposts, 0) AS reposts
    FROM posts_reactions_history
    ORDER BY post_id, synced_at DESC
),
before_cutoff AS (
    SELECT DISTINCT ON (post_id) post_id,
        COALESCE(likes, 0) AS likes,
        COALESCE(reposts, 0) AS reposts
    FROM posts_reactions_history
    WHERE synced_at < NOW() - INTERVAL '1 day'
    ORDER BY post_id, synced_at DESC
)
SELECT
    p.id,
    p.network_internal_id,
    COALESCE(p.content, '')::TEXT AS content,
    p.created_at,
    p.author,
    s.network,
    (GREATEST(0, l.likes - COALESCE(b.likes, 0)) + GREATEST(0, l.reposts - COALESCE(b.reposts, 0)))::BIGINT AS engagement_delta,
    (l.likes + l.reposts)::BIGINT AS engagement_total
FROM posts p
JOIN sources s ON p.source_id = s.id
JOIN latest l ON p.id = l.post_id
LEFT JOIN before_cutoff b ON p.id = b.post_id
WHERE s.user_id = $1
    AND p.post_type NOT IN ('tag', 'repost', 'quote')
ORDER BY (GREATEST(0, l.likes - COALESCE(b.likes, 0)) + GREATEST(0, l.reposts - COALESCE(b.reposts, 0))) DESC
LIMIT 10;
-- name: GetTopInteractedSince1DayViews :many
WITH latest AS (
    SELECT DISTINCT ON (post_id) post_id,
        COALESCE(views, 0) AS views
    FROM posts_reactions_history
    ORDER BY post_id, synced_at DESC
),
before_cutoff AS (
    SELECT DISTINCT ON (post_id) post_id,
        COALESCE(views, 0) AS views
    FROM posts_reactions_history
    WHERE synced_at < NOW() - INTERVAL '1 day'
    ORDER BY post_id, synced_at DESC
)
SELECT
    p.id,
    p.network_internal_id,
    COALESCE(p.content, '')::TEXT AS content,
    p.created_at,
    p.author,
    s.network,
    GREATEST(0, l.views - COALESCE(b.views, 0))::BIGINT AS engagement_delta,
    l.views::BIGINT AS engagement_total
FROM posts p
JOIN sources s ON p.source_id = s.id
JOIN latest l ON p.id = l.post_id
LEFT JOIN before_cutoff b ON p.id = b.post_id
WHERE s.user_id = $1
    AND p.post_type NOT IN ('tag', 'repost', 'quote')
ORDER BY GREATEST(0, l.views - COALESCE(b.views, 0)) DESC
LIMIT 10;
-- name: GetTopInteractedSince14Days :many
WITH latest AS (
    SELECT DISTINCT ON (post_id) post_id,
        COALESCE(likes, 0) AS likes,
        COALESCE(reposts, 0) AS reposts
    FROM posts_reactions_history
    ORDER BY post_id, synced_at DESC
),
before_cutoff AS (
    SELECT DISTINCT ON (post_id) post_id,
        COALESCE(likes, 0) AS likes,
        COALESCE(reposts, 0) AS reposts
    FROM posts_reactions_history
    WHERE synced_at < NOW() - INTERVAL '14 days'
    ORDER BY post_id, synced_at DESC
)
SELECT
    p.id,
    p.network_internal_id,
    COALESCE(p.content, '')::TEXT AS content,
    p.created_at,
    p.author,
    s.network,
    (GREATEST(0, l.likes - COALESCE(b.likes, 0)) + GREATEST(0, l.reposts - COALESCE(b.reposts, 0)))::BIGINT AS engagement_delta,
    (l.likes + l.reposts)::BIGINT AS engagement_total
FROM posts p
JOIN sources s ON p.source_id = s.id
JOIN latest l ON p.id = l.post_id
LEFT JOIN before_cutoff b ON p.id = b.post_id
WHERE s.user_id = $1
    AND p.post_type NOT IN ('tag', 'repost', 'quote')
ORDER BY (GREATEST(0, l.likes - COALESCE(b.likes, 0)) + GREATEST(0, l.reposts - COALESCE(b.reposts, 0))) DESC
LIMIT 10;
-- name: GetTopInteractedSince14DaysViews :many
WITH latest AS (
    SELECT DISTINCT ON (post_id) post_id,
        COALESCE(views, 0) AS views
    FROM posts_reactions_history
    ORDER BY post_id, synced_at DESC
),
before_cutoff AS (
    SELECT DISTINCT ON (post_id) post_id,
        COALESCE(views, 0) AS views
    FROM posts_reactions_history
    WHERE synced_at < NOW() - INTERVAL '14 days'
    ORDER BY post_id, synced_at DESC
)
SELECT
    p.id,
    p.network_internal_id,
    COALESCE(p.content, '')::TEXT AS content,
    p.created_at,
    p.author,
    s.network,
    GREATEST(0, l.views - COALESCE(b.views, 0))::BIGINT AS engagement_delta,
    l.views::BIGINT AS engagement_total
FROM posts p
JOIN sources s ON p.source_id = s.id
JOIN latest l ON p.id = l.post_id
LEFT JOIN before_cutoff b ON p.id = b.post_id
WHERE s.user_id = $1
    AND p.post_type NOT IN ('tag', 'repost', 'quote')
ORDER BY GREATEST(0, l.views - COALESCE(b.views, 0)) DESC
LIMIT 10;
