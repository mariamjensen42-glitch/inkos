# 7. Configuration

Configuration is loaded from a YAML file (recommended) or a legacy JSON file. There are also environment variables and CLI flags. The precedence is **CLI > environment > config file > defaults**.

## 7.1 Files

- `trae_config.yaml.example` — recommended, modern schema.
- `trae_config.json.example` — legacy schema (still parsed via `LegacyConfig`).
- `trae_agent/utils/config.py` — `Config` and friends.
- `trae_agent/utils/legacy_config.py` — backward-compatible JSON loader.

## 7.2 YAML Schema (Recommended)

```yaml
agents:
  trae_agent:
    enable_lakeview: true
    model: trae_agent_model
    max_steps: 200
    tools:
      - bash
      - str_replace_based_edit_tool
      - sequentialthinking
      - task_done

allow_mcp_servers:
  - playwright

mcp_servers:
  playwright:
    command: npx
    args:
      - "@playwright/mcp@0.0.27"

lakeview:
  model: lakeview_model

model_providers:
  anthropic:
    api_key: your_anthropic_api_key
    provider: anthropic
  openai:
    api_key: your_openai_api_key
    provider: openai

models:
  trae_agent_model:
    model_provider: anthropic
    model: claude-sonnet-4-20250514
    max_tokens: 4096
    temperature: 0.5
  lakeview_model:
    model_provider: anthropic
    model: claude-3.5-sonnet
    max_tokens: 4096
    temperature: 0.5
```

## 7.3 Field Reference

### `model_providers.<name>`

| Field | Required | Notes |
|---|---|---|
| `api_key` | required | Per-provider key. |
| `provider` | required | One of: `openai`, `anthropic`, `azure`, `ollama`, `openrouter`, `doubao`, `google`. |
| `base_url` | optional | Override the default endpoint. Useful for OpenRouter, Azure, and proxies. |
| `api_version` | optional | Required for Azure. |

### `models.<name>`

| Field | Required | Notes |
|---|---|---|
| `model_provider` | required | Reference to a key in `model_providers`. |
| `model` | required | Model name (e.g. `claude-sonnet-4-20250514`, `gpt-4o`). |
| `max_tokens` | optional | Defaults to 4096 if neither this nor `max_completion_tokens` is set. |
| `temperature` | required | Sampler. |
| `top_p` | required | Sampler. |
| `top_k` | required | Sampler (Anthropic / Google). |
| `parallel_tool_calls` | required | If true, multiple tool calls in a single LLM response are dispatched in parallel. |
| `max_retries` | required | Passed to `retry_with(...)`. |
| `supports_tool_calling` | optional (default true) | `BaseLLMClient.supports_tool_calling` falls back to this. |
| `candidate_count` | optional | Gemini-specific. |
| `stop_sequences` | optional | Gemini-specific. |
| `max_completion_tokens` | optional | Azure "gpt-5"/"o3"/"o4-mini" specific — overrides `max_tokens` for those. |

### `agents.trae_agent`

| Field | Required | Notes |
|---|---|---|
| `enable_lakeview` | optional (default `true`) | Enables Lakeview summaries. |
| `model` | required | Reference to a key in `models`. |
| `max_steps` | required | Iteration cap for the agent loop. |
| `tools` | optional (default `[bash, str_replace_based_edit_tool, sequentialthinking, task_done]`) | Tool names from `tools_registry`. |

### `mcp_servers.<name>`

Each entry is an `MCPServerConfig`:

| Field | Notes |
|---|---|
| `command`, `args`, `env`, `cwd` | Stdio transport (currently the only one implemented in `MCPClient`). |
| `url` | WebSocket transport (not yet implemented — raises `NotImplementedError`). |
| `http_url`, `headers` | Streamable HTTP transport (not yet implemented). |
| `tcp` | WebSocket transport (not yet implemented). |
| `timeout`, `trust`, `description` | Common metadata. |

### `allow_mcp_servers`

List of MCP server names that the agent is allowed to use. Servers not in this list are ignored by `discover_mcp_tools`.

### `lakeview.model`

Reference to a key in `models`. When `enable_lakeview: true`, this model is used to summarize each step. If `enable_lakeview: true` but `lakeview` is missing, `Config.create` raises `ConfigError`.

## 7.4 Dataclasses — `trae_agent/utils/config.py`

```python
@dataclass
class ModelProvider:
    api_key: str
    provider: str
    base_url: str | None = None
    api_version: str | None = None

@dataclass
class ModelConfig:
    model: str
    model_provider: ModelProvider
    temperature: float
    top_p: float
    top_k: int
    parallel_tool_calls: bool
    max_retries: int
    max_tokens: int | None = None
    supports_tool_calling: bool = True
    candidate_count: int | None = None
    stop_sequences: list[str] | None = None
    max_completion_tokens: int | None = None

    def get_max_tokens_param(self) -> int
    def should_use_max_completion_tokens(self) -> bool
    def resolve_config_values(
        *, model_providers, provider, model, model_base_url, api_key
    )

@dataclass
class MCPServerConfig: ...

@dataclass
class AgentConfig:
    allow_mcp_servers: list[str]
    mcp_servers_config: dict[str, MCPServerConfig]
    max_steps: int
    model: ModelConfig
    tools: list[str]

@dataclass
class TraeAgentConfig(AgentConfig):
    enable_lakeview: bool = True
    tools: list[str] = field(default_factory=[bash, str_replace, sequential, task_done])
    def resolve_config_values(*, max_steps)

@dataclass
class LakeviewConfig:
    model: ModelConfig

@dataclass
class Config:
    lakeview: LakeviewConfig | None = None
    model_providers: dict[str, ModelProvider] | None = None
    models: dict[str, ModelConfig] | None = None
    trae_agent: TraeAgentConfig | None = None

    @classmethod
    def create(*, config_file=None, config_string=None) -> "Config"
    @classmethod
    def create_from_legacy_config(*, legacy_config=None, config_file=None) -> "Config"
    def resolve_config_values(*, provider, model, model_base_url, api_key, max_steps) -> "Config"
```

