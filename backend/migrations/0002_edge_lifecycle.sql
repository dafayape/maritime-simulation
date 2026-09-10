-- 0002_edge_lifecycle.sql
-- Virtual Edge lifecycle fixes (SRS §3.4 / README "Master Data Virtual Edge"):
--
-- Previously telemetry_logs only kept a foreign key (edge_id, ON DELETE SET
-- NULL) to name the Syahbandar a packet landed at. Deleting an edge from
-- master data silently blanked the "Edge" column for its ENTIRE delivery
-- history, even though the packet genuinely arrived there. edge_code_snapshot
-- freezes the code at the moment the packet landed, independent of the FK's
-- lifecycle, so the dashboard's Reports table can keep showing the name and
-- separately indicate (green/red) whether that edge still exists today.

ALTER TABLE telemetry_logs
    ADD COLUMN IF NOT EXISTS edge_code_snapshot VARCHAR(50);

-- Backfill rows whose edge is still alive today (edge_id survived). Rows
-- whose edge was already deleted BEFORE this migration cannot be recovered —
-- the code was never persisted anywhere else; this is a one-time, known
-- data-quality gap documented in backend/docs/srs.md §3.4.
UPDATE telemetry_logs t
SET edge_code_snapshot = e.edge_code
FROM virtual_edges e
WHERE t.edge_id = e.id
  AND t.edge_code_snapshot IS NULL;
