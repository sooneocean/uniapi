import axios from 'axios';
import type { ModelInfo } from '../types';

const api = axios.create({ baseURL: '', withCredentials: true });

// Read CSRF token from cookie and set on all mutating requests
function getCSRFToken(): string {
  const match = document.cookie.match(/csrf_token=([^;]+)/);
  return match ? match[1] : '';
}

// Add request interceptor
api.interceptors.request.use((config) => {
  if (config.method && ['post', 'put', 'delete', 'patch'].includes(config.method)) {
    config.headers['X-CSRF-Token'] = getCSRFToken();
  }
  return config;
});

export async function fetchModels(): Promise<ModelInfo[]> {
  const resp = await api.get('/v1/models');
  return resp.data.data;
}

type ApiMessage =
  | { role: string; content: string }
  | { role: string; content: Array<{ type: string; text?: string; image_url?: { url: string } }> };

function buildApiMessages(
  messages: { role: string; content: string }[],
  images?: string[],
): ApiMessage[] {
  if (!images || images.length === 0) return messages;
  // Attach images to the last user message
  const result: ApiMessage[] = messages.map((m, idx) => {
    if (idx === messages.length - 1 && m.role === 'user' && images.length > 0) {
      const parts: Array<{ type: string; text?: string; image_url?: { url: string } }> = [
        { type: 'text', text: m.content },
        ...images.map((img) => ({ type: 'image_url', image_url: { url: img } })),
      ];
      return { role: m.role, content: parts };
    }
    return m;
  });
  return result;
}

export async function sendMessage(
  model: string,
  messages: { role: string; content: string }[],
  images?: string[],
): Promise<{ content: string; tokensIn: number; tokensOut: number; latencyMs: number }> {
  const apiMessages = buildApiMessages(messages, images);
  const resp = await api.post('/v1/chat/completions', { model, messages: apiMessages });
  const choice = resp.data.choices[0];
  return {
    content: choice.message.content,
    tokensIn: resp.data.usage.prompt_tokens,
    tokensOut: resp.data.usage.completion_tokens,
    latencyMs: resp.data.x_uniapi?.latency_ms ?? 0,
  };
}

export async function sendMessageStream(
  model: string,
  messages: { role: string; content: string }[],
  onChunk: (text: string) => void,
  onDone: (usage: { tokensIn: number; tokensOut: number; latencyMs: number }) => void,
  images?: string[],
): Promise<void> {
  const apiMessages = buildApiMessages(messages, images);
  const resp = await fetch('/v1/chat/completions', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'X-CSRF-Token': getCSRFToken(),
    },
    credentials: 'include',
    body: JSON.stringify({ model, messages: apiMessages, stream: true }),
  });

  const reader = resp.body!.getReader();
  const decoder = new TextDecoder();
  let buffer = '';

  while (true) {
    const { done, value } = await reader.read();
    if (done) break;
    buffer += decoder.decode(value, { stream: true });
    const lines = buffer.split('\n');
    buffer = lines.pop()!;
    for (const line of lines) {
      if (!line.startsWith('data: ')) continue;
      const data = line.slice(6);
      if (data === '[DONE]') {
        onDone({ tokensIn: 0, tokensOut: 0, latencyMs: 0 });
        return;
      }
      try {
        const chunk = JSON.parse(data);
        const content = chunk.choices?.[0]?.delta?.content;
        if (content) onChunk(content);
      } catch {}
    }
  }
}

export async function getStatus(): Promise<{ needs_setup: boolean; authenticated: boolean }> {
  const resp = await api.get('/api/status');
  return resp.data;
}

export async function setup(username: string, password: string): Promise<void> {
  await api.post('/api/setup', { username, password });
}

export async function login(username: string, password: string): Promise<any> {
  const resp = await api.post('/api/login', { username, password });
  return resp.data;
}

export async function logout(): Promise<void> {
  await api.post('/api/logout');
}

export async function getMe(): Promise<{ id: string; username: string; role: string }> {
  const resp = await api.get('/api/me');
  return resp.data;
}

// Providers
export async function getProviders() { return (await api.get('/api/providers')).data; }
export async function addProvider(data: any) { return (await api.post('/api/providers', data)).data; }
export async function deleteProvider(id: string) { await api.delete(`/api/providers/${id}`); }
export async function getProviderTemplates() { return (await api.get('/api/provider-templates')).data; }

