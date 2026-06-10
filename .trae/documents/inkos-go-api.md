# Plan: 将 InkOS 改造为 Go API 项目

## 1. 概述

将当前 TypeScript monorepo（`packages/cli` + `packages/core` + `packages/studio`，~132K 行）改造为单一 Go HTTP API 服务，删去 CLI/TUI/Studio 前端，仅保留 `packages/studio/src/api/server.ts` 中的 97 个 REST + SSE 端点。

**目标行为等价、数据兼容**：能直接打开现有 `inkos.json` / `books/<id>/` / `story/state/*.json` / `shorts/` / `covers/` / `story/memory.db` / `play.db` 等产物，调用同一组 HTTP 端点获得与原项目一致的结果。

### 关键技术决策（已与用户确认）

| 维度 | 决策 |
| --- | --- |
| API 范围 | **完整移植所有 97 个端点**（含 Play、radar、fanfic、spinoff、imitation、style 等） |
| LLM 协议 | **统一走 OpenAI-compatible** Chat Completions（含 stream）。不再做 30+ provider 适配；用户自行在服务配置里填 baseUrl/apiKey/model。 |
| 数据层 | **复用现有文件 + JSON 结构**。`books/<id>/`、`story/*.json` / `*.md`、`shorts/`、`covers/`、`inkos.json`、`.inkos/secrets.json`、`memory.db`、`play.db` 全部按原 schema 读写。 |
| HTTP 框架 | **gin** |
| 项目结构 | **cmd + internal 拆分**（`cmd/inkos-api/main.go` + `internal/...`） |
| Agent 运行时 | **重新实现**（Writer/Auditor/Reviser/Architect/Planner/Composer/Consolidator/Detection/StyleAnalyzer/ShortFiction/Cover），prompt 来自原 core 包，结构化输出按 JSON 解析。 |

### 不在本计划范围内

- CLI / TUI / Studio 前端（全部删除）
- 30+ provider 特殊协议探测逻辑（统一 OpenAI-compatible）
- Node 22+ SQLite 特性（如 better-sqlite3 → 用 modernc.org/sqlite，纯 Go 驱动）
- 复杂 TS schema 库（Zod）— Go 用 struct + validator

---

## 2. 现状分析

### 2.1 仓库结构

```
/workspace
├── packages/
│   ├── cli/        → 删除
│   ├── core/       → 业务逻辑源头，全部移植到 Go
│   └── studio/     → 仅保留 src/api/ 的逻辑，前端删
├── scripts/        → 删除
├── assets/         → 删除（前端用）
├── pnpm-workspace.yaml / pnpm-lock.yaml / .nvmrc → 删除
├── .github/        → 保留，重写 CI
└── .env.example / LICENSE / README.* → 保留/重写
```

### 2.2 核心依赖（要替换的）

| TS 依赖 | Go 替代 |
| --- | --- |
| `hono` + `@hono/node-server` | `github.com/gin-gonic/gin` |
| `ai` / `@mariozechner/pi-ai` | 自实现 OpenAI-compatible client（直接 HTTP POST） |
| `zod` | `github.com/go-playground/validator/v10` + 自写 schema |
| `better-sqlite3`（memory.db） | `modernc.org/sqlite`（pure Go） |
| 自实现文件 DB（play.db） | 同上 SQLite |
| `tsx` / `vite` / `vitest` | 删除 |
| `react` / `radix` / `tailwind` / `shadcn` | 删除 |
| `node:fs` / `node:path` | Go `os` / `path/filepath` |

### 2.3 端点清单（97 个全部要实现，按域分组）

参考 `packages/studio/src/api/server.ts`：

- **Books/Chapters** (28)：`/books`、`/books/:id`、`/books/create`、`/books/:id/create-status`、`/books/:id/chapters/:num`、`/books/:id/analytics`、`/books/:id/write-next`、`/books/:id/draft`、`/books/:id/eval`、`/books/:id/consolidate`、`/books/:id/plan`、`/books/:id/compose`、`/books/:id/repair-state/:chapter`、`/books/:id/foundation/revise`、`/books/:id/chapters/:num/approve`、`/books/:id/chapters/:num/reject`、`/books/:id/audit/:chapter`、`/books/:id/revise/:chapter`、`/books/:id/export`、`/books/:id/export-save`、`/books/:id/rewrite/:chapter`、`/books/:id/resync/:chapter`、`/books/:id/detect/:chapter`、`/books/:id/detect-all`、`/books/:id/detect/stats`、`/books/:id/style/import`、`/books/:id/import/chapters`、`/books/:id/import/canon`
- **Truth** (4)：`/books/:id/truth`、`/books/:id/truth/:file` (GET/PUT)
- **Project** (10)：`/project`、`/project/files/:file`、`/project/input-governance-mode`、`/project/detection`、`/project/model-overrides`、`/project/chapter-review-mode`、`/project/notify`、`/project/language`
- **LLM Services** (8)：`/services`、`/services/config`、`/services/config/import-env`、`/services/:service`、`/services/:service/test`、`/services/:service/secret`、`/services/:service/models`、`/services/models`、`/services/models/custom`
- **Cover** (3)：`/cover/config`、`/cover/secret/:service`
- **Genres** (6)：`/genres`、`/genres/:id`、`/genres/:id/copy`、`/genres/create`、`/genres/:id` (PUT/DELETE)
- **Play** (5)：`/play/runs/:worldId/:runId`、`/play/runs/:worldId/:runId/image-settings`、`/play/runs/:worldId/:runId/generate-image`、`/play/runs/:worldId/:runId/images/:file`
- **Sessions** (5)：`/sessions`、`/sessions/:sessionId`、`/sessions/:sessionId/play-mode`、`/sessions/:sessionId` (PUT/DELETE)
- **Agent** (1)：`/agent`（SSE 流式）
- **Events** (1)：`/events`（SSE 广播）
- **Interaction** (1)：`/interaction/session`
- **Daemon** (3)：`/daemon`、`/daemon/start`、`/daemon/stop`
- **Logs** (1)：`/logs`
- **Style** (1)：`/style/analyze`
- **Fanfic/Spinoff/Imitation** (5)：`/fanfic/init`、`/books/:id/fanfic`、`/books/:id/fanfic/refresh`、`/spinoff/init`、`/imitation/init`
- **Radar** (2)：`/radar/scan`、`/radar/history`
- **Doctor** (1)：`/doctor`

