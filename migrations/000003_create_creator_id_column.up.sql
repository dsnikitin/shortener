CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

ALTER TABLE shortener.urls
ADD COLUMN IF NOT EXISTS creator_id UUID NOT NULL;

CREATE INDEX idx_urls_creator_id ON shortener.urls(creator_id);

