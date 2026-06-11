# 5. Tools

The Tools subsystem implements the LLM's "hands". A `Tool` describes its name, parameters, and an async `execute(arguments)` method. A `ToolExecutor` dispatches `ToolCall` objects to the right tool.

## 5.1 Base Layer — `trae_agent/tools/base.py`

### Data Classes

| Class | Fields | Purpose |
|---|---|---|
| `ToolError(Exception)` | `message` | Base tool error. |
| `ToolExecResult` | `output, error, error_code` (0 = success) | Internal return of `Tool.execute`. |
| `ToolResult` | `call_id, name, success, result, error, id` | Standardized return sent to the LLM. |
| `ToolCall` | `name, call_id, arguments, id` | Parsed LLM-emitted call. |
| `ToolParameter` | `name, type, description, enum, items, required` | Single param spec. |
| `ToolCallArguments` | type alias | `dict[str, str \| int \| float \| dict \| list \| None]` |

### `class Tool(ABC)`

- `__init__(model_provider=None)` — stores the provider name (matters for OpenAI's strict schema).
- Abstract methods: `get_name()`, `get_description()`, `get_parameters()`, `async execute(arguments)`.
- `model_provider`, `name`, `description`, `parameters` are `@cached_property`.
- `json_definition()` — convenience for `{"name", "description", "parameters"}`.
- `get_input_schema()` — converts the parameter list into a JSON schema. **OpenAI strict mode** is handled here: every param is marked required (optional ones become `["<type>", "null"]`), and the top-level / nested objects get `additionalProperties: false`.
- `async close()` — virtual; `BashTool` overrides it to clean the session.

### `class ToolExecutor`

```python
class ToolExecutor:
    def __init__(self, tools: list[Tool])
    def tools -> dict[str, Tool]   # normalized lookup (lowercase, underscores removed)
    async def execute_tool_call(tool_call) -> ToolResult
    async def parallel_tool_call(tool_calls) -> list[ToolResult]
    async def sequential_tool_call(tool_calls) -> list[ToolResult]
    async def close_tools()        # gathers tool.close() coroutines
```

Lookup is name-normalized (`name.lower().replace("_", "")`) so the LLM can call a tool with slightly different formatting than its registry key.

## 5.2 Tool Registry — `trae_agent/tools/__init__.py`

```python
tools_registry: dict[str, type[Tool]] = {
    "bash": BashTool,
    "str_replace_based_edit_tool": TextEditorTool,
    "json_edit_tool": JSONEditTool,
    "sequentialthinking": SequentialThinkingTool,
    "task_done": TaskDoneTool,
    "ckg": CKGTool,
}
```

The agent looks up the concrete `Tool` class by string in `BaseAgent.__init__`.

## 5.3 `BashTool` — `bash`

- Backed by a `_BashSession` (a long-lived `asyncio.subprocess` shell).
- Sentinel-based protocol: each command is sent as `(<cmd>); echo ,,,,bash-command-exit-__ERROR_CODE__-banner,,,,` and the executor waits for the sentinel, extracts the exit code, and returns `(output, error, error_code)`.
- **Cross-platform**: on Windows, uses `cmd.exe /v:on` with `!errorlevel!` delayed expansion; on Unix, `/bin/bash` with `preexec_fn=os.setsid`.
- Timeout: 120 s per command (configurable on `_BashSession`).
- The `restart: true` argument re-spawns the session.
- `close()` terminates the process gracefully (terminate → wait 5 s → kill).

## 5.4 `TextEditorTool` — `str_replace_based_edit_tool`

Operations (selected by `command` arg):

| Command | Description |
|---|---|
| `view` | Show file content with line numbers (`cat -n`), or list a directory up to 2 levels deep via `find … -maxdepth 2`. Supports `view_range: [start, end]` (or `[start, -1]`). |
| `create` | Create a new file with `file_text`. Fails if path already exists. |
| `str_replace` | Replace `old_str` (unique) with `new_str`. Provides a snippet of the edited region. |
| `insert` | Insert `new_str` after line `insert_line`. |

Validation: path must be absolute, file/dir existence is checked, directories only allow `view`.

Long output is truncated via `run.maybe_truncate(..., MAX_RESPONSE_LEN=16000)` with a notice that the LLM should `grep` first.

## 5.5 `JSONEditTool` — `json_edit_tool`

Operations:

| Operation | Description |
|---|---|
| `view` | Show the whole file or a JSONPath. |
| `set` | Update value at a JSONPath. |
| `add` | Add a new key (object) or insert at index (array). |
| `remove` | Delete element(s) at a JSONPath. |

- Uses `jsonpath_ng` for parsing and matching.
- JSONPath must be valid; JSON file must parse cleanly; paths must be absolute.
- Pretty-printing is preserved (configurable).
- Implementation detail: `_add_json_value` uses `jsonpath_expr.left` / `right` to identify the parent and the target; supports `Fields` (object keys) and `Index` (array positions).

## 5.6 `SequentialThinkingTool` — `sequentialthinking`

A chain-of-thought notebook. Each invocation produces a `ThoughtData`:

```python
@dataclass
class ThoughtData:
    thought: str
    thought_number: int
    total_thoughts: int
    next_thought_needed: bool
    is_revision: bool | None = None
    revises_thought: int | None = None
    branch_from_thought: int | None = None
    branch_id: str | None = None
    needs_more_thoughts: bool | None = None
```

- Validates that `thought_number` and `total_thoughts ≥ 1`.
- Tracks `thought_history` and `branches: dict[branch_id, list[ThoughtData]]`.
- `total_thoughts` is auto-extended when `thought_number > total_thoughts`.
- Returns a JSON status with current thought number, branches, and history length.
- The system prompt encourages at least 5 thoughts.

## 5.7 `TaskDoneTool` — `task_done`

- No parameters.
- Returns "Task done." regardless of arguments.
- `TraeAgent.llm_indicates_task_completed()` looks for this exact call name.

## 5.8 `MCPTool` — discovered MCP tools

`MCPTool` is dynamically created by `MCPClient.connect_and_discover`:

```python
class MCPTool(Tool):
    def __init__(self, client, tool: mcp.types.Tool, model_provider=None)
    async def execute(self, arguments) -> ToolExecResult:
        output = await self.client.call_tool(self.name, arguments)
        if output.isError:
            return ToolExecResult(error=output.content[0].text)
        return ToolExecResult(output=output.content[0].text)
```

The schema is converted from the MCP `inputSchema` (JSON Schema) into `ToolParameter` instances. The registry key is the tool's real name (not `mcp`); the agent appends these into its tool list.

## 5.9 `CKGTool` — `ckg`

A code-knowledge-graph query tool.

| Command | Purpose |
|---|---|
| `search_function` | Find function definitions (name, body, line range, parent class/function). |
| `search_class` | Find class definitions (fields, methods, body). |
| `search_class_method` | Find class methods (with parent class). |

The CKG is built on first use by `CKGDatabase` (see [CKG Module](./11_subprojects.md#ckg-module)) using tree-sitter and stored in a SQLite database under `~/.trae-agent/ckg/<snapshot_hash>.db`. Snapshots are keyed by git commit hash (or file metadata for non-git repos). The CKG entry cache expires after a week.

Output is truncated via `MAX_RESPONSE_LEN`.

## 5.10 `DockerToolExecutor` — local vs. Docker routing

```python
class DockerToolExecutor:
    def __init__(self, original_executor, docker_manager, docker_tools,
                 host_workspace_dir, container_workspace_dir)
    async def close_tools()                 # delegates to original executor
    async def sequential_tool_call(calls)   # per-call routing
    async def parallel_tool_call(calls)     # alias to sequential (no parallelism over the shell)
    def _translate_path(host_path)          # host<->container path translation
    def _execute_in_docker(tool_call)      # builds a CLI command for bash/edit/json_edit
```

Routing rule: if `tool_call.name` is in `docker_tools` (`bash`, `str_replace_based_edit_tool`, `json_edit_tool`), the call is translated into a command for the prebuilt PyInstaller binary inside the container; otherwise the original (local) executor handles it. Path arguments named `path` are translated from host to container.

The CLI command building:

- `bash` → directly executes the inner `command` string inside the container.
- `str_replace_based_edit_tool` → `<tools>/edit_tool <sub_command> --key value …`.
- `json_edit_tool` → `<tools>/json_edit_tool --key value …`, with `value` JSON-serialized.

This is what makes Docker mode work for code-modifying tasks without re-implementing the tools in Python inside the container.

## 5.11 `edit_tool_cli.py` and `json_edit_tool_cli.py` — PyInstaller Entries

These scripts are minimal CLI wrappers around the edit and JSON-edit logic. They are intended to be packaged with PyInstaller (see `cli.py:build_with_pyinstaller()`) into single-file executables copied into the container at `/agent_tools/`. They contain:

- Stand-in shims for `Tool`, `ToolError`, `ToolExecResult`, etc.
- `maybe_truncate()` for output capping.
- Self-contained `run()` (subprocess) and a `TextEditorTool` / `JSONEditTool` that prints the result and exits.

When the `dist/` folder is missing and the user enables Docker mode, `build_with_pyinstaller()` rebuilds them on demand:

```python
subprocess.run(["pyinstaller", "--name", "edit_tool", "trae_agent/tools/edit_tool_cli.py"], check=True)
subprocess.run(["pyinstaller", "--name", "json_edit_tool", "--hidden-import=jsonpath_ng", "trae_agent/tools/json_edit_tool_cli.py"], check=True)
# then copies the executables to trae_agent/dist
```

## 5.12 `run.py` — Async Shell Helper

```python
async def run(cmd: str, timeout: float = 120.0, truncate_after: int | None = MAX_RESPONSE_LEN)
    -> tuple[int, str, str]   # (returncode, stdout, stderr)
```

A tiny wrapper around `asyncio.create_subprocess_shell` that:

- applies `maybe_truncate` to both stdout and stderr,
- kills the process on timeout (raising `TimeoutError`).

`maybe_truncate` is also used by `BashTool` indirectly (via the `view` command in `TextEditorTool`).

## 5.13 Building / Extending a Tool

To add a new tool:

1. Subclass `Tool`, implement `get_name`, `get_description`, `get_parameters`, `async execute`.
2. Add the class to `tools_registry` in `trae_agent/tools/__init__.py`.
3. Reference the key from `tools:` in your YAML config or pass it via `tool_names=` to `Agent.run(...)`.
4. (Optional) Add a parallel `*_cli.py` if you want the tool to also be available inside a Docker container.

Continue with [LLM Clients](./06_llm_clients.md).