合计 96 个 REST 端点 + 2 个 SSE 流（`/events`、`/agent`），对齐原 97 个。

### 2.4 业务逻辑规模

- **Agent** (`packages/core/src/agents/`)：architect、writer、auditor、reviser、planner、composer、consolidator、detector、foundation-reviewer、polisher、post-write-validator、state-validator、style-analyzer、short-fiction、play-action-interpreter、play-world-mutator、play-scene-renderer、play-scene-reconciler 等约 20 个。每个含 prompt + JSON 解析 + 后处理。
- **Pipeline** (`packages/core/src/pipeline/`)：runner、scheduler、chapter-persistence、chapter-review-cycle、chapter-state-recovery、chapter-truth-validation、persisted-governed-plan、short-fiction-runner、detection-runner。
- **Models**：book、chapter、project、state、play、runtime-state、detection、style-profile、length-governance、book-rules、genre-profile、input-governance、context-compression，约 13 套。
- **State manager**：listBooks / loadBookConfig / loadChapterIndex / chapterDir / bookDir / stateDir / truthFile / storyDir / snapshots 等。
- **Play DB** (`packages/core/src/play/`)：play-db.ts / play-file-db.ts / play-db-factory.ts / play-store.ts / play-reducer.ts，SQLite + JSON 双模式。
- **Memory DB** (`packages/core/src/state/memory-db.ts`)：SQLite 时序记忆。
- **LLM**：service-resolver、service-presets、secrets、cover-providers、probe、verify、provider（chat completion）、endpoints（30+ provider 配置），统一为 OpenAI-compatible 后只需保留 provider/chatCompletion、probe、verify、secrets、service-resolver（简化版）、listModelsForService。

---

## 3. 目标仓库结构

