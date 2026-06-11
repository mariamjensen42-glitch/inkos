# 11. Sub-projects

The main `trae_agent/` package is the agent. Surrounding it are four supporting trees: `evaluation/`, `server/`, `tests/`, and the CKG module under `trae_agent/tools/ckg/`.

## 11.1 `evaluation/` — SWE-bench Harness

Designed to run Trae Agent against SWE-bench–style benchmarks (SWE-bench, SWE-bench-Live, Multi-SWE-bench). Highlights from the README and code:

### `run_evaluation.py`

```bash
python -m evaluation.run_evaluation \
  --dataset <dataset> \
  --workspace <path> \
  --output <path> \
  --model <name> \
  --provider <name> \
  --instance-id <id>      # optional, single-instance debugging
```

The script:

1. Loads the dataset with the `datasets` library.
2. Resolves the per-instance base commit / repo.
3. For each instance, instantiates `Agent(AgentType.TraeAgent, config, ...)` and runs the agent on the SWE-bench task.
4. Captures the resulting git diff and stores it alongside the trajectory file in the output directory.
5. (Optional) Calls `evaluation/patch_selection` to pick the best patch if the agent produces multiple candidates.

### `setup.sh`

```bash
docker pull <image>
pip install datasets docker pexpect unidiff
```

Boilerplate for an evaluation VM.

### `utils.py`

