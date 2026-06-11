# 6. LLM Clients

The LLM client layer ([`trae_agent/utils/llm_clients/`](../trae-agent/trae_agent/utils/llm_clients/)) abstracts the differences between seven model providers behind a single `LLMClient.chat()` interface.

## 6.1 Provider Enumeration — `LLMProvider`

```python
class LLMProvider(Enum):
    OPENAI = "openai"
    ANTHROPIC = "anthropic"
    AZURE = "azure"
    OLLAMA = "ollama"
    OPENROUTER = "openrouter"
    DOUBAO = "doubao"
    GOOGLE = "google"
```

The enum is keyed by the string in `ModelProvider.provider`.

## 6.2 Common Types — `llm_basics.py`

```python
@dataclass
class LLMMessage:
    role: str              # "system" | "user" | "assistant"
    content: str | None
    tool_call: ToolCall | None
    tool_result: ToolResult | None

@dataclass
class LLMUsage:
    input_tokens: int
    output_tokens: int
    cache_creation_input_tokens: int = 0
    cache_read_input_tokens: int = 0
    reasoning_tokens: int = 0

@dataclass
class LLMResponse:
    content: str
    usage: LLMUsage | None
    model: str | None
    finish_reason: str | None
    tool_calls: list[ToolCall] | None
```

Each provider client converts its own wire format to/from these dataclasses.

## 6.3 `LLMClient` — Provider Switcher

```python
class LLMClient:
    def __init__(self, model_config: ModelConfig):
        self.provider = LLMProvider(model_config.model_provider.provider)
        self.model_config = model_config
        match self.provider:
            case LLMProvider.OPENAI:     self.client = OpenAIClient(model_config)
            case LLMProvider.ANTHROPIC:  self.client = AnthropicClient(model_config)
            case LLMProvider.AZURE:      self.client = AzureClient(model_config)
            case LLMProvider.OPENROUTER: self.client = OpenRouterClient(model_config)
            case LLMProvider.DOUBAO:     self.client = DoubaoClient(model_config)
            case LLMProvider.OLLAMA:     self.client = OllamaClient(model_config)
            case LLMProvider.GOOGLE:     self.client = GoogleClient(model_config)

    def set_trajectory_recorder(self, recorder) -> None: ...
    def set_chat_history(self, messages) -> None: ...
    def chat(self, messages, model_config, tools=None, reuse_history=True) -> LLMResponse: ...
    def supports_tool_calling(self, model_config) -> bool: ...
```

Note that the import of each provider client is **lazy** (inside each `case` branch) so that providers you don't use don't have to be installed.

## 6.4 `BaseLLMClient` (ABC)

- Stores `api_key`, `base_url`, `api_version`, and an optional `trajectory_recorder`.
- `set_trajectory_recorder(recorder)` — called by `LLMClient.set_trajectory_recorder()`.
- `set_chat_history(messages)` and `chat(messages, model_config, tools, reuse_history)` are abstract.
- `supports_tool_calling(model_config)` default uses `model_config.supports_tool_calling`.

## 6.5 `retry_with()` Decorator — `retry_utils.py`

```python
def retry_with(func, provider_name="OpenAI", max_retries=3) -> Callable
```

- Wraps the function in a try/except.
- On any exception, sleeps for a random 3–30 seconds and retries, up to `max_retries` times.
- Re-raises the last exception when retries are exhausted.

Every provider's API call is wrapped with this decorator, so transient errors (rate limits, timeouts, network blips) are absorbed without surfacing to the agent loop.

## 6.6 `OpenAIClient` — `openai_client.py`

- Uses the **OpenAI Responses API** (`openai.responses.create`).
- Builds `FunctionToolParam` tool schemas with `strict=True` (which is why `Tool.get_input_schema` has the OpenAI branch making all params required and adding `additionalProperties: false`).
- `temperature` is omitted for `o3`, `o4-mini`, `gpt-5` models (uses `openai.NOT_GIVEN`).
- `max_output_tokens=model_config.max_tokens` (note the parameter name on the Responses API).
- Maintains a `message_history: ResponseInputParam` that grows with each call (when `reuse_history=True`).
- Tool-call outputs are appended as `ResponseFunctionToolCallParam`; assistant text as `EasyInputMessageParam`.
- Records trajectory via `self.trajectory_recorder.record_llm_interaction(...)` on every chat.
- Helpers:
  - `parse_messages(messages)` — convert `LLMMessage` list to `ResponseInputParam`.
  - `parse_tool_call(tool_call)` — `ResponseFunctionToolCallParam`.
  - `parse_tool_call_result(result)` — `FunctionCallOutput`.

## 6.7 `AnthropicClient` — `anthropic_client.py`

- Uses `anthropic.Anthropic.messages.create`.
- Special-cases two tools to use Anthropic's **native tool types**:
  - `str_replace_based_edit_tool` → `TextEditor20250429` (`type="text_editor_20250429"`).
  - `bash` → `ToolBash20250124` (`type="bash_20250124"`).
  - Everything else → generic `ToolParam` with `name`, `description`, `input_schema`.