```
/workspace
├── cmd/
│   └── inkos-api/
│       └── main.go              # 入口：解析 --root / --port，启动 gin
├── internal/
│   ├── api/                     # gin router + handlers
│   │   ├── router.go            # 总装配，注册 97 个端点
│   │   ├── middleware.go        # cors、bookId 安全校验、错误处理
│   │   ├── errors.go            # ApiError → typed JSON
│   │   ├── sse.go               # SSE 广播器 (/events)
│   │   ├── handlers_books.go    # books/chapters/truth
│   │   ├── handlers_project.go  # project config
│   │   ├── handlers_services.go # LLM services
│   │   ├── handlers_cover.go    # cover
│   │   ├── handlers_genres.go   # genres
│   │   ├── handlers_play.go     # play world
│   │   ├── handlers_sessions.go # sessions
│   │   ├── handlers_agent.go    # /agent SSE
│   │   ├── handlers_daemon.go
│   │   ├── handlers_logs.go
│   │   ├── handlers_style.go
│   │   ├── handlers_fanfic.go
│   │   ├── handlers_radar.go
│   │   ├── handlers_doctor.go
│   │   └── handlers_misc.go     # interaction, project language, import
│   ├── model/                   # 所有 Go struct（替代 TS models/）
│   │   ├── book.go
│   │   ├── chapter.go
│   │   ├── project.go
│   │   ├── state.go
│   │   ├── runtime_state.go
│   │   ├── play.go
│   │   ├── detection.go
│   │   ├── style_profile.go
│   │   ├── length.go
│   │   ├── genre.go
│   │   ├── book_rules.go
│   │   └── validate.go          # 统一 validator
│   ├── store/                   # 文件 / JSON / SQLite 持久化
│   │   ├── project.go           # inkos.json + .inkos/secrets.json
│   │   ├── books.go             # books/<id>/ CRUD
│   │   ├── chapters.go          # chapters/*.md + index
│   │   ├── truth.go             # story/*.md (current_state/pending_hooks/...)
│   │   ├── state_json.go        # story/state/*.json (Zod → struct)
│   │   ├── memory_db.go         # SQLite 时序记忆（modernc.org/sqlite）
│   │   ├── play_db.go           # SQLite play 状态（兼容 play.db schema）
│   │   ├── play_file_db.go      # JSON play 状态（兼容 play-file-db）
│   │   ├── genre.go             # genres/<id>.md + profile
│   │   └── path_safety.go       # isSafeBookId 等
│   ├── llm/                     # OpenAI-compatible client
│   │   ├── client.go            # ChatCompletion(stream) + 错误重试
│   │   ├── openai.go            # OpenAI Chat Completions 请求/响应
│   │   ├── secrets.go           # 加载/保存 .inkos/secrets.json
│   │   ├── resolver.go          # 简化 service-resolver（baseUrl+key+model 即可）
│   │   ├── probe.go             # GET /v1/models 健康检查
│   │   ├── verify.go            # 兼容 transport 探测（极简）
│   │   └── types.go             # ChatMessage / Tool / Response
│   ├── agents/                  # 20 个 LLM 驱动的 agent
│   │   ├── base.go              # Agent 接口 + runWithJSON
│   │   ├── prompts/             # 各 agent 的 system prompt（直接拷 TS 源文）
│   │   ├── architect.go
│   │   ├── writer.go
│   │   ├── auditor.go
│   │   ├── reviser.go
│   │   ├── planner.go
│   │   ├── composer.go
│   │   ├── consolidator.go
│   │   ├── detector.go
│   │   ├── foundation_reviewer.go
│   │   ├── polisher.go
│   │   ├── post_write_validator.go
│   │   ├── state_validator.go
│   │   ├── style_analyzer.go
│   │   ├── short_fiction.go
│   │   ├── play_action_interpreter.go
│   │   ├── play_world_mutator.go
│   │   ├── play_scene_renderer.go
│   │   └── play_scene_reconciler.go
│   ├── pipeline/                # 编排
│   │   ├── runner.go            # PipelineRunner（plan→compose→write→audit→revise）
│   │   ├── scheduler.go
│   │   ├── chapter_persistence.go
│   │   ├── chapter_review_cycle.go
│   │   ├── chapter_state_recovery.go
│   │   ├── chapter_truth_validation.go
│   │   ├── persisted_governed_plan.go
│   │   ├── short_fiction_runner.go
│   │   └── detection_runner.go
│   ├── notify/                  # telegram / feishu / wechat-work / webhook
│   │   └── dispatcher.go
│   ├── project/                 # 项目级操作
│   │   ├── config.go            # loadProjectConfig
│   │   ├── language.go          # inferLanguage
│   │   ├── interact.go          # processProjectInteractionRequest
│   │   ├── radar.go             # 平台扫描
│   │   ├── export.go            # buildExportArtifact
│   │   ├── eval.go              # evaluateBookQuality
│   │   ├── consolidation.go
│   │   ├── chapter_split.go
│   │   ├── style.go             # analyze/import
│   │   ├── fanfic.go            # init / refresh
│   │   ├── spinoff.go
│   │   └── imitation.go
│   ├── session/                 # 会话管理
│   │   ├── store.go             # sessions.json
│   │   ├── transcript.go        # 会话历史
│   │   └── agent_session.go     # 工具调用循环
│   ├── logger/                  # 日志
│   │   ├── logger.go
│   │   └── sink.go              # 支持 SSE sink
│   ├── util/
│   │   ├── chapter_splitter.go
│   │   ├── chapter_memo_parser.go
│   │   ├── story_markdown.go
│   │   ├── outline_paths.go
│   │   ├── context_filter.go
│   │   ├── context_compression.go
│   │   ├── hook_lifecycle.go
│   │   ├── hook_arbiter.go
│   │   ├── hook_governance.go
│   │   ├── hook_health.go
│   │   ├── hook_ledger_validator.go
│   │   ├── hook_promotion.go
│   │   ├── hook_stale_detection.go
│   │   ├── governed_context.go
│   │   ├── governed_working_set.go
│   │   ├── planning_materials.go
│   │   ├── runtime_writer.go
│   │   ├── proxy_fetch.go
│   │   ├── web_search.go
│   │   ├── length_metrics.go
│   │   ├── length_normalizer.go
│   │   ├── cadence_policy.go
│   │   ├── chapter_cadence.go
│   │   ├── pov_filter.go
│   │   ├── narrative_control.go
│   │   ├── writing_methodology.go
│   │   ├── spot_fix_patches.go
│   │   ├── llm_endpoint_auth.go
│   │   ├── book_id.go
│   │   ├── book_eval.go
│   │   ├── analytics.go
│   │   ├── effective_llm_config.go
│   │   ├── llm_env.go
│   │   ├── sensitive_words.go
│   │   ├── ai_tells.go
│   │   ├── long_span_fatigue.go
│   │   ├── memory_retrieval.go
│   │   ├── config_loader.go
│   │   ├── config_migration.go
│   │   ├── secrets_migration.go
│   │   ├── secrets.go
│   │   ├── service_presets.go
│   │   ├── service_resolver.go
│   │   ├── lookup.go            # provider bank 极简版（OpenAI 默认 + custom）
│   │   ├── cover_providers.go
│   │   ├── play_image.go
│   │   ├── play_db_factory.go
│   │   └── draft_directive_parser.go
│   └── runner/                  # 顶层执行
│       ├── daemon.go            # /daemon/start 后台循环
│       └── doctor.go
├── go.mod
├── go.sum
├── README.md                    # 重写：API-only 文档
├── LICENSE                      # 保留 AGPL-3.0
├── .gitignore
├── .github/
│   └── workflows/
│       └── ci.yml               # go build + go test
└── .env.example                 # 保留 INKOS_LLM_* 模板
```

---

## 4. 实施阶段

### Phase 0：清理与重置（0.5 天）

