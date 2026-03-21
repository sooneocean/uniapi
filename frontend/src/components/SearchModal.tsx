import { useState, useEffect, useRef, useCallback } from 'react';
import { searchMessages } from '../api/client';

interface SearchResult {
  message_id: string;
  conversation_id: string;
  role: string;
  preview: string;
  created_at: string;
  conversation_title: string;
}

interface Props {
  onClose: () => void;
  onSelectConversation: (id: string) => void;
}

export default function SearchModal({ onClose, onSelectConversation }: Props) {
  const [query, setQuery] = useState('');
  const [results, setResults] = useState<SearchResult[]>([]);
  const [loading, setLoading] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    inputRef.current?.focus();
  }, []);

  const doSearch = useCallback(async (q: string) => {
    if (!q.trim()) {
      setResults([]);
      return;
    }
    setLoading(true);
    try {
      const data = await searchMessages(q.trim());
      setResults(data?.data ?? data ?? []);
    } catch {
      setResults([]);
    } finally {
      setLoading(false);
    }
  }, []);

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const val = e.target.value;
    setQuery(val);
    if (debounceRef.current) clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(() => doSearch(val), 300);
  };

  const handleSelect = (result: SearchResult) => {
    onSelectConversation(result.conversation_id);
    onClose();
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Escape') onClose();
  };

  const highlightText = (text: string, query: string) => {
    if (!query.trim()) return text;
    const parts = text.split(new RegExp(`(${query.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')})`, 'gi'));
    return parts.map((part, i) =>
      part.toLowerCase() === query.toLowerCase()
        ? <mark key={i} className="bg-yellow-500/40 text-yellow-200 rounded px-0.5">{part}</mark>
        : part
    );
  };

  return (
    <div
      className="fixed inset-0 z-50 flex items-start justify-center bg-black/70 backdrop-blur-sm pt-20"
      onClick={onClose}
    >
      <div
        className="w-full max-w-2xl mx-4 bg-gray-900 border border-gray-600 rounded-xl shadow-2xl overflow-hidden"
        onClick={(e) => e.stopPropagation()}
        onKeyDown={handleKeyDown}
      >
        {/* Search input */}
        <div className="flex items-center gap-3 px-4 py-3 border-b border-gray-700">
          <span className="text-gray-400 text-lg">&#128269;</span>
          <input
            ref={inputRef}
            type="text"
            value={query}
            onChange={handleChange}
            placeholder="Search messages..."
            className="flex-1 bg-transparent text-white placeholder-gray-500 text-base outline-none"
          />
          {loading && (
            <span className="text-gray-400 text-sm animate-pulse">Searching...</span>
          )}
          <kbd
            className="text-gray-500 text-xs px-2 py-1 border border-gray-700 rounded cursor-pointer hover:border-gray-500"
            onClick={onClose}
          >
            Esc
          </kbd>
        </div>

        {/* Results */}
        <div className="max-h-[60vh] overflow-y-auto">
          {results.length === 0 && query.trim() && !loading && (
            <div className="px-4 py-8 text-center text-gray-500 text-sm">
              No results found for "{query}"
            </div>
          )}
          {results.length === 0 && !query.trim() && (
            <div className="px-4 py-8 text-center text-gray-600 text-sm">
              Type to search across all messages
            </div>
          )}
          {results.map((result) => (
            <button
              key={result.message_id}
              className="w-full text-left px-4 py-3 border-b border-gray-800 hover:bg-gray-800 transition-colors"
              onClick={() => handleSelect(result)}
            >
              <div className="flex items-center justify-between mb-1">
                <span className="text-blue-400 text-sm font-medium truncate max-w-[70%]">
                  {result.conversation_title || 'Untitled'}
                </span>
                <span className="text-gray-600 text-xs ml-2 flex-shrink-0">
                  {new Date(result.created_at).toLocaleDateString()}
                </span>
              </div>
              <div className="flex items-start gap-2">
                <span className={`text-xs px-1.5 py-0.5 rounded flex-shrink-0 mt-0.5 ${
                  result.role === 'user' ? 'bg-blue-900 text-blue-300' : 'bg-green-900 text-green-300'
                }`}>
                  {result.role}
                </span>
                <p className="text-gray-300 text-sm line-clamp-2 leading-relaxed">
                  {highlightText(result.preview, query)}
                </p>
              </div>
            </button>
          ))}
        </div>

        {results.length > 0 && (
          <div className="px-4 py-2 text-xs text-gray-600 border-t border-gray-800">
            {results.length} result{results.length !== 1 ? 's' : ''} — click to open conversation
          </div>
        )}
      </div>
    </div>
  );
}
