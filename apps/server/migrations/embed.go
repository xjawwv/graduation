package migrations

import _ "embed"

// Initial contains the idempotent bootstrap schema.
//
//go:embed 001_initial.sql
var Initial string

// DisplayBackground adds the persistent per-room display surface color.
//
//go:embed 002_display_background.sql
var DisplayBackground string
