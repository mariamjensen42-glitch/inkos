# 4. Core Agent Module

The agent package is in `trae_agent/agent/`. It defines the agent loop, the software-engineering specialization, and supporting types.

## 4.1 Module Map

```text
trae_agent/agent/
├── __init__.py        # exports BaseAgent, TraeAgent, Agent
├── agent.py           # AgentType + Agent factory
├── base_agent.py      # BaseAgent (ABC) — generic loop
├── trae_agent.py      # TraeAgent (BaseAgent) — SE specialization
├── agent_basics.py    # AgentState, AgentStepState, AgentStep, AgentExecution, AgentError
├── docker_manager.py  # DockerManager (container lifecycle)
└── prompt/agent_prompt.py
```

## 4.2 `agent_basics.py` — Shared Types

```python
class AgentStepState(Enum):
    THINKING, CALLING_TOOL, REFLECTING, COMPLETED, ERROR

class AgentState(Enum):
    IDLE, RUNNING, COMPLETED, ERROR

@dataclass
class AgentStep:
    step_number: int
    state: AgentStepState
    thought: str | None = None
    tool_calls: list[ToolCall] | None = None
    tool_results: list[ToolResult] | None = None
    llm_response: LLMResponse | None = None
    reflection: str | None = None
    error: str | None = None
    extra: dict[str, object] | None = None
    llm_usage: LLMUsage | None = None

@dataclass
class AgentExecution:
    task: str
    steps: list[AgentStep]
    final_result: str | None = None
    success: bool = False
    total_tokens: LLMUsage | None = None
    execution_time: float = 0.0
    agent_state: AgentState = AgentState.IDLE

class AgentError(Exception): ...
```

- `AgentStep` is the unit observed by the CLI console and the trajectory recorder.
- `AgentExecution` is the final report returned by `execute_task()`.
- `AgentError` is raised by `TraeAgent.new_task()` when required args (`project_path`, `issue`) are missing.

## 4.3 `base_agent.py` — `BaseAgent`

### Constructor

```python
def __init__(self, agent_config, docker_config=None, docker_keep=True):
    self._llm_client = LLMClient(agent_config.model)
    self._model_config = agent_config.model
    self._max_steps = agent_config.max_steps
    self._initial_messages = []
    self._task = ""
    self._tools = [
        tools_registry[name](model_provider=provider)
        for name in agent_config.tools
    ]
    self.docker_manager = None
    self._tool_caller = original ToolExecutor(self._tools)
    if docker_config:
        self.docker_manager = DockerManager(...)
        self._tool_caller = DockerToolExecutor(...)
    self._cli_console: CLIConsole | None = None
    self._trajectory_recorder: TrajectoryRecorder | None = None
    clear_older_ckg()  # purge stale CKG SQLite caches
```

### Key Properties

| Property | Notes |
|---|---|
| `llm_client` | The underlying `LLMClient`. |
| `cli_console` / `set_cli_console()` | For status updates. |
| `trajectory_recorder` / `set_trajectory_recorder()` | Also pushed to the LLM client. |
| `tools` | Snapshot of the registered tool instances. |
| `task` / `initial_messages` / `model_config` / `max_steps` | Read-only views. |

### Abstract Methods (overridden in `TraeAgent`)

- `new_task(task, extra_args, tool_names)` — build the system+user message list and start the trajectory recorder.
- `cleanup_mcp_clients()` — release MCP resources.

### Loop: `execute_task()`

Iterates `1..max_steps`; each iteration:

1. `_run_llm_step(step, messages, execution)` — calls the LLM, handles task-completion vs. tool-call branches.
2. `_finalize_step(step, messages, execution)` — sets `state = COMPLETED`, calls `_record_handler` and `_update_cli_console`, and appends the step to `execution.steps`.
3. If state is `COMPLETED`, break. If any exception is raised, the state becomes `ERROR` and is recorded.
4. In `finally`: `_close_tools()`, MCP cleanup, total `execution_time` is measured, and the CLI console is updated.

### Helper Methods

- `_run_llm_step(step, messages, execution)`:
  1. State = THINKING.
  2. `llm_response = self._llm_client.chat(messages, model_config, tools)`.
  3. Token usage is accumulated into `execution.total_tokens`.
  4. If `llm_indicates_task_completed(llm_response)` → `execution.agent_state = COMPLETED` (or push a "task incomplete" message if `_is_task_completed` returns false).
  5. Else → dispatch tool calls and produce new messages.
