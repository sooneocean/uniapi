import { useState, useRef, useEffect, type KeyboardEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { v4 as uuidv4 } from 'uuid';
import type { Message, ToolCall } from '../types';
import {
  sendMessageStream,
  getConversation,
  saveMessage,
  deleteMessageAndAfter,
  exportConversation,
  autoTitle,
  shareConversation,
  unshareConversation,
} from '../api/client';
import ModelSelector from './ModelSelector';
import MessageBubble from './MessageBubble';
import StatusBar from './StatusBar';
import SystemPromptSelector from './SystemPromptSelector';
import VoiceInput from './VoiceInput';
import FileAttachment, { type AttachedFile, formatAttachedFiles } from './FileAttachment';
import { estimateTokens, estimateCost } from '../utils/tokenEstimator';

interface Props {
  conversationId: string | null;
  onConversationTitleUpdate?: (id: string, title: string) => void;
  onRegisterFocusInput?: (fn: () => void) => void;
}

export default function ChatArea({ conversationId, onConversationTitleUpdate, onRegisterFocusInput }: Props) {
  const { t } = useTranslation();
  const [messages, setMessages] = useState<Message[]>([]);
  const [conversationTitle, setConversationTitle] = useState('');
  const [input, setInput] = useState('');
  const [pendingImages, setPendingImages] = useState<string[]>([]);
  const [attachedFiles, setAttachedFiles] = useState<AttachedFile[]>([]);
  const [selectedModel, setSelectedModel] = useState('');
  const [loading, setLoading] = useState(false);
  const [lastStats, setLastStats] = useState({ tokensIn: 0, tokensOut: 0, latencyMs: 0 });
  const [systemPrompt, setSystemPrompt] = useState('');
  const [streamingSpeed, setStreamingSpeed] = useState(0);
  const [shareUrl, setShareUrl] = useState<string | null>(null);
  const [showShareDialog, setShowShareDialog] = useState(false);
  const [shareCopied, setShareCopied] = useState(false);
  const streamStartRef = useRef<number>(0);
  const tokenCountRef = useRef<number>(0);
  const bottomRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLTextAreaElement>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (onRegisterFocusInput) {
      onRegisterFocusInput(() => inputRef.current?.focus());
    }
  }, [onRegisterFocusInput]);

  // Load messages when conversation changes
  useEffect(() => {
    if (!conversationId) {
      setMessages([]);
      setConversationTitle('');
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
        setConversationTitle(conv.title ?? '');
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

  // Token estimation
  const estimatedTokens = (() => {
    let total = 0;
    if (systemPrompt.trim()) total += estimateTokens(systemPrompt.trim());
    for (const m of messages) {
      total += estimateTokens(m.content);
    }
    total += estimateTokens(input);
    return total;
  })();
  const estimatedCost = estimateCost(estimatedTokens, selectedModel);

  const doSend = async (text: string, history: Message[], imgs: string[]) => {
    if (!text || loading || !selectedModel) return;

    const userMsg: Message = {
      id: uuidv4(),
      role: 'user',
      content: text,
      images: imgs.length > 0 ? imgs : undefined,
      createdAt: new Date().toISOString(),
    };

    const nextMessages = [...history, userMsg];
    setMessages(nextMessages);
    setInput('');
    setPendingImages([]);
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

    // Reset streaming speed tracking
    streamStartRef.current = Date.now();
    tokenCountRef.current = 0;
    setStreamingSpeed(0);

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
          tokenCountRef.current++;
          const elapsed = (Date.now() - streamStartRef.current) / 1000;
          if (elapsed > 0.5) {
            setStreamingSpeed(Math.round(tokenCountRef.current / elapsed));
          }
          setMessages((prev) =>
            prev.map((m) => m.id === assistantId ? { ...m, content: m.content + chunk } : m)
          );
        },
        (usage) => {
          finalUsage = usage;
          const toolCalls = (usage as any).toolCalls as ToolCall[] | undefined;
          setMessages((prev) =>
            prev.map((m) =>
              m.id === assistantId
                ? {
                    ...m,
                    tokensIn: usage.tokensIn,
                    tokensOut: usage.tokensOut,
                    latencyMs: usage.latencyMs,
                    ...(toolCalls && toolCalls.length > 0 ? { toolCalls } : {}),
                  }
                : m
            )
          );
          setLastStats(usage);
        },
        imgs.length > 0 ? imgs : undefined,
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

        // Auto-title after first exchange when title is still default
        if ((conversationTitle === 'New Chat' || conversationTitle === '') && nextMessages.length === 1) {
          autoTitle(conversationId).then((title) => {
            setConversationTitle(title);
            if (onConversationTitleUpdate) {
              onConversationTitleUpdate(conversationId, title);
            }
          }).catch(() => {});
        }
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
      setStreamingSpeed(0);
    }
  };

  const handleSend = () => {
    const text = formatAttachedFiles(attachedFiles, input.trim());
    setAttachedFiles([]);
    doSend(text, messages, pendingImages);
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
      await doSend(lastUserMsg.content, historyBefore, []);
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

  const handlePaste = (e: React.ClipboardEvent) => {
    const items = e.clipboardData.items;
    for (const item of Array.from(items)) {
      if (item.type.startsWith('image/')) {
        e.preventDefault();
        const file = item.getAsFile();
        if (!file) return;
        const reader = new FileReader();
        reader.onload = () => {
          setPendingImages((prev) => [...prev, reader.result as string]);
        };
        reader.readAsDataURL(file);
      }
    }
  };

  const handleFileInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = e.target.files;
    if (!files) return;
    for (const file of Array.from(files)) {
      if (file.type.startsWith('image/')) {
        const reader = new FileReader();
        reader.onload = () => {
          setPendingImages((prev) => [...prev, reader.result as string]);
        };
        reader.readAsDataURL(file);
      }
    }
    // Reset file input so the same file can be selected again
    e.target.value = '';
  };

  // Determine last assistant message index
  const lastAssistantIdx = (() => {
    for (let i = messages.length - 1; i >= 0; i--) {
      if (messages[i].role === 'assistant') return i;
    }
    return -1;
  })();

  const handleDragOver = (e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
  };

  const handleDrop = async (e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    const files = e.dataTransfer.files;
    if (!files || files.length === 0) return;
    // Attempt to process as text/code files (FileAttachment logic inline here)
    const MAX_FILE_SIZE_DROP = 100 * 1024;
    const MAX_FILES_DROP = 5;
    const SUPPORTED_TYPES_DROP: Record<string, boolean> = {
      'text/plain': true, 'text/markdown': true, 'text/csv': true,
      'text/html': true, 'application/json': true, 'application/javascript': true,
      'text/javascript': true, 'text/css': true, 'text/xml': true,
      'application/xml': true, 'application/x-yaml': true, 'text/yaml': true,
    };
    const isCode = (name: string) => {
      const ext = name.split('.').pop()?.toLowerCase() || '';
      return ['py','js','ts','tsx','jsx','go','rs','java','c','cpp','h','hpp',
        'rb','php','swift','kt','scala','sh','bash','sql','r','lua',
        'toml','ini','cfg','env','dockerfile','makefile','yaml','yml',
        'md','txt','csv','log','xml','html','css','scss','json'].includes(ext);
    };
    const newFiles: AttachedFile[] = [];
    for (const file of Array.from(files)) {
      if (attachedFiles.length + newFiles.length >= MAX_FILES_DROP) break;
      if (file.size > MAX_FILE_SIZE_DROP) continue;
      if (!SUPPORTED_TYPES_DROP[file.type] && !isCode(file.name)) {
        // Try as image
        if (file.type.startsWith('image/')) {
          const reader = new FileReader();
          reader.onload = () => setPendingImages((prev) => [...prev, reader.result as string]);
          reader.readAsDataURL(file);
        }
        continue;
      }
      try {
        const content = await new Promise<string>((resolve, reject) => {
          const reader = new FileReader();
          reader.onload = () => resolve(reader.result as string);
          reader.onerror = reject;
          reader.readAsText(file);
        });
        newFiles.push({ name: file.name, content, type: file.type });
      } catch {}
    }
    if (newFiles.length > 0) {
      setAttachedFiles((prev) => [...prev, ...newFiles]);
    }
  };

  return (
    <div
      className="flex flex-col h-full"
      style={{ background: 'var(--bg-primary)' }}
      onDragOver={handleDragOver}
      onDrop={handleDrop}
    >
      {/* Share dialog */}
      {showShareDialog && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60" onClick={() => setShowShareDialog(false)}>
          <div className="bg-gray-800 border border-gray-600 rounded-xl p-6 shadow-2xl max-w-md w-full mx-4" onClick={(e) => e.stopPropagation()}>
            <h3 className="text-white font-semibold mb-4">Share Conversation</h3>
            {shareUrl ? (
              <>
                <p className="text-gray-300 text-sm mb-3">Anyone with this link can view this conversation:</p>
                <div className="flex gap-2">
                  <input
                    className="flex-1 bg-gray-700 text-gray-100 border border-gray-600 rounded-lg px-3 py-2 text-sm focus:outline-none"
                    value={window.location.origin + shareUrl}
                    readOnly
                  />
                  <button
                    className="px-3 py-2 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-sm transition-colors"
                    onClick={() => {
                      navigator.clipboard.writeText(window.location.origin + shareUrl);
                      setShareCopied(true);
                      setTimeout(() => setShareCopied(false), 2000);
                    }}
                  >
                    {shareCopied ? 'Copied!' : 'Copy'}
                  </button>
                </div>
                <button
                  className="mt-3 text-xs text-red-400 hover:text-red-300"
                  onClick={async () => {
                    if (conversationId) {
                      await unshareConversation(conversationId).catch(() => {});
                      setShareUrl(null);
                    }
                  }}
                >
                  Revoke link
                </button>
              </>
            ) : (
              <p className="text-gray-400 text-sm">Generating share link...</p>
            )}
            <button
              className="mt-4 w-full py-2 bg-gray-700 hover:bg-gray-600 text-gray-300 rounded-lg text-sm transition-colors"
              onClick={() => setShowShareDialog(false)}
            >
              Close
            </button>
          </div>
        </div>
      )}

      {/* Top bar with model selector */}
      <div className="flex items-center justify-between px-2 md:px-4 py-3 border-b border-gray-700" style={{ background: 'var(--bg-secondary)' }}>
        <span className="text-gray-300 text-sm font-medium">Chat</span>
        <div className="flex items-center gap-2">
          {/* Share button */}
          {conversationId && messages.length > 0 && (
            <button
              className="flex items-center gap-1 px-2 py-1 rounded text-xs font-medium bg-gray-700 border border-gray-600 text-gray-300 hover:bg-gray-600 transition-colors"
              title="Share conversation"
              onClick={async () => {
                setShowShareDialog(true);
                if (!shareUrl) {
                  const result = await shareConversation(conversationId).catch(() => null);
                  if (result) setShareUrl(result.share_url);
                }
              }}
            >
              🔗
            </button>
          )}
          {/* Export button */}
          {conversationId && messages.length > 0 && (
            <div className="relative group/export">
              <button
                className="flex items-center gap-1 px-2 py-1 rounded text-xs font-medium bg-gray-700 border border-gray-600 text-gray-300 hover:bg-gray-600 transition-colors"
                title="Export conversation"
              >
                ⬇ {t('chat.export')}
              </button>
              <div className="absolute right-0 top-full mt-1 z-50 hidden group-hover/export:block bg-gray-800 border border-gray-600 rounded-lg shadow-xl overflow-hidden">
                <button
                  className="block w-full text-left px-4 py-2 text-xs text-gray-300 hover:bg-gray-700 whitespace-nowrap"
                  onClick={() => handleExport('markdown')}
                >
                  {t('chat.exportMd')}
                </button>
                <button
                  className="block w-full text-left px-4 py-2 text-xs text-gray-300 hover:bg-gray-700 whitespace-nowrap"
                  onClick={() => handleExport('json')}
                >
                  {t('chat.exportJson')}
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
        streamingSpeed={loading ? streamingSpeed : 0}
      />

      {/* System prompt selector */}
      <div className="px-2 md:px-4 pt-2" style={{ background: 'var(--bg-secondary)' }}>
        <SystemPromptSelector value={systemPrompt} onChange={setSystemPrompt} />
      </div>

      {/* Pending image previews */}
      {pendingImages.length > 0 && (
        <div className="px-2 md:px-4 pt-2 flex flex-wrap gap-2" style={{ background: 'var(--bg-secondary)' }}>
          {pendingImages.map((src, i) => (
            <div key={i} className="relative">
              <img
                src={src}
                alt={`pending image ${i + 1}`}
                className="w-16 h-16 object-cover rounded-lg border border-gray-600"
              />
              <button
                className="absolute -top-1 -right-1 bg-gray-700 hover:bg-red-600 text-white rounded-full w-4 h-4 flex items-center justify-center text-xs leading-none"
                onClick={() => setPendingImages((prev) => prev.filter((_, idx) => idx !== i))}
                title="Remove image"
              >
                ×
              </button>
            </div>
          ))}
        </div>
      )}

      {/* File chips */}
      {attachedFiles.length > 0 && (
        <div className="px-2 md:px-4 pt-2 flex flex-wrap gap-1" style={{ background: 'var(--bg-secondary)' }}>
          {attachedFiles.map((file, i) => (
            <div
              key={i}
              className="flex items-center gap-1 px-2 py-1 bg-gray-700 border border-gray-600 rounded-lg text-xs text-gray-300"
            >
              <span className="max-w-[150px] truncate" title={file.name}>{file.name}</span>
              <button
                onClick={() => setAttachedFiles((prev) => prev.filter((_, idx) => idx !== i))}
                className="text-gray-500 hover:text-red-400 transition-colors ml-1"
                title="Remove file"
              >
                ✕
              </button>
            </div>
          ))}
        </div>
      )}

      {/* Input area */}
      <div className="px-2 md:px-4 py-3 border-t border-gray-700" style={{ background: 'var(--bg-secondary)' }}>
        {/* Token estimate */}
        {(input || messages.length > 0) && (
          <div className="text-xs text-gray-500 mb-1">
            {t('chat.tokenEstimate', {
              tokens: estimatedTokens.toLocaleString(),
              cost: estimatedCost < 0.001 ? estimatedCost.toFixed(6) : estimatedCost.toFixed(4),
            })}
          </div>
        )}
        <div className="flex items-end gap-2">
          {/* Hidden image file input */}
          <input
            ref={fileInputRef}
            type="file"
            accept="image/*"
            multiple
            className="hidden"
            onChange={handleFileInputChange}
          />
          {/* Image attach button */}
          <button
            type="button"
            onClick={() => fileInputRef.current?.click()}
            disabled={loading}
            className="px-3 py-3 bg-gray-700 text-gray-300 rounded-xl text-sm hover:bg-gray-600 disabled:opacity-50 disabled:cursor-not-allowed transition-colors flex-shrink-0"
            title="Attach image"
          >
            📎
          </button>
          {/* File attachment button */}
          <FileAttachment
            attachedFiles={attachedFiles}
            onFilesChange={setAttachedFiles}
            disabled={loading}
          />
          <textarea
            ref={inputRef}
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={handleKeyDown}
            onPaste={handlePaste}
            placeholder={t('chat.placeholder')}
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
          {/* Voice input */}
          <VoiceInput
            onTranscript={(text) => setInput((prev) => prev + (prev ? ' ' : '') + text)}
            disabled={loading}
          />
          <button
            onClick={handleSend}
            disabled={loading || (!input.trim() && pendingImages.length === 0 && attachedFiles.length === 0) || !selectedModel}
            className="px-4 py-3 bg-blue-600 text-white rounded-xl text-sm font-medium hover:bg-blue-500 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
          >
            {t('chat.send')}
          </button>
        </div>
      </div>
    </div>
  );
}
