# 9. Trajectory Recording & MCP

Two cross-cutting subsystems are described here: the trajectory recorder (observation) and the MCP client (tool extension).

## 9.1 Trajectory Recorder — `trae_agent/utils/trajectory_recorder.py`

### Purpose

Capture every LLM call and every agent step into a JSON file for post-hoc analysis, debugging, and evaluation.

### Class

```python
class TrajectoryRecorder:
    def __init__(self, trajectory_path: str | None = None)
    def start_recording(task, provider, model, max_steps)
    def record_llm_interaction(messages, response, provider, model, tools=None)
    def record_agent_step(step_number, state, llm_messages=None, llm_response=None,
                          tool_calls=None, tool_results=None, reflection=None, error=None)
    def update_lakeview(step_number, lakeview_summary)
    def finalize_recording(success, final_result=None)
    def save_trajectory()
    def get_trajectory_path() -> str
```

### Default Path

```python
if trajectory_path is None:
    timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
    trajectory_path = f"trajectories/trajectory_{timestamp}.json"
```

The CLI also accepts an explicit `--trajectory-file` (`-t`) and a positional `Agent.trajectory_file` member exposes the resolved path.

### File Schema

```jsonc
{
  "task": "...",
  "start_time": "2025-...",
  "end_time":   "2025-...",
  "provider":   "anthropic",
  "model":      "claude-sonnet-4-20250514",
  "max_steps":  200,
  "success":    true,
  "final_result": "...",
  "execution_time": 12.34,

  "llm_interactions": [
    {
      "timestamp": "...",
      "provider": "anthropic",
      "model": "claude-sonnet-4-20250514",
      "input_messages": [...],
      "response": {
        "content": "...",
        "model": "claude-...",
        "finish_reason": "end_turn",
        "usage": {
          "input_tokens": 1234,
          "output_tokens": 56,
          "cache_creation_input_tokens": 0,
          "cache_read_input_tokens": 0,
          "reasoning_tokens": 0
        },
        "tool_calls": [
          { "call_id": "...", "name": "bash", "arguments": {...}, "id": null }
        ]
      },
      "tools_available": ["bash", "str_replace_based_edit_tool", ...]
    }
  ],

  "agent_steps": [
    {
      "step_number": 1,
      "timestamp": "...",
      "state": "completed",
      "llm_messages": [...],
      "llm_response": { ... },
      "tool_calls": [...],
      "tool_results": [...],
      "reflection": "...",
      "error": null,
      "lakeview_summary": "..."
    }
  ]
}
```

### Wiring

- `Agent.__init__` creates a `TrajectoryRecorder` (auto-path or explicit), then calls `self.agent.set_trajectory_recorder(recorder)`.
- `BaseAgent.set_trajectory_recorder` propagates the recorder to the LLM client (`self._llm_client.set_trajectory_recorder(recorder)`).
- `BaseAgent._record_handler` is called from `_finalize_step` and writes the step into the trajectory.
- Each provider client calls `record_llm_interaction` after a successful chat.

## 9.2 MCP Client — `trae_agent/utils/mcp_client.py`

### Purpose

Wrap the [Model Context Protocol (MCP)](https://modelcontextprotocol.io/) stdio transport so that the agent can call tools exposed by external MCP servers (e.g., Playwright, Airtable, etc.).

### Class

```python
class MCPClient:
    def __init__(self)
    def get_mcp_server_status(name) -> MCPServerStatus
    def update_mcp_server_status(name, status)
    async def connect_and_discover(name, config, container, model_provider)
    async def connect_to_server(name, transport)
    async def call_tool(name, args) -> mcp.CallToolResult
    async def list_tools() -> mcp.ListToolsResult
    async def cleanup(name)
```

Enums:
- `MCPServerStatus`: `DISCONNECTED`, `CONNECTING`, `CONNECTED`.
- `MCPDiscoveryState` (declared but currently informational): `NOT_STARTED`, `IN_PROGRESS`, `COMPLETED`.

### `connect_and_discover`

1. Inspects the `MCPServerConfig` to pick a transport. **Only stdio is implemented**; SSE / streamable HTTP / WebSocket raise `NotImplementedError` with a clear message.
2. Builds a `StdioServerParameters(command, args, env, cwd)`, then `await self.exit_stack.enter_async_context(stdio_client(params))`.
3. Calls `connect_to_server(name, transport)` which opens a `ClientSession` and runs `session.initialize()`.
4. `list_tools()` returns the list of MCP tools; for each tool, an `MCPTool` is created and appended to the caller's `container` (a `list[Tool]`).

### `call_tool`

Just delegates to `self.session.call_tool(name, args)` and returns the raw `mcp.CallToolResult`. `MCPTool.execute` interprets the response and produces a `ToolExecResult` (output or error).

### `cleanup`

Closes the `AsyncExitStack`, releasing all stdio transports, and marks the server as `DISCONNECTED`.

## 9.3 MCP Integration in `TraeAgent`

```python
async def discover_mcp_tools(self):
    if self.mcp_servers_config:
        for name, cfg in self.mcp_servers_config.items():
            if self.allow_mcp_servers is None or name not in self.allow_mcp_servers:
                continue
            client = MCPClient()
            try:
                await client.connect_and_discover(name, cfg, self.mcp_tools, self._llm_client.provider.value)
                self.mcp_clients.append(client)
            except Exception:
                with contextlib.suppress(Exception): await client.cleanup(name)
                continue
            except asyncio.CancelledError:
                with contextlib.suppress(Exception): await client.cleanup(name)
                continue

async def cleanup_mcp_clients(self):
    for client in self.mcp_clients:
        with contextlib.suppress(Exception):
            await client.cleanup("cleanup")
    self.mcp_clients.clear()
```

Notes:

- The provider's value (`self._llm_client.provider.value`) is passed to `MCPTool` so the tool schema can apply the OpenAI strict-mode handling if appropriate.
- `Agent.run` calls `await self.agent.initialise_mcp()` before kicking off the step loop, but only if `allow_mcp_servers` is non-empty.
- `Agent.run` then wraps `execute_task()` in a `try/finally` that always calls `cleanup_mcp_clients()`, so even unhandled exceptions cannot leak async resources.

## 9.4 Persistence Paths Summary

| Path | Set in | Purpose |
|---|---|---|
| `trajectories/trajectory_<timestamp>.json` | `TrajectoryRecorder.__init__` | Default trajectory file (auto-created). |
| `~/.trae-agent/ckg/*.db` | `trae_agent/utils/constants.py` (`LOCAL_STORAGE_PATH`) and `tools/ckg/ckg_database.py` | CKG SQLite cache. |
| `~/.trae-agent/ckg/storage_info.json` | `ckg_database.py` | CKG snapshot metadata. |
| `trae_agent/dist/edit_tool` | `cli.py:build_with_pyinstaller()` | PyInstaller-packed binary used by Docker mode. |
| `trae_agent/dist/json_edit_tool` | same | Same. |
| `trae_agent/dist/_internal` | same | PyInstaller runtime files for `json_edit_tool`. |

Continue with [Docker Mode](./10_docker_mode.md).
