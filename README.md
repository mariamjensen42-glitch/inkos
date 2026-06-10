# inkos-api

A single-binary Go HTTP API that powers the InkOS novel-writing pipeline.
Reads and writes the same `inkos.json`, `books/<id>/`, `story/*.md`,
`memory.db`, and `play.db` artifacts the original TypeScript InkOS
monorepo used — but exposes them as a lean REST + SSE service with no
front-end, no CLI, and no TUI.

## Highlights

- **97 REST + 2 SSE endpoints** covering books, chapters, truth files,
  project config, LLM services, cover, genres, sessions, agent
  sessions, daemon, logs, fanfic/spinoff/imitation, radar, and the
  doctor diagnostic.
- **OpenAI-compatible LLM client**. Configure any base URL + API key +
  model pair in `inkos.json`; InkOS does not care which vendor you use.
- **Data-compatible** with the original InkOS file layout.
- **Single binary**, no Node.js, no Python, no external services.
- **AES-256** safe book-id handling, file-locked JSON writes, and a
  pure-Go SQLite driver (`modernc.org/sqlite`).

## Quickstart

```bash
# Build
go build -o inkos-api ./cmd/inkos-api

# Run against a project directory
./inkos-api --root /path/to/project --port 4567
```

Test:

```bash
curl http://localhost:4567/api/v1/health
curl http://localhost:4567/api/v1/project
curl http://localhost:4567/api/v1/books
```

## Configuration

`inkos.json` (in the project root) controls the LLM, governance, and
notification settings. API keys live in `.inkos/secrets.json` (mode
`0600`).

```json
{
  "name": "my-project",
  "language": "zh",
  "llm": {
    "provider": "openai",
    "baseUrl": "https://api.openai.com/v1",
    "model": "gpt-4o"
  },
  "services": {
    "moonshot": {
      "provider": "custom",
      "baseUrl": "https://api.moonshot.cn/v1",
      "model": "kimi-k2.5"
    }
  },
  "currentService": "moonshot",
  "writing": { "reviewRetries": 2, "reviewMode": "auto" },
  "inputGovernanceMode": "v2"
}
```

`.inkos/secrets.json`:

```json
{
  "services": {
    "moonshot": { "apiKey": "sk-..." },
    "openai":   { "apiKey": "sk-..." }
  }
}
```

Environment variables are also honored:

- `INKOS_LLM_PROVIDER`
- `INKOS_LLM_SERVICE`
- `INKOS_LLM_BASE_URL`
- `INKOS_LLM_MODEL`

## API Surface

All endpoints are mounted under `/api/v1`.

### Project

| Method | Path | Description |
| --- | --- | --- |
| GET / PUT | `/project` | Full project config (inkos.json) |
| POST | `/project/language` | Set `language` field |
| GET / PUT | `/project/input-governance-mode` | `v2` or `legacy` |
| GET / PUT | `/project/detection` | AIGC detection config |
| GET / PUT | `/project/model-overrides` | Per-agent model overrides |
| GET / PUT | `/project/chapter-review-mode` | `auto` or `manual` |
| GET / PUT | `/project/notify` | Notification channels |
| GET | `/project/files/*file` | Static file under `shorts/` or `covers/` |

### Books

| Method | Path | Description |
| --- | --- | --- |
| GET | `/books` | List books |
| POST | `/books/create` | Create book + start architect |
| GET | `/books/:id` | Book config + chapter index |
| PUT | `/books/:id` | Patch book config |
| DELETE | `/books/:id` | Delete book |
| GET | `/books/:id/create-status` | Architect progress |
| GET | `/books/:id/analytics` | Aggregated stats |
| GET | `/books/:id/eval` | Quality eval summary |
| GET / POST | `/books/:id/export` (+ `?format=md\|txt`) | Export approved chapters |
| POST | `/books/:id/export-save` | Save export to disk |
| GET / PUT | `/books/:id/chapters/:num` | Read/write a chapter file |
| POST | `/books/:id/chapters/:num/approve` | Mark approved |
| POST | `/books/:id/chapters/:num/reject` | Mark rejected |

### Truth files

| Method | Path | Description |
| --- | --- | --- |
| GET | `/books/:id/truth` | List whitelisted truth files |
| GET / PUT | `/books/:id/truth/*file` | Read/write one file |

Allowed files: `story_bible.md`, `book_rules.md`, `author_intent.md`,
`current_focus.md`, `current_state.md`, `pending_hooks.md`,
`chapter_summaries.md`, `character_matrix.md`, `volume_outline.md`,
`style_profile.md`.

