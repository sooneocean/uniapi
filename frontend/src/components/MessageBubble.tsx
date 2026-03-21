import { useEffect, useRef } from 'react';
import ReactMarkdown from 'react-markdown';
import rehypeRaw from 'rehype-raw';
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter';
import { oneDark } from 'react-syntax-highlighter/dist/esm/styles/prism';
import katex from 'katex';
import 'katex/dist/katex.min.css';
import mermaid from 'mermaid';
import type { Message } from '../types';

mermaid.initialize({ startOnLoad: false, theme: 'dark' });

// Pre-process content to replace LaTeX with rendered HTML
function renderLatex(text: string): string {
  // Block math: $$...$$
  text = text.replace(/\$\$([\s\S]+?)\$\$/g, (_, math) => {
    try { return katex.renderToString(math.trim(), { displayMode: true, throwOnError: false }); }
    catch { return `$$${math}$$`; }
  });
  // Inline math: $...$
  text = text.replace(/\$([^\$\n]+?)\$/g, (_, math) => {
    try { return katex.renderToString(math.trim(), { displayMode: false, throwOnError: false }); }
    catch { return `$${math}$`; }
  });
  return text;
}

function MermaidDiagram({ code }: { code: string }) {
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (!ref.current) return;
    const id = 'mermaid-' + Math.random().toString(36).slice(2);
    mermaid.render(id, code).then(({ svg }) => {
      if (ref.current) ref.current.innerHTML = svg;
    }).catch(() => {
      if (ref.current) ref.current.textContent = 'Diagram error';
    });
  }, [code]);
  return <div ref={ref} className="my-2" />;
}

interface Props {
  message: Message;
  isLastAssistant?: boolean;
  onEdit?: (messageId: string, content: string) => void;
  onRegenerate?: () => void;
}

export default function MessageBubble({ message, isLastAssistant, onEdit, onRegenerate }: Props) {
  const isUser = message.role === 'user';
  const processedContent = isUser ? message.content : renderLatex(message.content);

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
          <div>
            {message.images && message.images.length > 0 && (
              <div className="flex flex-wrap gap-2 mb-2">
                {message.images.map((src, i) => (
                  <img
                    key={i}
                    src={src}
                    alt={`attached image ${i + 1}`}
                    className="max-w-[200px] max-h-[200px] rounded-lg object-cover"
                  />
                ))}
              </div>
            )}
            <p className="text-sm whitespace-pre-wrap break-words">{message.content}</p>
          </div>
        ) : (
          <div className="text-sm prose prose-invert prose-sm max-w-none">
            <ReactMarkdown
              rehypePlugins={[rehypeRaw]}
              components={{
                code({ node: _node, className, children, ...props }) {
                  const match = /language-(\w+)/.exec(className || '');
                  const inline = !match && !String(children).includes('\n');
                  if (!inline && match && match[1] === 'mermaid') {
                    return <MermaidDiagram code={String(children)} />;
                  }
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
              {processedContent}
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