- `parse_messages` separates the system message from the chat history (`self.system_message`).
- For each `tool_use` block, it adds an `anthropic.types.MessageParam(role="assistant", content=[content_block])` so the model can correlate the next tool result.
- `parse_tool_call_result` returns a `ToolResultBlockParam` with `is_error=not tool_call_result.success`.
- Records trajectory on every chat.

## 6.8 `OpenAICompatibleClient` — `openai_compatible_base.py`

A shared base for providers that speak the OpenAI Chat Completions API.

### `ProviderConfig` (ABC)

- `create_client(api_key, base_url, api_version) -> openai.OpenAI`
- `get_service_name()`, `get_provider_name()` (for logs / trajectory), `get_extra_headers()`, `supports_tool_calling(model_name)`.

### `OpenAICompatibleClient(BaseLLMClient)`

- `_create_response(...)` chooses `max_completion_tokens` for Azure "gpt-5"/"o3"/"o4-mini", otherwise `max_tokens`.
- `temperature` is omitted for "o3", "o4-mini", "gpt-5".
- `parse_messages` dispatches to three helpers: `_msg_tool_call_handler`, `_msg_tool_result_handler`, `_msg_role_handler`. These emit `ChatCompletionFunctionMessageParam` / `ChatCompletionToolMessageParam` / role-typed messages.
- Records trajectory on every chat.

### Concrete Subclasses

| Class | `ProviderConfig` | Notes |
|---|---|---|
| `AzureClient` | `AzureProvider` | Uses `openai.AzureOpenAI(azure_endpoint=base_url, api_version=api_version, api_key=api_key)`. |
| `OpenRouterClient` | `OpenRouterProvider` | Default `base_url` is `https://openrouter.ai/api/v1`. Adds optional `HTTP-Referer` / `X-Title` headers from `OPENROUTER_SITE_URL` / `OPENROUTER_SITE_NAME` env vars. Tool-capable pattern matching. |
| `DoubaoClient` | `DoubaoProvider` | Just a base_url + api_key OpenAI client (Volcengine Ark). |

## 6.9 `GoogleClient` — `google_client.py`

- Uses the `google-genai` SDK (`genai.Client`).
- Builds `types.Tool(function_declarations=[...])` for tool support.
- `GenerateContentConfig` includes `temperature`, `top_p`, `top_k`, `max_output_tokens`, `candidate_count`, `stop_sequences`, and `system_instruction`.
- The system instruction is **separated out** of the message history (Gemini's API doesn't put `system` messages inline with chat content). The internal `message_history` is a list of `types.Content`.
- Parses `candidate.content.parts`, treating `part.text` as the assistant text and `part.function_call` as a tool call (synthesizes a UUID for `call_id`).
- Records trajectory on every chat.

## 6.10 `OllamaClient` — `ollama_client.py`

- Uses the **Ollama Python SDK** directly (`from ollama import chat as ollama_chat`).
- Builds `tools_param` as a list of `{"type": "function", "function": {...}}` (the Ollama format, not OpenAI's strict schema).
- Defaults `base_url` to `http://localhost:11434/v1` if not set, but the actual chat goes through the Ollama SDK, not the OpenAI endpoint.
- `usage` is not populated (Ollama doesn't return it consistently).

## 6.11 Tool-schema Generation Pattern

A pattern shared across all providers:

```python
tool_schemas = [
    FunctionToolParam(
        name=tool.name,
        description=tool.description,
        parameters=tool.get_input_schema(),  # <-- the OpenAI strict branch lives here
        strict=True,                         # only for OpenAI Responses API
        type="function",
    )
    for tool in tools
]
```

`tool.get_input_schema()` is provider-aware because of the `model_provider` stored on each `Tool` instance.

## 6.12 Token-Usage & Tool-Call Wiring

Every provider client, after each successful chat, calls:

```python
if self.trajectory_recorder:
    self.trajectory_recorder.record_llm_interaction(
        messages=messages, response=llm_response,
        provider=<name>, model=model_config.model, tools=tools,
    )
```

This is what makes the trajectory file include every LLM call's input, output, token usage, and tool calls.

## 6.13 Adding a New Provider

1. Subclass `BaseLLMClient` (or `OpenAICompatibleClient` if the vendor speaks OpenAI chat completions).
2. Implement `set_chat_history(messages)` and `chat(messages, model_config, tools, reuse_history)`.
3. Add a new branch in `LLMClient.__init__` with the `LLMProvider` enum entry.
4. Make sure all `ToolCall` and `ToolResult` are converted via the SDK's native types.
5. Add a `retry_with(...)` wrap around the underlying API call.
6. Record trajectory on success.

Continue with [Configuration](./07_configuration.md).
