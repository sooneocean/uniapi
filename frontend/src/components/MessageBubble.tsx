import ReactMarkdown from 'react-markdown';
import type { Message } from '../types';

interface Props {
  message: Message;
}

export default function MessageBubble({ message }: Props) {
  const isUser = message.role === 'user';

  return (
    <div className={`flex ${isUser ? 'justify-end' : 'justify-start'} mb-4`}>
      <div
        className={`max-w-[70%] rounded-2xl px-4 py-3 ${
          isUser
            ? 'bg-blue-600 text-white'
            : 'bg-gray-700 text-gray-100'
        }`}
      >
        {isUser ? (
          <p className="text-sm whitespace-pre-wrap break-words">{message.content}</p>
        ) : (
          <div className="text-sm prose prose-invert prose-sm max-w-none">
            <ReactMarkdown>{message.content}</ReactMarkdown>
          </div>
        )}
        {(message.tokensIn !== undefined || message.latencyMs !== undefined) && (
          <div className="mt-1 text-xs opacity-60">
            {message.tokensIn !== undefined && (
              <span>
                {message.tokensIn} ↑ {message.tokensOut} ↓
              </span>
            )}
            {message.latencyMs !== undefined && message.latencyMs > 0 && (
              <span className="ml-2">{message.latencyMs}ms</span>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
