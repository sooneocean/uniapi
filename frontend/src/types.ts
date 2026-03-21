export interface ContentBlock {
  type: 'text' | 'image' | 'image_url';
  text?: string;
  image_url?: { url: string };
}

export interface Message {
  id: string;
  role: 'user' | 'assistant' | 'system';
  content: string;
  images?: string[]; // base64 data URLs attached to the message
  model?: string;
  tokensIn?: number;
  tokensOut?: number;
  cost?: number;
  latencyMs?: number;
  createdAt: string;
}

export interface Conversation {
  id: string;
  title: string;
  preview?: string;
  createdAt: string;
  updatedAt: string;
}

export interface ModelInfo {
  id: string;
  owned_by: string;
}
