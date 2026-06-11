# 3. Architecture

## 3.1 Layered View

```text
┌──────────────────────────────────────────────────────────────┐
│  CLI  (trae_agent/cli.py)                                   │
│  • run / interactive / show-config / tools                   │
│  • config-file resolution (yaml → json fallback)            │
│  • docker preflight (check_docker, build_with_pyinstaller)  │
└──────────────────────┬───────────────────────────────────────┘
                       │
                       ▼
┌──────────────────────────────────────────────────────────────┐
│  Agent factory  (trae_agent/agent/agent.py)                 │
│  Agent(agent_type, config, trajectory_file, cli_console,    │
│        docker_config, docker_keep)                          │
│  → constructs TraeAgent (or any future agent)               │
└──────────────────────┬───────────────────────────────────────┘
                       │
                       ▼
┌──────────────────────────────────────────────────────────────┐
│  Concrete agent  (TraeAgent)                                │
│  • new_task(task, extra_args) → builds LLMMessage list      │
│  • execute_task()  → loops max_steps times                  │
│       _run_llm_step → _tool_call_handler → _finalize_step   │
│  • cleanup_mcp_clients()                                     │
│  • get_git_diff(), remove_patches_to_tests()                 │
└──────┬──────────────────────────────────┬────────────────────┘
       │                                  │
       ▼                                  ▼
┌────────────────────────┐    ┌────────────────────────────────┐
│  LLMClient             │    │  ToolExecutor / DockerToolEx.  │
│  utils/llm_clients/    │    │  tools/base.py                 │
│  chat() → provider     │    │  • execute_tool_call()         │
│  client.chat()         │    │  • parallel_tool_call()        │
│  → LLMResponse         │    │  • sequential_tool_call()      │
└────────────┬───────────┘    └──────────────┬─────────────────┘
             │                               │
             ▼                               ▼
   7 LLM providers              bash, edit, json_edit,
   (openai / anthropic /        sequentialthinking,
    google / doubao /           task_done, ckg,
    azure / openrouter /        mcp
    ollama)
```

## 3.2 End-to-end Request Flow

A typical `trae-cli run "..."` call flows like this:

1. **Argument parsing** — [cli.py](../trae-agent/trae_agent/cli.py) `run()` collects click options, validates Docker flags (they are mutually exclusive), and resolves a config file.
2. **Config creation** — `Config.create(config_file=...)` loads YAML (or JSON if `.json` suffix), building `model_providers`, `models`, `agents`, `mcp_servers`, `lakeview`. `resolve_config_values()` overlays CLI/env values on top of file values.
3. **Console creation** — `ConsoleFactory.create_console(...)` returns either a `SimpleCLIConsole` or a `RichCLIConsole` (TUI), tied to the lake-view config.
4. **Agent instantiation** — `Agent(agent_type, config, ...)` constructs the underlying `TraeAgent` and wires up the trajectory recorder and CLI console.
5. **Task initialization** — `agent.run(task, task_args)` calls `new_task(...)` to build `system` + `user` LLM messages (project path, issue text) and start trajectory recording.
6. **MCP discovery (optional)** — If MCP servers are configured, `initialise_mcp()` discovers tools and appends them to the agent's tool list.
7. **Step loop** — `execute_task()` iterates up to `max_steps`:
   - `_run_llm_step` calls `LLMClient.chat(messages, model_config, tools)`.
   - If the response includes tool calls, `_tool_call_handler` dispatches them via the executor (parallel or sequential), optionally applies reflection.
   - `_finalize_step` records the step and updates the console.
8. **Completion check** — `TraeAgent.llm_indicates_task_completed()` returns true iff the model called `task_done`. `TraeAgent._is_task_completed()` may further require a non-empty git diff when `--must-patch` is set.
9. **Cleanup** — `cleanup_mcp_clients()`, `close_tools()`, optional Docker `stop()`. Trajectory is finalized.
10. **Output** — Trajectory file path is printed; if `RichConsole`, the TUI keeps the step history on screen.

