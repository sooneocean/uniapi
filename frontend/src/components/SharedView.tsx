import { useState, useEffect } from 'react';
import { getSharedConversation } from '../api/client';
import MessageBubble from './MessageBubble';
import type { Message } from '../types';

interface Props {
  token: string;
}

export default function SharedView({ token }: Props) {
  const [title, setTitle] = useState('');
  const [messages, setMessages] = useState<Message[]>([]);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    getSharedConversation(token)
      .then((data) => {
        setTitle(data.conversation?.Title || data.conversation?.title || 'Shared Conversation');
        const msgs: Message[] = (data.messages ?? []).map((m: any) => ({
          id: m.id || m.ID,
          role: m.role || m.Role,
          content: m.content || m.Content,
          model: m.model || m.Model || undefined,
          tokensIn: m.tokens_in || m.TokensIn || undefined,
          tokensOut: m.tokens_out || m.TokensOut || undefined,
          latencyMs: m.latency_ms || m.LatencyMs || undefined,
          createdAt: m.created_at || m.CreatedAt || new Date().toISOString(),
        }));
        setMessages(msgs);
      })
      .catch(() => setError('Conversation not found or link has been revoked.'))
      .finally(() => setLoading(false));
  }, [token]);

  if (loading) {
    return (
      <div className="flex items-center justify-center h-screen bg-gray-900 text-white">
        Loading...
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex items-center justify-center h-screen bg-gray-900">
        <div className="text-center">
          <p className="text-gray-400 text-lg mb-2">{error}</p>
          <p className="text-gray-600 text-sm">The share link may have been revoked or is invalid.</p>
        </div>
      </div>
    );
  }

  return (
    <div className="flex flex-col h-screen bg-gray-900">
      {/* Header */}
      <div className="px-4 md:px-8 py-4 border-b border-gray-700 bg-gray-800 flex items-center justify-between">
        <div>
          <h1 className="text-white font-semibold text-lg">{title}</h1>
          <p className="text-gray-400 text-xs mt-0.5">Shared via UniAPI</p>
        </div>
        <a
          href="/"
          className="text-blue-400 hover:text-blue-300 text-sm transition-colors"
        >
          Open UniAPI →
        </a>
      </div>

      {/* Messages */}
      <div className="flex-1 overflow-y-auto px-4 md:px-8 py-6 max-w-4xl mx-auto w-full">
        {messages.length === 0 ? (
          <p className="text-gray-500 text-center mt-8">No messages in this conversation.</p>
        ) : (
          messages.map((msg) => (
            <MessageBubble key={msg.id} message={msg} />
          ))
        )}
      </div>

      {/* Footer */}
      <div className="py-3 border-t border-gray-700 bg-gray-800 text-center text-xs text-gray-500">
        Shared via <span className="text-gray-400 font-medium">UniAPI</span> — read-only view
      </div>
    </div>
  );
}