### `ModelConfig.should_use_max_completion_tokens()`

```python
return (
    self.max_completion_tokens is not None
    and self.model_provider.provider == "azure"
    and ("gpt-5" in self.model or "o3" in self.model or "o4-mini" in self.model)
)
```

### `ModelConfig.resolve_config_values(...)`

Overlays CLI/env values on top of the config-file value. Provider switch is special:

- If `provider` matches a key in `model_providers`, the existing `ModelProvider` is reused.
- Otherwise, a new `ModelProvider` is registered using the CLI `api_key` and `model_base_url` (raises `ConfigError` if `api_key` is missing).

### `Config.create(...)` Algorithm

1. Parse YAML from `config_file` (or `config_string`).
2. If the file ends with `.json`, fall back to `create_from_legacy_config()`.
3. Build `model_providers` from the YAML.
4. Build `models`, attaching each `model_provider` reference.
5. Parse `lakeview` and resolve the lakeview model.
6. Parse `mcp_servers` and `allow_mcp_servers` at the top level.
7. Build the matching `TraeAgentConfig` from `agents.trae_agent`.

### `Config.resolve_config_values(...)`

Calls `trae_agent.resolve_config_values(max_steps=...)` and `trae_agent.model.resolve_config_values(provider, model, model_base_url, api_key)`. After this, the dataclass is fully resolved.

## 7.5 CLI / Env Precedence

Implemented in `resolve_config_value(cli_value, config_value, env_var)`:

```python
if cli_value is not None:  return cli_value
if env_var and os.getenv(env_var): return os.getenv(env_var)
if config_value is not None: return config_value
return None
```

So:

```text
CLI flag  >  ENV var  >  config file value  >  default
```

For the provider, the env var name is constructed as `<PROVIDER>_API_KEY` and `<PROVIDER>_BASE_URL` (uppercase). For example, `ANTHROPIC_API_KEY` and `ANTHROPIC_BASE_URL` are recognized when the provider is `anthropic`.

## 7.6 Legacy JSON Config

The legacy file (`trae_config.json`) is parsed via `LegacyConfig(config_path)`. It has the following top-level keys:

| Key | Notes |
|---|---|
| `default_provider` | Single provider name (no per-model resolution). |
| `max_steps` | Default 20. |
| `enable_lakeview` | Default true. |
| `model_providers` | Map of provider configs with `model`, `api_key`, `base_url`, `max_tokens`, `temperature`, `top_p`, `top_k`, `parallel_tool_calls`, `max_retries`, `api_version`, `candidate_count`, `stop_sequences`. |
| `mcp_servers` | Map of MCP server configs. |
| `lakeview_config` | `model_provider`, `model_name`. |
| `allow_mcp_servers` | List of MCP server names. |

`Config.create_from_legacy_config()` converts the legacy schema into the modern `Config`:

- All model provider/model settings come from `default_provider`.
- `lakeview` reuses the same `model_config` if `lakeview_config` is missing.
- A `models` entry called `default_model` is synthesized.

The CLI will auto-fallback from a `.yaml` config to a same-named `.json` if YAML is missing (`cli.resolve_config_file()`).

## 7.7 Errors

`ConfigError` is raised for:

- Empty / missing `model_providers` or `models`.
- An agent referencing an unknown `model_provider` or `model_name`.
- `enable_lakeview: true` but no `lakeview` section.
- Trying to switch providers via CLI without supplying an `api_key`.

## 7.8 Quick Reference Table — How to Provide Each Value

| Value | YAML | JSON | ENV | CLI |
|---|---|---|---|---|
| Model name | `models.X.model` | `model_providers.<provider>.model` | — | `--model` |
| API key | `model_providers.X.api_key` | `model_providers.<provider>.api_key` | `<PROVIDER>_API_KEY` | `--api-key` |
| Base URL | `model_providers.X.base_url` | `model_providers.<provider>.base_url` | `<PROVIDER>_BASE_URL` | `--model-base-url` |
| Provider | `model_providers.X.provider` (for a model) | `default_provider` | — | `--provider` |
| Max steps | `agents.trae_agent.max_steps` | `max_steps` | — | `--max-steps` |
| Working dir | — | — | — | `--working-dir` |
| Config file | — | — | `TRAE_CONFIG_FILE` | `--config-file` |
| Trajectory file | — | — | — | `--trajectory-file` / `-t` |
| Must patch | — | — | — | `--must-patch` |

Continue with [CLI & UI](./08_cli_ui.md).
