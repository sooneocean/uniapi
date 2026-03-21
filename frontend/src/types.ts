export interface ContentBlock {
  type: 'text' | 'image' | 'image_url';
  text?: string;
  image_url?: { url: string };
}

export interface ToolCall {
  id: string;
  type: string; // "function"
  function: {
    name: string;
    arguments: string; // JSON string
  };
}

export interface Message {
  id: string;
  role: 'user' | 'assistant' | 'system' | 'tool';
  content: string;
  images?: string[]; // base64 data URLs attached to the message
  toolCalls?: ToolCall[]; // tool calls from the assistant
  toolCallId?: string; // for tool result messages
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
  folder?: string;
  pinned?: boolean;
  share_token?: string;
  createdAt: string;
  updatedAt: string;
}

export interface ModelInfo {
  id: string;
  owned_by: string;
}