## 3.3 The Agent Loop in Detail

`BaseAgent.execute_task` ([base_agent.py](../trae-agent/trae_agent/agent/base_agent.py)) is the heart of the system:

```text
initialize → for step in 1..max_steps:
                step = AgentStep(state=THINKING)
                try:
                    messages = _run_llm_step(step, messages, execution)
                    _finalize_step(step, messages, execution)
                    if state == COMPLETED: break
                except Exception as e:
                    state = ERROR
                    step.error = str(e)
                    _finalize_step(...)
                    break
            finally:
                close_tools()
                cleanup_mcp_clients()
```

`_run_llm_step` decides:
- If the LLM signals completion (in `TraeAgent`, the `task_done` tool), mark COMPLETED and return messages unchanged.
- Otherwise, dispatch tool calls and convert results into new `LLMMessage(tool_result=...)` messages.

`_tool_call_handler` may add a reflection message if any tool failed.

## 3.4 How Tools are Wired to the LLM

```text
TraeAgent._tools  (list[Tool])
       │
       ▼
BaseAgent._llm_client.chat(messages, model_config, tools)
       │
       ▼
Provider client → for each Tool, call tool.get_input_schema() → JSON schema
       │
       ▼
LLM response → list[ToolCall] → Back in agent loop
       │
       ▼
_tool_caller.execute_tool_call(tc)  (ToolExecutor or DockerToolExecutor)
       │
       ▼
ToolResult(success, result, error) → turned into LLMMessage(tool_result=...)
```

The same `Tool` object is used for both the schema sent to the LLM and the runtime execution. `Tool.get_input_schema()` ([base.py](../trae-agent/trae_agent/tools/base.py)) has a special branch for OpenAI: it marks every parameter as required and adds `additionalProperties: false` for the strict-function-call API.

## 3.5 Concurrency & Async

- The agent loop is **async** (`asyncio.run` from `cli.py`).
- `ToolExecutor.parallel_tool_call` uses `asyncio.gather`. If `model_config.parallel_tool_calls` is false, it falls back to `sequential_tool_call`.
- `BashTool` keeps a persistent bash process and uses sentinel-based exit-code recovery.
- `MCPClient` uses `AsyncExitStack` to manage stdio transports and `ClientSession`.
- `DockerToolExecutor.execute_tool_call` blocks synchronously on `docker_manager.execute()` (sync, not async) because pexpect-based shells are easier to drive that way; the surrounding dispatcher wraps them in `await`.

## 3.6 Observability

- **Trajectory file** — every LLM call and step is appended to a JSON file under `trajectories/` (or a custom path).
- **CLI console** — `SimpleCLIConsole` prints rich tables; `RichCLIConsole` shows a Textual TUI.
- **Lakeview** — background summary jobs annotate each step with a `<task>` description and a tag (`WRITE_TEST`, `VERIFY_FIX`, …).

## 3.7 Why the Layered Design?

| Layer | Why it exists |
|---|---|
| CLI | User-facing, owns config and signal handling. |
| `Agent` | Insulates the CLI from any specific agent implementation. |
| `BaseAgent` | Reusable loop; only depends on `LLMClient` + `ToolExecutor`. |
| `TraeAgent` | Software-engineering–specific behaviors (task-done, patch diff). |
| `LLMClient` | Picks a provider; isolates the agent from API differences. |
| Provider client | Implements the chat protocol for one vendor. |
| `Tool` | LLM-callable unit; tools are registered in `tools_registry`. |
| `ToolExecutor` | Decides parallel vs. sequential and local vs. Docker. |
| `TrajectoryRecorder` | Pluggable observer — every LLM client has a reference. |
| CLI Console / Lakeview | Pluggable observers — driven by `update_status` callbacks. |

Continue with [Core Agent Module](./04_core_agent.md) for details on `BaseAgent` and `TraeAgent`.
