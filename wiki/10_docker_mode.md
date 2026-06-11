# 10. Docker Mode

Trae Agent can execute its tool calls inside a Docker container. This is useful for:

- Sandboxing the agent (no risk of damaging the host).
- Reproducible execution environments.
- Running against a pre-built dev image.

## 10.1 CLI Options

```bash
trae-cli run "..." \
  --docker-image python:3.11 \
  --working-dir /path/to/project

trae-cli run "..." --docker-container-id 91998a56056c
trae-cli run "..." --dockerfile-path /abs/path/Dockerfile
trae-cli run "..." --docker-image-file /path/to/image.tar
trae-cli run "..." --docker-image python:3.11 --docker-keep false
```

Flags `--docker-image`, `--docker-container-id`, `--dockerfile-path`, and `--docker-image-file` are **mutually exclusive** — passing more than one aborts with an error.

## 10.2 What the CLI Does When Docker Mode is Enabled

In [`cli.py:run`](../trae-agent/trae_agent/cli.py):

1. Pick the right `docker_config` dict from the four flags.
2. Call `check_docker()` to verify both the CLI is on PATH and the daemon is responsive.
3. If `trae_agent/dist/` (or `trae_agent/dist/_internal`) is missing, call `build_with_pyinstaller()` to build the `edit_tool` and `json_edit_tool` executables. This is a one-time cost per machine.
4. Build an `Agent(agent_type, config, trajectory_file, cli_console, docker_config, docker_keep)`.
5. Pass `docker_config["workspace_dir"] = working_dir` to the agent.
6. If `docker_config` is `None`, the CLI `os.chdir`s into `working_dir`. Otherwise it leaves the host CWD alone (the agent works inside the container).
7. On `KeyboardInterrupt` or any other exception, print a Docker-friendly hint if the error is a `DockerException`.

## 10.3 `DockerManager` — `trae_agent/agent/docker_manager.py`

### Class Constants

```python
CONTAINER_TOOLS_PATH = "/agent_tools"
container_workspace  = "/workspace"
```

### Lifecycle

```python
manager = DockerManager(
    image=cfg.get("image"),
    container_id=cfg.get("container_id"),
    dockerfile_path=cfg.get("dockerfile_path"),
    docker_image_file=cfg.get("docker_image_file"),
    workspace_dir=cfg["workspace_dir"],
    tools_dir=tools_dir,           # trae_agent/dist
    interactive=False,
)
manager.start()
```

`start()`:

