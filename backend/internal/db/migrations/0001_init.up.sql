-- Initial schema. gen_random_uuid() is built into Postgres 13+, so no extension is needed.

CREATE TYPE video_status AS ENUM ('scheduled', 'uploading', 'published', 'failed');

CREATE TABLE channels (
    id                  uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    youtube_channel_id  text        NOT NULL UNIQUE,
    title               text        NOT NULL,
    -- OAuth tokens are encrypted by the application (AES-GCM) before storage, never kept as plaintext.
    access_token_enc    bytea       NOT NULL,
    refresh_token_enc   bytea       NOT NULL,
    token_expires_at    timestamptz NOT NULL,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE videos (
    id                uuid         PRIMARY KEY DEFAULT gen_random_uuid(),
    channel_id        uuid         NOT NULL REFERENCES channels (id) ON DELETE RESTRICT,
    -- Length limits mirror YouTube's own: 100-char titles, 5000-char descriptions.
    title             text         NOT NULL CHECK (char_length(title) BETWEEN 1 AND 100),
    description       text         NOT NULL DEFAULT '' CHECK (char_length(description) <= 5000),
    tags              text[]       NOT NULL DEFAULT '{}',
    storage_key       text         NOT NULL CHECK (char_length(storage_key) BETWEEN 1 AND 1024),
    scheduled_at      timestamptz  NOT NULL,
    status            video_status NOT NULL DEFAULT 'scheduled',
    youtube_video_id  text         UNIQUE,
    last_error        text,
    attempts          integer      NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    created_at        timestamptz  NOT NULL DEFAULT now(),
    updated_at        timestamptz  NOT NULL DEFAULT now()
);

-- Dashboard lists newest first with keyset pagination on (created_at, id).
CREATE INDEX videos_created_at_id_idx ON videos (created_at DESC, id DESC);
CREATE INDEX videos_channel_created_at_id_idx ON videos (channel_id, created_at DESC, id DESC);
-- Recovery sweeps look for due or stuck videos by status.
CREATE INDEX videos_status_scheduled_at_idx ON videos (status, scheduled_at);

CREATE TABLE video_stats (
    video_id     uuid        NOT NULL REFERENCES videos (id) ON DELETE CASCADE,
    captured_at  timestamptz NOT NULL,
    views        bigint      NOT NULL DEFAULT 0 CHECK (views >= 0),
    likes        bigint      NOT NULL DEFAULT 0 CHECK (likes >= 0),
    comments     bigint      NOT NULL DEFAULT 0 CHECK (comments >= 0),
    PRIMARY KEY (video_id, captured_at)
);