1. 删除 `packages/cli/`、`packages/studio/src/components/`、`packages/studio/src/pages/`、`packages/studio/src/store/`、`packages/studio/src/hooks/`、`packages/studio/src/constants/`、`packages/studio/src/shared/`、`packages/studio/index.html`、`packages/studio/vite.config.ts`、`packages/studio/components.json`、`packages/studio/tailwind` 残留。
2. 删除根目录 `pnpm-workspace.yaml`、`pnpm-lock.yaml`、`package.json`（重建为 `go.mod` 友好的 `package.json` 或直接删）、`.nvmrc`、`.node-version`、根 `tsconfig.json`。
3. 删除 `scripts/`、`assets/`、`skills/SKILL.md`、`.github/ISSUE_TEMPLATE/`、`README.en.md`、`README.ja.md`、`CHANGELOG.md`、`CONTRIBUTING.md`。
4. 保留：`packages/core/src/`（作为 Go 实现参考，不再运行）、`LICENSE`、`.env.example`。
5. 初始化 `go.mod`：`go mod init github.com/narcooo/inkos`，go 1.22+。
6. 添加 `gin`、`modernc.org/sqlite`、`validator`、`yaml.v3`、`gopkg.in/yaml.v3` 等基础依赖。

### Phase 1：骨架与基础（2 天）

1. **`cmd/inkos-api/main.go`**：解析 `--root` / `--port` / `--host`，加载 `inkos.json`，构造 `*gin.Engine`，监听。
2. **`internal/api/router.go`**：装配 CORS、SSE broadcaster、error middleware、bookId middleware。
3. **`internal/api/middleware.go`** + **`errors.go`**：等价于 `app.use("/*", cors())` 和 `app.onError()`。
4. **`internal/api/sse.go`**：等价于 `lib/run-store.ts` 的 broadcast 机制。维护 `map[chan Event]struct{}`，提供 `Broadcast(event, data)`。
5. **`internal/store/project.go`** + **`secrets.go`**：读 `inkos.json`（ProjectConfig）和 `.inkos/secrets.json`，写回带文件锁。
6. **`internal/llm/client.go`** + **`openai.go`**：实现 `ChatCompletion(ctx, req, opts) (*Response, error)` 和 `ChatCompletionStream(ctx, req, opts) (<-chan Chunk, error)`。`opts` 包含 `baseURL`、`apiKey`、`model`、`temperature`、`maxTokens`。
7. **`internal/llm/resolver.go`**：极简版 service-resolver。接受 `service` 名（`openai` / `custom` / 用户存的），从 secrets 找 key，构造 client。**不实现** 30+ provider bank；只保留 `openai`（`https://api.openai.com/v1`）和 `custom`（用户填 baseUrl），用户要别的服务一律走 `custom`。
8. **`internal/llm/probe.go`** + **`verify.go`**：`GET {baseURL}/models` 测试连通性。
9. **`internal/util/path_safety.go`** + **`book_id.go`**：等价于 `isSafeBookId`、`normalizeApiBookId`。
10. 烟雾测试：`cmd/inkos-api` 启动，`curl http://localhost:4567/api/v1/services` 拿到空配置列表，`curl /api/v1/project` 拿到 `inkos.json`。

### Phase 2：模型与状态层（3 天）

1. **`internal/model/`**：逐个翻译 TS model → Go struct：
   - `book.go`：`BookConfig`、`Platform`、`Genre`、`BookStatus`、`FanficMode`
   - `chapter.go`：`ChapterMeta`、`ChapterStatus`
   - `project.go`：`ProjectConfig`、`LLMConfig`、`NotifyChannel`、`DetectionConfig`、`QualityGates`、`FoundationConfig`、`WritingConfig`、`AgentLLMOverride`、`InputGovernanceMode`
   - `state.go`：`CurrentState`、`ParticleLedger`、`PendingHooks`、`PendingHook`、`LedgerEntry`
   - `runtime_state.go`：`RuntimeStateDelta`、`HooksState`、`ChapterSummariesState`、`StateManifest`（全部 Zod schema → Go struct + validator tag）
   - `play.go`：`PlayEntity`、`PlayEdge`、`PlayStateSlot`、`PlayEvidenceTransition`、`PlayEvent`、`PlayMutation`（Zod → struct）
   - `detection.go`：`DetectionHistoryEntry`、`DetectionStats`
   - `style_profile.go`、`length.go`、`book_rules.go`、`genre_profile.go`
2. **`internal/validate.go`**：基于 `validator/v10` 注册自定义 validator（enum 严格匹配 `Platform` / `Genre` / `Status`）。
3. **`internal/store/books.go`**：`listBooks` / `loadBookConfig` / `bookDir` / `chapterDir` / `stateDir` / `getNextChapterNumber`。**完全沿用 TS 路径规则**：`books/<id>/book.json`、`books/<id>/chapters/`、`books/<id>/story/`。
4. **`internal/store/chapters.go`**：`loadChapterIndex` / `readChapter` / `writeChapter` / `listChaptersByStatus`。
5. **`internal/store/truth.go`**：`readTruthFile` / `writeTruthFile`，路径 `story/<file>.md`，白名单字段（`current_state.md`、`pending_hooks.md`、`chapter_summaries.md`、`character_matrix.md` 等）。
6. **`internal/store/state_json.go`**：`loadRuntimeState` / `applyRuntimeStateDelta` / `saveRuntimeState`（等价于 TS `runtime-state-store.ts`）。
7. **`internal/store/memory_db.go`** + **`play_db.go`**：用 `modernc.org/sqlite` 建表，沿用 TS 的 schema（`memory.db` 的 facts/hooks/summaries 表，`play.db` 的 runs/entities/edges/slots/events/mutations 表）。
8. **`internal/store/play_file_db.go`**：JSON 模式（无 SQLite 时）。
9. **`internal/store/genre.go`**：`listAvailableGenres`、`readGenreProfile`（读 `packages/core/genres/<id>.md` 的 frontmatter）。
10. **测试**：用现有 `inkos.json` + 至少一本现有 book 跑 `GET /api/v1/books` 与 `GET /api/v1/books/:id`，结果与原项目一致。

