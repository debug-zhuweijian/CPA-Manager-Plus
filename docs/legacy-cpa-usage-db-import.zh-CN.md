# 旧 CPA usage.db 导入 CPA Manager Plus

本工具用于把旧 CPA 魔改统计库中的 `request_logs` 转换为 CPA Manager Plus 可导入的 JSONL 事件文件。它不会直接写入 Manager Plus 的 `usage.sqlite`，也不会修改旧 `usage.db`。

## 适用范围

- 旧库表名为 `request_logs`。
- 目标是 CPA Manager Plus 完整 Docker 模式。
- 旧 `api_key` 不会以明文写入导出文件，只会写入 hash。
- `source`、`channel_name` 中看起来像邮箱或密钥的值会被脱敏后写入展示字段。

## 转换规则

- OpenAI/GPT/Codex 总输入语义：`input_tokens` 保持总输入，`cached_tokens` 表示其中缓存命中部分。
- Claude/Anthropic split 语义：优先使用 `uncached_input_tokens` 作为 `input_tokens`，并单独写入 `cache_read_tokens` / `cache_creation_tokens`。
- 无 split 语义的旧行会把 `cached_tokens` 约束到不超过 `input_tokens`，避免缓存率超过 100%。
- `event_hash` 基于旧行 `id`、时间、模型、凭证索引和 token 字段稳定生成，重复导入会被 Manager Plus 跳过。

## 操作流程

先对旧库创建只读快照。示例：

```powershell
sqlite3 -readonly "I:\claude-docs\my-project\cpa-runtime-data\cli-proxy-api\data\usage.db" ".backup 'I:\claude-docs\my-project\cpa-runtime-data\cli-proxy-api\data\usage.snapshot.db'"
```

导出 JSONL：

```powershell
cd I:\claude-docs\git-project\6.2\CPA-Manager-Plus\apps\manager-server
go run ./cmd/cpa-legacy-usage-export --input "I:\claude-docs\my-project\cpa-runtime-data\cli-proxy-api\data\usage.snapshot.db" --output "I:\claude-docs\my-project\cpa-runtime-data\cli-proxy-api\data\usage-manager-plus-import.jsonl"
```

导入 Manager Plus：

```powershell
curl.exe -X POST "http://127.0.0.1:18317/v0/management/usage/import" `
  -H "Authorization: Bearer <CPA_MANAGER_ADMIN_KEY>" `
  --data-binary "@I:\claude-docs\my-project\cpa-runtime-data\cli-proxy-api\data\usage-manager-plus-import.jsonl"
```

如果文件超过 Manager Plus 单次导入限制，使用 `--limit` 和 `--offset` 分批导出后分批导入。

## 验收

- 重复导入同一份 JSONL 时，新增数量应为 0，跳过数量应增加。
- 监控中心的请求数、失败数、模型维度、凭证维度应和旧库聚合结果接近。
- Token 统计中缓存率不应超过 100%。
- 对 split cache 行，花费统计不应同时把 `cached_tokens` 和 `cache_read_tokens` 当成两份缓存命中。