// Users
export async function getUsers() { return (await api.get('/api/users')).data; }
export async function addUser(data: any) { return (await api.post('/api/users', data)).data; }
export async function deleteUser(id: string) { await api.delete(`/api/users/${id}`); }

// API Keys
export async function getAPIKeys() { return (await api.get('/api/api-keys')).data; }
export async function createAPIKey(label: string) { return (await api.post('/api/api-keys', { label })).data; }
export async function deleteAPIKey(id: string) { await api.delete(`/api/api-keys/${id}`); }

// Conversations
export async function getConversations() { return (await api.get('/api/conversations')).data; }
export async function createConversation(title: string) { return (await api.post('/api/conversations', { title })).data; }
export async function getConversation(id: string) { return (await api.get(`/api/conversations/${id}`)).data; }
export async function deleteConversation(id: string) { await api.delete(`/api/conversations/${id}`); }
export async function saveMessage(
  conversationId: string,
  message: {
    role: string;
    content: string;
    model?: string;
    tokens_in?: number;
    tokens_out?: number;
    cost?: number;
    latency_ms?: number;
  }
) {
  return (await api.post(`/api/conversations/${conversationId}/messages`, message)).data;
}

export async function deleteMessageAndAfter(conversationId: string, messageId: string) {
  await api.delete(`/api/conversations/${conversationId}/messages/${messageId}`);
}

export async function exportConversation(id: string, format: 'markdown' | 'json') {
  const resp = await api.get(`/api/conversations/${id}/export?format=${format}`, {
    responseType: format === 'markdown' ? 'blob' : 'json',
  });
  if (format === 'markdown') {
    const url = URL.createObjectURL(resp.data);
    const a = document.createElement('a');
    a.href = url;
    a.download = `conversation.md`;
    a.click();
    URL.revokeObjectURL(url);
  }
  return resp.data;
}

// System prompts
export async function getSystemPrompts() { return (await api.get('/api/system-prompts')).data; }
export async function createSystemPrompt(data: { name: string; content: string; is_default?: boolean }) {
  return (await api.post('/api/system-prompts', data)).data;
}
export async function updateSystemPrompt(id: string, data: { name: string; content: string; is_default?: boolean }) {
  return (await api.put(`/api/system-prompts/${id}`, data)).data;
}
export async function deleteSystemPrompt(id: string) { await api.delete(`/api/system-prompts/${id}`); }

// Usage
export async function getUsage(range: string) { return (await api.get(`/api/usage?range=${range}`)).data; }
export async function getAllUsage(range: string) { return (await api.get(`/api/usage/all?range=${range}`)).data; }

// Audit log
export async function getAuditLog(limit = 50, offset = 0) {
  return (await api.get(`/api/audit-log?limit=${limit}&offset=${offset}`)).data;
}

// Auto-title
export async function autoTitle(conversationId: string): Promise<string> {
  const resp = await api.post(`/api/conversations/${conversationId}/auto-title`);
  return resp.data.title;
}

// Admin dashboard
export async function getDashboard() {
  return (await api.get('/api/dashboard')).data;
}

// Database backup
export async function downloadBackup() {
  const resp = await api.get('/api/backup', { responseType: 'blob' });
  const url = URL.createObjectURL(resp.data);
  const a = document.createElement('a');
  a.href = url;
  a.download = `uniapi-backup-${new Date().toISOString().slice(0, 10)}.db`;
  a.click();
  URL.revokeObjectURL(url);
}

// OAuth / Binding
export async function getOAuthProviders() {
  return (await api.get('/api/oauth/providers')).data;
}

export async function bindSessionToken(provider: string, token: string, shared: boolean) {
  return (await api.post(`/api/oauth/bind/${provider}/session-token`, { token, shared })).data;
}

export async function getOAuthAccounts() {
  return (await api.get('/api/oauth/accounts')).data;
}

export async function unbindAccount(id: string) {
  await api.delete(`/api/oauth/accounts/${id}`);
}

export async function reauthAccount(id: string) {
  return (await api.post(`/api/oauth/accounts/${id}/reauth`)).data;
}
