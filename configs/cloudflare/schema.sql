-- D1 Telemetry schema for snowden.system
-- Run: wrangler d1 execute snowden-telemetry --file=schema.sql

CREATE TABLE IF NOT EXISTS events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  region TEXT NOT NULL DEFAULT 'unknown',
  event TEXT NOT NULL,
  protocol TEXT NOT NULL DEFAULT 'unknown',
  latency_ms INTEGER DEFAULT 0,
  timestamp INTEGER NOT NULL,
  created_at TEXT DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_events_event ON events(event);
CREATE INDEX IF NOT EXISTS idx_events_region ON events(region);
CREATE INDEX IF NOT EXISTS idx_events_timestamp ON events(timestamp);

-- Anonymous stats view (no user IDs, just aggregates)
CREATE VIEW IF NOT EXISTS stats_daily AS
SELECT
  DATE(created_at) as date,
  event,
  protocol,
  COUNT(*) as count,
  AVG(latency_ms) as avg_latency
FROM events
GROUP BY DATE(created_at), event, protocol
ORDER BY date DESC;
