import { useState, useEffect } from 'react';
import ReactMarkdown from 'react-markdown';
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter';
import { oneDark } from 'react-syntax-highlighter/dist/esm/styles/prism';
import { fetchModels, compareModels } from '../api/client';
import type { ModelInfo } from '../types';

interface CompareResult {
  model: string;
  content: string;
  tokens_in: number;
  tokens_out: number;
  latency_ms: number;
  cost: number;
  error?: string;
}

interface Props {
  onClose: () => void;
}

export default function CompareMode({ onClose }: Props) {
  const [models, setModels] = useState<ModelInfo[]>([]);
  const [selectedModels, setSelectedModels] = useState<string[]>([]);
  const [prompt, setPrompt] = useState('');
  const [systemPrompt, setSystemPrompt] = useState('');
  const [results, setResults] = useState<CompareResult[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    fetchModels()
      .then((m) => {
        setModels(m);
        // Pre-select up to 2 models
        if (m.length >= 2) {
          setSelectedModels([m[0].id, m[1].id]);
        } else if (m.length === 1) {
          setSelectedModels([m[0].id]);
        }
      })
      .catch(() => {});
  }, []);

  const toggleModel = (modelId: string) => {
    setSelectedModels((prev) => {
      if (prev.includes(modelId)) {
        return prev.filter((m) => m !== modelId);
      }
      if (prev.length >= 4) return prev; // max 4
      return [...prev, modelId];
    });
  };

  const handleCompare = async () => {
    if (!prompt.trim()) {
      setError('Please enter a prompt.');
      return;
    }
    if (selectedModels.length < 2) {
      setError('Select at least 2 models.');
      return;
    }
    setError('');
    setLoading(true);
    setResults([]);
    try {
      const res = await compareModels(prompt, selectedModels, systemPrompt || undefined);
      setResults(res);
    } catch (e: any) {
      setError(e?.response?.data?.error || e.message || 'Request failed');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-start justify-center overflow-auto py-8 px-4" style={{ background: 'rgba(0,0,0,0.7)' }}>
      <div className="w-full max-w-6xl rounded-2xl shadow-2xl flex flex-col gap-4 p-6" style={{ background: 'var(--bg-secondary)', color: 'var(--text-primary)' }}>
        {/* Header */}
        <div className="flex items-center justify-between">
          <h2 className="text-lg font-bold flex items-center gap-2">
            <span>&#9878;</span> Compare Models
          </h2>
          <button
            onClick={onClose}
            className="text-gray-400 hover:text-white text-xl px-2 py-1 rounded hover:bg-gray-700 transition-colors"
            aria-label="Close"
          >
            &#10005;
          </button>
        </div>

        {/* System prompt */}
        <div>
          <label className="block text-sm font-medium mb-1 text-gray-300">System Prompt (optional)</label>
          <textarea
            className="w-full rounded-lg px-3 py-2 text-sm resize-none border border-gray-600 focus:outline-none focus:border-indigo-500"
            style={{ background: 'var(--bg-primary)', color: 'var(--text-primary)' }}
            rows={2}
            placeholder="Optional system instructions..."
            value={systemPrompt}
            onChange={(e) => setSystemPrompt(e.target.value)}
          />
        </div>

        {/* Model selection */}
        <div>
          <label className="block text-sm font-medium mb-2 text-gray-300">
            Models <span className="text-gray-500">(select 2–4)</span>
          </label>
          {models.length === 0 ? (
            <p className="text-sm text-gray-500">No models available. Configure a provider first.</p>
          ) : (
            <div className="flex flex-wrap gap-2">
              {models.map((m) => {
                const checked = selectedModels.includes(m.id);
                const disabled = !checked && selectedModels.length >= 4;
                return (
                  <button
                    key={m.id}
                    onClick={() => !disabled && toggleModel(m.id)}
                    className={`text-xs px-3 py-1.5 rounded-full border transition-colors ${
                      checked
                        ? 'border-indigo-500 bg-indigo-600 text-white'
                        : disabled
                        ? 'border-gray-600 text-gray-600 cursor-not-allowed'
                        : 'border-gray-600 text-gray-300 hover:border-indigo-400 hover:text-white'
                    }`}
                  >
                    {checked ? '✓ ' : ''}{m.id}
                  </button>
                );
              })}
            </div>
          )}
        </div>

        {/* Prompt */}
        <div>
          <label className="block text-sm font-medium mb-1 text-gray-300">Prompt</label>
          <textarea
            className="w-full rounded-lg px-3 py-2 text-sm resize-none border border-gray-600 focus:outline-none focus:border-indigo-500"
            style={{ background: 'var(--bg-primary)', color: 'var(--text-primary)' }}
            rows={4}
            placeholder="Enter your prompt here..."
            value={prompt}
            onChange={(e) => setPrompt(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) handleCompare();
            }}
          />
        </div>

        {error && <p className="text-red-400 text-sm">{error}</p>}

        <button
          onClick={handleCompare}
          disabled={loading || selectedModels.length < 2 || !prompt.trim()}
          className="self-start px-5 py-2 rounded-lg font-medium text-sm bg-indigo-600 hover:bg-indigo-500 text-white disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
        >
          {loading ? 'Comparing...' : 'Compare'}
        </button>

        {/* Loading state */}
        {loading && (
          <div className="flex items-center gap-3 text-gray-400 text-sm">
            <svg className="animate-spin h-5 w-5 text-indigo-400" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
              <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
              <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8z" />
            </svg>
            Waiting for all models to respond...
          </div>
        )}

        {/* Results */}
        {results.length > 0 && (
          <div>
            <h3 className="text-sm font-semibold text-gray-300 mb-3">Results</h3>
            <div className="grid gap-4" style={{ gridTemplateColumns: `repeat(${Math.min(results.length, 2)}, minmax(0, 1fr))` }}>
              {results.map((r) => (
                <ResultCard key={r.model} result={r} />
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

function ResultCard({ result }: { result: CompareResult }) {
  const [copied, setCopied] = useState(false);

  const handleCopy = () => {
    navigator.clipboard.writeText(result.content || result.error || '').then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    });
  };

  return (
    <div className="rounded-xl border border-gray-700 flex flex-col overflow-hidden" style={{ background: 'var(--bg-primary)' }}>
      {/* Card header */}
      <div className="flex items-center justify-between px-4 py-2 border-b border-gray-700" style={{ background: 'var(--bg-secondary)' }}>
        <div>
          <span className="text-sm font-semibold text-indigo-300">{result.model}</span>
          {!result.error && (
            <span className="ml-2 text-xs text-gray-400">
              {(result.latency_ms / 1000).toFixed(2)}s
              {' · '}
              ${result.cost.toFixed(4)}
              {' · '}
              {result.tokens_in}↑ {result.tokens_out}↓
            </span>
          )}
        </div>
        <button
          onClick={handleCopy}
          className="text-xs px-2 py-1 rounded border border-gray-600 text-gray-400 hover:text-white hover:border-gray-400 transition-colors"
        >
          {copied ? 'Copied!' : 'Copy'}
        </button>
      </div>

      {/* Card body */}
      <div className="p-4 text-sm overflow-auto max-h-96 flex-1">
        {result.error ? (
          <p className="text-red-400">{result.error}</p>
        ) : (
          <div className="prose prose-invert prose-sm max-w-none">
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
              {result.content}
            </ReactMarkdown>
          </div>
        )}
      </div>
    </div>
  );
}
