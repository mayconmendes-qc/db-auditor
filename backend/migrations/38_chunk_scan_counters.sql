-- Timescale retention advisor (spec 09): per-chunk scan/write counters (ADR-0001 D9).
-- Null means "não coletado" on snapshots taken before this migration.

ALTER TABLE chunk_snapshot ADD COLUMN IF NOT EXISTS seq_scan bigint;
ALTER TABLE chunk_snapshot ADD COLUMN IF NOT EXISTS idx_scan bigint;
ALTER TABLE chunk_snapshot ADD COLUMN IF NOT EXISTS n_tup_ins bigint;
ALTER TABLE chunk_snapshot ADD COLUMN IF NOT EXISTS n_tup_upd bigint;
ALTER TABLE chunk_snapshot ADD COLUMN IF NOT EXISTS n_tup_del bigint;
ALTER TABLE chunk_snapshot ADD COLUMN IF NOT EXISTS stats_reset timestamptz;
