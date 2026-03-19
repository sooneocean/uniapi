import { useState, useEffect } from 'react';
import { getProviders, addProvider, deleteProvider } from '../api/client';

interface Provider {
  id: string;
  type: string;
  label: string;
  models: string[];
  enabled: boolean;
  managed_by_config?: boolean;
  base_url?: string;
}

export default function ProviderSettings() {
  const [providers, setProviders] = useState<Provider[]>([]);
  const [loading, setLoading] = useState(true);
  const [showForm, setShowForm] = useState(false);
  const [error, setError] = useState('');

  const [formType, setFormType] = useState('anthropic');
  const [formLabel, setFormLabel] = useState('');
  const [formApiKey, setFormApiKey] = useState('');
  const [formModels, setFormModels] = useState('');
  const [formBaseUrl, setFormBaseUrl] = useState('');
  const [submitting, setSubmitting] = useState(false);

  const load = async () => {
    try {
      setLoading(true);
      const data = await getProviders();
      setProviders(Array.isArray(data) ? data : data.providers ?? []);
    } catch {
      setError('Failed to load providers');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, []);

  const handleAdd = async (e: React.FormEvent) => {
    e.preventDefault();
    setSubmitting(true);
    setError('');
    try {
      const payload: any = {
        type: formType,
        label: formLabel,
        api_key: formApiKey,
        models: formModels.split(',').map((m) => m.trim()).filter(Boolean),
      };
      if (formType === 'openai_compatible' && formBaseUrl) {
        payload.base_url = formBaseUrl;
      }
      await addProvider(payload);
      setShowForm(false);
      setFormType('anthropic');
      setFormLabel('');
      setFormApiKey('');
      setFormModels('');
      setFormBaseUrl('');
      await load();
    } catch {
      setError('Failed to add provider');
    } finally {
      setSubmitting(false);
    }
  };

  const handleDelete = async (id: string) => {
    if (!confirm('Delete this provider?')) return;
    try {
      await deleteProvider(id);
      await load();
    } catch {
      setError('Failed to delete provider');
    }
  };

  return (
    <div className="text-white">
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-lg font-semibold">Providers</h2>
        <button
          onClick={() => setShowForm(!showForm)}
          className="px-3 py-1.5 bg-blue-600 text-white rounded-lg text-sm hover:bg-blue-500 transition-colors"
        >
          {showForm ? 'Cancel' : '+ Add Provider'}
        </button>
      </div>

      {error && <p className="text-red-400 text-sm mb-3">{error}</p>}

      {showForm && (
        <form onSubmit={handleAdd} className="bg-gray-700 rounded-lg p-4 mb-4 space-y-3">
          <div>
            <label className="block text-sm text-gray-300 mb-1">Type</label>
            <select
              value={formType}
              onChange={(e) => setFormType(e.target.value)}
              className="w-full bg-gray-600 text-white rounded px-3 py-2 text-sm border border-gray-500 focus:outline-none focus:border-blue-500"
            >
              <option value="anthropic">Anthropic</option>
              <option value="openai">OpenAI</option>
              <option value="gemini">Gemini</option>
              <option value="openai_compatible">OpenAI Compatible</option>
            </select>
          </div>
          <div>
            <label className="block text-sm text-gray-300 mb-1">Label</label>
            <input
              type="text"
              value={formLabel}
              onChange={(e) => setFormLabel(e.target.value)}
              required
              placeholder="My Provider"
              className="w-full bg-gray-600 text-white rounded px-3 py-2 text-sm border border-gray-500 focus:outline-none focus:border-blue-500 placeholder-gray-400"
            />
          </div>
          <div>
            <label className="block text-sm text-gray-300 mb-1">API Key</label>
            <input
              type="password"
              value={formApiKey}
              onChange={(e) => setFormApiKey(e.target.value)}
              required
              placeholder="sk-..."
              className="w-full bg-gray-600 text-white rounded px-3 py-2 text-sm border border-gray-500 focus:outline-none focus:border-blue-500 placeholder-gray-400"
            />
          </div>
          <div>
            <label className="block text-sm text-gray-300 mb-1">Models (comma-separated)</label>
            <input
              type="text"
              value={formModels}
              onChange={(e) => setFormModels(e.target.value)}
              placeholder="gpt-4o, gpt-4o-mini"
              className="w-full bg-gray-600 text-white rounded px-3 py-2 text-sm border border-gray-500 focus:outline-none focus:border-blue-500 placeholder-gray-400"
            />
          </div>
          {formType === 'openai_compatible' && (
            <div>
              <label className="block text-sm text-gray-300 mb-1">Base URL</label>
              <input
                type="url"
                value={formBaseUrl}
                onChange={(e) => setFormBaseUrl(e.target.value)}
                placeholder="https://api.example.com/v1"
                className="w-full bg-gray-600 text-white rounded px-3 py-2 text-sm border border-gray-500 focus:outline-none focus:border-blue-500 placeholder-gray-400"
              />
            </div>
          )}
          <button
            type="submit"
            disabled={submitting}
            className="px-4 py-2 bg-blue-600 text-white rounded-lg text-sm hover:bg-blue-500 disabled:opacity-50 transition-colors"
          >
            {submitting ? 'Adding...' : 'Add Provider'}
          </button>
        </form>
      )}

      {loading ? (
        <p className="text-gray-400 text-sm">Loading...</p>
      ) : providers.length === 0 ? (
        <p className="text-gray-400 text-sm">No providers configured.</p>
      ) : (
        <div className="space-y-3">
          {providers.map((p) => (
            <div key={p.id} className="bg-gray-700 rounded-lg p-4 flex items-start justify-between">
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2 mb-1">
                  <span className="text-white font-medium">{p.label}</span>
                  <span className="text-xs bg-gray-600 text-gray-300 px-2 py-0.5 rounded">{p.type}</span>
                  <span className={`text-xs px-2 py-0.5 rounded ${p.enabled ? 'bg-green-900 text-green-300' : 'bg-red-900 text-red-300'}`}>
                    {p.enabled ? 'enabled' : 'disabled'}
                  </span>
                </div>
                <div className="flex flex-wrap gap-1 mt-2">
                  {(p.models ?? []).map((m) => (
                    <span key={m} className="text-xs bg-gray-600 text-gray-300 px-2 py-0.5 rounded-full">{m}</span>
                  ))}
                </div>
              </div>
              <button
                onClick={() => handleDelete(p.id)}
                disabled={p.managed_by_config}
                title={p.managed_by_config ? 'Managed by config file' : 'Delete provider'}
                className="ml-3 px-3 py-1.5 bg-red-800 text-red-200 rounded text-sm hover:bg-red-700 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
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