- `_tool_call_handler(tool_calls, step)`:
  - Empty list → push a "It seems that you have not completed the task." user message.
  - Otherwise: set step state to `CALLING_TOOL`, dispatch via `_tool_caller.parallel_tool_call()` or `sequential_tool_call()`, attach results as `LLMMessage(tool_result=...)` messages, then optionally append a `REFLECTING` step with a self-reflection.
- `_finalize_step(step, messages, execution)` — record & push to console.
- `reflect_on_result(tool_results)` — default: returns a concatenated failure summary for any failed tool; `TraeAgent` overrides to `None`.
- `llm_indicates_task_completed(llm_response)` — default uses keyword matching; `TraeAgent` overrides to require `task_done` tool call.
- `_is_task_completed(llm_response)` — default returns True; `TraeAgent` requires a non-empty git diff (after removing test patches) when `must_patch == "true"`.
- `task_incomplete_message()` — "ERROR! Your Patch is empty..." in `TraeAgent`.

## 4.4 `trae_agent.py` — `TraeAgent`

`TraeAgent` is the SE-specialized agent. It extends the loop with:

- **Default tool set** (`TraeAgentToolNames`): `str_replace_based_edit_tool`, `sequentialthinking`, `json_edit_tool`, `task_done`, `bash`. These are registered in `new_task()` when no tools are configured.
- **MCP discovery** (`initialise_mcp`, `discover_mcp_tools`): for each MCP server listed in `allow_mcp_servers`, connect via `MCPClient` and append discovered tools to `self._tools`. Failures are caught and the failed client is cleaned up.
- **Patch verification**:
  - `get_git_diff()` — runs `git --no-pager diff` in `project_path` (or `git --no-pager diff base_commit HEAD` if a base commit was supplied).
  - `remove_patches_to_tests(model_patch)` — filters out diffs whose target path matches `/test/`, `/tests/`, `/testing/`, `test_`, or `tox.ini`. Attribution note: the function is derived from the Aider SWE-bench test harness (see comments at the top of `trae_agent.py`).
  - `_is_task_completed` returns false if `must_patch == "true"` and the resulting diff is empty.
- **`execute_task()` override**: after `super().execute_task()`, finalizes the trajectory and, if `patch_path` is set, writes the post-task git diff to that file.
- **`cleanup_mcp_clients()` override**: closes every recorded `MCPClient`.
- **`llm_indicates_task_completed()` override**: returns true iff the LLM emitted a `task_done` tool call.

## 4.5 `agent.py` — `Agent` Factory

```python
class AgentType(Enum):
    TraeAgent = "trae_agent"

class Agent:
    def __init__(self, agent_type, config, trajectory_file=None, cli_console=None,
                 docker_config=None, docker_keep=True):
        ...
        match self.agent_type:
            case AgentType.TraeAgent:
                self.agent_config = config.trae_agent
                self.agent = TraeAgent(self.agent_config, docker_config, docker_keep)
                self.agent.set_cli_console(cli_console)
        if cli_console and config.trae_agent.enable_lakeview:
            cli_console.set_lakeview(config.lakeview)
        self.agent.set_trajectory_recorder(self.trajectory_recorder)

    async def run(self, task, extra_args=None, tool_names=None):
        self.agent.new_task(task, extra_args, tool_names)
        if self.agent.allow_mcp_servers:
            await self.agent.initialise_mcp()
        # print task details via cli_console
        execution = await self.agent.execute_task()
        return execution
```

`Agent.run()` is the bridge between the CLI and the concrete agent. The CLI always uses this single entry point.

## 4.6 `prompt/agent_prompt.py` — System Prompt

The single constant `TRAE_AGENT_SYSTEM_PROMPT` instructs the LLM to:

1. **Understand the Problem** — read the issue.
2. **Explore and Locate** — find relevant files.
3. **Reproduce the Bug (Crucial Step)** — write a reproduction script before fixing.
4. **Debug and Diagnose** — pinpoint the root cause.
5. **Develop and Implement a Fix** — apply the patch.
6. **Verify and Test Rigorously** — re-run the reproduction, run existing tests, write new tests.
7. **Summarize Your Work** — produce a final summary.
8. **Use `sequential_thinking`** — encourage at least 5 thoughts, up to 25.
9. **Call `task_done`** — when the issue is solved.

Key rule: **all file paths must be absolute**, prefixed with the `[Project root path]` from the user message.

## 4.7 `docker_manager.py` — `DockerManager`

A wrapper around the `docker` SDK plus a `pexpect`-driven persistent bash shell.

### Constructor

```python
DockerManager(
    image=None, container_id=None, dockerfile_path=None,
    docker_image_file=None, workspace_dir=None,
    tools_dir=None, interactive=False,
)
```

