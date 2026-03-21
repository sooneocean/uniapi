import { useEffect, useState, lazy, Suspense } from 'react';
import ReactMarkdown from 'react-markdown';
import rehypeRaw from 'rehype-raw';
import { PrismLight as SyntaxHighlighter } from 'react-syntax-highlighter';
import { oneDark } from 'react-syntax-highlighter/dist/esm/styles/prism';
// Register only needed languages
import tsx from 'react-syntax-highlighter/dist/esm/languages/prism/tsx';
import typescript from 'react-syntax-highlighter/dist/esm/languages/prism/typescript';
import javascript from 'react-syntax-highlighter/dist/esm/languages/prism/javascript';
import python from 'react-syntax-highlighter/dist/esm/languages/prism/python';
import bash from 'react-syntax-highlighter/dist/esm/languages/prism/bash';
import json from 'react-syntax-highlighter/dist/esm/languages/prism/json';
import go from 'react-syntax-highlighter/dist/esm/languages/prism/go';
import rust from 'react-syntax-highlighter/dist/esm/languages/prism/rust';
import sql from 'react-syntax-highlighter/dist/esm/languages/prism/sql';
import css from 'react-syntax-highlighter/dist/esm/languages/prism/css';
import markdown from 'react-syntax-highlighter/dist/esm/languages/prism/markdown';
import yaml from 'react-syntax-highlighter/dist/esm/languages/prism/yaml';
import docker from 'react-syntax-highlighter/dist/esm/languages/prism/docker';
import java from 'react-syntax-highlighter/dist/esm/languages/prism/java';
import c from 'react-syntax-highlighter/dist/esm/languages/prism/c';
import cpp from 'react-syntax-highlighter/dist/esm/languages/prism/cpp';

SyntaxHighlighter.registerLanguage('tsx', tsx);
SyntaxHighlighter.registerLanguage('typescript', typescript);
SyntaxHighlighter.registerLanguage('javascript', javascript);
SyntaxHighlighter.registerLanguage('python', python);
SyntaxHighlighter.registerLanguage('bash', bash);
SyntaxHighlighter.registerLanguage('json', json);
SyntaxHighlighter.registerLanguage('go', go);
SyntaxHighlighter.registerLanguage('rust', rust);
SyntaxHighlighter.registerLanguage('sql', sql);
SyntaxHighlighter.registerLanguage('css', css);
SyntaxHighlighter.registerLanguage('markdown', markdown);
SyntaxHighlighter.registerLanguage('yaml', yaml);
SyntaxHighlighter.registerLanguage('docker', docker);
SyntaxHighlighter.registerLanguage('java', java);
SyntaxHighlighter.registerLanguage('c', c);
SyntaxHighlighter.registerLanguage('cpp', cpp);
import type { Message, ToolCall } from '../types';

// Lazy-load heavy libraries — only bundled into separate chunks, loaded on demand
const MermaidDiagram = lazy(() => import('./MermaidDiagram'));

function ToolCallCard({ toolCall }: { toolCall: ToolCall }) {
  let formattedArgs = toolCall.function.arguments;
  try {
    formattedArgs = JSON.stringify(JSON.parse(toolCall.function.arguments), null, 2);
  } catch {
    // keep as-is if not valid JSON
  }
  return (
    <div className="my-2 rounded-lg border border-yellow-600/40 bg-yellow-900/20 overflow-hidden text-sm">
      <div className="flex items-center gap-2 px-3 py-2 bg-yellow-900/30 border-b border-yellow-600/30">
        <span className="text-yellow-400 font-medium">Tool Call: {toolCall.function.name}</span>
        {toolCall.id && (
          <span className="text-yellow-600 text-xs ml-auto font-mono">{toolCall.id}</span>
        )}
      </div>
      <div className="px-3 py-2">
        <div className="text-gray-400 text-xs mb-1">Arguments:</div>
        <pre className="text-gray-300 text-xs whitespace-pre-wrap break-all font-mono">{formattedArgs}</pre>
      </div>
    </div>
  );
}

function ToolResultCard({ content, toolCallId }: { content: string; toolCallId?: string }) {
  return (
    <div className="my-2 rounded-lg border border-green-600/40 bg-green-900/20 overflow-hidden text-sm">
      <div className="flex items-center gap-2 px-3 py-2 bg-green-900/30 border-b border-green-600/30">
        <span className="text-green-400 font-medium">Tool Result</span>
        {toolCallId && (
          <span className="text-green-600 text-xs ml-auto font-mono">{toolCallId}</span>
        )}
      </div>
      <div className="px-3 py-2">
        <pre className="text-gray-300 text-xs whitespace-pre-wrap break-all font-mono">{content}</pre>
      </div>
    </div>
  );
}

// Pre-process content to replace LaTeX with rendered HTML.
// KaTeX CSS is imported lazily alongside the dynamic import below.
let katexModule: typeof import('katex') | null = null;

async function loadKatex() {
  if (!katexModule) {
    const [mod] = await Promise.all([
      import('katex'),
      import('katex/dist/katex.min.css' as string),
    ]);
    katexModule = mod;
  }
  return katexModule;
}

// Synchronous render — only called after katexModule is loaded
function renderLatexSync(text: string, katex: typeof import('katex')): string {
  // Block math: $$...$$
  text = text.replace(/\$\$([\s\S]+?)\$\$/g, (_, math) => {
    try { return katex.default.renderToString(math.trim(), { displayMode: true, throwOnError: false }); }
    catch { return `$$${math}$$`; }
  });
  // Inline math: $...$
  text = text.replace(/\$([^\$\n]+?)\$/g, (_, math) => {
    try { return katex.default.renderToString(math.trim(), { displayMode: false, throwOnError: false }); }
    catch { return `$${math}$`; }
  });
  return text;
}

interface Props {
  message: Message;
  isLastAssistant?: boolean;
  onEdit?: (messageId: string, content: string) => void;
  onRegenerate?: () => void;
}

export default function MessageBubble({ message, isLastAssistant, onEdit, onRegenerate }: Props) {
  const isUser = message.role === 'user';
  const isTool = message.role === 'tool';
  const hasLatex = !isUser && !isTool && message.content.includes('$');
  const [processedContent, setProcessedContent] = useState(message.content);

  useEffect(() => {
    if (!hasLatex) {
      setProcessedContent(message.content);
      return;
    }
    // Dynamically import KaTeX only when content contains '$'
    loadKatex().then((katex) => {
      setProcessedContent(renderLatexSync(message.content, katex));
    });
  }, [message.content, hasLatex]);

  // Tool result message
  if (isTool) {
    return (
      <div className="flex justify-start mb-4">
        <div className="max-w-[80%]">
          <ToolResultCard content={message.content} toolCallId={message.toolCallId} />
        </div>
      </div>
    );
  }

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
            {message.content && (
              <ReactMarkdown
                rehypePlugins={[rehypeRaw]}
                components={{
                  code({ node: _node, className, children, ...props }) {
                    const match = /language-(\w+)/.exec(className || '');
                    const inline = !match && !String(children).includes('\n');
                    if (!inline && match && match[1] === 'mermaid') {
                      return (
                        <Suspense fallback={<div className="text-gray-500 text-sm py-2">Loading diagram...</div>}>
                          <MermaidDiagram code={String(children)} />
                        </Suspense>
                      );
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
            )}
            {/* Render tool calls if present */}
            {message.toolCalls && message.toolCalls.length > 0 && (
              <div className="mt-2">
                {message.toolCalls.map((tc, i) => (
                  <ToolCallCard key={tc.id || i} toolCall={tc} />
                ))}
              </div>
            )}
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
