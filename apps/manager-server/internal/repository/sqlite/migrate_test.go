package sqlite

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestMigrateCreatesMonitoringUsageIndexes(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	if err := Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	rows, err := db.Query(`pragma index_list(usage_events)`)
	if err != nil {
		t.Fatalf("index list: %v", err)
	}
	defer rows.Close()

	indexes := map[string]bool{}
	for rows.Next() {
		var seq int
		var name string
		var unique int
		var origin string
		var partial int
		if err := rows.Scan(&seq, &name, &unique, &origin, &partial); err != nil {
			t.Fatalf("scan index: %v", err)
		}
		indexes[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate indexes: %v", err)
	}

	for _, name := range []string{
		"idx_usage_events_timestamp_id_desc",
		"idx_usage_events_failed_timestamp_id_desc",
		"idx_usage_events_timestamp_model_resolved",
		"idx_usage_events_timestamp_auth_model",
		"idx_usage_events_timestamp_account_model",
		"idx_usage_events_timestamp_api_key_model",
		"idx_usage_events_timestamp_source_failure",
		"idx_usage_events_timestamp_total_failed_model",
	} {
		if !indexes[name] {
			t.Fatalf("missing usage_events monitoring index %q", name)
		}
	}
}
