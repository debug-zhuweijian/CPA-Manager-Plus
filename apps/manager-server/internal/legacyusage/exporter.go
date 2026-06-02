package legacyusage

import (
	"bufio"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/mail"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/seakee/cpa-manager-plus/apps/manager-server/internal/usage"
)

const defaultLegacyEndpoint = "legacy request_logs"

var secretLikePattern = regexp.MustCompile(`(?i)^(sk-|sk_|AIza|hf_|ghp_|github_pat_|sess-|pk_|rk_|tp-|[A-Za-z0-9_-]{32,})`)

type ExportOptions struct {
	Limit  int
	Offset int
}

type ExportSummary struct {
	Rows    int64
	Written int64
}

type requestLogRow struct {
	ID                  int64
	Timestamp           string
	APIKey              string
	APIKeyName          string
	Model               string
	Source              string
	ChannelName         string
	AuthIndex           string
	AuthType            string
	Failed              int64
	LatencyMS           int64
	FirstTokenMS        int64
	InputTokens         int64
	OutputTokens        int64
	ReasoningTokens     int64
	CachedTokens        int64
	TotalTokens         int64
	UncachedInputTokens int64
	CacheReadTokens     int64
	CacheCreationTokens int64
	TokenSemantics      string
	InputTokensTotal    int64
}

// ExportRequestLogs writes Manager Plus usage JSONL converted from a legacy CPA request_logs table.
func ExportRequestLogs(ctx context.Context, db *sql.DB, w io.Writer, opts ExportOptions) (ExportSummary, error) {
	if db == nil {
		return ExportSummary{}, errors.New("database is nil")
	}
	if w == nil {
		return ExportSummary{}, errors.New("writer is nil")
	}
	if _, err := db.ExecContext(ctx, `pragma query_only = ON`); err != nil {
		return ExportSummary{}, fmt.Errorf("enable query-only mode: %w", err)
	}
	columns, err := requestLogColumns(ctx, db)
	if err != nil {
		return ExportSummary{}, err
	}
	if _, ok := columns["request_logs"]; ok {
		return ExportSummary{}, errors.New("invalid request_logs column metadata")
	}
	if _, ok := columns["id"]; !ok {
		return ExportSummary{}, errors.New("request_logs.id column is required")
	}
	if _, ok := columns["timestamp"]; !ok {
		return ExportSummary{}, errors.New("request_logs.timestamp column is required")
	}

	query := buildRequestLogsQuery(columns, opts)
	args := queryArgs(opts)
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return ExportSummary{}, fmt.Errorf("query request_logs: %w", err)
	}
	defer rows.Close()

	buffered := bufio.NewWriter(w)
	defer buffered.Flush()

	var summary ExportSummary
	encoder := json.NewEncoder(buffered)
	for rows.Next() {
		row := requestLogRow{}
		if err := rows.Scan(
			&row.ID,
			&row.Timestamp,
			&row.APIKey,
			&row.APIKeyName,
			&row.Model,
			&row.Source,
			&row.ChannelName,
			&row.AuthIndex,
			&row.AuthType,
			&row.Failed,
			&row.LatencyMS,
			&row.FirstTokenMS,
			&row.InputTokens,
			&row.OutputTokens,
			&row.ReasoningTokens,
			&row.CachedTokens,
			&row.TotalTokens,
			&row.UncachedInputTokens,
			&row.CacheReadTokens,
			&row.CacheCreationTokens,
			&row.TokenSemantics,
			&row.InputTokensTotal,
		); err != nil {
			return summary, fmt.Errorf("scan request_logs row: %w", err)
		}
		summary.Rows++
		event, err := convertRow(row)
		if err != nil {
			return summary, err
		}
		if err := encoder.Encode(event); err != nil {
			return summary, fmt.Errorf("write jsonl: %w", err)
		}
		summary.Written++
	}
	if err := rows.Err(); err != nil {
		return summary, fmt.Errorf("iterate request_logs: %w", err)
	}
	return summary, nil
}

func requestLogColumns(ctx context.Context, db *sql.DB) (map[string]struct{}, error) {
	rows, err := db.QueryContext(ctx, `pragma table_info(request_logs)`)
	if err != nil {
		return nil, fmt.Errorf("read request_logs schema: %w", err)
	}
	defer rows.Close()

	columns := map[string]struct{}{}
	for rows.Next() {
		var cid int
		var name string
		var columnType string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
			return nil, fmt.Errorf("scan request_logs schema: %w", err)
		}
		columns[name] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate request_logs schema: %w", err)
	}
	if len(columns) == 0 {
		return nil, errors.New("request_logs table does not exist")
	}
	return columns, nil
}