### Phase 3：LLM agent 重写（5 天）

1. **`internal/agents/base.go`**：
   - `type Agent interface { Run(ctx, input) (output, error) }`
   - `runWithJSON[T any](ctx, client, systemPrompt, userPrompt, schemaExample) (T, error)`：调 LLM 拿到文本，按 markdown fence 切出 JSON，做 struct unmarshal + validator 校验，失败自动重试 1 次。
   - `parseToolCall(text) (name string, args map, ok bool)`：用于 agent session。
2. **`internal/agents/prompts/`**：从 `packages/core/src/agents/*.ts` 提取所有 `XX_PROMPT` / `buildPrompt` 函数，翻译为 Go `const` / `func`，**prompt 文本逐字保留**（包括中文、规则、禁忌、风格指南）。
3. **逐个 agent**：
   - `architect.go`：`story_bible.md` + `book_rules.md` + `author_intent.md` 生成
   - `writer.go`：章节正文 + 字数治理 + length normalizer
   - `auditor.go`：37 维度审计，返回结构化 findings
   - `reviser.go`：基于 audit findings 修订
   - `planner.go`：`chapter-XXXX.intent.md`（must-keep / must-avoid）
   - `composer.go`：`context.json` + `rule-stack.yaml` + `trace.json`
   - `consolidator.go`：合并章节摘要
   - `detector.go`：AIGC 检测（高频词、句式）
   - `foundation_reviewer.go`：基础设定审阅
   - `polisher.go`、`post_write_validator.go`、`state_validator.go`
   - `style_analyzer.go`：句长/词频指纹
   - `short_fiction.go`：完整短篇（含 sales-package / cover-prompt）
   - 4 个 play agent（action_interpreter / world_mutator / scene_renderer / scene_reconciler）
4. **`internal/util/`** 中各 helper（chapter_splitter、chapter_memo_parser、pov_filter、length_metrics、length_normalizer、hook_lifecycle、hook_arbiter、hook_governance、hook_health、hook_ledger_validator、hook_promotion、hook_stale_detection、ai_tells、long_span_fatigue、sensitive_words、spot_fix_patches、narrative_control、writing_methodology、governed_context、governed_working_set、planning_materials、context_compression、context_filter、runtime_writer、outline_paths、chapter_cadence、cadence_policy、proxy_fetch、web_search、llm_endpoint_auth、book_eval、analytics、effective_llm_config、llm_env、config_loader、config_migration、secrets_migration、draft_directive_parser、cover_providers 等）：逐个翻译。
5. **关键测试**：
   - `writer` 调一个真实 baseURL，验证能产出章节文本。
   - `auditor` 用 mock LLM 验证 37 维度输出结构。
   - `applyRuntimeStateDelta` 单元测试（与 TS 已有 vitest 对照）。

### Phase 4：Pipeline 与项目编排（3 天）

1. **`internal/pipeline/runner.go`**：等价于 `runner.ts` 的 `PipelineRunner`。`Run(ctx, book, stage) (Result, error)`，状态机覆盖 `plan→compose→write→audit→revise`，重试由 `writing.reviewRetries` 控制。
2. **`internal/pipeline/scheduler.go`**：daemon 调度的章节选择。
3. **`internal/pipeline/chapter_persistence.go`** + **`chapter_review_cycle.go`** + **`chapter_state_recovery.go`** + **`chapter_truth_validation.go`** + **`persisted_governed_plan.go`**。
4. **`internal/pipeline/short_fiction_runner.go`** + **`detection_runner.go`**。
5. **`internal/project/`**：
   - `config.go`：`loadProjectConfig(root, opts)`（含 env 覆盖、migration、effective-llm-config）。
   - `interact.go`：`processProjectInteractionRequest`，把 `intent` 路由到具体工具。
   - `radar.go`：平台扫描（web_search 调用 → 趋势报告）。
   - `export.go`：`buildExportArtifact`（txt/md/epub）。
   - `eval.go`：`evaluateBookQuality`。
   - `consolidation.go`、`chapter_split.go`、`style.go`、`fanfic.go`、`spinoff.go`、`imitation.go`。
6. **`internal/notify/dispatcher.go`**：telegram/feishu/wechat-work/webhook（HMAC-SHA256 签名）。
7. **`internal/session/`**：
   - `store.go`：`sessions.json` 增删改查。
   - `transcript.go`：会话历史读写。
   - `agent_session.go`：`runAgentSession`，暴露工具循环（read / edit / grep / ls / propose_action / sub_agent / play_start / play_step / ...）。

### Phase 5：HTTP handlers — 第一批（核心 CRUD + project + services）（3 天）

1. **`handlers_books.go`**：
   - `GET /api/v1/books`
   - `GET /api/v1/books/:id`
   - `POST /api/v1/books/create`（SSE 进度 + `bookCreateStatus`）
   - `GET /api/v1/books/:id/create-status`
   - `DELETE /api/v1/books/:id`
   - `PUT /api/v1/books/:id`
2. **`handlers_chapters.go`**（或合并到 books）：
   - `GET /api/v1/books/:id/chapters/:num`
   - `PUT /api/v1/books/:id/chapters/:num`
   - `POST /api/v1/books/:id/chapters/:num/approve`
   - `POST /api/v1/books/:id/chapters/:num/reject`
3. **`handlers_truth.go`**：
   - `GET /api/v1/books/:id/truth`
   - `GET /api/v1/books/:id/truth/:file`
   - `PUT /api/v1/books/:id/truth/:file`
