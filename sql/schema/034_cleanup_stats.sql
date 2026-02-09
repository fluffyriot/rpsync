-- +goose Up

UPDATE posts_reactions_history prh
SET
    views = NULL
FROM posts p
    JOIN sources s ON p.source_id = s.id
WHERE
    prh.post_id = p.id
    AND s.network IN (
        'Bluesky',
        'Mastodon',
        'FurTrack'
    );

UPDATE posts_reactions_history prh
SET
    reposts = NULL
FROM posts p
    JOIN sources s ON p.source_id = s.id
WHERE
    prh.post_id = p.id
    AND s.network IN (
        'Murrtube',
        'BadPups',
        'FurTrack'
    );

UPDATE posts_reactions_history prh
SET
    reposts = NULL
FROM posts p
    JOIN sources s ON p.source_id = s.id
WHERE
    prh.post_id = p.id
    AND s.network = 'Instagram'
    AND p.post_type <> 'tag';

UPDATE posts_reactions_history prh
SET
    views = NULL,
    reposts = NULL
FROM posts p
    JOIN sources s ON p.source_id = s.id
WHERE
    prh.post_id = p.id
    AND s.network = 'Instagram'
    AND p.post_type = 'tag';

-- +goose Down