func buildRequestLogsQuery(columns map[string]struct{}, opts ExportOptions) string {
	selects := []string{
		columnExpr(columns, "id", "0"),
		columnExpr(columns, "timestamp", "''"),
		columnExpr(columns, "api_key", "''"),
		columnExpr(columns, "api_key_name", "''"),
		columnExpr(columns, "model", "''"),
		columnExpr(columns, "source", "''"),
		columnExpr(columns, "channel_name", "''"),
		columnExpr(columns, "auth_index", "''"),
		columnExpr(columns, "auth_type", "''"),
		columnExpr(columns, "failed", "0"),
		columnExpr(columns, "latency_ms", "0"),
		columnExpr(columns, "first_token_ms", "0"),
		columnExpr(columns, "input_tokens", "0"),
		columnExpr(columns, "output_tokens", "0"),
		columnExpr(columns, "reasoning_tokens", "0"),
		columnExpr(columns, "cached_tokens", "0"),
		columnExpr(columns, "total_tokens", "0"),
		columnExpr(columns, "uncached_input_tokens", "0"),
		columnExpr(columns, "cache_read_tokens", "0"),
		columnExpr(columns, "cache_creation_tokens", "0"),
		columnExpr(columns, "token_semantics", "''"),
		columnExpr(columns, "input_tokens_total", "0"),
	}
	query := "select " + strings.Join(selects, ", ") + " from request_logs order by id"
	if opts.Limit > 0 {
		query += " limit ?"
		if opts.Offset > 0 {
			query += " offset ?"
		}
	} else if opts.Offset > 0 {
		query += " limit -1 offset ?"
	}
	return query
}

func columnExpr(columns map[string]struct{}, name string, fallback string) string {
	if _, ok := columns[name]; ok {
		return name
	}
	return fallback + " as " + name
}

func queryArgs(opts ExportOptions) []any {
	args := []any{}
	if opts.Limit > 0 {
		args = append(args, opts.Limit)
		if opts.Offset > 0 {
			args = append(args, opts.Offset)
		}
	} else if opts.Offset > 0 {
		args = append(args, opts.Offset)
	}
	return args
}

func convertRow(row requestLogRow) (usage.Event, error) {
	timestampMS, timestamp, err := parseTimestamp(row.Timestamp)
	if err != nil {
		return usage.Event{}, fmt.Errorf("request_logs id %d timestamp: %w", row.ID, err)
	}
	input, cached, cacheRead, cacheCreation, total := normalizeTokens(row)
	latency := optionalPositive(row.LatencyMS)
	ttft := optionalPositive(row.FirstTokenMS)
	rawSource := firstNonEmpty(row.Source, row.ChannelName, row.APIKeyName, row.AuthIndex)
	source := maskDisplay(rawSource)
	provider := inferProvider(row)
	authType := strings.TrimSpace(row.AuthType)
	if authType == "" {
		authType = inferAuthType(rawSource)
	}
	model := strings.TrimSpace(row.Model)
	if model == "" {
		model = "-"
	}

	event := usage.Event{
		RequestID:             "legacy-cpa-request-log:" + strconv.FormatInt(row.ID, 10),
		TimestampMS:           timestampMS,
		Timestamp:             timestamp,
		Provider:              provider,
		ExecutorType:          provider,
		Model:                 model,
		ResolvedModel:         model,
		Endpoint:              defaultLegacyEndpoint,
		AuthType:              authType,
		AuthIndex:             strings.TrimSpace(row.AuthIndex),
		Source:                source,
		SourceHash:            hashString(rawSource),
		APIKeyHash:            hashString(row.APIKey),
		AccountSnapshot:       source,
		AuthLabelSnapshot:     source,
		AuthFileSnapshot:      maskDisplay(row.ChannelName),
		AuthProviderSnapshot:  provider,
		AuthSnapshotAtMS:      timestampMS,
		InputTokens:           input,
		OutputTokens:          maxInt64(row.OutputTokens, 0),
		ReasoningTokens:       maxInt64(row.ReasoningTokens, 0),
		CachedTokens:          cached,
		CacheTokens:           cached,
		CacheReadTokens:       cacheRead,
		CacheCreationTokens:   cacheCreation,
		TotalTokens:           total,
		LatencyMS:             latency,
		TTFTMS:                ttft,
		Failed:                row.Failed != 0,
		RawJSON:               legacyRawJSON(row, provider, source),
		CreatedAtMS:           timestampMS,
		AuthProjectIDSnapshot: "",
	}
	event.EventHash = legacyEventHash(event, row)
	return event, nil
}

