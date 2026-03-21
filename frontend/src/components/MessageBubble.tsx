import ReactMarkdown from 'react-markdown';
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter';
import { oneDark } from 'react-syntax-highlighter/dist/esm/styles/prism';
import type { Message } from '../types';

interface Props {
  message: Message;
  isLastAssistant?: boolean;
  onEdit?: (messageId: string, content: string) => void;
  onRegenerate?: () => void;
}

export default function MessageBubble({ message, isLastAssistant, onEdit, onRegenerate }: Props) {
  const isUser = message.role === 'user';

  return (
    <div className={`flex ${isUser ? 'justify-end' : 'justify-start'} mb-4 group`}>
      <div
        className={`max-w-[70%] rounded-2xl px-4 py-3 relative ${
          isUser
            ? 'bg-blue-600 text-white'
            : 'bg-gray-700 text-gray-100'
        }`}
      >
        {isUser ? (
          <p className="text-sm whitespace-pre-wrap break-words">{message.content}</p>
        ) : (
          <div className="text-sm prose prose-invert prose-sm max-w-none">
            <ReactMarkdown
              components={{
                code({ node: _node, className, children, ...props }) {
                  const match = /language-(\w+)/.exec(className || '');
                  const inline = !match && !String(children).includes('\n');
                  if (!inline && match) {
                    return (
                      <div className="relative group">
                        <button
                          className="absolute top-2 right-2 opacity-0 group-hover:opacity-100 text-xs bg-gray-600 px-2 py-1 rounded z-10 text-gray-200 hover:bg-gray-500 transition-colors"
                          onClick={() => navigator.clipboard.writeText(String(children))}
                        >
                          Copy
                        </button>
                        <SyntaxHighlighter style={oneDark} language={match[1]} PreTag="div">
                          {String(children).replace(/\n$/, '')}
                        </SyntaxHighlighter>
                      </div>
                    );
                  }
                  return (
                    <code className="bg-gray-600 px-1 rounded" {...props}>
                      {children}
                    </code>
                  );
                },
              }}
            >
              {message.content}
            </ReactMarkdown>
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

        {/* Action buttons */}
        {isUser && onEdit && (
          <button
            className="absolute -top-2 -right-2 opacity-0 group-hover:opacity-100 bg-gray-600 hover:bg-gray-500 text-gray-200 rounded-full w-6 h-6 flex items-center justify-center text-xs transition-all shadow"
            onClick={() => onEdit(message.id, message.content)}
            title="Edit message"
          >
            ✎
          </button>
        )}
        {!isUser && isLastAssistant && onRegenerate && (
          <button
            className="absolute -top-2 -right-2 opacity-0 group-hover:opacity-100 bg-gray-600 hover:bg-gray-500 text-gray-200 rounded-full w-6 h-6 flex items-center justify-center text-xs transition-all shadow"
            onClick={onRegenerate}
            title="Regenerate response"
          >
            ↻
          </button>
        )}
      </div>
    </div>
  );
}
