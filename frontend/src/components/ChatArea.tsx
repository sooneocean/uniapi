import { useState, useRef, useEffect, type KeyboardEvent } from 'react';
import { v4 as uuidv4 } from 'uuid';
import type { Message } from '../types';
import { sendMessage } from '../api/client';
import ModelSelector from './ModelSelector';
import MessageBubble from './MessageBubble';
import StatusBar from './StatusBar';

export default function ChatArea() {
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState('');
  const [selectedModel, setSelectedModel] = useState('');
  const [loading, setLoading] = useState(false);
  const [lastStats, setLastStats] = useState({ tokensIn: 0, tokensOut: 0, latencyMs: 0 });
  const bottomRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages, loading]);

  const handleSend = async () => {
    const text = input.trim();
    if (!text || loading || !selectedModel) return;

    const userMsg: Message = {
      id: uuidv4(),
      role: 'user',
      content: text,
      createdAt: new Date().toISOString(),
    };

    const nextMessages = [...messages, userMsg];
    setMessages(nextMessages);
    setInput('');
    setLoading(true);

    try {
      const apiMessages = nextMessages.map((m) => ({ role: m.role, content: m.content }));
      const result = await sendMessage(selectedModel, apiMessages);

      const assistantMsg: Message = {
        id: uuidv4(),
        role: 'assistant',
        content: result.content,
        model: selectedModel,
        tokensIn: result.tokensIn,
        tokensOut: result.tokensOut,
        latencyMs: result.latencyMs,
        createdAt: new Date().toISOString(),
      };

      setMessages((prev) => [...prev, assistantMsg]);
      setLastStats({
        tokensIn: result.tokensIn,
        tokensOut: result.tokensOut,
        latencyMs: result.latencyMs,
      });
    } catch (err) {
      const errorMsg: Message = {
        id: uuidv4(),
        role: 'assistant',
        content: 'Error: Failed to get a response. Please check your connection and API key.',
        createdAt: new Date().toISOString(),
      };
      setMessages((prev) => [...prev, errorMsg]);
    } finally {
      setLoading(false);
    }
  };

  const handleKeyDown = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  return (
    <div className="flex flex-col h-full bg-gray-900">
      {/* Top bar with model selector */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-gray-700 bg-gray-800">
        <span className="text-gray-300 text-sm font-medium">Chat</span>
        <ModelSelector selectedModel={selectedModel} onModelChange={setSelectedModel} />
      </div>

      {/* Message list */}
      <div className="flex-1 overflow-y-auto px-4 py-4">
        {messages.length === 0 && (
          <div className="flex flex-col items-center justify-center h-full text-center">
            <p className="text-gray-500 text-lg mb-2">Start a conversation</p>
            <p className="text-gray-600 text-sm">Select a model and type a message below</p>
          </div>
        )}
        {messages.map((msg) => (
          <MessageBubble key={msg.id} message={msg} />
        ))}
        {loading && (
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

      {/* Input area */}
      <div className="px-4 py-3 border-t border-gray-700 bg-gray-800">
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
