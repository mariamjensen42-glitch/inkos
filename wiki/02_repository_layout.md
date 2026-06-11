# 2. Repository Layout

This page is a map of the [trae-agent](../trae-agent/) repository: every important file/folder and what it contains.

## 2.1 Top-Level Tree

```text
trae-agent/
├── .github/                       # CI / issue templates
│   ├── ISSUE_TEMPLATE/
│   └── workflows/
│       ├── pre-commit.yml
│       └── unit-test.yml
├── .vscode/
│   └── launch.template.json
├── docs/
│   ├── TRAJECTORY_RECORDING.md    # Detailed doc of the trajectory recorder
│   ├── legacy_config.md           # Old JSON config reference
│   ├── roadmap.md                 # Forward-looking plan (SDK, sandbox, MLOps …)
│   └── tools.md                   # Tool-by-tool reference (already shipped)
├── evaluation/                    # SWE-bench / SWE-bench-Live / Multi-SWE-bench harness
│   ├── run_evaluation.py
│   ├── setup.sh
│   ├── utils.py
│   └── patch_selection/           # Agent-based patch selection
├── server/                        # FastAPI HTTP server (WIP, see Readme.md)
│   └── Readme.md
├── tests/                         # Unit tests (pytest)
│   ├── agent/
│   ├── tools/
│   ├── utils/
│   └── test_cli.py
├── trae_agent/                    # ★ Source package
│   ├── __init__.py
│   ├── cli.py                     # `trae-cli` entrypoint
│   ├── agent/
│   ├── tools/
│   ├── prompt/
│   └── utils/
├── .gitignore
├── .pre-commit-config.yaml        # ruff + standard hooks
├── .python-version                # 3.12
├── CONTRIBUTING.md
├── LICENSE                        # MIT
├── Makefile                       # Convenience targets: install-dev, test, fix-format …
├── README.md                      # User-facing documentation
├── pyproject.toml                 # Dependencies, scripts, ruff, pytest, coverage
├── trae_config.yaml.example       # YAML config example
├── trae_config.json.example       # Legacy JSON config example
└── uv.lock                        # Reproducible lockfile (uv)
```

## 2.2 Source Package: `trae_agent/`

| Path | Purpose |
|---|---|
| `cli.py` | `click` group `cli`, sub-commands: `run`, `interactive`, `show-config`, `tools`. Contains helpers: `resolve_config_file`, `check_docker`, `build_with_pyinstaller`. |
| `__init__.py` | Re-exports `BaseAgent`, `TraeAgent`, `LLMClient`, `Tool`, `ToolExecutor`; declares `__version__ = "0.1.0"`. |
| `agent/` | The agent loop. |
| `tools/` | Tool abstraction and built-in tools. |
| `prompt/` | The TRAE_AGENT_SYSTEM_PROMPT used by `TraeAgent`. |
| `utils/` | Cross-cutting infrastructure (LLM clients, config, CLI, MCP, trajectory, lakeview). |

## 2.3 `trae_agent/agent/`

| File | Role |
|---|---|
| `__init__.py` | Re-exports `BaseAgent`, `TraeAgent`, `Agent`. |
| `agent.py` | `AgentType` enum and `Agent` factory. Given an agent type, config, and CLI console, instantiates the matching concrete agent. |
| `base_agent.py` | `BaseAgent(ABC)` — provider-agnostic loop (`execute_task` → `_run_llm_step` → `_tool_call_handler` → `_finalize_step`). Holds `_llm_client`, `_tool_caller`, `_initial_messages`, etc. |
| `trae_agent.py` | `TraeAgent(BaseAgent)` — software-engineering specialization: handles `task_done`, must-patch verification, git-diff capture, MCP discovery. |
| `agent_basics.py` | Dataclasses & enums shared by the agent loop: `AgentState`, `AgentStepState`, `AgentStep`, `AgentExecution`, `AgentError`. |
| `docker_manager.py` | `DockerManager` — container lifecycle, persistent pexpect shell, command execution. |
| `prompt/agent_prompt.py` | The TRAE_AGENT_SYSTEM_PROMPT used as the LLM system message. |

## 2.4 `trae_agent/tools/`

