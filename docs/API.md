# UniAPI — API Reference

Base URL: `http://localhost:9000` (or your deployed instance)

## Authentication

UniAPI supports two authentication methods:

**Session JWT** — obtained from `POST /api/login`. Sent as a cookie (`token`) or `Authorization: Bearer <jwt>` header. Valid for 7 days.

**API Key** — generated via `POST /api/api-keys`. Has the prefix `uniapi-sk-`. Sent as `Authorization: Bearer uniapi-sk-<key>`. Used for programmatic/OpenAI-compatible access.

Rate limiting applies to auth endpoints: 10 requests per minute per IP.

---

## Auth Endpoints

### GET /api/status

Check server state. No authentication required.

**Response**
```json
{
  "needs_setup": false,
  "authenticated": true
}
```

- `needs_setup: true` means no admin account exists yet. The UI will redirect to the setup wizard.
- `authenticated` reflects whether the current request carries a valid JWT.

---

### POST /api/setup

Create the first admin account. Only available when `needs_setup` is `true`. Rate-limited.

**Request**
```json
{
  "username": "admin",
  "password": "changeme"
}
```

**Response** `200 OK`
```json
{ "ok": true }
```

**Errors**
- `400` — setup already completed, or missing fields

---

### POST /api/login

Authenticate and receive a session token. Rate-limited to 10 attempts per minute per IP.

**Request**
```json
{
  "username": "alice",
  "password": "secret"
}
```

**Response** `200 OK`

Sets an `HttpOnly` cookie named `token`. Also returns:
```json
{
  "ok": true,
  "user": {
    "id": "uuid",
    "username": "alice",
    "role": "admin"
  }
}
```

**Errors**
- `401` — invalid credentials

---

### POST /api/logout

Clear the session cookie. No request body required. Auth not required.

**Response** `200 OK`
```json
{ "ok": true }
```

---

### GET /api/me

Return the currently authenticated user's profile. Requires auth.

**Response** `200 OK`
```json
{
  "id": "uuid",
  "username": "alice",
  "role": "admin"
}
```

**Errors**
- `401` — not authenticated

---

## OpenAI-Compatible API

The `/v1/*` endpoints are a drop-in replacement for the OpenAI API. Authenticate with a `uniapi-sk-` key or a session JWT.

### POST /v1/chat/completions

Send a chat message and receive a completion. Supports streaming.

**Request**
```json
{
  "model": "gpt-4o",
  "messages": [
    {"role": "system", "content": "You are a helpful assistant."},
    {"role": "user", "content": "What is 2+2?"}
  ],
  "max_tokens": 1024,
  "temperature": 0.7,
  "stream": false,
  "provider": ""
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `model` | string | yes | Model ID (e.g. `gpt-4o`, `claude-sonnet-4-20250514`) |
| `messages` | array | yes | Conversation history |
| `max_tokens` | int | no | Maximum tokens to generate |
| `temperature` | float | no | Sampling temperature (0–2) |
| `stream` | bool | no | Enable SSE streaming (default: `false`) |
| `provider` | string | no | Force a specific provider name |

**Response (non-streaming)** `200 OK`
```json
{
  "id": "chatcmpl-a1b2c3d4",
  "object": "chat.completion",
  "created": 1700000000,
  "model": "gpt-4o",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "4"
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 22,
    "completion_tokens": 1,
    "total_tokens": 23
  },
  "x_uniapi": {
    "latency_ms": 312
  }
}
```

**Response (streaming)** — Server-Sent Events

Each event is a JSON chunk with `"object": "chat.completion.chunk"`. The stream ends with `data: [DONE]`.

```
data: {"id":"chatcmpl-a1b2c3d4","object":"chat.completion.chunk","created":1700000000,"model":"gpt-4o","choices":[{"index":0,"delta":{"content":"4"},"finish_reason":null}]}

