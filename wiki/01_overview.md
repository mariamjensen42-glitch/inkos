# 1. Project Overview

## 1.1 What is Trae Agent?

**Trae Agent** is an LLM-based software-engineering agent released by ByteDance. It accepts natural-language tasks and executes them by orchestrating LLM providers and a toolbox of file/terminal/code-search utilities.

According to [README.md](../trae-agent/README.md) and the technical report ([arXiv:2507.23370](https://arxiv.org/abs/2507.23370)):

> Trae Agent offers a transparent, modular architecture that researchers and developers can easily modify, extend, and analyze, making it an ideal platform for **studying AI agent architectures, conducting ablation studies, and developing novel agent capabilities**.

The "research-friendly" stance is the project's distinguishing principle. The code is organized to make it easy to plug in new tools, swap LLM providers, or change the agent loop without rewiring the surrounding infrastructure.

## 1.2 Key Features

- 🌊 **Lakeview** — Concise, emoji-tagged summaries of each agent step (see `trae_agent/utils/lake_view.py`).
- 🤖 **Multi-LLM Support** — OpenAI, Anthropic, Azure, Google Gemini, Doubao, OpenRouter, Ollama (see `trae_agent/utils/llm_clients/`).
- 🛠️ **Rich Tool Ecosystem** — File editing (string-replace), bash, JSON editing (JSONPath), sequential thinking, task-done, code knowledge graph, MCP.
- 🎯 **Interactive Mode** — Conversational REPL (`trae-cli interactive`) with two console styles.
- 📊 **Trajectory Recording** — Every LLM call and step is dumped to JSON (`trae_agent/utils/trajectory_recorder.py`).
- ⚙️ **Flexible Configuration** — YAML first (recommended), with JSON legacy and env-var fallbacks.
- 🚀 **Easy Installation** — `uv` based, single `pyproject.toml`.
- 🐳 **Docker Mode** — Run the agent inside an arbitrary container image, with workspace volume mount and tool binaries copied into the container.

## 1.3 Design Philosophy

| Principle | How it shows up in the code |
|---|---|
| **Modularity** | `BaseAgent` is abstract; `TraeAgent` is one specialization. New agents can subclass. |
| **Provider-agnostic LLM** | `LLMClient` delegates to provider-specific subclasses behind a uniform `chat()` interface. |
| **Tools are first-class** | The `Tool` abstract class and `tools_registry` decouple tools from any specific agent. |
| **Inspectability** | `TrajectoryRecorder` writes structured JSON for every interaction, and `Lakeview` adds semantic tags. |
| **Standardized tool integration** | MCP support in `MCPClient` allows tools from external servers. |
| **Sandboxing** | `DockerManager` + `DockerToolExecutor` keep the host system safe. |
| **Backward compatibility** | Legacy JSON config still parses via `LegacyConfig` and `Config.create_from_legacy_config()`. |

## 1.4 Tech Stack

| Layer | Technology |
|---|---|
| Language | Python ≥ 3.12 (`.python-version`, `pyproject.toml`) |
| Package / build | `uv`, `hatchling` |
| CLI | `click` + `asyncclick` |
| LLM SDKs | `openai ≥ 1.86`, `anthropic 0.54–0.60`, `google-genai ≥ 1.24`, `ollama ≥ 0.5.1` |
| Protocol | `mcp 1.12.2` (Model Context Protocol) |
| Models / IO | `pydantic ≥ 2`, `pyyaml`, `python-dotenv`, `socksio` |
| AST / Code search | `tree-sitter 0.21.3` + `tree-sitter-languages 1.10.2`, `jsonpath-ng` |
| UI | `rich ≥ 13`, `textual ≥ 0.50` (TUI) |
| Tool packaging | `pyinstaller 6.15.0` |
| Tests | `pytest`, `pytest-asyncio`, `pytest-mock`, `pytest-cov` |
| Eval | `datasets`, `docker`, `pexpect`, `unidiff` |
| Lint / format | `ruff ≥ 0.12.4` |
| Pre-commit | configured in `.pre-commit-config.yaml` |

## 1.5 Version & Citation

- Package version: `0.1.0` (per `pyproject.toml` and `trae_agent/__init__.py`).
- License: MIT.
- Citation: see the BibTeX block at the end of the [README](../trae-agent/README.md).

## 1.6 High-level Component Map

```text
trae_agent/
├── cli.py                   # Click entry point → registers `trae-cli` console script
├── agent/                   # Agent loop, base/specialized agents
├── tools/                   # Tool abstraction + built-in tools
├── prompt/                  # System prompt(s)
└── utils/
    ├── config.py            # YAML/JSON config dataclasses + loader
    ├── cli/                 # Simple & Rich TUI consoles
    ├── llm_clients/         # Per-provider API clients
    ├── mcp_client.py        # MCP (Model Context Protocol) integration
    ├── trajectory_recorder.py
    ├── lake_view.py
    ├── legacy_config.py
    └── constants.py
```

For a tour of the **runtime** request flow, continue with [Architecture](./03_architecture.md).
