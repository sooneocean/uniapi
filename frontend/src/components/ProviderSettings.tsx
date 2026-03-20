import { useState, useEffect } from 'react';
import { getProviders, addProvider, deleteProvider, getOAuthProviders, getOAuthAccounts, unbindAccount, reauthAccount, getProviderTemplates } from '../api/client';
import SessionTokenDialog from './SessionTokenDialog';

interface Provider {
  id: string;
  type: string;
  label: string;
  models: string[];
  enabled: boolean;
  managed_by_config?: boolean;
  base_url?: string;
}

interface OAuthProvider {
  name: string;
  display_name: string;
  supports_session_token: boolean;
  supports_oauth: boolean;
}

interface OAuthAccount {
  id: string;
  provider: string;
  label: string;
  auth_type: string;
  needs_reauth: boolean;
  owner_user_id: string;
}

interface ProviderTemplate {
  name: string;
  display_name: string;
  type: string;
  base_url: string;
  default_models: string[];
  description: string;
  api_key_url: string;
}

export default function ProviderSettings() {
  const [providers, setProviders] = useState<Provider[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  // OAuth/binding state
  const [oauthProviders, setOAuthProviders] = useState<OAuthProvider[]>([]);
  const [oauthAccounts, setOAuthAccounts] = useState<OAuthAccount[]>([]);
  const [oauthLoading, setOAuthLoading] = useState(true);
  const [sessionDialog, setSessionDialog] = useState<{ provider: string; displayName: string } | null>(null);

  // Template state
  const [templates, setTemplates] = useState<ProviderTemplate[]>([]);
  const [selectedTemplate, setSelectedTemplate] = useState<ProviderTemplate | null>(null);
  const [showCustomForm, setShowCustomForm] = useState(false);

  // Template quick-add form
  const [tplApiKey, setTplApiKey] = useState('');
  const [tplLabel, setTplLabel] = useState('');
  const [tplSubmitting, setTplSubmitting] = useState(false);

  // Custom provider form
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

  const loadOAuth = async () => {
    try {
      setOAuthLoading(true);
      const [provData, accData] = await Promise.all([getOAuthProviders(), getOAuthAccounts()]);
      setOAuthProviders(Array.isArray(provData) ? provData : provData.providers ?? []);
      setOAuthAccounts(Array.isArray(accData) ? accData : accData.accounts ?? []);
    } catch {
      // silently fail — OAuth may not be configured
    } finally {
      setOAuthLoading(false);
    }
  };

  const loadTemplates = async () => {
    try {
      const data = await getProviderTemplates();
      setTemplates(Array.isArray(data) ? data : []);
    } catch {
      // silently fail
    }
  };

  useEffect(() => { load(); loadOAuth(); loadTemplates(); }, []);

  const handleOAuthUnbind = async (id: string) => {
    if (!confirm('Unbind this account?')) return;
    try {
      await unbindAccount(id);
      await loadOAuth();
    } catch {
      setError('Failed to unbind account');
    }
  };

  const handleOAuthReauth = async (id: string) => {
    try {
      await reauthAccount(id);
      await loadOAuth();
    } catch {
      setError('Failed to reauth account');
    }
  };

  const handleTemplateAdd = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedTemplate) return;
    setTplSubmitting(true);
    setError('');
    try {
      const payload: any = {
        provider: selectedTemplate.name,
        label: tplLabel || selectedTemplate.display_name,
        api_key: tplApiKey || 'ollama',
        models: selectedTemplate.default_models,
      };
      if (selectedTemplate.type === 'openai_compatible') {
        payload.base_url = selectedTemplate.base_url;
      }
      await addProvider(payload);
      setSelectedTemplate(null);
      setTplApiKey('');
      setTplLabel('');
      await load();
    } catch {
      setError('Failed to add provider');
    } finally {
      setTplSubmitting(false);
    }
  };

  const handleAdd = async (e: React.FormEvent) => {
    e.preventDefault();
    setSubmitting(true);
    setError('');
    try {
      const payload: any = {
        provider: formType,
        label: formLabel,
        api_key: formApiKey,
        models: formModels.split(',').map((m) => m.trim()).filter(Boolean),
      };
      if (formType === 'openai_compatible' && formBaseUrl) {
        payload.base_url = formBaseUrl;
      }
      await addProvider(payload);
      setShowCustomForm(false);
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

  const sharedAccounts = oauthAccounts.filter((a) => a.owner_user_id === '');

  return (
    <div className="text-white">
      {/* Account Binding section (admin only) */}
      <div className="mb-6">
        <h2 className="text-lg font-semibold mb-3">Account Binding</h2>
        {oauthLoading ? (
          <p className="text-gray-400 text-sm">Loading...</p>
        ) : oauthProviders.length === 0 ? (
          <p className="text-gray-400 text-sm">No OAuth providers configured.</p>
        ) : (
          <div className="space-y-3 mb-4">
            {oauthProviders.map((prov) => (
              <div key={prov.name} className="bg-gray-700 rounded-lg p-4 flex items-center justify-between">
                <span className="text-white font-medium">{prov.display_name}</span>
                <div className="flex gap-2">
                  {prov.supports_session_token && (
                    <button
                      onClick={() => setSessionDialog({ provider: prov.name, displayName: prov.display_name })}
                      className="px-3 py-1.5 text-sm bg-blue-600 text-white rounded hover:bg-blue-500 transition-colors"
                    >
                      Paste Session Token
                    </button>
                  )}
                  {prov.supports_oauth && (
                    <button
                      onClick={() => window.open(`/api/oauth/bind/${prov.name}/authorize?shared=true`, '_blank', 'width=600,height=700')}
                      className="px-3 py-1.5 text-sm bg-green-700 text-white rounded hover:bg-green-600 transition-colors"
                    >
                      OAuth Connect
                    </button>
                  )}
                </div>
              </div>
            ))}
          </div>
        )}

        {sharedAccounts.length > 0 && (
          <div>
            <h3 className="text-sm font-medium text-gray-300 mb-2">Bound Shared Accounts</h3>
            <div className="space-y-2">
              {sharedAccounts.map((acc) => (
                <div key={acc.id} className="bg-gray-700 rounded-lg p-3 flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <span className="text-white text-sm">{acc.label || acc.provider}</span>
                    <span className="text-xs bg-gray-600 text-gray-300 px-2 py-0.5 rounded">{acc.provider}</span>
                    <span className="text-xs bg-gray-600 text-gray-400 px-2 py-0.5 rounded">{acc.auth_type}</span>
                    {acc.needs_reauth ? (
                      <span className="text-xs bg-yellow-900 text-yellow-300 px-2 py-0.5 rounded">needs reauth</span>
                    ) : (
                      <span className="text-xs bg-green-900 text-green-300 px-2 py-0.5 rounded">normal</span>
                    )}
                  </div>
                  <div className="flex gap-2">
                    {acc.needs_reauth && (
                      <button
                        onClick={() => handleOAuthReauth(acc.id)}
                        className="px-2 py-1 text-xs bg-yellow-700 text-white rounded hover:bg-yellow-600 transition-colors"
                      >
                        Reauth
                      </button>
                    )}
                    <button
                      onClick={() => handleOAuthUnbind(acc.id)}
                      className="px-2 py-1 text-xs bg-red-800 text-red-200 rounded hover:bg-red-700 transition-colors"
                    >
                      Unbind
                    </button>
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>

      <hr className="border-gray-700 mb-6" />

      <div className="mb-6">
        <h2 className="text-lg font-semibold mb-3">Add Provider</h2>

        {error && <p className="text-red-400 text-sm mb-3">{error}</p>}

        {/* Template grid */}
        {templates.length > 0 && !selectedTemplate && !showCustomForm && (
          <div className="grid grid-cols-2 gap-3 mb-4">
            {templates.map((tpl) => (
              <button
                key={tpl.name}
                onClick={() => { setSelectedTemplate(tpl); setTplLabel(tpl.display_name); setTplApiKey(''); }}
                className="bg-gray-700 hover:bg-gray-600 rounded-lg p-4 text-left transition-colors border border-gray-600 hover:border-blue-500"
              >
                <div className="font-medium text-white text-sm mb-1">{tpl.display_name}</div>
                <div className="text-gray-400 text-xs">{tpl.description}</div>
              </button>
            ))}
            <button
              onClick={() => setShowCustomForm(true)}
              className="bg-gray-700 hover:bg-gray-600 rounded-lg p-4 text-left transition-colors border border-dashed border-gray-500 hover:border-blue-500"
            >
              <div className="font-medium text-white text-sm mb-1">Custom Provider</div>
              <div className="text-gray-400 text-xs">Manual base URL and model names</div>
            </button>
          </div>
        )}

        {/* Template quick-add form */}
        {selectedTemplate && (
          <form onSubmit={handleTemplateAdd} className="bg-gray-700 rounded-lg p-4 mb-4 space-y-3">
            <div className="flex items-center justify-between mb-1">
              <span className="font-medium text-white">{selectedTemplate.display_name}</span>
              <button
                type="button"
                onClick={() => { setSelectedTemplate(null); setTplApiKey(''); setTplLabel(''); }}
                className="text-gray-400 hover:text-white text-sm"
              >
                Back
              </button>
            </div>
            <div className="text-xs text-gray-400 mb-2">
              Models: {selectedTemplate.default_models.join(', ')}
            </div>
            <div>
              <label className="block text-sm text-gray-300 mb-1">Label</label>
              <input
                type="text"
                value={tplLabel}
                onChange={(e) => setTplLabel(e.target.value)}
                required
                placeholder={selectedTemplate.display_name}
                className="w-full bg-gray-600 text-white rounded px-3 py-2 text-sm border border-gray-500 focus:outline-none focus:border-blue-500 placeholder-gray-400"
              />
            </div>
            {selectedTemplate.name !== 'ollama' && (
              <div>
                <label className="block text-sm text-gray-300 mb-1">
                  API Key
                  {selectedTemplate.api_key_url && (
                    <a
                      href={selectedTemplate.api_key_url}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="ml-2 text-blue-400 hover:text-blue-300 text-xs"
                    >
                      Get API key
                    </a>
                  )}
                </label>
                <input
                  type="password"
                  value={tplApiKey}
                  onChange={(e) => setTplApiKey(e.target.value)}
                  required
                  placeholder="Paste your API key"
                  className="w-full bg-gray-600 text-white rounded px-3 py-2 text-sm border border-gray-500 focus:outline-none focus:border-blue-500 placeholder-gray-400"
                />
              </div>
            )}
            {selectedTemplate.name === 'ollama' && (
              <p className="text-xs text-gray-400">No API key required. Make sure Ollama is running at {selectedTemplate.base_url}.</p>
            )}
            <button
              type="submit"
              disabled={tplSubmitting}
              className="px-4 py-2 bg-blue-600 text-white rounded-lg text-sm hover:bg-blue-500 disabled:opacity-50 transition-colors"
            >
              {tplSubmitting ? 'Adding...' : 'Add Provider'}
            </button>
          </form>
        )}

        {/* Custom provider form */}
        {showCustomForm && (
          <form onSubmit={handleAdd} className="bg-gray-700 rounded-lg p-4 mb-4 space-y-3">
            <div className="flex items-center justify-between mb-1">
              <span className="font-medium text-white">Custom Provider</span>
              <button
                type="button"
                onClick={() => setShowCustomForm(false)}
                className="text-gray-400 hover:text-white text-sm"
              >
                Back
              </button>
            </div>
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
      </div>

      <hr className="border-gray-700 mb-6" />

      <div>
        <h2 className="text-lg font-semibold mb-4">Configured Providers</h2>
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

      {sessionDialog && (
        <SessionTokenDialog
          provider={sessionDialog.provider}
          displayName={sessionDialog.displayName}
          shared={true}
          onClose={() => setSessionDialog(null)}
          onSuccess={() => { setSessionDialog(null); loadOAuth(); }}
        />
      )}
    </div>
  );
}
