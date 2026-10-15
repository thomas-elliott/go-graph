CREATE TABLE IF NOT EXISTS nodes (
    id         text PRIMARY KEY,
    labels     jsonb,
    properties jsonb,
    tsv        tsvector GENERATED ALWAYS AS (
        setweight(to_tsvector('english', coalesce(properties->>'dest', '')), 'A') ||
        setweight(to_tsvector('english', coalesce(properties->>'heading', properties->>'name', '')), 'B') ||
        setweight(to_tsvector('english', coalesce(properties->>'text', properties->>'content', '')), 'C')
    ) STORED
);

CREATE INDEX IF NOT EXISTS nodes_tsv_idx ON nodes USING gin (tsv);

CREATE TABLE IF NOT EXISTS edges (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    from_id    text NOT NULL REFERENCES nodes (id) ON DELETE CASCADE,
    to_id      text NOT NULL REFERENCES nodes (id) ON DELETE CASCADE,
    type       text NOT NULL DEFAULT '',
    properties jsonb,
    UNIQUE (from_id, to_id, type)
);

CREATE INDEX IF NOT EXISTS edges_to_idx ON edges (to_id);

CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS embeddings (
    node_id   text PRIMARY KEY REFERENCES nodes (id) ON DELETE CASCADE,
    model     text NOT NULL,
    embedding vector NOT NULL
);
