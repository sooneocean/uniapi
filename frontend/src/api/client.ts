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
