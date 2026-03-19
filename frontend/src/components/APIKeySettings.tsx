import { useState, useEffect } from 'react';
import { getAPIKeys, createAPIKey, deleteAPIKey } from '../api/client';

interface APIKey {
  id: string;
  label: string;
  created_at?: string;
  expires_at?: string;
}

export default function APIKeySettings() {
  const [keys, setKeys] = useState<APIKey[]>([]);
  const [loading, setLoading] = useState(true);
  const [showForm, setShowForm] = useState(false);
  const [error, setError] = useState('');
  const [newLabel, setNewLabel] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [revealedKey, setRevealedKey] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  const load = async () => {
    try {
      setLoading(true);
      const data = await getAPIKeys();
      setKeys(Array.isArray(data) ? data : data.api_keys ?? data.keys ?? []);
    } catch {
      setError('Failed to load API keys');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, []);

  const handleGenerate = async (e: React.FormEvent) => {
    e.preventDefault();
    setSubmitting(true);
    setError('');
    setRevealedKey(null);
    try {
      const result = await createAPIKey(newLabel);
      setRevealedKey(result.key ?? result.api_key ?? result.token ?? '');
      setNewLabel('');
      setShowForm(false);
      await load();
    } catch {
      setError('Failed to generate API key');
    } finally {
      setSubmitting(false);
    }
  };

  const handleDelete = async (id: string) => {
    if (!confirm('Delete this API key? This cannot be undone.')) return;
    try {
      await deleteAPIKey(id);
      if (revealedKey) setRevealedKey(null);
      await load();
    } catch {
      setError('Failed to delete API key');
    }
  };

  const handleCopy = () => {
    if (revealedKey) {
      navigator.clipboard.writeText(revealedKey);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  };

  const formatDate = (dateStr?: string) => {
    if (!dateStr) return '—';
    return new Date(dateStr).toLocaleDateString();
  };

  return (
    <div className="text-white">
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-lg font-semibold">API Keys</h2>
        <button
          onClick={() => { setShowForm(!showForm); setRevealedKey(null); }}
          className="px-3 py-1.5 bg-blue-600 text-white rounded-lg text-sm hover:bg-blue-500 transition-colors"
        >
          {showForm ? 'Cancel' : '+ Generate Key'}
        </button>
      </div>

      {error && <p className="text-red-400 text-sm mb-3">{error}</p>}

      {revealedKey && (
        <div className="bg-green-900 border border-green-700 rounded-lg p-4 mb-4">
          <p className="text-green-300 text-sm font-medium mb-2">New API Key (shown once — copy it now!)</p>
          <div className="flex items-center gap-2">
            <code className="flex-1 bg-gray-800 text-green-300 rounded px-3 py-2 text-xs font-mono break-all">{revealedKey}</code>
            <button
              onClick={handleCopy}
              className="px-3 py-2 bg-green-700 text-green-100 rounded text-sm hover:bg-green-600 transition-colors whitespace-nowrap"
            >
              {copied ? 'Copied!' : 'Copy'}
            </button>
          </div>
        </div>
      )}

      {showForm && (
        <form onSubmit={handleGenerate} className="bg-gray-700 rounded-lg p-4 mb-4 space-y-3">
          <div>
            <label className="block text-sm text-gray-300 mb-1">Label</label>
            <input
              type="text"
              value={newLabel}
              onChange={(e) => setNewLabel(e.target.value)}
              required
              placeholder="My App Key"
              className="w-full bg-gray-600 text-white rounded px-3 py-2 text-sm border border-gray-500 focus:outline-none focus:border-blue-500 placeholder-gray-400"
            />
          </div>
          <button
            type="submit"
            disabled={submitting}
            className="px-4 py-2 bg-blue-600 text-white rounded-lg text-sm hover:bg-blue-500 disabled:opacity-50 transition-colors"
          >
            {submitting ? 'Generating...' : 'Generate Key'}
          </button>
        </form>
      )}

      {loading ? (
        <p className="text-gray-400 text-sm">Loading...</p>
      ) : keys.length === 0 ? (
        <p className="text-gray-400 text-sm">No API keys yet.</p>
      ) : (
        <div className="space-y-2">
          {keys.map((k) => (
            <div key={k.id} className="bg-gray-700 rounded-lg p-4 flex items-center justify-between">
              <div>
                <p className="text-white font-medium">{k.label}</p>
                <p className="text-gray-400 text-xs mt-0.5">
                  Created: {formatDate(k.created_at)}
                  {k.expires_at && <> &middot; Expires: {formatDate(k.expires_at)}</>}
                </p>
              </div>
              <button
                onClick={() => handleDelete(k.id)}
                className="px-3 py-1.5 bg-red-800 text-red-200 rounded text-sm hover:bg-red-700 transition-colors"
              >
                Delete
              </button>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
