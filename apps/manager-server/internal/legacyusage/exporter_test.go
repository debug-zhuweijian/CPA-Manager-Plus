package legacyusage

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"

	"github.com/seakee/cpa-manager-plus/apps/manager-server/internal/usage"
	_ "modernc.org/sqlite"
)

func TestExportRequestLogsMapsOpenAIAndSplitCacheSemantics(t *testing.T) {
	db := newLegacyDB(t)
	insertLegacyRow(t, db, map[string]any{
		"id":                    1,
		"timestamp":             "2026-01-02T03:04:05Z",
		"model":                 "gpt-5.5",
		"source":                "alice@example.com",
		"auth_type":             "oauth",
		"auth_index":            "auth-openai",
		"input_tokens":          100,
		"input_tokens_total":    100,
		"output_tokens":         10,
		"cached_tokens":         40,
		"total_tokens":          110,
		"token_semantics":       "openai_total_input",
		"cache_read_tokens":     0,
		"cache_creation_tokens": 0,
	})
	insertLegacyRow(t, db, map[string]any{
		"id":                    2,
		"timestamp":             "2026-01-02T04:04:05Z",
		"model":                 "claude-sonnet-4",
		"source":                "bob@example.com",
		"auth_type":             "oauth",
		"auth_index":            "auth-claude",
		"input_tokens":          100,
		"input_tokens_total":    100,
		"uncached_input_tokens": 70,
		"output_tokens":         5,
		"cached_tokens":         30,
		"cache_read_tokens":     30,
		"cache_creation_tokens": 0,
		"total_tokens":          105,
		"token_semantics":       "anthropic_split_input",
	})

	events := exportEvents(t, db)
	if len(events) != 2 {
		t.Fatalf("events len = %d", len(events))
	}
	openai := events[0]
	if openai.InputTokens != 100 || openai.CachedTokens != 40 ||
		openai.CacheReadTokens != 0 || openai.CacheCreationTokens != 0 {
		t.Fatalf("openai tokens = %#v", openai)
	}
	if openai.Source != "ali***@example.com" || openai.Provider != "openai" {
		t.Fatalf("openai identity = %#v", openai)
	}

	split := events[1]
	if split.InputTokens != 70 || split.CachedTokens != 30 ||
		split.CacheReadTokens != 30 || split.CacheCreationTokens != 0 ||
		split.TotalTokens != 105 {
		t.Fatalf("split tokens = %#v", split)
	}
	if usage.CompatibleCachedTokens(split.CachedTokens, split.CacheTokens, split.CacheReadTokens, split.CacheCreationTokens) != 0 {
		t.Fatalf("split compatible cached should not double count: %#v", split)
	}
	if split.Provider != "claude" || split.Source != "bob***@example.com" {
		t.Fatalf("split identity = %#v", split)
	}
}

func TestExportRequestLogsClampsLegacyCachedTokensAndRedactsSecrets(t *testing.T) {
	db := newLegacyDB(t)
	sourceSecret := "sk" + "-test-secret-value"
	clientSecret := "sk" + "-client-secret"
	insertLegacyRow(t, db, map[string]any{
		"id":              3,
		"timestamp":       "2026-01-02T05:04:05Z",
		"model":           "kimi-k2.6",
		"source":          sourceSecret,
		"api_key":         clientSecret,
		"auth_index":      "auth-secret",
		"input_tokens":    100,
		"output_tokens":   9,
		"cached_tokens":   150,
		"total_tokens":    109,
		"token_semantics": "",
	})

	var out bytes.Buffer
	summary, err := ExportRequestLogs(context.Background(), db, &out, ExportOptions{})
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if summary.Rows != 1 || summary.Written != 1 {
		t.Fatalf("summary = %#v", summary)
	}
	if strings.Contains(out.String(), sourceSecret) ||
		strings.Contains(out.String(), clientSecret) {
		t.Fatalf("export leaked raw secret: %s", out.String())
	}
	event := decodeEvent(t, []byte(out.String()))
	if event.CachedTokens != 100 || event.InputTokens != 100 {
		t.Fatalf("clamped tokens = %#v", event)
	}
	if event.Source != "m:sk-t...alue" || event.SourceHash == "" || event.APIKeyHash == "" {
		t.Fatalf("secret identity = %#v", event)
	}
}