4. **`handlers_project.go`**：
   - `GET /api/v1/project`
   - `PUT /api/v1/project`
   - `GET /api/v1/project/files/:file`
   - `GET/PUT /api/v1/project/input-governance-mode`
   - `GET/PUT /api/v1/project/detection`
   - `GET/PUT /api/v1/project/model-overrides`
   - `GET/PUT /api/v1/project/chapter-review-mode`
   - `GET/PUT /api/v1/project/notify`
   - `POST /api/v1/project/language`
5. **`handlers_services.go`**：
   - `GET /api/v1/services`
   - `GET /api/v1/services/config`
   - `POST /api/v1/services/config/import-env`
   - `PUT /api/v1/services/config`
   - `GET /api/v1/services/:service/secret`
   - `PUT /api/v1/services/:service/secret`
   - `DELETE /api/v1/services/:service`
   - `POST /api/v1/services/:service/test`（probe + chat fallback）
   - `GET /api/v1/services/models`
   - `GET /api/v1/services/models/custom`
   - `GET /api/v1/services/:service/models`
6. **`handlers_cover.go`**：
   - `GET /api/v1/cover/config`
   - `PUT /api/v1/cover/config`
   - `GET /api/v1/cover/secret/:service`
   - `PUT /api/v1/cover/secret/:service`
7. **`handlers_genres.go`**：
   - `GET /api/v1/genres`
   - `GET /api/v1/genres/:id`
   - `POST /api/v1/genres/:id/copy`
   - `POST /api/v1/genres/create`
   - `PUT /api/v1/genres/:id`
   - `DELETE /api/v1/genres/:id`
8. **冒烟**：用现有 `inkos.json` 和一本 book，依次 curl 以上端点，响应 JSON 与原项目 `inkos studio` 等价。

### Phase 6：HTTP handlers — 第二批（agent + 流水线 + session + SSE）（3 天）

1. **`handlers_agent.go`**：
   - `POST /api/v1/agent`（SSE 流，回放 tool execs / final text）
2. **`handlers_events.go`**：
   - `GET /api/v1/events`（SSE 广播）
3. **`handlers_sessions.go`**：
   - `GET /api/v1/sessions`
   - `GET /api/v1/sessions/:sessionId`
   - `POST /api/v1/sessions`
   - `PUT /api/v1/sessions/:sessionId/play-mode`
   - `PUT /api/v1/sessions/:sessionId`
   - `DELETE /api/v1/sessions/:sessionId`
4. **`handlers_interaction.go`**：
   - `GET /api/v1/interaction/session`
5. **`handlers_daemon.go`**：
   - `GET /api/v1/daemon`
   - `POST /api/v1/daemon/start`
   - `POST /api/v1/daemon/stop`
6. **`handlers_logs.go`**：
   - `GET /api/v1/logs`（`inkos.log` JSONL 倒序分页）
7. **流水线端点**（在 `handlers_books.go` 或独立 `handlers_pipeline.go`）：
   - `POST /api/v1/books/:id/write-next`
   - `POST /api/v1/books/:id/draft`
   - `POST /api/v1/books/:id/plan`
   - `POST /api/v1/books/:id/compose`
   - `POST /api/v1/books/:id/audit/:chapter`
   - `POST /api/v1/books/:id/revise/:chapter`
   - `POST /api/v1/books/:id/rewrite/:chapter`
   - `POST /api/v1/books/:id/resync/:chapter`
   - `POST /api/v1/books/:id/repair-state/:chapter`
   - `POST /api/v1/books/:id/foundation/revise`
   - `POST /api/v1/books/:id/consolidate`
   - `GET /api/v1/books/:id/eval`
   - `GET /api/v1/books/:id/analytics`
   - `GET /api/v1/books/:id/export`
   - `POST /api/v1/books/:id/export-save`
8. **测试**：用 mock LLM 跑通 `write-next` 完整链路 → `book/<id>/chapters/0001.md` 落盘 + `story/state/*.json` 同步 + `audit` 通过。

### Phase 7：HTTP handlers — 第三批（play / 风格 / 同人 / radar / doctor / import / 检测）（3 天）

1. **`handlers_play.go`**：
   - `GET /api/v1/play/runs/:worldId/:runId`
   - `PUT /api/v1/play/runs/:worldId/:runId/image-settings`
   - `POST /api/v1/play/runs/:worldId/:runId/generate-image`
   - `GET /api/v1/play/runs/:worldId/:runId/images/:file`
2. **`handlers_style.go`**：
   - `POST /api/v1/style/analyze`
   - `POST /api/v1/books/:id/style/import`
3. **`handlers_fanfic.go`**：
   - `POST /api/v1/fanfic/init`
   - `GET /api/v1/books/:id/fanfic`
   - `POST /api/v1/books/:id/fanfic/refresh`
   - `POST /api/v1/spinoff/init`
   - `POST /api/v1/imitation/init`
4. **`handlers_import.go`**：
   - `POST /api/v1/books/:id/import/chapters`
   - `POST /api/v1/books/:id/import/canon`
5. **`handlers_radar.go`**：
   - `POST /api/v1/radar/scan`
   - `GET /api/v1/radar/history`
6. **`handlers_doctor.go`**：
   - `GET /api/v1/doctor`
7. **`handlers_detection.go`**：
   - `POST /api/v1/books/:id/detect/:chapter`
   - `POST /api/v1/books/:id/detect-all`
   - `GET /api/v1/books/:id/detect/stats`
8. **测试**：每类功能至少一个 happy-path 端到端测试。

### Phase 8：测试与验证（2 天）