- Helpers for dataset loading, file preprocessing, repo bootstrapping (e.g., resetting to the right base commit, applying the agent's patch, running tests).
- `parse_dockerfile` / `parse_test_log` — convenience for downstream scoring.

### `patch_selection/`

When the agent runs, the trajectory contains a long assistant message and an optional list of patches. A second agent (or a heuristic) selects the best patch. The README documents the selection process.

## 11.2 `server/` — FastAPI HTTP Server (WIP)

The `server/Readme.md` describes an early-stage HTTP server that exposes Trae Agent via a REST/JSON API. The current state is scaffolding; consult the file directly for the latest endpoints, models, and status.

## 11.3 `tests/` — Unit Tests

A pytest-based suite covering the agent, tools, utils, and CLI.

### Layout

```text
tests/
├── agent/
│   ├── test_base_agent.py
│   └── test_trae_agent.py
├── tools/
│   ├── test_base.py
│   ├── test_bash_tool.py
│   ├── test_edit_tool.py
│   ├── test_json_edit_tool.py
│   ├── test_task_done_tool.py
│   └── test_sequential_thinking_tool.py
├── utils/
│   ├── test_config.py
│   ├── test_legacy_config.py
│   ├── test_mcp_client.py
│   ├── test_console_factory.py
│   └── test_docker.py
├── e2e/
│   ├── test_examples.py
│   └── test_examples_docker.py
├── test_cli.py
└── conftest.py
```

### Notable Tests

- `test_cli.py` — verifies the `trae-cli` console script.
- `e2e/test_examples.py` / `test_examples_docker.py` — run real `Agent` runs against an LLM to verify the full flow.
- `test_docker.py` — checks `check_docker()` output and skips when Docker is not available.

### How to Run

```bash
make uv-test
# or
uv run pytest
```

Configuration lives under `[tool.pytest.ini_options]` in `pyproject.toml`: `asyncio_mode = "auto"`, `testpaths = ["tests"]`, etc.

## 11.4 CKG Module — `trae_agent/tools/ckg/`

A code knowledge graph built with tree-sitter and stored in SQLite.

### Files

| File | Purpose |
|---|---|
| `tools/ckg/__init__.py` | Re-exports `CKGDatabase`, `FunctionEntry`, `ClassEntry`. |
| `tools/ckg/base.py` | `FunctionEntry`, `ClassEntry`, `BASE_DIR_PER_LANGUAGE`, `map_python_to_java`, `map_python_to_go`, `map_python_to_cpp`. |
| `tools/ckg/ckg_database.py` | `CKGDatabase` — main API. |
| `tools/ckg_tool.py` | The `ckg` Tool. |

### `CKGDatabase`

```python
class CKGDatabase:
    def __init__(self, project_path, commit_id=None, regenerate=False)
    def create_ckg_database()
    def search_function(function_name) -> list[FunctionEntry]
    def search_class(class_name) -> list[ClassEntry]
    def search_class_method(class_name, method_name) -> list[FunctionEntry]
    def empty_or_missing() -> bool
```

#### Snapshotting

The database file path is computed as:

```python
snapshot_id = get_directory_hash(...)  # or commit_id
db_path = LOCAL_STORAGE_PATH / "ckg" / f"{snapshot_id}.db"
```

`get_directory_hash` excludes `.git`, `__pycache__`, `node_modules`, etc. and produces a stable hash for the working tree. If a `commit_id` is provided, the CKG is rebuilt whenever the commit ID changes.

#### Database Schema

```sql
CREATE TABLE files (path TEXT PRIMARY KEY, module_name TEXT);
CREATE TABLE functions (
  file_path TEXT, function_name TEXT, function_signature TEXT,
  function_text TEXT, start_line INTEGER, end_line INTEGER,
  class_name TEXT, function_id INTEGER PRIMARY KEY AUTOINCREMENT,
  parent_function TEXT
);
CREATE TABLE classes (
  class_name TEXT, file_path TEXT, class_text TEXT,
  start_line INTEGER, end_line INTEGER, class_id INTEGER PRIMARY KEY AUTOINCREMENT
);
CREATE TABLE methods (
  class_id INTEGER, function_id INTEGER, FOREIGN KEY ...
);
```

#### Tree-sitter Parsing

The parser dispatches on file extension:

| Language | Tree-sitter grammar |
|---|---|
| Python | `tree_sitter_python` |
| Java | `tree_sitter_java` |
| Go | `tree_sitter_go` |
| C++ | `tree_sitter_cpp` |
| Rust | `tree_sitter_rust` |
| TypeScript | `tree_sitter_typescript` |
| JavaScript | `tree_sitter_javascript` |
| C# | `tree_sitter_c_sharp` |
| Ruby | `tree_sitter_ruby` |

`_traverse(node, file_path, db_cursor, ...)` walks the AST and inserts functions / classes / methods. `_extract_method_text` joins the function + parent class body when needed.

#### `map_python_to_<lang>`

Converts Python-style class/function names to Java/Go/C++ names for cross-language search.

#### `CKGTool` (`ckg`)

`CKGTool.execute(arguments)` dispatches on `arguments["command"]`:

| Command | Returns |
|---|---|
| `search_function` | `ToolExecResult(output=str([...]))` with all `FunctionEntry` matching the name. |
| `search_class` | `ToolExecResult(output=str([...]))` with all `ClassEntry` matching the class name. |
| `search_class_method` | `ToolExecResult(output=str([...]))` with all `FunctionEntry` matching the method name in the given class. |

Output is truncated via `MAX_RESPONSE_LEN`. The CKG is created once per project (snapshot); the same `CKGDatabase` instance is reused across calls.

#### Storage Lifecycle

- `clear_older_ckg()` is called by `BaseAgent.__init__` and deletes any database files older than 7 days in the `ckg/` directory.
- `storage_info.json` records `created_at` and `snapshot_id` for each entry.

## 11.5 `docs/`

| File | Purpose |
|---|---|
| `docs/TRAJECTORY_RECORDING.md` | Detailed doc of `TrajectoryRecorder` (covered above). |
| `docs/legacy_config.md` | Field reference for the legacy JSON config. |
| `docs/roadmap.md` | Forward-looking plan: SDK, sandbox enhancements, MLOps, etc. |
| `docs/tools.md` | Tool-by-tool user guide. |

## 11.6 `docs/roadmap.md` — Highlights

(Summarized; see the actual file for the full list.)

- **Software Engineering SDK** — Public SDK for embedding Trae Agent in other products.
- **Sandbox Improvements** — A stable sandbox image + cloud-isolation strategy.
- **Agent Observability & MLOps** — Better trajectory analysis, cost & latency tracking, fine-tuning data extraction.
- **Visual Workflow** — Drag-and-drop agent workflow editor.
- **Smart Routing** — Model router that picks the best/cheapest model per step.

Continue with [Running & Extension Guide](./12_running_and_extension.md).
