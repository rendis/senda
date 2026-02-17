-- Fix tracking_id column length: was VARCHAR(32), but tracking IDs are 36 chars ("trk_" + 32 hex).
ALTER TABLE emails ALTER COLUMN tracking_id TYPE VARCHAR(64);