func TestExportRequestLogsInfersMimoProviderFromModel(t *testing.T) {
	db := newLegacyDB(t)
	insertLegacyRow(t, db, map[string]any{
		"id":            31,
		"timestamp":     "2026-01-02T05:30:00Z",
		"model":         "mimo-v2.5-pro",
		"source":        "tp-client-secret",
		"auth_type":     "apikey",
		"auth_index":    "mimo-auth",
		"input_tokens":  10,
		"output_tokens": 2,
		"total_tokens":  12,
	})

	event := exportEvents(t, db)[0]
	if event.Provider != "mimo" || event.ExecutorType != "mimo" ||
		event.AuthProviderSnapshot != "mimo" {
		t.Fatalf("provider identity = %#v", event)
	}
}

func TestExportRequestLogsEventHashIsStable(t *testing.T) {
	db := newLegacyDB(t)
	insertLegacyRow(t, db, map[string]any{
		"id":            4,
		"timestamp":     "2026-01-02T06:04:05Z",
		"model":         "glm-5.1",
		"channel_name":  "Zhipu",
		"auth_index":    "auth-1",
		"input_tokens":  10,
		"output_tokens": 2,
		"total_tokens":  12,
	})

	first := exportEvents(t, db)
	second := exportEvents(t, db)
	if first[0].EventHash == "" || first[0].EventHash != second[0].EventHash {
		t.Fatalf("hashes first=%q second=%q", first[0].EventHash, second[0].EventHash)
	}
}

func TestExportRequestLogsLimitAndOffset(t *testing.T) {
	db := newLegacyDB(t)
	for id := 1; id <= 3; id++ {
		insertLegacyRow(t, db, map[string]any{
			"id":            id,
			"timestamp":     "2026-01-02T03:04:05Z",
			"model":         "gpt-5.5",
			"input_tokens":  id,
			"output_tokens": 1,
			"total_tokens":  id + 1,
		})
	}

	var out bytes.Buffer
	summary, err := ExportRequestLogs(context.Background(), db, &out, ExportOptions{Limit: 1, Offset: 1})
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if summary.Rows != 1 || summary.Written != 1 {
		t.Fatalf("summary = %#v", summary)
	}
	event := decodeEvent(t, []byte(out.String()))
	if event.RequestID != "legacy-cpa-request-log:2" {
		t.Fatalf("request id = %q", event.RequestID)
	}
}

func newLegacyDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatalf("close sqlite: %v", err)
		}
	})
	_, err = db.Exec(`create table request_logs (
		id integer primary key,
		timestamp text not null,
		api_key text not null default '',
		api_key_name text not null default '',
		model text not null default '',
		source text not null default '',
		channel_name text not null default '',
		auth_index text not null default '',
		auth_type text not null default '',
		failed integer not null default 0,
		latency_ms integer not null default 0,
		first_token_ms integer not null default 0,
		input_tokens integer not null default 0,
		output_tokens integer not null default 0,
		reasoning_tokens integer not null default 0,
		cached_tokens integer not null default 0,
		total_tokens integer not null default 0,
		uncached_input_tokens integer not null default 0,
		cache_read_tokens integer not null default 0,
		cache_creation_tokens integer not null default 0,
		token_semantics text not null default '',
		input_tokens_total integer not null default 0
	)`)
	if err != nil {
		t.Fatalf("create request_logs: %v", err)
	}
	return db
}

func insertLegacyRow(t *testing.T, db *sql.DB, values map[string]any) {
	t.Helper()
	columns := []string{}
	placeholders := []string{}
	args := []any{}
	for key, value := range values {
		columns = append(columns, key)
		placeholders = append(placeholders, "?")
		args = append(args, value)
	}
	query := "insert into request_logs (" + strings.Join(columns, ",") + ") values (" + strings.Join(placeholders, ",") + ")"
	if _, err := db.Exec(query, args...); err != nil {
		t.Fatalf("insert request_logs: %v", err)
	}
}

func exportEvents(t *testing.T, db *sql.DB) []usage.Event {
	t.Helper()
	var out bytes.Buffer
	if _, err := ExportRequestLogs(context.Background(), db, &out, ExportOptions{}); err != nil {
		t.Fatalf("export: %v", err)
	}
	events := []usage.Event{}
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		events = append(events, decodeEvent(t, []byte(line)))
	}
	return events
}

func decodeEvent(t *testing.T, data []byte) usage.Event {
	t.Helper()
	var event usage.Event
	if err := json.Unmarshal(data, &event); err != nil {
		t.Fatalf("decode event: %v\n%s", err, string(data))
	}
	return event
}
