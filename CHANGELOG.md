# Changelog

All notable changes to UniAPI will be documented in this file.

## [Unreleased]

### Added
- Voice input via Web Speech API
- File attachments (drag & drop code/text files)
- API Playground for interactive testing
- Webhook notifications for provider errors, logins, account binding
- Custom model aliases (e.g., "fast" → gpt-4o-mini)
- Response caching for identical requests
- LaTeX math rendering (KaTeX)
- Mermaid diagram rendering
- Conversation folders and pinning
- Conversation sharing via public links
- Streaming speed display (tok/s)
- Model comparison mode (side-by-side)
- PWA support (installable as app)
- Auto-generated conversation titles
- Admin dashboard with system stats
- Database backup download
- i18n support (English + Traditional Chinese)
- Image paste for vision models
- Conversation search
- Code syntax highlighting with copy button
- Dark/light theme toggle
- Mobile responsive design
- Keyboard shortcuts (Ctrl+K, Ctrl+N, etc.)
- Message edit and regenerate
- Conversation export (Markdown, JSON)
- System prompt presets
- Sidebar conversation previews
- Token estimation before sending

### Security
- CSRF double-submit cookie protection
- Configurable CORS origin allowlist
- Metrics endpoint requires admin auth
- Health endpoint hides error details
- Fixed rate limiter sliding window bug
- Safe type assertions throughout
- Password minimum 8 characters
- API rate limiting (60 req/min)
- Separate JWT and encryption key derivation

### Performance
- API key lookup cached (10min TTL)
- Credential caching in provider closures (5min refresh)
- Model alias resolution cached (5min TTL)
- Streaming uses reusable JSON encoder
- Router uses pre-built model→account index
- Cache sweeper lock optimization (read-first, batch delete)
- Usage recording batched (500ms/20 records)
- 3 new composite DB indexes
- Connection pool tuned (20 conns, 3s timeout)

### Infrastructure
- Prometheus metrics (/metrics, admin-only)
- Structured JSON logging with request ID tracing
- Audit logging for admin operations
- Graceful shutdown (SIGINT/SIGTERM)
- Version-tracked database migrations
- Multi-stage Dockerfile
- Cross-compilation (linux amd64/arm64, darwin amd64/arm64)

## [0.1.0] - 2026-03-20

### Added
- Initial release
- Go backend with Gin web framework
- SQLite database with WAL mode
- In-memory cache with TTL
- Provider adapters: OpenAI, Anthropic (Claude), Gemini
- OpenAI-compatible API (/v1/chat/completions)
- Provider templates: DeepSeek, Mistral, Groq, Ollama, Together AI
- OAuth/session token account binding framework
- Load balancing (round-robin, least-used) with failover
- SSE streaming support
- React chat UI embedded in binary
- JWT authentication with bcrypt passwords
- API key management
- Per-user usage tracking with cost splitting
- Settings UI (providers, users, API keys)
- Setup wizard for first-run
- AES-256-GCM credential encryption