- **Dockerfile** path → `client.images.build(...)` with a unique tag `trae-agent-custom:<uuid>`.
- **Image file** → `client.images.load(open(file).read())` and pick the first tag.
- **Container ID** → `client.containers.get(...)` (marked `_is_managed = False` so `stop()` won't try to remove it).
- **Image name** → `client.containers.run(image, command="sleep infinity", detach=True, volumes={host: /workspace})`.

After starting, `_copy_tools_to_container()` and `_start_persistent_shell()` run.

### Tool Copy

```bash
docker cp <abs_tools_dir> <container.id>:/agent_tools
```

(`<abs_tools_dir>` defaults to `trae_agent/dist/`.)

### Persistent Shell

Uses `pexpect.spawn("docker exec -it <id> /bin/bash", encoding="utf-8", timeout=120)` and waits for `$` or `#` to confirm the shell is ready. This shell is shared by every docker-routed tool call.

### `execute(command, timeout=300)`

```text
sendline <command>
sendline echo ---CMD_DONE---$?
expect ---CMD_DONE---(\d+)
   ↳ exit_code = int(match.group(1))
   ↳ output_before_marker = shell.before
   ↳ filter echoed commands
   ↳ expect next prompt
return (exit_code, output)
```

If the command times out, returns `(-1, "Error: Command '...' timed out after N seconds. Partial output:\n...")`.

### `stop()`

Closes the pexpect shell; if `_is_managed`, also `container.stop()` and `container.remove()`.

## 10.4 `DockerToolExecutor` — `trae_agent/tools/docker_tool_executor.py`

A `ToolExecutor`-compatible object that splits each call between local and docker execution.

### Routing

```python
if tool_call.name in self._docker_tools_set:   # {"bash", "str_replace_based_edit_tool", "json_edit_tool"}
    return self._execute_in_docker(tool_call)
else:
    return await self._original_executor.sequential_tool_call([tool_call])[0]
```

`parallel_tool_call` is implemented as sequential, because the persistent shell cannot run multiple commands concurrently.

### Path Translation

```python
def _translate_path(self, host_path):
    if not self._host_workspace_dir:
        return host_path
    abs_host_path = os.path.abspath(host_path)
    if os.path.commonpath([abs_host_path, self._host_workspace_dir]) == self._host_workspace_dir:
        relative = os.path.relpath(abs_host_path, self._host_workspace_dir)
        return os.path.normpath(os.path.join(self._container_workspace_dir, relative))
    return host_path
```

Only `path` arguments are translated (this is a deliberate scope — it covers `str_replace_based_edit_tool` and `bash` for typical SWE-bench flows).

### Command Building

For each docker-routed tool:

| Tool | Build |
|---|---|
| `bash` | `command_to_run = arguments["command"]` (passed as-is to the shell). |
| `str_replace_based_edit_tool` | `cmd = /agent_tools/edit_tool <sub_command> --<key> <value> ...` |
| `json_edit_tool` | `cmd = /agent_tools/json_edit_tool --<key> '<value>' ...` (the `value` argument is JSON-serialized). |

The resulting command is sent via `docker_manager.execute(command_to_run)`, which returns `(exit_code, output)`. The executor wraps that into a `ToolResult`.

### `close_tools`

Delegates to the original (local) executor.

## 10.5 PyInstaller Packaging — `cli.build_with_pyinstaller()`

```python
subprocess.run(["pyinstaller", "--name", "edit_tool",
                "trae_agent/tools/edit_tool_cli.py"], check=True)
subprocess.run(["pyinstaller", "--name", "json_edit_tool",
                "--hidden-import=jsonpath_ng",
                "trae_agent/tools/json_edit_tool_cli.py"], check=True)

mkdir trae_agent/dist
cp dist/edit_tool/edit_tool        trae_agent/dist
cp -r dist/json_edit_tool/_internal trae_agent/dist
cp   dist/json_edit_tool/json_edit_tool trae_agent/dist
rm -rf dist
```

Notes:

- The `edit_tool_cli.py` and `json_edit_tool_cli.py` modules intentionally re-define minimal versions of the `Tool` base class so that the resulting single-file binary has no dependency on the rest of `trae_agent`.
- `jsonpath_ng` is a hidden import because PyInstaller can't detect dynamic imports.
- The binary location is the `tools_dir` passed to `DockerManager`, which is `<project>/trae_agent/dist` by default.
- PyInstaller is itself a dependency (`pyinstaller==6.15.0` in `pyproject.toml`).

## 10.6 End-to-end Docker Flow

```text
trae-cli run "fix bug" --docker-image python:3.11 -w /code/project
   │
   ▼
check_docker() → OK
   │
   ▼
build_with_pyinstaller() (first time only) → trae_agent/dist/{edit_tool,json_edit_tool}
   │
   ▼
Agent(..., docker_config={"image": "python:3.11", "workspace_dir": "/code/project"})
   │
   ▼
TraeAgent(BaseAgent) → DockerManager.start()
                          ├─ pull/run container (sleep infinity)
                          ├─ mount /code/project → /workspace
                          ├─ docker cp trae_agent/dist → /agent_tools
                          └─ pexpect bash shell ready
   │
   ▼
DockerToolExecutor wraps local ToolExecutor
   │
   ▼
step loop:
   bash("ls /workspace")              → docker_manager.execute("ls /workspace")
   str_replace_based_edit_tool(...)   → docker_manager.execute("/agent_tools/edit_tool view --path /workspace/foo.py")
   json_edit_tool(...)                → docker_manager.execute("/agent_tools/json_edit_tool --operation set --file_path /workspace/x.json --value '...'")
   │
   ▼
agent.execute_task() ends
   │
   ▼
docker_manager.stop()  (unless --docker-keep true)
```

Continue with [Sub-projects](./11_subprojects.md).