Exactly one of `image`, `container_id`, `dockerfile_path`, or `docker_image_file` must be supplied (else `ValueError`).

### `start()`

- If `dockerfile_path`: `client.images.build(...)` with a unique tag `trae-agent-custom:<uuid>`.
- Else if `docker_image_file`: `client.images.load(...)` from a tar archive.
- Else if `container_id`: `client.containers.get(container_id)` (marks `_is_managed = False`).
- Else if `image`: `client.containers.run(image, command="sleep infinity", detach=True, volumes={host_workspace: "/workspace"})`.
- Then `_copy_tools_to_container()` and `_start_persistent_shell()`.

### `execute(command, timeout=300)`

Returns `(exit_code, output)`. Uses a marker pattern (`echo ---CMD_DONE---$?`) over the pexpect shell to capture the exit code reliably and filter echoed commands from the output.

### `stop()`

Closes the pexpect shell. If the container was started by this instance (`_is_managed == True`), stops and removes it.

### Constants

- `CONTAINER_TOOLS_PATH = "/agent_tools"` — the path inside the container where PyInstaller-packed tool binaries are copied.

## 4.8 Class & Function Reference

### `BaseAgent`

| Symbol | Signature | Purpose |
|---|---|---|
| `__init__` | `(agent_config, docker_config=None, docker_keep=True)` | Build LLM client, tools, executor. |
| `llm_client` | property | Exposes the `LLMClient`. |
| `trajectory_recorder` / `set_trajectory_recorder` | `TrajectoryRecorder \| None` | Observation hook. |
| `cli_console` / `set_cli_console` | `CLIConsole \| None` | UI hook. |
| `tools` | property | Current tool list. |
| `task` / `initial_messages` / `model_config` / `max_steps` | properties | Read-only access. |
| `new_task` | `@abstractmethod (task, extra_args, tool_names)` | Build the conversation seed. |
| `execute_task` | `async () -> AgentExecution` | Run the loop. |
| `cleanup_mcp_clients` | `@abstractmethod async ()` | Release MCP. |
| `reflect_on_result` | `(tool_results) -> str \| None` | Custom reflection. |
| `llm_indicates_task_completed` | `(llm_response) -> bool` | Override-able completion check. |
| `_is_task_completed` | `(llm_response) -> bool` | Custom strict check. |
| `task_incomplete_message` | `() -> str` | Prompt injected on incomplete. |
| `_run_llm_step` | `async (step, messages, execution) -> messages` | LLM interaction per step. |
| `_tool_call_handler` | `async (tool_calls, step) -> messages` | Tool dispatch. |
| `_finalize_step` | `async (step, messages, execution)` | Persist & notify. |
| `_record_handler` | `(step, messages)` | Hand to trajectory recorder. |
| `_update_cli_console` | `(step, execution)` | Notify the UI. |
| `_update_llm_usage` | `(llm_response, execution)` | Accumulate tokens. |
| `_close_tools` | `async ()` | Free tool resources. |

### `TraeAgent`

| Symbol | Notes |
|---|---|
| `TraeAgentToolNames` | Module-level list of default tools. |
| `__init__(trae_agent_config, docker_config=None, docker_keep=True)` | Stores MCP configs and patch flags. |
| `initialise_mcp` / `discover_mcp_tools` | Async MCP tool discovery. |
| `new_task` | Validates `project_path` / `issue`, builds the user message, starts the trajectory recorder. |
| `execute_task` | Wraps the parent, finalizes the trajectory, writes the patch if `patch_path` is set. |
| `get_system_prompt` | Returns `TRAE_AGENT_SYSTEM_PROMPT`. |
| `reflect_on_result` | Returns `None` (no internal reflection on the SE flow). |
| `get_git_diff` | `git --no-pager diff` (or with base commit) in `project_path`. |
| `remove_patches_to_tests` | Filters out test-related diffs. |
| `llm_indicates_task_completed` | True iff `task_done` is called. |
| `_is_task_completed` | Enforces a non-empty patch when `must_patch == "true"`. |
| `task_incomplete_message` | The standard "ERROR! Your Patch is empty..." string. |
| `cleanup_mcp_clients` | Closes every `MCPClient`. |

### `Agent`

| Symbol | Notes |
|---|---|
| `AgentType` | Enum (currently only `trae_agent`). |
| `__init__` | Factory: builds the right `BaseAgent` subclass and sets console + trajectory recorder. |
| `run` | `async (task, extra_args, tool_names) -> AgentExecution` |

Continue with [Tools](./05_tools.md).
