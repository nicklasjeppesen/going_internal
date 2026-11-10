-- The outbox of super/events/outbox (Postgres).
--
-- domain_events: an event is stored in the same transaction as the change it
-- is about. published_at = '' means it hasn't been fanned out to its
-- listeners yet; claimed_at is set while a process works on it.
CREATE TABLE IF NOT EXISTS domain_events (
    id           BIGSERIAL PRIMARY KEY,
    name         TEXT NOT NULL DEFAULT '',
    payload      TEXT NOT NULL DEFAULT '',
    user_id      BIGINT NOT NULL DEFAULT 0,
    published_at TEXT NOT NULL DEFAULT '',
    claimed_at   TEXT NOT NULL DEFAULT '',
    attempts     INTEGER NOT NULL DEFAULT 0,
    last_error   TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS domain_events_pending ON domain_events (published_at, id);

-- event_deliveries: one row per event and named listener, run and retried on
-- its own. delivered_at = '' means not delivered yet.
CREATE TABLE IF NOT EXISTS event_deliveries (
    id           BIGSERIAL PRIMARY KEY,
    event_id     BIGINT NOT NULL REFERENCES domain_events(id) ON DELETE CASCADE,
    listener     TEXT NOT NULL,
    async        INTEGER NOT NULL DEFAULT 0,
    delivered_at TEXT NOT NULL DEFAULT '',
    claimed_at   TEXT NOT NULL DEFAULT '',
    attempts     INTEGER NOT NULL DEFAULT 0,
    last_error   TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (event_id, listener)
);
CREATE INDEX IF NOT EXISTS event_deliveries_pending ON event_deliveries (delivered_at, id);
