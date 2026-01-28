-- +goose Up
ALTER TABLE sources DROP CONSTRAINT network_check;

ALTER TABLE sources
ADD CONSTRAINT network_check CHECK (
    network IN (
        'Instagram',
        'Bluesky',
        'Murrtube',
        'BadPups',
        'TikTok',
        'Mastodon',
        'Reddit',
        'Telegram',
        'Google Analytics',
        'YouTube',
        'FurTrack'
    )
);

-- +goose Down
ALTER TABLE sources DROP CONSTRAINT network_check;

ALTER TABLE sources
ADD CONSTRAINT network_check CHECK (
    network IN (
        'Instagram',
        'Bluesky',
        'Murrtube',
        'BadPups',
        'TikTok',
        'Mastodon',
        'Reddit',
        'Telegram',
        'Google Analytics',
        'YouTube'
    )
);