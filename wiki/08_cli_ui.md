# 8. CLI & UI

The CLI is defined in [`trae_agent/cli.py`](../trae-agent/trae_agent/cli.py) using `click`. The console UI is defined in `trae_agent/utils/cli/`.

## 8.1 Console Script

`pyproject.toml` registers:

```toml
[project.scripts]
trae-cli = "trae_agent.cli:main"
```

So `trae-cli` is the user-facing binary. `python -m trae_agent.cli` works equivalently.

## 8.2 Sub-commands

### `trae-cli run`

```bash
trae-cli run [TASK]
  -f, --file PATH                # Path to a file containing the task description.
  -p, --provider NAME           # LLM provider (e.g., openai, anthropic, google, doubao, openrouter, ollama, azure)
  -m, --model NAME              # Specific model to use
      --model-base-url URL      # Override provider base URL
  -k, --api-key KEY             # API key (or use env var)
      --max-steps N             # Max number of execution steps
  -w, --working-dir DIR         # Working directory for the agent
  -mp, --must-patch             # Require a non-empty patch
      --config-file PATH        # Default: trae_config.yaml
  -t, --trajectory-file PATH    # Save trajectory file
  -pp, --patch-path PATH        # Save the final patch to this file
  --docker-image IMAGE          # Run in a new container
  --docker-container-id ID      # Attach to an existing container
  --dockerfile-path PATH        # Build a new image from a Dockerfile
  --docker-image-file PATH      # Load a local image tar
      --docker-keep BOOL        # Keep the container (default true)
  -ct, --console-type [simple|rich]
  -at, --agent-type [trae_agent]
```

Flags at the Docker level are mutually exclusive — passing more than one aborts with a `sys.exit(1)`.

### `trae-cli interactive`

```bash
trae-cli interactive
  -p, --provider NAME
  -m, --model NAME
      --model-base-url URL
  -k, --api-key KEY
      --config-file PATH
      --max-steps N
  -t, --trajectory-file PATH
  -ct, --console-type [simple|rich]
  -at, --agent-type [trae_agent]
```

In interactive mode the CLI offers a REPL with built-in commands:

| Command | Effect |
|---|---|
| *(any text)* | Run a new task. |
| `help` | Show available commands. |
| `status` | Show current agent status (provider, model, tools, etc.). |
| `clear` | Clear the screen. |
| `exit` / `quit` | End the session. |

The implementation has two flavours:

- **Simple console**: `_run_simple_interactive_loop` — a `while True` loop that reads input, prints, and runs the agent.
- **Rich console (TUI)**: `_run_rich_interactive_loop` — delegates to the Textual app (`RichConsoleApp`) which owns the entire interaction (input box, execution log, token display).

### `trae-cli show-config`

Pretty-prints the resolved configuration as two rich tables: one for the agent's general settings and one for the chosen provider.

### `trae-cli tools`

Iterates `tools_registry`, instantiates each tool, and prints `name` and `description` in a rich table.

## 8.3 `cli.py` Helpers

- `resolve_config_file(config_file)` — if the file ends with `.yaml`/`.yml` but the YAML doesn't exist, looks for a same-named `.json` (prints a warning) and uses that. Otherwise exits with a clear error.
- `check_docker(timeout=3)` — checks whether the docker CLI is on PATH and whether the daemon responds (`docker version --format '{{.Server.Version}}'`). Returns a dict with `cli`, `daemon`, `version`, `error`.
- `build_with_pyinstaller()` — invoked when Docker mode is enabled for the first time. Runs `pyinstaller` for `edit_tool` and `json_edit_tool`, then copies the executables into `trae_agent/dist/`.

## 8.4 CLI Console — `trae_agent/utils/cli/`

### `cli_console.py`

