# UniAPI

![CI](https://github.com/sooneocean/uniapi/actions/workflows/ci.yml/badge.svg)

A zero-dependency, single-binary AI aggregation platform. Share AI subscriptions with your team through a unified chat interface.

## Features

- **Single Binary** — Download and run. No PostgreSQL, no Redis, no Docker required.
- **Chat UI** — ChatGPT-style interface with model switching, streaming, markdown rendering, code highlighting
- **Multi-Provider** — OpenAI, Claude, Gemini, DeepSeek, Mistral, Groq, Ollama, Together AI, and any OpenAI-compatible service
- **Tool Calling** — Full function calling support; pass tools to any model that supports them
- **RAG Knowledge Base** — Upload documents and auto-inject relevant context into prompts
- **Prompt Workflows** — Chain multi-step model calls with `{{input}}` / `{{step_N}}` placeholders
- **Chat Rooms** — Multi-user rooms with `@ai` trigger and real-time SSE broadcast
- **Plugin System** — Register HTTP endpoints as model tools, shared or private
- **Model Comparison** — Side-by-side responses from two models simultaneously
- **Voice Input** — Web Speech API dictation
- **File Attachments** — Drag-and-drop code and text files into messages
- **API Playground** — Interactive in-browser API testing
- **Webhooks** — POST notifications for provider errors, logins, account binding, quota warnings
- **Model Aliases** — Map friendly names (e.g. `fast`) to real model IDs
- **Response Caching** — Identical requests return cached responses instantly
- **LaTeX / Mermaid** — KaTeX math rendering and Mermaid diagram rendering in chat
- **Conversation Sharing** — Share conversations via public read-only links
- **Streaming Speed** — Real-time tok/s display during generation
- **Custom Themes** — Dark/light theme toggle with persistent preference
- **Data Import/Export** — Export conversations as Markdown or JSON; CSV usage export
- **User Quotas** — Per-user daily/monthly token and cost limits with warning thresholds
- **Prompt Templates** — Save and reuse system prompt presets
- **Auto-Title** — Conversations are automatically titled from the first message
- **Admin Dashboard** — System stats, user management, audit log
- **DB Backup** — One-click database backup download from the admin panel
- **Cost Splitting** — Token-level usage tracking with per-user breakdown
- **OAuth & Session Binding** — Connect AI accounts via OAuth or session tokens
- **Provider Templates** — One-click setup for 8+ popular AI providers
- **OpenAI-Compatible API** — Drop-in replacement for the OpenAI API. Works with Cursor, Continue, ChatBox, etc.
- **Team Management** — Admin/member roles, shared and private accounts
- **Secure** — AES-256-GCM encryption, JWT auth, bcrypt passwords, CSRF protection, rate limiting
- **Observable** — Prometheus metrics, structured JSON logging, audit trail
- **Mobile Ready** — Responsive design, PWA-installable

## Quick Start

### Download & Run

```bash
# Download the latest release
curl -L https://github.com/sooneocean/uniapi/releases/latest/download/uniapi-$(uname -s | tr '[:upper:]' '[:lower:]')-$(uname -m) -o uniapi
chmod +x uniapi
./uniapi
```

Open http://localhost:9000 — the setup wizard will guide you through creating the first admin account.

### Docker

```bash
docker run -p 9000:9000 -v ~/.uniapi:/data uniapi/uniapi
```

### Build from Source

```bash
git clone https://github.com/sooneocean/uniapi.git
cd uniapi
make build
./bin/uniapi
```

Requires: Go 1.22+, Node.js 20+

## Configuration

UniAPI works out of the box with no configuration. Optionally create `~/.uniapi/config.yaml`:

```yaml
server:
  port: 9000
  host: "0.0.0.0"

providers:
  - name: openai
    type: openai
    accounts:
      - label: "Team Account"
        api_key: "sk-..."
        models: ["gpt-4o", "gpt-4o-mini"]

  - name: claude
    type: anthropic
    accounts:
      - label: "Claude Account"
        api_key: "sk-ant-..."
        models: ["claude-sonnet-4-20250514"]

  - name: deepseek
    type: openai_compatible
    base_url: "https://api.deepseek.com"
    accounts:
      - label: "DeepSeek"
        api_key: "sk-..."
        models: ["deepseek-chat"]

routing:
  strategy: round_robin  # round_robin | least_used | sticky
  max_retries: 3
  failover_attempts: 2

storage:
  retention_days: 90  # 0 = keep forever

oauth:
  base_url: "https://your-domain.com"  # required for OAuth redirect URLs
  openai:
    client_id: "..."
    client_secret: "..."
  anthropic:
    client_id: "..."
    client_secret: "..."

webhooks:
  - url: "https://hooks.example.com/uniapi"
    events: ["provider_error", "quota_warning", "user_login", "account_bound"]

response_cache:
  enabled: true
  ttl: 5m
```

Alternatively, configure everything through the web UI at Settings > Providers.

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `UNIAPI_PORT` | `9000` | Server port |
| `UNIAPI_DATA_DIR` | `~/.uniapi` | Data directory |
| `UNIAPI_SECRET` | auto-generated | Encryption secret (persist across restarts) |
| `UNIAPI_LOG_LEVEL` | `info` | Log level: `debug`, `info`, `warn`, `error` |

### CLI Flags

```
./uniapi --port 8080 --data-dir /var/lib/uniapi --config /etc/uniapi/config.yaml
```

## API Usage

UniAPI exposes an OpenAI-compatible API. Authenticate with a `uniapi-sk-` key generated under Settings > API Keys, or with your session JWT.

```bash
curl http://localhost:9000/v1/chat/completions \
  -H "Authorization: Bearer uniapi-sk-your-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o",
    "messages": [{"role": "user", "content": "Hello!"}],
    "stream": true
  }'
```

### Endpoints

| Method | Path | Description |
|--------|------|-------------|
| POST | `/v1/chat/completions` | Chat completion (OpenAI format) |
| GET | `/v1/models` | List available models |
| GET | `/health` | Health check |
| GET | `/metrics` | Prometheus metrics (admin only) |

See [docs/API.md](docs/API.md) for the full API reference.

### Use with Third-Party Tools

Set the base URL to your UniAPI instance:

- **Cursor**: Settings > Models > OpenAI API Base: `http://localhost:9000/v1`
- **Continue**: `~/.continue/config.json` → `"apiBase": "http://localhost:9000/v1"`
- **ChatBox**: Settings > API Host: `http://localhost:9000`

## Supported Providers

| Provider | Type | Models |
|----------|------|--------|
| OpenAI | `openai` | gpt-4o, gpt-4o-mini, gpt-4-turbo, o3-mini |
| Anthropic | `anthropic` | claude-sonnet-4, claude-haiku-4, claude-opus-4 |
| Google Gemini | `gemini` | gemini-2.5-pro, gemini-2.5-flash |
| DeepSeek | `openai_compatible` | deepseek-chat, deepseek-reasoner |
| Mistral | `openai_compatible` | mistral-large, mistral-small, codestral |
| Groq | `openai_compatible` | llama-3.3-70b, llama-3.1-8b, mixtral-8x7b |
| Ollama | `openai_compatible` | llama3, codellama, mistral (local) |
| Together AI | `openai_compatible` | Meta-Llama-3.1-405B, Mixtral-8x22B |
| Any OpenAI-compatible | `openai_compatible` | custom `base_url` |

## Architecture

```
Single Go Binary (~20MB)
├── React Chat UI (embedded via go:embed)
│   ├── Streaming chat with markdown, LaTeX, Mermaid, code highlighting
│   ├── Model comparison, voice input, file attachments, API playground
│   ├── Conversation folders, pinning, sharing, search, export
│   └── Admin dashboard, usage charts, DB backup
├── OpenAI-Compatible API (/v1/*)
│   ├── Chat completions (streaming + non-streaming)
│   ├── Tool calling, response caching, model aliases
│   └── RAG context injection, plugin tool injection
├── Provider Adapters (OpenAI, Anthropic, Gemini, + compatible)
│   └── OAuth & session token credential binding
├── Router (round-robin / least-used, failover, per-user account binding)
├── Feature Modules
│   ├── RAG — document ingestion, keyword-based chunk retrieval
│   ├── Workflows — multi-step chained prompt execution
│   ├── Plugins — HTTP endpoint tools registered per user
│   ├── Chat Rooms — multi-user rooms with SSE broadcast
│   ├── Webhooks — async event delivery
│   ├── Quota Engine — daily/monthly token and cost limits
│   └── Memory Manager — conversation summarisation / compaction
├── SQLite (users, conversations, usage, audit log, knowledge base)
└── In-Memory Cache (rate limits, alias resolution, session state)
```

Data is stored in `~/.uniapi/` by default:
- `data.db` — SQLite database
- `secret` — auto-generated encryption key

## Keyboard Shortcuts

| Shortcut | Action |
|----------|--------|
| `Ctrl+K` | Focus model search / command palette |
| `Ctrl+N` | New conversation |
| `Ctrl+Enter` | Send message |
| `Ctrl+Shift+C` | Copy last response |
| `Ctrl+/` | Toggle sidebar |
| `Ctrl+,` | Open settings |
| `Esc` | Cancel generation / close modal |

## Deployment

### Reverse Proxy (Caddy)

```
uniapi.example.com {
    reverse_proxy localhost:9000
}
```

### Systemd Service

```ini
[Unit]
Description=UniAPI
After=network.target

[Service]
ExecStart=/usr/local/bin/uniapi
Environment=UNIAPI_DATA_DIR=/var/lib/uniapi
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

```bash
sudo cp uniapi /usr/local/bin/
sudo systemctl enable --now uniapi
```

## Development

```bash
# Run all tests
go test ./... -v -race

# Frontend dev server (hot reload on :5173)
cd frontend && npm run dev

# Production frontend build
cd frontend && npm run build

# Full binary build (frontend + Go)
make build

# Linux cross-compile
make build-linux

# Docker image
make docker
```

## License

MIT