data: [DONE]
```

**Errors**
- `400` — invalid request body
- `401` — missing or invalid authentication
- `502` — upstream provider error

---

### GET /v1/models

List all models available across configured providers.

**Response** `200 OK`
```json
{
  "object": "list",
  "data": [
    {
      "id": "gpt-4o",
      "object": "model",
      "created": 1700000000,
      "owned_by": "openai"
    },
    {
      "id": "claude-sonnet-4-20250514",
      "object": "model",
      "created": 1700000000,
      "owned_by": "anthropic"
    }
  ]
}
```

---

## Provider Management

All provider endpoints require **admin** role.

### GET /api/providers

List all configured provider accounts (API key and OAuth/session accounts added via UI).

**Response** `200 OK`
```json
[
  {
    "id": "uuid",
    "provider": "openai",
    "label": "Team Account",
    "models": ["gpt-4o", "gpt-4o-mini"],
    "max_concurrent": 5,
    "enabled": true,
    "config_managed": false,
    "created_at": "2024-01-01T00:00:00Z"
  }
]
```

`config_managed: true` means the account was loaded from `config.yaml` and cannot be deleted via API.

---

### POST /api/providers

Add a new provider account.

**Request**
```json
{
  "provider": "openai",
  "label": "My Account",
  "api_key": "sk-...",
  "models": ["gpt-4o", "gpt-4o-mini"],
  "max_concurrent": 5
}
```

| Field | Required | Description |
|-------|----------|-------------|
| `provider` | yes | Provider type: `openai`, `anthropic`, `gemini`, `openai_compatible` |
| `label` | yes | Human-readable name |
| `api_key` | yes | Provider API key |
| `models` | no | List of model IDs to expose (defaults to none) |
| `max_concurrent` | no | Max parallel requests (default: 5) |

**Response** `201 Created` — same shape as a single item from `GET /api/providers`.

---

### DELETE /api/providers/:id

Remove a provider account. Cannot delete config-managed accounts.

**Response** `200 OK`
```json
{ "ok": true }
```

**Errors**
- `400` — cannot delete config-managed provider
- `404` — provider not found

---

### GET /api/provider-templates

List built-in provider templates for one-click setup. No auth required beyond being logged in.

**Response** `200 OK`
```json
[
  {
    "name": "openai",
    "display_name": "OpenAI",
    "type": "openai",
    "base_url": "https://api.openai.com",
    "default_models": ["gpt-4o", "gpt-4o-mini", "gpt-4-turbo", "o3-mini"],
    "description": "GPT-4o, o3, and more",
    "api_key_url": "https://platform.openai.com/api-keys"
  }
]
```

Available templates: `openai`, `anthropic`, `gemini`, `deepseek`, `mistral`, `groq`, `ollama`, `together`.

---

## User Management

All user management endpoints require **admin** role.

### GET /api/users

List all users.

**Response** `200 OK`
```json
[
  {
    "id": "uuid",
    "username": "alice",
    "role": "admin",
    "created_at": "2024-01-01T00:00:00Z"
  }
]
```

---

### POST /api/users

Create a new user.

**Request**
```json
{
  "username": "bob",
  "password": "s3cret",
  "role": "member"
}
```

`role` is optional; defaults to `"member"`. Valid values: `"admin"`, `"member"`.

**Response** `201 Created`
```json
{
  "id": "uuid",
  "username": "bob",
  "role": "member",
  "created_at": "2024-01-01T00:00:00Z"
}
```

---

### DELETE /api/users/:id

Delete a user.

**Response** `200 OK`
```json
{ "ok": true }
```

---

## API Key Management

Users manage their own API keys. Admin role not required.

### GET /api/api-keys

List the authenticated user's API keys. Keys are shown without the secret value.

**Response** `200 OK`
```json
[
  {
    "id": "uuid",
    "label": "Cursor",
    "created_at": "2024-01-01T00:00:00Z",
    "expires_at": null
  }
]
```

---

### POST /api/api-keys

Generate a new API key. The raw key is only returned once — store it securely.

**Request**
```json
{ "label": "Cursor" }
```

**Response** `201 Created`
```json
{
  "id": "uuid",
  "key": "uniapi-sk-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
}
```

---

### DELETE /api/api-keys/:id

Revoke an API key. Users can only delete their own keys.

**Response** `200 OK`
```json
{ "ok": true }
```

---

## Conversations

Conversation history is per-user. Users can only access their own conversations.

### GET /api/conversations

List all conversations for the authenticated user.

**Response** `200 OK`
```json
[
  {
    "id": "uuid",
    "user_id": "uuid",
    "title": "My Chat",
    "created_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-01-01T00:00:00Z"
  }
]
```

---

### POST /api/conversations

Create a new conversation.

**Request**
```json
{ "title": "New Chat" }
```

**Response** `201 Created` — conversation object.

---

### GET /api/conversations/:id

Get a conversation with all its messages.

**Response** `200 OK`
```json
{
  "id": "uuid",
  "user_id": "uuid",
  "title": "My Chat",
  "created_at": "2024-01-01T00:00:00Z",
  "updated_at": "2024-01-01T00:00:00Z",
  "messages": [
    {
      "id": "uuid",
      "conversation_id": "uuid",
      "role": "user",
      "content": "Hello",
      "model": "",
      "tokens_in": 0,
      "tokens_out": 0,
      "cost": 0,
      "latency_ms": 0,
      "created_at": "2024-01-01T00:00:00Z"
    }
  ]
}
```

**Errors**
- `403` — conversation belongs to another user
- `404` — not found

---

### PUT /api/conversations/:id

Rename a conversation.

**Request**
```json
{ "title": "Renamed Chat" }
```

**Response** `200 OK`
```json
{ "ok": true }
```

---

### DELETE /api/conversations/:id

Delete a conversation and all its messages.

**Response** `200 OK`
```json
{ "ok": true }
```

---

### POST /api/conversations/:id/messages

Append a message to a conversation (used by the chat UI to persist history).

**Request**
```json
{
  "role": "assistant",
  "content": "Hello! How can I help?",
  "model": "gpt-4o",
  "tokens_in": 10,
  "tokens_out": 8,
  "cost": 0.000054,
  "latency_ms": 412
}
```

**Response** `200 OK`
```json
{ "ok": true, "id": "uuid" }
```

---

## OAuth & Account Binding

OAuth allows users to connect personal AI accounts (e.g. Claude.ai, ChatGPT). The connected credentials are encrypted at rest and automatically refreshed.

### GET /api/oauth/providers

List OAuth providers that have been configured (client ID/secret set).

**Response** `200 OK`
```json
[
  {
    "name": "openai",
    "display_name": "ChatGPT / OpenAI",
    "auth_type": "oauth"
  }
]
```

---

### GET /api/oauth/bind/:provider/authorize

Start the OAuth flow for the given provider. Redirects to the provider's consent page.

**Query Parameters**

| Parameter | Description |
|-----------|-------------|
| `shared` | `true` to bind as a shared team account (admin only) |

**Behavior** — Redirects (`302`) to the OAuth authorization URL. After the user approves, the provider redirects back to `/api/oauth/callback/:provider`. The callback page posts a message to the opener window and closes itself.

---

### POST /api/oauth/bind/:provider/session-token

Bind a provider using a session/cookie token (for providers that don't support OAuth).

**Request**
```json
{
  "token": "<session-token-from-browser>",
  "shared": false
}
```

`shared: true` binds the account for all team members (admin only).

**Response** `200 OK`
```json
{
  "ok": true,
  "account": {
    "id": "uuid",
    "provider": "anthropic",
    "label": "My Claude"
  }
}
```

---

### GET /api/oauth/accounts

List all OAuth/session accounts visible to the current user (own accounts + shared accounts).

**Response** `200 OK`
```json
[
  {
    "id": "uuid",
    "provider": "openai",
    "label": "My ChatGPT",
    "auth_type": "oauth",
    "models": ["gpt-4o"],
    "owner_user_id": "uuid",
    "needs_reauth": false,
    "enabled": true,
    "token_expires_at": "2025-01-01T00:00:00Z"
  }
]
```

---

### DELETE /api/oauth/accounts/:id

Unbind and remove an OAuth/session account. Users can remove their own accounts; admins can remove any.

**Response** `200 OK`
```json
{ "ok": true }
```

---

### POST /api/oauth/accounts/:id/reauth

Re-authorize an account that has expired or been revoked.

**Response** `200 OK`

For OAuth accounts:
```json
{
  "action": "oauth",
  "authorize_url": "https://..."
}
```

For session-token accounts:
```json
{
  "action": "session_token",
  "provider": "anthropic"
}
```

---

## Usage

### GET /api/usage

Get token usage and cost breakdown for the authenticated user.

**Query Parameters**

| Parameter | Values | Description |
|-----------|--------|-------------|
| `range` | `daily` (default), `weekly`, `monthly` | Time window |

**Response** `200 OK`
```json
[
  {
    "Provider": "openai",
    "Model": "gpt-4o",
    "Date": "2024-01-15",
    "TokensIn": 1500,
    "TokensOut": 320,
    "Cost": 0.0218,
    "RequestCount": 5
  }
]
```

---

### GET /api/usage/all

Get usage summary for all users. Requires **admin** role.

**Query Parameters** — same `range` parameter as above.

**Response** `200 OK`
```json
[
  {
    "Username": "alice",
    "UserID": "uuid",
    "TokensIn": 45000,
    "TokensOut": 12000,
    "Cost": 1.87,
    "RequestCount": 42
  }
]
```

Results are ordered by cost descending.

---

### GET /api/audit-log

Retrieve the audit log. Requires **admin** role.

**Query Parameters**

| Parameter | Default | Description |
|-----------|---------|-------------|
| `limit` | `50` | Number of entries to return |
| `offset` | `0` | Pagination offset |

**Response** `200 OK`
```json
{
  "entries": [
    {
      "id": "uuid",
      "user_id": "uuid",
      "username": "alice",
      "action": "login",
      "resource_type": "user",
      "resource_id": "uuid",
      "detail": "",
      "ip": "127.0.0.1",
      "created_at": "2024-01-15T10:00:00Z"
    }
  ],
  "total": 128
}
```

Logged actions include: `setup`, `login`, `create_provider`, `delete_provider`, `create_user`, `delete_user`, `create_api_key`, `bind_account`, `unbind_account`.

---

## Monitoring

### GET /health

Health check endpoint. No authentication required.

**Response** `200 OK`
```json
{
  "status": "ok",
  "db": "connected"
}
```

**Response** `503 Service Unavailable` (database unreachable)
```json
{
  "status": "unhealthy",
  "db": "disconnected",
  "error": "..."
}
```

---

### GET /metrics

Prometheus metrics in text exposition format. No authentication required.

**Response** — `text/plain; version=0.0.4` (Prometheus format)

Key metrics:

| Metric | Type | Description |
|--------|------|-------------|
| `uniapi_requests_total` | Counter | HTTP requests by method, path, status |
| `uniapi_request_duration_seconds` | Histogram | HTTP request latency |
| `uniapi_active_connections` | Gauge | Current active connections |
| `uniapi_provider_requests_total` | Counter | Provider requests by model and status |
| `uniapi_provider_latency_seconds` | Histogram | Provider response latency by model |
| `uniapi_tokens_processed_total` | Counter | Tokens processed by direction (input/output) and model |

---

## Error Format

Errors from `/api/*` endpoints:
```json
{ "error": "human-readable message" }
```

Errors from `/v1/*` endpoints (OpenAI-compatible format):
```json
{
  "error": {
    "type": "authentication_error",
    "message": "invalid API key"
  }
}
```

Error types: `invalid_request_error`, `authentication_error`, `api_error`, `rate_limit_error`.

## Request Tracing

Every response includes an `X-Request-ID` header for log correlation. Pass your own `X-Request-ID` request header to propagate an existing trace ID.