1. **单元测试**：把 `packages/core/src/__tests__/` 中关键 vitest 用例翻译到 Go：
   - `agent-session.test.ts` → `agents/agent_session_test.go`
   - `book-session.test.ts` → `session/book_session_test.go`
   - `chapter-persistence.test.ts` → `pipeline/chapter_persistence_test.go`
   - `chapter-state-recovery.test.ts` → `pipeline/chapter_state_recovery_test.go`
   - `chapter-truth-validation.test.ts` → `pipeline/chapter_truth_validation_test.go`
   - `pipeline-runner.test.ts` → `pipeline/runner_test.go`
   - `play-reducer.test.ts` → `play/play_reducer_test.go`
   - `runtime-state-store.test.ts` → `state/runtime_state_store_test.go`
   - `state-validator.test.ts` → `state/state_validator_test.go`
   - `provider.test.ts` → `llm/provider_test.go`
   - `service-resolver.test.ts` → `llm/resolver_test.go`
   - `secrets.test.ts` → `llm/secrets_test.go`
2. **集成测试**：`internal/api/router_test.go` 用 `httptest` 启 gin，跑通 20 个代表性端点。
3. **数据兼容测试**：用现有 `inkos.json` + 一本书作为 fixture，加载、列出、读取章节、跑一次 `plan`，所有路径与 JSON 与原项目一致。
4. **真实 LLM 烟雾测试**（可选）：在 `INKOS_LLM_BASE_URL` 指向测试模型时，`POST /write-next` 跑通一整章。
5. **CI**：`.github/workflows/ci.yml` 跑 `go vet ./...` + `go test ./...` + `go build ./...`。

### Phase 9：文档与发布（1 天）

1. **README.md**：重写为 API-only 文档，列出全部 97 个端点、curl 示例、配置项（`--root`、`--port`、`INKOS_LLM_*` 环境变量）、OpenAI-compatible 配置说明。
2. **CHANGELOG.md**：`2.0.0` 标注「重写为 Go API，删除 CLI/TUI/Studio 前端」。
3. **删除 TS 残留**：`packages/core/src/`、`packages/core/package.json` 不再保留（Go 实现不再需要它们）。若需保留作为 Go 实现参考，可移到 `legacy/ts-reference/` 并在 README 标注。
4. **LICENSE** 保留 AGPL-3.0。

---

## 5. 关键设计决策

### 5.1 LLM 协议统一为 OpenAI-compatible

- 单一 `internal/llm/client.go` 直接 POST `{baseURL}/chat/completions`（含 `stream: true`）。
- 服务配置形如：
  ```json
  {
    "openai": { "baseURL": "https://api.openai.com/v1", "apiKey": "sk-...", "defaultModel": "gpt-4o" },
    "moonshot": { "baseURL": "https://api.moonshot.cn/v1", "apiKey": "sk-...", "defaultModel": "kimi-k2.5" },
    "minimax": { "baseURL": "https://api.minimax.chat/v1", "apiKey": "sk-...", "defaultModel": "MiniMax-M2.7" }
  }
  ```
- 不再需要 provider bank、模型卡、自动 transport 探测。用户手动确保 baseURL 兼容。
- `probe.go` 仅做 `GET {baseURL}/models` 健康检查，输出模型列表让用户在前端/CLI 选。

### 5.2 数据兼容策略

- 路径完全沿用：`books/<id>/book.json`、`books/<id>/chapters/NNNN.md`、`books/<id>/story/*.md`、`books/<id>/story/state/*.json`、`books/<id>/story/runtime/chapter-XXXX.*`、`books/<id>/story/memory.db`、`books/<id>/play.db` 或 `books/<id>/play.json`。
- JSON 字段名使用 `json:"snake_case"` tag，与 TS camelCase 一一对应（zod schema 字段名 → Go struct json tag）。
- `inkos.json`、`.inkos/secrets.json` 同上。
- SQLite schema 直接照搬 `packages/core/src/state/memory-db.ts` 和 `play-db.ts` 的 `CREATE TABLE` 语句，字段名保持一致。

### 5.3 SSE 流式

- gin 用 `c.SSEvent(name, data)` + `c.Stream(...)` 发送。
- broadcast hub：`internal/api/sse.go` 维护 `subs map[*gin.Context]struct{}` + `sync.RWMutex`。
- `/agent` 与 `/events` 共用 hub；`sessionId` 字段在 `runAgentSession` 时通过 opt 注入 logger sink。

### 5.4 Agent JSON 解析

- 每个 agent prompt 末尾追加 `Return JSON in a single \`\`\`json ... \`\`\` block.`
- `runWithJSON[T]` 解析 fence → `json.Unmarshal` → `validator.Struct` → 失败重试 1 次并把错误信息回灌给 LLM。
- 关键 schema（writer / auditor / reviser / planner / composer / play agents）需要与 TS 输出 1:1 对齐，否则状态 delta 写入会失败。

### 5.5 文件锁与并发

- TS 用 `proper-lockfile`。Go 用 `golang.org/x/sync/singleflight` + 文件级 `flock`（`github.com/gofrs/flock`）做并发写入保护。
- `bookCreateStatus` 用 `sync.Map`。

### 5.6 测试策略

- 不追求覆盖率指标，重点覆盖：状态机、JSON delta 解析、路径安全、bookId 校验、文件读写幂等性、SQLite schema 兼容。
- 真实 LLM 端到端测试放 `-tags=integration`，默认 skip。

---

## 6. 假设与风险

### 假设

