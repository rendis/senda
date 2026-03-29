-- Denormalize open_tracking_enabled from workspace onto emails so the send
-- worker no longer needs a workspace lookup at send time.
ALTER TABLE emails ADD COLUMN open_tracking_enabled BOOLEAN NOT NULL DEFAULT false;