### Pipeline

| Method | Path | Description |
| --- | --- | --- |
| POST | `/books/:id/write-next` | Plan → compose → write → audit → revise |
| POST | `/books/:id/draft` | Write-only (skip audit/revise) |
| POST | `/books/:id/plan` | Generate chapter intent |
| POST | `/books/:id/compose` | Assemble context |
| POST | `/books/:id/audit/:chapter` | Multi-dimensional audit |
| POST | `/books/:id/revise/:chapter` | Apply audit fixes |
| POST | `/books/:id/rewrite/:chapter` | Rewrite from scratch |
| POST | `/books/:id/resync/:chapter` | Rebuild truth from state |
| POST | `/books/:id/repair-state/:chapter` | Repair chapter state |
| POST | `/books/:id/foundation/revise` | Re-run architect |
| POST | `/books/:id/consolidate` | Merge chapter summaries |
| POST | `/books/:id/detect/:chapter` | AIGC detection (1 chapter) |
| POST | `/books/:id/detect-all` | AIGC detection (all) |
| GET | `/books/:id/detect/stats` | Aggregated detection stats |
| POST | `/books/:id/style/import` | Import a style profile |
| POST | `/books/:id/import/chapters` | Bulk import chapters |
| POST | `/books/:id/import/canon` | Import parent-book canon |
| GET | `/books/:id/fanfic` | Fanfic metadata |
| POST | `/books/:id/fanfic/refresh` | Refresh fanfic foundation |

### LLM services

| Method | Path | Description |
| --- | --- | --- |
| GET | `/services` | List configured services |
| GET / PUT | `/services/config` | Full services config |
| POST | `/services/config/import-env` | Import `INKOS_LLM_*` env |
| GET / PUT | `/services/:service/secret` | Per-service API key |
| DELETE | `/services/:service` | Remove service |
| POST | `/services/:service/test` | Probe (`GET /models`) + chat |
| GET | `/services/:service/models` | Probe models |
| GET | `/services/models` | Default service models |
| GET | `/services/models/custom` | Custom (user-defined) models |

### Cover / Genres

| Method | Path | Description |
| --- | --- | --- |
| GET / PUT | `/cover/config` | Cover service config |
| GET / PUT | `/cover/secret/:service` | Cover API key |
| GET / POST / PUT / DELETE | `/genres[/...]` | Genre CRUD + copy |

### Sessions / Agent / SSE

| Method | Path | Description |
| --- | --- | --- |
| GET / POST / PUT / DELETE | `/sessions[/...]` | Session CRUD |
| GET | `/interaction/session` | Current interaction session |
| POST | `/agent` | Agent session (SSE) |
| GET | `/events` | Global event stream (SSE) |

### Play (interactive world)

| Method | Path | Description |
| --- | --- | --- |
| GET | `/play/runs/:worldId/:runId` | World state |
| PUT | `/play/runs/.../image-settings` | Image-gen settings |
| POST | `/play/runs/.../generate-image` | Generate image |
| GET | `/play/runs/.../images/:file` | Fetch generated image |

### Daemon / Logs

| Method | Path | Description |
| --- | --- | --- |
| GET | `/daemon` | Daemon status |
| POST | `/daemon/start` | Start daemon |
| POST | `/daemon/stop` | Stop daemon |
| GET | `/logs` | Recent log lines |

### Style / Fanfic / Radar / Doctor

| Method | Path | Description |
| --- | --- | --- |
| POST | `/style/analyze` | Extract style profile from text |
| POST | `/fanfic/init` | Create fanfic book |
| POST | `/spinoff/init` | Create spinoff book |
| POST | `/imitation/init` | Create imitation book |
| POST | `/radar/scan` | Platform trend scan |
| GET | `/radar/history` | Past scans |
| GET | `/doctor` | Diagnostic dump |

## LLM compatibility

Any service speaking the OpenAI Chat Completions protocol is supported:

- `https://api.openai.com/v1`
- `https://api.moonshot.cn/v1`
- `https://api.deepseek.com/v1`
- `https://openrouter.ai/api/v1`
- `https://api.mistral.ai/v1`
- `https://generativelanguage.googleapis.com/v1beta/openai/`
- `https://api.together.xyz/v1`
- `http://localhost:11434/v1` (Ollama)
- Any OpenAI-compatible proxy (kkaiapi, kimi, etc.)

## Development

```bash
go vet ./...
go test ./...
go build -o inkos-api ./cmd/inkos-api
```

## License

AGPL-3.0