| File | Tool name (registry key) | Purpose |
|---|---|---|
| `__init__.py` | — | Exposes `tools_registry: dict[str, type[Tool]]`. |
| `base.py` | — | `Tool`, `ToolExecutor`, `ToolCall`, `ToolResult`, `ToolParameter`, `ToolError`, JSON-schema generation. |
| `bash_tool.py` | `bash` | Persistent bash session with timeout and exit-code recovery (Windows-aware). |
| `edit_tool.py` | `str_replace_based_edit_tool` | `view` / `create` / `str_replace` / `insert` on local files. |
| `edit_tool_cli.py` | — | PyInstaller entry — runs `str_replace_based_edit_tool` as a CLI inside Docker. |
| `json_edit_tool.py` | `json_edit_tool` | `view` / `set` / `add` / `remove` against JSONPath. |
| `json_edit_tool_cli.py` | — | PyInstaller entry — runs the JSON edit tool as a CLI inside Docker. |
| `sequential_thinking_tool.py` | `sequentialthinking` | Structured chain-of-thought with branching & revision. |
| `task_done_tool.py` | `task_done` | Marker tool that `TraeAgent` interprets as completion. |
| `mcp_tool.py` | (dynamic) | Adapter around a discovered MCP tool. |
| `ckg_tool.py` | `ckg` | Query a code knowledge graph (functions / classes / methods). |
| `ckg/ckg_database.py` | — | Tree-sitter-based SQLite code knowledge graph builder. |
| `ckg/base.py` | — | `FunctionEntry`, `ClassEntry`, language-extension map. |
| `docker_tool_executor.py` | — | Routes docker-enabled tool calls to a Docker container. |
| `run.py` | — | `run()` async shell helper with `maybe_truncate()`. |
| `dist/` | — | Pre-built PyInstaller executables (generated at runtime if absent). |

## 2.5 `trae_agent/utils/`

| File | Role |
|---|---|
| `config.py` | Dataclasses (`Config`, `ModelProvider`, `ModelConfig`, `AgentConfig`, `TraeAgentConfig`, `LakeviewConfig`, `MCPServerConfig`) + `Config.create()` and `resolve_config_values()`. |
| `legacy_config.py` | Backward-compatible JSON loader (`LegacyConfig`). |
| `constants.py` | `LOCAL_STORAGE_PATH = Path.home() / ".trae-agent"`. |
| `mcp_client.py` | `MCPClient` (stdio transport), connection & tool discovery. |
| `trajectory_recorder.py` | JSONL-style trajectory writer. |
| `lake_view.py` | Lakeview summarizer (extract_task / extract_tag / create_lakeview_step). |
| `cli/cli_console.py` | `CLIConsole` ABC + helpers. |
| `cli/console_factory.py` | `ConsoleFactory.create_console()`. |
| `cli/simple_console.py` | `SimpleCLIConsole` (rich-based, plain text). |
| `cli/rich_console.py` | `RichCLIConsole` (textual TUI). |
| `cli/rich_console.tcss` | Textual CSS for the TUI. |
| `cli/__init__.py` | Re-exports. |
| `llm_clients/llm_client.py` | `LLMClient` provider switcher, `LLMProvider` enum. |
| `llm_clients/base_client.py` | `BaseLLMClient(ABC)`. |
| `llm_clients/llm_basics.py` | `LLMMessage`, `LLMResponse`, `LLMUsage`. |
| `llm_clients/openai_client.py` | OpenAI Responses API. |
| `llm_clients/openai_compatible_base.py` | Shared base for OpenAI-compatible providers. |
| `llm_clients/anthropic_client.py` | Anthropic Messages API (with native `text_editor_20250429` / `bash_20250124` tools). |
| `llm_clients/google_client.py` | Google Gemini via `google-genai`. |
| `llm_clients/azure_client.py` | Azure OpenAI (uses `openai.AzureOpenAI`). |
| `llm_clients/doubao_client.py` | Doubao (Volcengine Ark). |
| `llm_clients/openrouter_client.py` | OpenRouter router. |
| `llm_clients/ollama_client.py` | Local Ollama models. |
| `llm_clients/retry_utils.py` | `retry_with()` decorator. |
| `llm_clients/readme.md` | Brief note on the refactor of the LLM client layer. |

## 2.6 Build / Config Files

- `pyproject.toml`
  - `requires-python = ">=3.12"`.
  - Dependencies (see [Tech Stack](./01_overview.md#14-tech-stack)).
  - Console script: `trae-cli = "trae_agent.cli:main"`.
  - `hatchling` build backend, wheel packages `["trae_agent"]`.
  - Tooling config: `pytest`, `coverage`, `ruff`.
- `Makefile` — `install-dev`, `uv-test`, `pre-commit`, `fix-format`, `clean`.
- `.pre-commit-config.yaml` — `ruff format`, `ruff check` (with auto-fix), pre-commit hooks.
- `uv.lock` — exact transitive versions for `uv`.
- `trae_config.yaml.example` / `trae_config.json.example` — sample configs (see [Configuration](./07_configuration.md)).

## 2.7 Where to Start Reading

1. [cli.py](../trae-agent/trae_agent/cli.py) — see how a `run` command wires up the system.
2. [agent/agent.py](../trae-agent/trae_agent/agent/agent.py) — see how `Agent` is constructed.
3. [agent/base_agent.py](../trae-agent/trae_agent/agent/base_agent.py) — see the agent loop.
4. [tools/base.py](../trae-agent/trae_agent/tools/base.py) — see how tools are abstracted.
5. [utils/llm_clients/llm_client.py](../trae-agent/trae_agent/utils/llm_clients/llm_client.py) — see how providers are selected.
