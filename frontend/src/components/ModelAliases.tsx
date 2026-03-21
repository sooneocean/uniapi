import { useState, useEffect } from 'react';
import { getModelAliases, createModelAlias, deleteModelAlias } from '../api/client';
import { fetchModels } from '../api/client';
import type { ModelInfo } from '../types';

interface ModelAlias {
  alias: string;
  model_id: string;
  user_id?: string;
  created_at: string;
}

export default function ModelAliases() {
  const [aliases, setAliases] = useState<ModelAlias[]>([]);
  const [models, setModels] = useState<ModelInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [showForm, setShowForm] = useState(false);
  const [error, setError] = useState('');
  const [newAlias, setNewAlias] = useState('');
  const [newModelId, setNewModelId] = useState('');
  const [submitting, setSubmitting] = useState(false);

  const load = async () => {
    try {
      setLoading(true);
      const [aliasData, modelData] = await Promise.all([getModelAliases(), fetchModels()]);
      setAliases(Array.isArray(aliasData) ? aliasData : []);
      setModels(Array.isArray(modelData) ? modelData : []);
    } catch {
      setError('Failed to load model aliases');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, []);

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newAlias.trim() || !newModelId.trim()) return;
    setSubmitting(true);
    setError('');
    try {
      await createModelAlias(newAlias.trim(), newModelId.trim());
      setNewAlias('');
      setNewModelId('');
      setShowForm(false);
      await load();
    } catch {
      setError('Failed to create alias');
    } finally {
      setSubmitting(false);
    }
  };

  const handleDelete = async (alias: string) => {
    if (!confirm(`Delete alias "${alias}"?`)) return;
    try {
      await deleteModelAlias(alias);
      await load();
    } catch {
      setError('Failed to delete alias');
    }
  };

  return (
    <div className="text-white mt-6">
      <div className="flex items-center justify-between mb-4">
        <div>
          <h2 className="text-lg font-semibold">Model Aliases</h2>
          <p className="text-gray-400 text-xs mt-0.5">Create friendly names for models, e.g. "fast" → "gpt-4o-mini"</p>
        </div>
        <button
          onClick={() => setShowForm(!showForm)}
          className="px-3 py-1.5 bg-blue-600 text-white rounded-lg text-sm hover:bg-blue-500 transition-colors"
        >
          {showForm ? 'Cancel' : '+ Add Alias'}
        </button>
      </div>

      {error && <p className="text-red-400 text-sm mb-3">{error}</p>}

      {showForm && (
        <form onSubmit={handleCreate} className="bg-gray-700 rounded-lg p-4 mb-4 space-y-3">
          <div>
            <label className="block text-sm text-gray-300 mb-1">Alias Name</label>
            <input
              type="text"
              value={newAlias}
              onChange={(e) => setNewAlias(e.target.value)}
              required
              placeholder="fast"
              className="w-full bg-gray-600 text-white rounded px-3 py-2 text-sm border border-gray-500 focus:outline-none focus:border-blue-500 placeholder-gray-400"
            />
          </div>
          <div>
            <label className="block text-sm text-gray-300 mb-1">Model</label>
            {models.length > 0 ? (
              <select
                value={newModelId}
                onChange={(e) => setNewModelId(e.target.value)}
                required
                className="w-full bg-gray-600 text-white rounded px-3 py-2 text-sm border border-gray-500 focus:outline-none focus:border-blue-500"
              >
                <option value="">Select a model...</option>
                {models.map((m) => (
                  <option key={m.id} value={m.id}>{m.id}</option>
                ))}
              </select>
            ) : (
              <input
                type="text"
                value={newModelId}
                onChange={(e) => setNewModelId(e.target.value)}
                required
                placeholder="gpt-4o-mini"
                className="w-full bg-gray-600 text-white rounded px-3 py-2 text-sm border border-gray-500 focus:outline-none focus:border-blue-500 placeholder-gray-400"
              />
            )}
          </div>
          <button
            type="submit"
            disabled={submitting}
            className="px-4 py-2 bg-blue-600 text-white rounded-lg text-sm hover:bg-blue-500 disabled:opacity-50 transition-colors"
          >
            {submitting ? 'Creating...' : 'Create Alias'}
          </button>
        </form>
      )}

      {loading ? (
        <p className="text-gray-400 text-sm">Loading...</p>
      ) : aliases.length === 0 ? (
        <p className="text-gray-400 text-sm">No model aliases yet.</p>
      ) : (
        <div className="space-y-2">
          {aliases.map((a) => (
            <div key={a.alias} className="bg-gray-700 rounded-lg p-4 flex items-center justify-between">
              <div>
                <p className="text-white font-medium font-mono">
                  <span className="text-blue-300">{a.alias}</span>
                  <span className="text-gray-400 mx-2">→</span>
                  <span className="text-green-300">{a.model_id}</span>
                </p>
                <p className="text-gray-400 text-xs mt-0.5">
                  {a.user_id ? 'Personal alias' : 'Global alias'}
                </p>
              </div>
              <button
                onClick={() => handleDelete(a.alias)}
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
