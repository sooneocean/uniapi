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
