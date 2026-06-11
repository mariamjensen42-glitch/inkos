# Trae Agent — Code Wiki

> A Code Wiki for [bytedance/trae-agent](https://github.com/bytedance/trae-agent).
> **Trae Agent** is an LLM-based agent for general-purpose software engineering tasks. It provides a powerful CLI interface that understands natural language instructions and executes complex software engineering workflows using various tools and LLM providers.

## Index

| # | Document | Description |
|---|----------|-------------|
| 1 | [Project Overview](./01_overview.md) | What Trae Agent is, key features, design philosophy, and tech stack |
| 2 | [Repository Layout](./02_repository_layout.md) | Top-level folder structure, key files, build system |
| 3 | [Architecture](./03_architecture.md) | End-to-end architecture, request flow, layered design |
| 4 | [Core Agent Module](./04_core_agent.md) | `BaseAgent`, `TraeAgent`, `Agent`, agent basics, prompt, Docker manager |
| 5 | [Tools](./05_tools.md) | Tool base class, `ToolExecutor`, and all built-in tools |
| 6 | [LLM Clients](./06_llm_clients.md) | Provider enumeration, client architecture, per-provider details |
| 7 | [Configuration](./07_configuration.md) | YAML schema, `Config`, `TraeAgentConfig`, `ModelConfig`, legacy JSON, MCP |
| 8 | [CLI & UI](./08_cli_ui.md) | `trae-cli` commands, `SimpleCLIConsole`, `RichCLIConsole`, Lakeview |
| 9 | [Trajectory Recording & MCP](./09_trajectory_mcp.md) | `TrajectoryRecorder`, `MCPClient`, persistence paths |
| 10 | [Docker Mode](./10_docker_mode.md) | `DockerManager`, `DockerToolExecutor`, packaging of tools via PyInstaller |
| 11 | [Sub-projects](./11_subprojects.md) | `evaluation/`, `server/`, `tests/`, and the CKG module |
| 12 | [Running & Extension Guide](./12_running_and_extension.md) | How to install, run, configure, troubleshoot, and extend |

## Quick Links

- Source root: [trae_agent/](../trae-agent/trae_agent/)
- Entry point: [trae_agent/cli.py](../trae-agent/trae_agent/cli.py) (`trae-cli`)
- Configuration example: [trae_config.yaml.example](../trae-agent/trae_config.yaml.example)
- README: [README.md](../trae-agent/README.md)

## At a glance

```text
                       CLI  (trae-cli)
                          │
                          ▼
              ┌─────────────────────┐
              │  Agent (factory)    │
              └──────────┬──────────┘
                         │
                         ▼
              ┌─────────────────────┐
              │ TraeAgent (SE)      │ ◀── TRAE_AGENT_SYSTEM_PROMPT
              └──────────┬──────────┘
                         │
            ┌────────────┴────────────┐
            ▼                         ▼
   ┌────────────────┐         ┌────────────────┐
   │ LLMClient      │         │ ToolExecutor   │
   │ (multi-provider)│         │ (bash, edit, …)│
   └───────┬────────┘         └────────┬───────┘
           │                           │
           ▼                           ▼
   openai/anthropic/google/    local tools or Docker
   doubao/azure/openrouter/
   ollama
```

## License

MIT — Copyright (c) 2025 ByteDance Ltd. and/or its affiliates. See [LICENSE](../trae-agent/LICENSE).
