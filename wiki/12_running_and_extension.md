# 12. Running & Extension Guide

This page is a hands-on walk-through: install, run, configure, troubleshoot, and extend Trae Agent.

## 12.1 Prerequisites

- **Python ≥ 3.12** (the repo pins this in `pyproject.toml` and `.python-version`).
- [`uv`](https://github.com/astral-sh/uv) recommended (a `uv.lock` is included).
- Optional: `docker` CLI + daemon, for Docker mode.
- Optional: `node` + `npx`, for some MCP servers like Playwright.

## 12.2 Installation

### With `uv` (recommended)

```bash
git clone https://github.com/bytedance/trae-agent.git
cd trae-agent
uv sync
uv run trae-cli --help
```

### With `pip`

```bash
git clone https://github.com/bytedance/trae-agent.git
cd trae-agent
pip install -e .
trae-cli --help
```

`pyproject.toml` declares `trae-cli = "trae_agent.cli:main"` as the console script entry point.

## 12.3 Configuration

```bash
cp trae_config.yaml.example trae_config.yaml
# edit and fill in your api_key(s)
```

Or copy the legacy JSON file:

```bash
cp trae_config.json.example trae_config.json
```

See [Configuration](./07_configuration.md) for the full schema.

## 12.4 Running a One-shot Task

```bash
trae-cli run "add a /healthz endpoint to the FastAPI app" \
  --working-dir /path/to/project \
  --trajectory-file runs/healthz.json
```

Or read the task from a file:

```bash
trae-cli run -f task.md --working-dir /path/to/project
```

Common flags (see `trae-cli run --help`):

- `-p, --provider` — provider name.
- `-m, --model` — model name.
- `--model-base-url` — custom endpoint.
- `-k, --api-key` — key (prefer env vars).
- `--max-steps` — iteration cap.
- `--must-patch` — fail if the agent produces an empty patch.
- `--config-file` — defaults to `trae_config.yaml`.
- `-ct, --console-type simple|rich`.

## 12.5 Running an Interactive Session

```bash
trae-cli interactive --console-type rich
```

Built-in commands:

- `help`
- `status` — current provider, model, tools.
- `clear`
- `exit` / `quit`

## 12.6 Inspecting Configuration

```bash
trae-cli show-config
```

Prints the resolved provider + agent settings as a rich table.

## 12.7 Listing Tools

```bash
trae-cli tools
```

Iterates `tools_registry` and prints each tool's name and description.

## 12.8 Running with Docker

```bash
trae-cli run "fix the failing test" \
  --docker-image python:3.11 \
  --working-dir /code/project
```

Other forms:

```bash
trae-cli run "..." --docker-container-id 91998a56056c
trae-cli run "..." --dockerfile-path ./agent.Dockerfile
trae-cli run "..." --docker-image-file ./agent-image.tar
trae-cli run "..." --docker-image python:3.11 --docker-keep false
```

The first time, Trae Agent will run `pyinstaller` to build the `edit_tool` and `json_edit_tool` executables into `trae_agent/dist/`. Subsequent runs reuse them.

## 12.9 Output & Artifacts

| Artifact | Default Location |
|---|---|
| Trajectory JSON | `trajectories/trajectory_<timestamp>.json` (overridable with `-t`) |
| Final patch (only `--patch-path` is set) | The path you pass in. |
| CKG cache | `~/.trae-agent/ckg/<hash>.db` (auto-cleaned after 7 days) |
| Docker tool binaries | `trae_agent/dist/{edit_tool,json_edit_tool}` |

## 12.10 Common Configuration Examples

### Anthropic

```yaml
model_providers:
  anthropic:
    api_key: sk-ant-...
    provider: anthropic

models:
  trae_agent_model:
    model_provider: anthropic
    model: claude-sonnet-4-20250514
    max_tokens: 4096
    temperature: 0.5
    top_p: 1.0
    top_k: 0
    parallel_tool_calls: true
    max_retries: 3
    supports_tool_calling: true

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

lakeview:
  model: trae_agent_model
```

### OpenAI

```yaml
model_providers:
  openai:
    api_key: sk-...
    provider: openai

models:
  trae_agent_model:
    model_provider: openai
    model: gpt-4o
    max_tokens: 4096
    temperature: 0.5
    ...
```

### Azure OpenAI (gpt-5)

```yaml
model_providers:
  azure:
    api_key: ...
    provider: azure
    base_url: https://<resource>.openai.azure.com/
    api_version: 2025-01-01-preview

models:
  trae_agent_model:
    model_provider: azure
    model: gpt-5
    max_completion_tokens: 4096   # honored for azure + gpt-5
    ...
```

### Google Gemini

```yaml
model_providers:
  google:
    api_key: ...
    provider: google

models:
  trae_agent_model:
    model_provider: google
    model: gemini-2.5-pro
    ...
```

### OpenRouter

```yaml
model_providers:
  openrouter:
    api_key: sk-or-...
    provider: openrouter
    base_url: https://openrouter.ai/api/v1

models:
  trae_agent_model:
    model_provider: openrouter
    model: anthropic/claude-sonnet-4
    ...
```

### Doubao (Volcengine Ark)

```yaml
model_providers:
  doubao:
    api_key: ...
    provider: doubao
    base_url: https://ark.cn-beijing.volces.com/api/v3

models:
  trae_agent_model:
    model_provider: doubao
    model: doubao-...
    ...
```

### Local Ollama

```yaml
model_providers:
  ollama:
    provider: ollama
    base_url: http://localhost:11434/v1

models:
  trae_agent_model:
    model_provider: ollama
    model: llama3.1
    ...
```

## 12.11 Adding MCP Servers

Example with the Playwright MCP server:

```yaml
mcp_servers:
  playwright:
    command: npx
    args: ["@playwright/mcp@0.0.27"]

allow_mcp_servers:
  - playwright
```

After this, the Playwright tools appear alongside the built-in tools (visible via `trae-cli tools`).

## 12.12 Environment Variables

| Variable | Effect |
|---|---|
| `<PROVIDER>_API_KEY` | Used as the API key for `<PROVIDER>` (e.g., `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`). |
| `<PROVIDER>_BASE_URL` | Used as the base URL for `<PROVIDER>` (e.g., `OPENAI_BASE_URL`). |
| `OPENROUTER_SITE_URL`, `OPENROUTER_SITE_NAME` | Optional OpenRouter HTTP headers. |
| `TRAE_CONFIG_FILE` | CLI default `--config-file`. |
| `LOCAL_STORAGE_PATH` (constant in `utils/constants.py`) | Default `~/.trae-agent`. |

## 12.13 Troubleshooting

| Symptom | Fix |
|---|---|
| `Error: Provider 'X' not in model_providers` | Add `model_providers.<X>` and a `models.<X>_*` entry. |
| `must_patch` rejects every task | Check that the model is calling the right tool. Re-run with `--must-patch false` for debugging. |
| `DockerException: Cannot connect to the Docker daemon` | Start Docker, then re-run. |
| `FileNotFoundError: trae_agent/dist/edit_tool` | Re-run with Docker enabled to trigger `build_with_pyinstaller()`. |
| `ConnectError` against an OpenAI-compatible base URL | The base URL must be the full path (e.g., `https://api.deepseek.com/v1`), not just the host. |
| `MCP server 'X' failed` | Inspect the MCP server's logs; ensure the binary is on PATH inside the agent's environment. |
| `ModelConfigError: model_provider references unknown model_provider` | Make sure every `model.<name>.model_provider` references an existing `model_providers.<key>`. |
| `ollama` returns empty `usage` | That's by design — Ollama doesn't return token counts reliably. |

## 12.14 Pre-commit Hooks

```bash
uv run pre-commit install
uv run pre-commit run --all-files
```

`.pre-commit-config.yaml` registers `ruff format` and `ruff check --fix`.

## 12.15 Running Tests

```bash
make uv-test
# or
uv run pytest
```

`pyproject.toml` configures:

- `testpaths = ["tests"]`
- `asyncio_mode = "auto"`
- `addopts = "--cov=trae_agent --cov-report=term-missing --cov-config=pyproject.toml"`
- `pythonpath = ["."]`

## 12.16 Building a New Tool

(See [Tools](./05_tools.md) for details; quick version.)

```python
# trae_agent/tools/my_tool.py
from .base import Tool, ToolExecResult, ToolParameter

class MyTool(Tool):
    def get_name(self) -> str:
        return "my_tool"
    def get_description(self) -> str:
        return "What my tool does."
    def get_parameters(self) -> list[ToolParameter]:
        return [
            ToolParameter(
                name="input",
                type="string",
                description="The input.",
                required=True,
            )
        ]
    async def execute(self, arguments) -> ToolExecResult:
        return ToolExecResult(output=f"got: {arguments['input']}")
```

Then in `trae_agent/tools/__init__.py`:

```python
from .my_tool import MyTool
tools_registry["my_tool"] = MyTool
```

And in your config:

```yaml
agents:
  trae_agent:
    tools: [..., my_tool]
```

Or pass it at runtime:

```python
from trae_agent.agent import Agent, AgentType
await Agent(AgentType.TraeAgent, config).run("...", tool_names=["my_tool"])
```

## 12.17 Implementing a New Agent

```python
from trae_agent.agent.base_agent import BaseAgent
from trae_agent.agent.agent_basics import AgentError

class MyAgent(BaseAgent):
    def new_task(self, task, extra_args, tool_names):
        if not extra_args or "url" not in extra_args:
            raise AgentError("--url is required")
        self._initial_messages = [
            self._llm_client.convert_system_message(self.get_system_prompt()),
            self._llm_client.convert_user_message(f"Scrape {extra_args['url']}"),
        ]
        self._task = task
        # ... start trajectory recorder

    async def cleanup_mcp_clients(self):
        pass
```

Register it in `AgentType` / `Agent` factory.

## 12.18 Implementing a New LLM Provider

1. Pick the right base class:
   - `BaseLLMClient` for a custom protocol.
   - `OpenAICompatibleClient` for OpenAI Chat Completions-compatible vendors.
2. Implement `set_chat_history(messages)` and `chat(messages, model_config, tools, reuse_history)`.
3. Wrap the wire call in `@retry_with(retryable_func, "<name>", max_retries=model_config.max_retries)`.
4. Convert the SDK's response to `LLMResponse(content, usage, model, finish_reason, tool_calls)`.
5. Record the interaction via `self.trajectory_recorder.record_llm_interaction(...)`.
6. Add a branch in `LLMClient.__init__` and a new `LLMProvider` enum entry.

## 12.19 Citation

The technical report (and BibTeX) is in the [README](../trae-agent/README.md). Cite as:

```bibtex
@article{trae-agent,
  title  = {Trae Agent: An LLM-based Agent for General Purpose Software Engineering Tasks},
  year   = {2025},
  url    = {https://arxiv.org/abs/2507.23370},
}
```
