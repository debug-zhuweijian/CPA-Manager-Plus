package usagemonitoring

import (
	"encoding/json"
	"fmt"
	"strings"
)

func SupportsStatsFilter(filter AnalyticsFilter) bool {
	return strings.TrimSpace(filter.SearchQuery) == "" &&
		len(filter.ProjectIDs) == 0 &&
		len(filter.HeaderErrorKinds) == 0 &&
		len(filter.HeaderErrorCodes) == 0 &&
		len(filter.HeaderQuotaPlans) == 0 &&
		len(filter.HeaderTraceIDs) == 0 &&
		filter.MinLatencyMS == 0 &&
		strings.TrimSpace(filter.CacheStatus) == ""
}

// SupportsEventProjectionFilter keeps the packed search document equivalent to
// the legacy per-column LIKE predicates. User-supplied LIKE wildcards can span
// the projection's field separator, so those uncommon searches must retain the
// raw usage_events path instead of returning a false cross-field match.
func SupportsEventProjectionFilter(filter AnalyticsFilter) bool {
	query := strings.TrimSpace(filter.SearchQuery)
	return !strings.ContainsAny(query, "%_\x1f")
}

// PrefersEventProjection identifies event count/page requests that must scan
// row attributes. A time-only request is already served efficiently by the
// usage_events timestamp index and avoids the projection's two-step detail
// lookup; scoped requests benefit from scanning the narrow projection first.
func PrefersEventProjection(filter AnalyticsFilter) bool {
	return strings.TrimSpace(filter.SearchQuery) != "" ||
		strings.TrimSpace(filter.SearchAPIKeyHash) != "" ||
		len(filter.Models) > 0 ||
		len(filter.Providers) > 0 ||
		len(filter.Accounts) > 0 ||
		len(filter.CredentialIDs) > 0 ||
		len(filter.AuthFiles) > 0 ||
		len(filter.AuthIndices) > 0 ||
		len(filter.APIKeyHashes) > 0 ||
		len(filter.SourceHashes) > 0 ||
		len(filter.ProjectIDs) > 0 ||
		len(filter.RequestTypes) > 0 ||
		len(filter.HeaderErrorKinds) > 0 ||
		len(filter.HeaderErrorCodes) > 0 ||
		len(filter.HeaderQuotaPlans) > 0 ||
		len(filter.HeaderTraceIDs) > 0 ||
		!filter.IncludeFailed ||
		filter.FailedOnly ||
		filter.MinLatencyMS > 0 ||
		strings.TrimSpace(filter.CacheStatus) != ""
}

func SupportsSelectorFilter(filter AnalyticsFilter) bool {
	return strings.TrimSpace(filter.SearchQuery) == "" &&
		strings.TrimSpace(filter.SearchAPIKeyHash) == "" &&
		len(filter.Models) == 0 &&
		len(filter.Providers) == 0 &&
		len(filter.Accounts) == 0 &&
		len(filter.CredentialIDs) == 0 &&
		len(filter.AuthFiles) == 0 &&
		len(filter.AuthIndices) == 0 &&
		len(filter.APIKeyHashes) == 0 &&
		len(filter.SourceHashes) == 0 &&
		len(filter.ProjectIDs) == 0 &&
		len(filter.RequestTypes) == 0 &&
		len(filter.HeaderErrorKinds) == 0 &&
		len(filter.HeaderErrorCodes) == 0 &&
		len(filter.HeaderQuotaPlans) == 0 &&
		len(filter.HeaderTraceIDs) == 0 &&
		filter.IncludeFailed &&
		!filter.FailedOnly &&
		filter.MinLatencyMS == 0 &&
		strings.TrimSpace(filter.CacheStatus) == ""
}

func storedStatsConditions(filter AnalyticsFilter, revision string, fromMS, toMS int64) ([]string, []any) {
	conditions := []string{"structure_revision = ?", "bucket_ms >= ?", "bucket_ms < ?"}
	args := []any{revision, fromMS, toMS}
	appendStatsScopeConditions(filter, "", "billing_model", &conditions, &args)
	return conditions, args
}

func rawStatsConditions(filter AnalyticsFilter, fromMS, toMS, afterID int64, useAfterID bool) ([]string, []any) {
	conditions := []string{"e.timestamp_ms >= ?", "e.timestamp_ms < ?"}
	args := []any{fromMS, toMS}
	if useAfterID {
		conditions = append(conditions, "e.id > ?")
		args = append(args, afterID)
	}
	appendStatsScopeConditions(filter, "e.", "resolved_model", &conditions, &args)
	return conditions, args
}