func normalizeTokens(row requestLogRow) (input, cached, cacheRead, cacheCreation, total int64) {
	inputTotal := firstPositive(row.InputTokensTotal, row.InputTokens)
	cacheRead = maxInt64(row.CacheReadTokens, 0)
	cacheCreation = maxInt64(row.CacheCreationTokens, 0)
	cached = maxInt64(row.CachedTokens, 0)
	isSplit := cacheRead > 0 || cacheCreation > 0 || strings.Contains(strings.ToLower(row.TokenSemantics), "split")
	if isSplit {
		input = row.UncachedInputTokens
		if input <= 0 && inputTotal > 0 {
			input = inputTotal - cacheRead - cacheCreation
		}
		input = maxInt64(input, 0)
	} else {
		input = maxInt64(inputTotal, 0)
		if cached > input {
			cached = input
		}
		cacheRead = 0
		cacheCreation = 0
	}
	total = maxInt64(row.TotalTokens, 0)
	if total == 0 {
		total = input + maxInt64(row.OutputTokens, 0) + maxInt64(row.ReasoningTokens, 0) +
			usage.CompatibleCachedTokens(cached, cached, cacheRead, cacheCreation) +
			cacheRead + cacheCreation
	}
	return input, cached, cacheRead, cacheCreation, total
}

func parseTimestamp(value string) (int64, string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, "", errors.New("empty timestamp")
	}
	if number, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
		if number < 10_000_000_000 {
			number *= 1000
		}
		return number, time.UnixMilli(number).UTC().Format(time.RFC3339Nano), nil
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999Z07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05.999999999",
		"2006-01-02T15:04:05",
	}
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, trimmed)
		if err == nil {
			return parsed.UnixMilli(), parsed.UTC().Format(time.RFC3339Nano), nil
		}
	}
	return 0, "", fmt.Errorf("unsupported value %q", trimmed)
}

func optionalPositive(value int64) *int64 {
	if value <= 0 {
		return nil
	}
	return &value
}

func inferProvider(row requestLogRow) string {
	seed := strings.ToLower(strings.Join([]string{row.Model, row.ChannelName, row.AuthType}, " "))
	switch {
	case strings.Contains(seed, "grok") || strings.Contains(seed, "xai"):
		return "xai"
	case strings.Contains(seed, "gemini") || strings.Contains(seed, "vertex"):
		return "gemini"
	case strings.Contains(seed, "claude") || strings.Contains(seed, "anthropic"):
		return "claude"
	case strings.Contains(seed, "codex"):
		return "codex"
	case strings.HasPrefix(strings.ToLower(row.Model), "gpt-"):
		return "openai"
	case strings.Contains(seed, "glm") || strings.Contains(seed, "zhipu") || strings.Contains(row.ChannelName, "智谱"):
		return "zhipu"
	case strings.Contains(seed, "deepseek"):
		return "deepseek"
	case strings.Contains(seed, "kimi"):
		return "kimi"
	case strings.Contains(seed, "qwen"):
		return "qwen"
	default:
		return firstNonEmpty(strings.TrimSpace(row.AuthType), "unknown")
	}
}

func inferAuthType(source string) string {
	if looksSecret(source) {
		return "apikey"
	}
	if _, err := mail.ParseAddress(source); err == nil {
		return "oauth"
	}
	return ""
}

func legacyRawJSON(row requestLogRow, provider string, source string) string {
	payload := map[string]any{
		"format":          "legacy_cpa_request_logs",
		"id":              row.ID,
		"provider":        provider,
		"model":           row.Model,
		"source":          source,
		"auth_index":      row.AuthIndex,
		"auth_type":       row.AuthType,
		"token_semantics": row.TokenSemantics,
	}
	data, _ := json.Marshal(payload)
	return string(data)
}

func legacyEventHash(event usage.Event, row requestLogRow) string {
	parts := []string{
		"legacy-cpa-request-logs-v1",
		strconv.FormatInt(row.ID, 10),
		event.Timestamp,
		event.Model,
		event.AuthIndex,
		strconv.FormatInt(event.InputTokens, 10),
		strconv.FormatInt(event.OutputTokens, 10),
		strconv.FormatInt(event.ReasoningTokens, 10),
		strconv.FormatInt(event.CachedTokens, 10),
		strconv.FormatInt(event.CacheReadTokens, 10),
		strconv.FormatInt(event.CacheCreationTokens, 10),
		strconv.FormatBool(event.Failed),
	}
	return hashString(strings.Join(parts, "|"))
}

func maskDisplay(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if address, err := mail.ParseAddress(trimmed); err == nil {
		email := address.Address
		parts := strings.SplitN(email, "@", 2)
		if len(parts) == 2 {
			prefix := parts[0]
			if len(prefix) > 3 {
				prefix = prefix[:3]
			}
			return prefix + "***@" + parts[1]
		}
	}
	if looksSecret(trimmed) {
		if len(trimmed) <= 8 {
			return "m:****"
		}
		return "m:" + trimmed[:4] + "..." + trimmed[len(trimmed)-4:]
	}
	return trimmed
}

func looksSecret(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || strings.ContainsAny(trimmed, " /\\") {
		return false
	}
	return secretLikePattern.MatchString(trimmed)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func firstPositive(values ...int64) int64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func maxInt64(left, right int64) int64 {
	if left > right {
		return left
	}
	return right
}

func hashString(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(trimmed))
	return hex.EncodeToString(sum[:])
}