- Defines `ConsoleMode` (`RUN`, `INTERACTIVE`) and `ConsoleType` (`SIMPLE`, `RICH`).
- `AGENT_STATE_INFO` — maps each `AgentStepState` to a color/emoji pair.
- `ConsoleStep` — wrapper around `AgentStep` plus a `lake_view_panel_generator` task and an `agent_step_printed` flag.
- `CLIConsole(ABC)` — base for the two implementations:
  - `start()` — runs the console loop until the agent is `COMPLETED` / `ERROR`.
  - `update_status(step, execution)` — called by the agent loop on every state change.
  - `print_task_details(details)` — initial print of the task.
  - `print(message, color, bold)` — generic printer.
  - `get_task_input() / get_working_dir_input()` — for interactive mode.
  - `stop()` — cleanup.
  - `set_lakeview(config)` — attaches a `LakeView` summarizer (or `None`).
- `generate_agent_step_table(step)` — converts an `AgentStep` into a rich `Table` for printing.

### `simple_console.py` — `SimpleCLIConsole`

- Uses `rich.console.Console` for all output.
- `update_status(step, execution)` caches each step in `console_step_history`. When the step state is `COMPLETED` or `ERROR` (and not yet printed), it calls `_print_step_update` and starts a Lakeview background task for the step.
- `start()` waits for the agent to finish, then prints the Lakeview summary (if enabled) and the execution summary (success, steps, execution time, total tokens, final result as a Markdown `Panel`).
- Interactive-mode input comes from `input()`.

### `rich_console.py` — `RichCLIConsole`

- Wraps a `textual.app.App` (`RichConsoleApp`).
- Composes a `Header`, a `RichLog` (execution log), an `Input` (for interactive mode), a task display, a `TokenDisplay` (live token count), and a `Footer`.
- CSS file: `rich_console.tcss` (Textual CSS).
- `TokenDisplay` is a `Static` widget whose `total_tokens`/`input_tokens`/`output_tokens` are `reactive` and update live as the agent progresses.
- Interactive commands (`help`, `status`, `clear`, `exit`, `quit`) are routed to dedicated handler methods.
- For `RUN` mode, the TUI displays the task in a static panel; for `INTERACTIVE` mode, it adds the input box with `SuggestFromList` for the built-in commands.

### `console_factory.py`

- `ConsoleFactory.create_console(console_type, mode, lakeview_config)` → `SimpleCLIConsole` or `RichCLIConsole`.
- `ConsoleFactory.get_recommended_console_type(mode)`:
  - `INTERACTIVE` → `RICH` (TUI shines for repeated input).
  - `RUN` → `SIMPLE` (one-shot task, less overhead).

## 8.5 Lakeview — `trae_agent/utils/lake_view.py`

A summarizer that runs in parallel with the agent loop and decorates each step with two pieces of metadata:

1. A natural-language **task description** extracted by `EXTRACTOR_PROMPT`. Format: `<task>...</task><details>...</details>`.
2. A set of **tags** chosen from `KNOWN_TAGS`:

| Tag | Emoji | Meaning |
|---|---|---|
| `WRITE_TEST` | ☑️ | Writing/reproducing a test. |
| `VERIFY_TEST` | ✅ | Running tests. |
| `EXAMINE_CODE` | 👁️ | Exploring code. |
| `WRITE_FIX` | 📝 | Modifying source. |
| `VERIFY_FIX` | 🔥 | Verifying the fix. |
| `REPORT` | 📣 | Reporting progress. |
| `THINK` | 🧠 | Pure thinking step. |
| `OUTLIER` | ⁉️ | Other. |

`extract_task_in_step` and `extract_tag_in_step` both retry up to 10 times to coerce the model into the right format. `create_lakeview_step(agent_step)` is the public entry used by the consoles.

The console prints the resulting panel after each step:

```text
[emoji] The agent <task>
<italic>details</italic>
```

## 8.6 End-to-end Status Flow

```text
BaseAgent._update_cli_console(step, execution)
       │
       ▼
CLIConsole.update_status(step, execution)
       │
       ├── SimpleCLIConsole: prints rich table when step COMPLETED
       │                      and starts Lakeview background task
       │
       └── RichCLIConsole:   updates the RichLog, task display, and TokenDisplay widgets
```

Continue with [Trajectory Recording & MCP](./09_trajectory_mcp.md).