func appendStatsScopeConditions(filter AnalyticsFilter, prefix, resolvedModelColumn string, conditions *[]string, args *[]any) {
	column := func(name string) string { return prefix + name }
	addInCondition := func(expression string, values []string) {
		normalized := normalizeFilterValues(values)
		if len(normalized) == 0 {
			return
		}
		*conditions = append(*conditions, fmt.Sprintf("coalesce(%s, '') in (select value from json_each(?))", expression))
		*args = append(*args, encodeJSONFilterValues(normalized))
	}

	hash := strings.TrimSpace(strings.ToLower(filter.SearchAPIKeyHash))
	if hash != "" {
		*conditions = append(*conditions, "lower(coalesce("+column("api_key_hash")+", '')) = ?")
		*args = append(*args, hash)
	}
	addInCondition(column("model"), filter.Models)
	addProviderStatsCondition(filter.Providers, prefix, resolvedModelColumn, conditions, args)
	addAccountStatsCondition(filter.Accounts, prefix, conditions, args)
	credentialExpr := fmt.Sprintf("coalesce(nullif(%sauth_file_snapshot, ''), nullif(%sauth_index, ''), nullif(%ssource_hash, ''), nullif(%ssource, ''), '-')", prefix, prefix, prefix, prefix)
	addInCondition(credentialExpr, filter.CredentialIDs)
	addInCondition(column("auth_file_snapshot"), filter.AuthFiles)
	addInCondition(column("auth_index"), filter.AuthIndices)
	addInCondition(column("api_key_hash"), filter.APIKeyHashes)
	addInCondition(column("source_hash"), filter.SourceHashes)
	addInCondition(column("executor_type"), filter.RequestTypes)
	if !filter.IncludeFailed {
		*conditions = append(*conditions, column("failed")+" = 0")
	}
	if filter.FailedOnly {
		*conditions = append(*conditions, column("failed")+" = 1")
	}
}

func addProviderStatsCondition(values []string, prefix, resolvedModelColumn string, conditions *[]string, args *[]any) {
	normalized := normalizeLowerFilterValues(values)
	if len(normalized) == 0 {
		return
	}
	encoded := encodeJSONFilterValues(normalized)
	providerExpression := effectiveProviderExpression(
		prefix+"provider",
		prefix+"auth_provider_snapshot",
		prefix+"model",
		prefix+resolvedModelColumn,
	)
	*conditions = append(*conditions, providerExpression+" in (select value from json_each(?))")
	*args = append(*args, encoded)
}

func effectiveProviderExpression(providerColumn, authProviderColumn, modelColumn, resolvedModelColumn string) string {
	provider := "lower(coalesce(nullif(" + authProviderColumn + ", ''), nullif(" + providerColumn + ", ''), ''))"
	modelProvider := inferredProviderCase(modelColumn)
	resolvedModelProvider := inferredProviderCase(resolvedModelColumn)
	return "case " +
		"when " + preferredInferredProviderCondition(provider, modelProvider) + " then " + modelProvider + " " +
		"when " + preferredInferredProviderCondition(provider, resolvedModelProvider) + " then " + resolvedModelProvider + " " +
		"else " + provider + " end"
}

func preferredInferredProviderCondition(provider, inferred string) string {
	return inferred + " <> '' and (" +
		provider + " in ('', 'apikey', 'api-key', 'api_key') or (" +
		inferred + " in ('zhipu', 'mimo', 'minimax', 'deepseek', 'kimi', 'qwen', 'xai', 'gemini') and " +
		provider + " in ('claude', 'anthropic', 'openai', 'codex', 'apikey', 'api-key', 'api_key')))"
}

func inferredProviderCase(column string) string {
	value := "lower(coalesce(" + column + ", ''))"
	return "case " +
		"when instr(" + value + ", 'grok') > 0 or instr(" + value + ", 'xai') > 0 then 'xai' " +
		"when instr(" + value + ", 'gemini') > 0 or instr(" + value + ", 'vertex') > 0 then 'gemini' " +
		"when instr(" + value + ", 'claude') > 0 or instr(" + value + ", 'anthropic') > 0 then 'claude' " +
		"when instr(" + value + ", 'codex') > 0 then 'codex' " +
		"when " + value + " like 'gpt-%' then 'openai' " +
		"when instr(" + value + ", 'glm') > 0 or instr(" + value + ", 'zhipu') > 0 then 'zhipu' " +
		"when instr(" + value + ", 'mimo') > 0 or instr(" + value + ", 'xiaomimimo') > 0 then 'mimo' " +
		"when instr(" + value + ", 'minimax') > 0 or instr(" + value + ", 'abab') > 0 then 'minimax' " +
		"when instr(" + value + ", 'deepseek') > 0 then 'deepseek' " +
		"when instr(" + value + ", 'kimi') > 0 then 'kimi' " +
		"when instr(" + value + ", 'qwen') > 0 then 'qwen' " +
		"else '' end"
}

func addAccountStatsCondition(values []string, prefix string, conditions *[]string, args *[]any) {
	normalized := normalizeLowerFilterValues(values)
	if len(normalized) == 0 {
		return
	}
	encoded := encodeJSONFilterValues(normalized)
	accountConditions := []string{
		"lower(coalesce(" + prefix + "account_snapshot, '')) in (select value from json_each(?))",
		"lower(coalesce(" + prefix + "auth_label_snapshot, '')) in (select value from json_each(?))",
		"lower(coalesce(" + prefix + "source, '')) in (select value from json_each(?))",
		"lower(coalesce(" + prefix + "auth_index, '')) in (select value from json_each(?))",
	}
	*conditions = append(*conditions, "("+strings.Join(accountConditions, " or ")+")")
	for range accountConditions {
		*args = append(*args, encoded)
	}
}

func encodeJSONFilterValues(values []string) string {
	encoded, _ := json.Marshal(values)
	return string(encoded)
}

func normalizeFilterValues(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func normalizeLowerFilterValues(values []string) []string {
	normalized := normalizeFilterValues(values)
	for index, value := range normalized {
		normalized[index] = strings.ToLower(value)
	}
	return normalized
}