1. **Prompt 文本可直接复用**：原 TS agent 的 prompt（中英文 + Markdown）是给 LLM 看的，Go 端无需重写。
2. **JSON 字段名稳定**：原 zod schema 用 snake_case，Go `json:"snake_case"` tag 一一对应即可。
3. **SQLite schema 可照搬**：modernc.org/sqlite 完整支持原 DDL（含 `WITHOUT ROWID`、`json_extract`）。
4. **现有项目数据无需迁移**：`books/<id>/` 目录里所有 JSON / MD / SQLite 都能被新 Go 服务直接读写。
5. **30+ provider 适配确实可砍**：用户接受统一走 OpenAI-compatible，自己配置 baseURL。

### 风险

| 风险 | 等级 | 缓解 |
| --- | --- | --- |
| 132K 行 TS 移植量大、时间长 | 高 | 分 9 阶段，每阶段独立可运行；优先 Phase 1-6 让核心 CRUD + 流水线跑通 |
| Zod schema 复杂，部分约束（如 `refine`）难翻译 | 中 | 关键 schema 必翻；次要校验用 `validator` 标签兜底，复杂规则单写函数 |
| LLM JSON 输出不稳定导致 agent 失败 | 中 | 现有 prompt 已有「必须返回 JSON fence + 失败重试」机制，保留 |
| Play DB SQLite 双模式（file/SQLite）实现复杂 | 中 | 先实现 SQLite 一套；JSON 模式用同样 schema 落到 `play.json`，reducer 共用 |
| 路径安全/并发竞争等隐藏 bug | 中 | 路径安全单元测试 + `flock` 锁 + 与原项目数据 diff 验证 |
| SSE 断线重连 | 低 | gin stream 在 `ctx.Done()` 时自动关闭；前端重连逻辑不归本服务管 |
| 第三方 provider 协议差异（Anthropic / Gemini 原生） | 低 | 文档说明「请用 OpenAI-compatible 端点」；如必要可后续加 adapter，但不在初版 |

---

## 7. 验证步骤

执行每个阶段后，对照下列清单验证：

### Phase 1
- [ ] `go run ./cmd/inkos-api --root .` 启动成功，监听 4567。
- [ ] `curl localhost:4567/api/v1/services` 返回 `{services:[]}` 或已有 services。
- [ ] `curl localhost:4567/api/v1/project` 返回 `inkos.json` 内容。

### Phase 2
- [ ] `curl localhost:4567/api/v1/books` 列出与 TS 版相同的 book ids。
- [ ] `curl localhost:4567/api/v1/books/<id>` 返回 book.json + chapter index。
- [ ] `curl localhost:4567/api/v1/genres` 返回内置 + 用户自定义 genre。

### Phase 3
- [ ] `go test ./internal/agents/...` 全部通过。
- [ ] 用 mock LLM 跑 `writer.Run` 拿到 ≥ N 字章节文本。
- [ ] `auditor.Run` 返回的 JSON 与 TS 37 维度字段一致。

### Phase 4
- [ ] 跑通 `PipelineRunner.Run` mock 完整链路（plan→write→audit→revise）。
- [ ] `export.Book` 生成 `final.txt` / `final.md` / `final.epub` 三种格式。

### Phase 5
- [ ] 全部 Phase 5 端点 curl 验证 200/4xx 行为。
- [ ] 修改 `inkos.json` → 重新 `GET /project` 看到更新。

### Phase 6
- [ ] `POST /api/v1/agent` 走通 SSE 流，工具调用 + 最终 text 完整。
- [ ] `POST /api/v1/books/<id>/write-next` 跑通一整章，章节文件落盘。
- [ ] `POST /api/v1/daemon/start` 后台循环写日志，daemon/stop 终止。

### Phase 7
- [ ] Play `GET /play/runs/.../...` 拿到 world state。
- [ ] `POST /fanfic/init` + `GET /books/<id>/fanfic` 跑通同人初始化。
- [ ] `GET /doctor` 报告 effective config + connectivity。

### Phase 8
- [ ] `go test ./...` 全部通过。
- [ ] CI 流水线绿。
- [ ] 用现有真实项目 fixture 跑全量端到端，无 panic / 无 500。

### Phase 9
- [ ] README 列全 97 端点 + curl 示例。
- [ ] `go build -o inkos-api ./cmd/inkos-api` 生成单二进制。
- [ ] `INKOS_LLM_BASE_URL=... INKOS_LLM_API_KEY=... ./inkos-api --root /path/to/project` 启动后 `curl localhost:4567/api/v1/books` 正常。

---

## 8. 总结

把 InkOS 改造为 Go API 项目的核心动作：

1. **删除**：CLI、TUI、Studio 前端、pnpm、Node 工具链（`packages/cli/`、`packages/studio/src/{components,pages,store,hooks,constants,shared}`、`assets/`、`scripts/`、`pnpm-*`）。
2. **保留并翻译**：业务模型（`packages/core/src/models/`）、agent prompt（`packages/core/src/agents/`）、pipeline（`packages/core/src/pipeline/`）、项目编排（`packages/core/src/interaction/`）、state/Play DB schema。
3. **简化**：30+ provider 适配 → 单一 OpenAI-compatible client；Zod → validator；better-sqlite3 → modernc.org/sqlite；Hono → gin。
4. **完整实现**：97 个 REST + SSE 端点，与现有项目数据（`books/<id>/`、`inkos.json`、`secrets.json`、SQLite）100% 兼容。

预计工作量 ~22 天（单人），分 9 阶段交付；每个阶段结束都能跑通一部分端点，直到最终全量等价。
