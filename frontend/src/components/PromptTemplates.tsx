import { useState, useEffect } from 'react';
import { getTemplates, createTemplate, updateTemplate, deleteTemplate, useTemplate } from '../api/client';

interface Template {
  id: string;
  user_id: string;
  title: string;
  description: string;
  system_prompt: string;
  user_prompt: string;
  tags: string;
  shared: boolean;
  use_count: number;
  created_at: string;
}

interface Props {
  onApply?: (systemPrompt: string, userPrompt: string) => void;
  compact?: boolean; // compact picker mode (dropdown)
}

export default function PromptTemplates({ onApply, compact }: Props) {
  const [templates, setTemplates] = useState<Template[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [search, setSearch] = useState('');
  const [showCreate, setShowCreate] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);

  // Form state
  const [formTitle, setFormTitle] = useState('');
  const [formDescription, setFormDescription] = useState('');
  const [formSystemPrompt, setFormSystemPrompt] = useState('');
  const [formUserPrompt, setFormUserPrompt] = useState('');
  const [formTags, setFormTags] = useState('');
  const [formShared, setFormShared] = useState(false);
  const [submitting, setSubmitting] = useState(false);

  const load = async () => {
    try {
      setLoading(true);
      const data = await getTemplates();
      setTemplates(Array.isArray(data) ? data : []);
    } catch {
      setError('Failed to load templates');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, []);

  const resetForm = () => {
    setFormTitle('');
    setFormDescription('');
    setFormSystemPrompt('');
    setFormUserPrompt('');
    setFormTags('');
    setFormShared(false);
  };

  const startEdit = (t: Template) => {
    setEditingId(t.id);
    setFormTitle(t.title);
    setFormDescription(t.description);
    setFormSystemPrompt(t.system_prompt);
    setFormUserPrompt(t.user_prompt);
    setFormTags(t.tags);
    setFormShared(t.shared);
    setShowCreate(true);
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSubmitting(true);
    setError('');
    const payload = {
      title: formTitle,
      description: formDescription,
      system_prompt: formSystemPrompt,
      user_prompt: formUserPrompt,
      tags: formTags,
      shared: formShared,
    };
    try {
      if (editingId) {
        await updateTemplate(editingId, payload);
      } else {
        await createTemplate(payload);
      }
      setShowCreate(false);
      setEditingId(null);
      resetForm();
      await load();
    } catch {
      setError('Failed to save template');
    } finally {
      setSubmitting(false);
    }
  };

  const handleDelete = async (id: string) => {
    if (!confirm('Delete this template?')) return;
    try {
      await deleteTemplate(id);
      await load();
    } catch {
      setError('Failed to delete template');
    }
  };

  const handleUse = async (t: Template) => {
    try {
      await useTemplate(t.id);
      if (onApply) {
        onApply(t.system_prompt, t.user_prompt);
      }
      // Update local use count
      setTemplates(prev => prev.map(tmpl => tmpl.id === t.id ? { ...tmpl, use_count: tmpl.use_count + 1 } : tmpl));
    } catch {
      // ignore
    }
  };

  const filtered = templates.filter(t => {
    if (!search) return true;
    const s = search.toLowerCase();
    return (
      t.title.toLowerCase().includes(s) ||
      t.description.toLowerCase().includes(s) ||
      t.tags.toLowerCase().includes(s)
    );
  });

  if (compact) {
    return (
      <div className="space-y-1 max-h-80 overflow-y-auto">
        {loading ? (
          <p className="text-gray-400 text-xs p-2">Loading...</p>
        ) : filtered.length === 0 ? (
          <p className="text-gray-400 text-xs p-2">No templates found.</p>
        ) : (
          filtered.map(t => (
            <button
              key={t.id}
              onClick={() => handleUse(t)}
              className="w-full text-left px-3 py-2 rounded hover:bg-gray-600 transition-colors"
            >
              <div className="text-sm text-white font-medium">{t.title}</div>
              {t.description && <div className="text-xs text-gray-400 truncate">{t.description}</div>}
              {t.tags && (
                <div className="flex gap-1 mt-1 flex-wrap">
                  {t.tags.split(',').filter(Boolean).map(tag => (
                    <span key={tag} className="text-xs bg-gray-700 text-gray-300 px-1.5 py-0.5 rounded">{tag.trim()}</span>
                  ))}
                </div>
              )}
            </button>
          ))
        )}
      </div>
    );
  }

  return (
    <div className="text-white">
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-lg font-semibold">Prompt Templates</h2>
        <button
          onClick={() => { setShowCreate(!showCreate); setEditingId(null); resetForm(); }}
          className="px-3 py-1.5 bg-blue-600 text-white rounded-lg text-sm hover:bg-blue-500 transition-colors"
        >
          {showCreate && !editingId ? 'Cancel' : '+ New Template'}
        </button>
      </div>

      {error && <p className="text-red-400 text-sm mb-3">{error}</p>}

      {showCreate && (
        <form onSubmit={handleSubmit} className="bg-gray-700 rounded-lg p-4 mb-4 space-y-3">
          <h3 className="text-sm font-semibold text-gray-200">{editingId ? 'Edit Template' : 'New Template'}</h3>
          <div>
            <label className="block text-xs text-gray-400 mb-1">Title *</label>
            <input
              type="text"
              value={formTitle}
              onChange={(e) => setFormTitle(e.target.value)}
              required
              placeholder="Code Reviewer"
              className="w-full bg-gray-600 text-white rounded px-3 py-2 text-sm border border-gray-500 focus:outline-none focus:border-blue-500 placeholder-gray-400"
            />
          </div>
          <div>
            <label className="block text-xs text-gray-400 mb-1">Description</label>
            <input
              type="text"
              value={formDescription}
              onChange={(e) => setFormDescription(e.target.value)}
              placeholder="Brief description..."
              className="w-full bg-gray-600 text-white rounded px-3 py-2 text-sm border border-gray-500 focus:outline-none focus:border-blue-500 placeholder-gray-400"
            />
          </div>
          <div>
            <label className="block text-xs text-gray-400 mb-1">System Prompt *</label>
            <textarea
              value={formSystemPrompt}
              onChange={(e) => setFormSystemPrompt(e.target.value)}
              required
              rows={3}
              placeholder="You are a helpful code reviewer..."
              className="w-full bg-gray-600 text-white rounded px-3 py-2 text-sm border border-gray-500 focus:outline-none focus:border-blue-500 placeholder-gray-400 resize-y"
            />
          </div>
          <div>
            <label className="block text-xs text-gray-400 mb-1">Starter User Prompt</label>
            <textarea
              value={formUserPrompt}
              onChange={(e) => setFormUserPrompt(e.target.value)}
              rows={2}
              placeholder="Review this code:"
              className="w-full bg-gray-600 text-white rounded px-3 py-2 text-sm border border-gray-500 focus:outline-none focus:border-blue-500 placeholder-gray-400 resize-y"
            />
          </div>
          <div>
            <label className="block text-xs text-gray-400 mb-1">Tags (comma-separated)</label>
            <input
              type="text"
              value={formTags}
              onChange={(e) => setFormTags(e.target.value)}
              placeholder="code,review,programming"
              className="w-full bg-gray-600 text-white rounded px-3 py-2 text-sm border border-gray-500 focus:outline-none focus:border-blue-500 placeholder-gray-400"
            />
          </div>
          <label className="flex items-center gap-2 cursor-pointer">
            <input
              type="checkbox"
              checked={formShared}
              onChange={(e) => setFormShared(e.target.checked)}
              className="rounded"
            />
            <span className="text-sm text-gray-300">Share with other users</span>
          </label>
          <div className="flex gap-2">
            <button
              type="submit"
              disabled={submitting}
              className="px-4 py-2 bg-blue-600 text-white rounded-lg text-sm hover:bg-blue-500 disabled:opacity-50 transition-colors"
            >
              {submitting ? 'Saving...' : (editingId ? 'Update' : 'Create')}
            </button>
            <button
              type="button"
              onClick={() => { setShowCreate(false); setEditingId(null); resetForm(); }}
              className="px-4 py-2 bg-gray-600 text-white rounded-lg text-sm hover:bg-gray-500 transition-colors"
            >
              Cancel
            </button>
          </div>
        </form>
      )}

      <div className="mb-3">
        <input
          type="text"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="Search templates..."
          className="w-full bg-gray-700 text-white rounded px-3 py-2 text-sm border border-gray-600 focus:outline-none focus:border-blue-500 placeholder-gray-400"
        />
      </div>

      {loading ? (
        <p className="text-gray-400 text-sm">Loading...</p>
      ) : filtered.length === 0 ? (
        <p className="text-gray-400 text-sm">No templates found.</p>
      ) : (
        <div className="grid grid-cols-1 gap-3">
          {filtered.map((t) => (
            <div key={t.id} className="bg-gray-700 rounded-lg p-4">
              <div className="flex items-start justify-between gap-2">
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2 flex-wrap">
                    <span className="font-medium text-white">{t.title}</span>
                    {t.shared && (
                      <span className="text-xs bg-green-900 text-green-300 px-1.5 py-0.5 rounded">shared</span>
                    )}
                    <span className="text-xs text-gray-500">{t.use_count} uses</span>
                  </div>
                  {t.description && (
                    <p className="text-sm text-gray-400 mt-0.5">{t.description}</p>
                  )}
                  {t.tags && (
                    <div className="flex gap-1 mt-1.5 flex-wrap">
                      {t.tags.split(',').filter(Boolean).map(tag => (
                        <span key={tag} className="text-xs bg-gray-600 text-gray-300 px-1.5 py-0.5 rounded">{tag.trim()}</span>
                      ))}
                    </div>
                  )}
                  <p className="text-xs text-gray-500 mt-1.5 line-clamp-2">{t.system_prompt}</p>
                </div>
                <div className="flex gap-1 flex-shrink-0">
                  {onApply && (
                    <button
                      onClick={() => handleUse(t)}
                      className="px-2.5 py-1 bg-blue-700 text-blue-100 rounded text-xs hover:bg-blue-600 transition-colors"
                    >
                      Use
                    </button>
                  )}
                  <button
                    onClick={() => startEdit(t)}
                    className="px-2.5 py-1 bg-gray-600 text-gray-200 rounded text-xs hover:bg-gray-500 transition-colors"
                  >
                    Edit
                  </button>
                  <button
                    onClick={() => handleDelete(t.id)}
                    className="px-2.5 py-1 bg-red-800 text-red-200 rounded text-xs hover:bg-red-700 transition-colors"
                  >
                    Del
                  </button>
                </div>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
