import axios from 'axios';
import type { ModelInfo } from '../types';

const api = axios.create({ baseURL: '', withCredentials: true });

export async function fetchModels(): Promise<ModelInfo[]> {
  const resp = await api.get('/v1/models');
  return resp.data.data;
}

export async function sendMessage(
  model: string,
  messages: { role: string; content: string }[],
): Promise<{ content: string; tokensIn: number; tokensOut: number; latencyMs: number }> {
  const resp = await api.post('/v1/chat/completions', { model, messages });
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
): Promise<void> {
  const resp = await fetch('/v1/chat/completions', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'include',
    body: JSON.stringify({ model, messages, stream: true }),
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

// Usage
export async function getUsage(range: string) { return (await api.get(`/api/usage?range=${range}`)).data; }
export async function getAllUsage(range: string) { return (await api.get(`/api/usage/all?range=${range}`)).data; }
