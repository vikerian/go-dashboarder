CREATE EXTENSION IF NOT EXISTS timescaledb;

CREATE TABLE IF NOT EXISTS events (
  id        BIGSERIAL PRIMARY KEY,
  ts        TIMESTAMPTZ NOT NULL,
  topic     TEXT        NOT NULL,
  source    TEXT        NULL,
  kind      TEXT        NULL,
  key       TEXT        NULL,
  payload   JSONB       NOT NULL
);

-- hypertable on ts
SELECT create_hypertable('events', 'ts', if_not_exists => TRUE);

CREATE INDEX IF NOT EXISTS events_topic_ts_idx ON events (topic, ts DESC);
CREATE INDEX IF NOT EXISTS events_source_ts_idx ON events (source, ts DESC);
CREATE INDEX IF NOT EXISTS events_kind_ts_idx ON events (kind, ts DESC);
