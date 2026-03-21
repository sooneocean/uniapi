import { useState, useRef, useEffect, type KeyboardEvent } from 'react';
import { v4 as uuidv4 } from 'uuid';
import type { Message } from '../types';
import {
  sendMessageStream,
  getConversation,
  saveMessage,
  deleteMessageAndAfter,
  exportConversation,
} from '../api/client';
import ModelSelector from './ModelSelector';
import MessageBubble from './MessageBubble';
import StatusBar from './StatusBar';
import SystemPromptSelector from './SystemPromptSelector';

interface Props {
  conversationId: string | null;
  onConversationTitleUpdate?: (id: string, title: string) => void;
}

export default function ChatArea({ conversationId, onConversationTitleUpdate }: Props) {
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState('');
  const [selectedModel, setSelectedModel] = useState('');
  const [loading, setLoading] = useState(false);
  const [lastStats, setLastStats] = useState({ tokensIn: 0, tokensOut: 0, latencyMs: 0 });
  const [systemPrompt, setSystemPrompt] = useState('');
  const bottomRef = useRef<HTMLDivElement>(null);

  // Load messages when conversation changes
  useEffect(() => {
    if (!conversationId) {
      setMessages([]);
      return;
    }
    getConversation(conversationId)
      .then((conv) => {
        const msgs: Message[] = (conv.messages ?? []).map((m: any) => ({
          id: m.id,
          role: m.role as Message['role'],
          content: m.content,
          model: m.model || undefined,
          tokensIn: m.tokens_in || undefined,
          tokensOut: m.tokens_out || undefined,
          cost: m.cost || undefined,
          latencyMs: m.latency_ms || undefined,
          createdAt: m.created_at,
        }));
        setMessages(msgs);
        if (onConversationTitleUpdate && conv.title) {
          onConversationTitleUpdate(conversationId, conv.title);
        }
      })
      .catch(() => {
        setMessages([]);
      });
  }, [conversationId]);

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages, loading]);

  const doSend = async (text: string, history: Message[]) => {
    if (!text || loading || !selectedModel) return;

    const userMsg: Message = {
      id: uuidv4(),
      role: 'user',
      content: text,
      createdAt: new Date().toISOString(),
    };

    const nextMessages = [...history, userMsg];
    setMessages(nextMessages);
    setInput('');
    setLoading(true);

    if (conversationId) {
      saveMessage(conversationId, { role: 'user', content: text }).catch(() => {});
    }

    const assistantId = uuidv4();
    const assistantMsg: Message = {
      id: assistantId,
      role: 'assistant',
      content: '',
      model: selectedModel,
      createdAt: new Date().toISOString(),
    };
    setMessages((prev) => [...prev, assistantMsg]);

    let finalContent = '';
    let finalUsage = { tokensIn: 0, tokensOut: 0, latencyMs: 0 };

    try {
      // Build API messages, prepend system prompt if set
      const apiMessages: { role: string; content: string }[] = [];
      if (systemPrompt.trim()) {
        apiMessages.push({ role: 'system', content: systemPrompt.trim() });
      }
      nextMessages.forEach((m) => apiMessages.push({ role: m.role, content: m.content }));

      await sendMessageStream(
        selectedModel,
        apiMessages,
        (chunk) => {
          finalContent += chunk;
          setMessages((prev) =>
            prev.map((m) => m.id === assistantId ? { ...m, content: m.content + chunk } : m)
          );
        },
        (usage) => {
          finalUsage = usage;
          setMessages((prev) =>
            prev.map((m) =>
              m.id === assistantId
                ? { ...m, tokensIn: usage.tokensIn, tokensOut: usage.tokensOut, latencyMs: usage.latencyMs }
                : m
            )
          );
          setLastStats(usage);
        },
      );

      if (conversationId) {
        saveMessage(conversationId, {
          role: 'assistant',
          content: finalContent,
          model: selectedModel,
          tokens_in: finalUsage.tokensIn,
          tokens_out: finalUsage.tokensOut,
          latency_ms: finalUsage.latencyMs,
        }).catch(() => {});
      }
    } catch {
      setMessages((prev) =>
        prev.map((m) =>
          m.id === assistantId
            ? { ...m, content: 'Error: Failed to get a response. Please check your connection and API key.' }
            : m
        )
      );
    } finally {
      setLoading(false);
    }
  };

  const handleSend = () => {
    doSend(input.trim(), messages);
  };

  const handleEdit = async (messageId: string, content: string) => {
    if (!conversationId || loading) return;
    try {
      await deleteMessageAndAfter(conversationId, messageId);
      // Remove from state all messages from this one onwards
      setMessages((prev) => {
        const idx = prev.findIndex((m) => m.id === messageId);
        return idx >= 0 ? prev.slice(0, idx) : prev;
      });
      setInput(content);
    } catch {
      // ignore
    }
  };

  const handleRegenerate = async () => {
    if (!conversationId || loading) return;
    // Find last assistant message
    const lastAssistantIdx = [...messages].reverse().findIndex((m) => m.role === 'assistant');
    if (lastAssistantIdx === -1) return;
    const assistantMsg = [...messages].reverse()[lastAssistantIdx];

    // Find last user message before the assistant message
    const assistantActualIdx = messages.length - 1 - lastAssistantIdx;
    const userMessages = messages.slice(0, assistantActualIdx).filter((m) => m.role === 'user');
    if (userMessages.length === 0) return;
    const lastUserMsg = userMessages[userMessages.length - 1];

    try {
      await deleteMessageAndAfter(conversationId, assistantMsg.id);
      // Remove assistant message from state
      setMessages((prev) => prev.slice(0, assistantActualIdx));
      // Re-send with the history up to (not including) the last user message's position
      const historyBefore = messages.slice(0, assistantActualIdx - 1);
      // Don't use doSend because it re-saves the user message to DB; just stream directly
      await doSend(lastUserMsg.content, historyBefore);
    } catch {
      // ignore
    }
  };

  const handleExport = async (format: 'markdown' | 'json') => {
    if (!conversationId) return;
    try {
      await exportConversation(conversationId, format);
    } catch {
      // ignore
    }
  };

  const handleKeyDown = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  // Determine last assistant message index
  const lastAssistantIdx = (() => {
    for (let i = messages.length - 1; i >= 0; i--) {
      if (messages[i].role === 'assistant') return i;
    }
    return -1;
  })();

  return (
    <div className="flex flex-col h-full" style={{ background: 'var(--bg-primary)' }}>
      {/* Top bar with model selector */}
      <div className="flex items-center justify-between px-2 md:px-4 py-3 border-b border-gray-700" style={{ background: 'var(--bg-secondary)' }}>
        <span className="text-gray-300 text-sm font-medium">Chat</span>
        <div className="flex items-center gap-2">
          {/* Export button */}
          {conversationId && messages.length > 0 && (
            <div className="relative group/export">
              <button
                className="flex items-center gap-1 px-2 py-1 rounded text-xs font-medium bg-gray-700 border border-gray-600 text-gray-300 hover:bg-gray-600 transition-colors"
                title="Export conversation"
              >
                ⬇ Export
              </button>
              <div className="absolute right-0 top-full mt-1 z-50 hidden group-hover/export:block bg-gray-800 border border-gray-600 rounded-lg shadow-xl overflow-hidden">
                <button
                  className="block w-full text-left px-4 py-2 text-xs text-gray-300 hover:bg-gray-700 whitespace-nowrap"
                  onClick={() => handleExport('markdown')}
                >
                  Markdown (.md)
                </button>
                <button
                  className="block w-full text-left px-4 py-2 text-xs text-gray-300 hover:bg-gray-700 whitespace-nowrap"
                  onClick={() => handleExport('json')}
                >
                  JSON
                </button>
              </div>
            </div>
          )}
          <div className="max-w-[60%] md:max-w-none">
            <ModelSelector selectedModel={selectedModel} onModelChange={setSelectedModel} />
          </div>
        </div>
      </div>

      {/* Message list */}
      <div className="flex-1 overflow-y-auto px-2 md:px-4 py-4">
        {messages.length === 0 && (
          <div className="flex flex-col items-center justify-center h-full text-center">
            <p className="text-gray-500 text-lg mb-2">Start a conversation</p>
            <p className="text-gray-600 text-sm">Select a model and type a message below</p>
          </div>
        )}
        {messages.map((msg, idx) => (
          <MessageBubble
            key={msg.id}
            message={msg}
            isLastAssistant={idx === lastAssistantIdx && !loading}
            onEdit={msg.role === 'user' ? handleEdit : undefined}
            onRegenerate={idx === lastAssistantIdx && !loading ? handleRegenerate : undefined}
          />
        ))}
        {loading && messages.length > 0 && messages[messages.length - 1].role === 'assistant' && messages[messages.length - 1].content === '' && (
          <div className="flex justify-start mb-4">
            <div className="bg-gray-700 rounded-2xl px-4 py-3">
              <span className="text-gray-300 text-sm animate-pulse">...</span>
            </div>
          </div>
        )}
        <div ref={bottomRef} />
      </div>

      {/* Status bar */}
      <StatusBar
        tokensIn={lastStats.tokensIn}
        tokensOut={lastStats.tokensOut}
        latencyMs={lastStats.latencyMs}
      />

      {/* System prompt selector */}
      <div className="px-2 md:px-4 pt-2" style={{ background: 'var(--bg-secondary)' }}>
        <SystemPromptSelector value={systemPrompt} onChange={setSystemPrompt} />
      </div>

      {/* Input area */}
      <div className="px-2 md:px-4 py-3 border-t border-gray-700" style={{ background: 'var(--bg-secondary)' }}>
        <div className="flex items-end gap-2">
          <textarea
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="Type a message... (Enter to send, Shift+Enter for newline)"
            rows={1}
            disabled={loading}
            className="flex-1 bg-gray-700 text-gray-100 placeholder-gray-500 border border-gray-600 rounded-xl px-4 py-3 text-sm resize-none focus:outline-none focus:border-blue-500 disabled:opacity-50 max-h-40 overflow-y-auto"
            style={{ minHeight: '44px' }}
            onInput={(e) => {
              const el = e.currentTarget;
              el.style.height = 'auto';
              el.style.height = Math.min(el.scrollHeight, 160) + 'px';
            }}
          />
          <button
            onClick={handleSend}
            disabled={loading || !input.trim() || !selectedModel}
            className="px-4 py-3 bg-blue-600 text-white rounded-xl text-sm font-medium hover:bg-blue-500 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
          >
            Send
          </button>
        </div>
      </div>
    </div>
  );
}
