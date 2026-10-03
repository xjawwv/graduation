ALTER TABLE room_settings
  ADD COLUMN display_background VARCHAR(16) NOT NULL DEFAULT '#09090b' AFTER default_display_mode;
