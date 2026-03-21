import { useState, useEffect } from 'react';
import { getPlugins, registerPlugin, deletePlugin, testPlugin } from '../api/client';

interface Plugin {
  id: string;
  user_id: string;
  name: string;
  description: string;
  endpoint: string;
  method: string;
  headers: Record<string, string>;
  input_schema: any;
  enabled: boolean;
  shared: boolean;
}

const DEFAULT_SCHEMA = JSON.stringify({
  type: 'object',
  properties: {
    query: { type: 'string', description: 'Input query' },
  },
  required: ['query'],
}, null, 2);

export default function PluginManager() {
  const [plugins, setPlugins] = useState<Plugin[]>([]);
  const [showForm, setShowForm] = useState(false);
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [endpoint, setEndpoint] = useState('');
  const [method, setMethod] = useState('POST');
  const [schemaText, setSchemaText] = useState(DEFAULT_SCHEMA);
  const [shared, setShared] = useState(false);
  const [schemaError, setSchemaError] = useState('');
  const [saving, setSaving] = useState(false);
  const [testInput, setTestInput] = useState<Record<string, string>>({});
  const [testResult, setTestResult] = useState<Record<string, string>>({});
  const [testing, setTesting] = useState<Record<string, boolean>>({});

  const load = async () => {
    try {
      const data = await getPlugins();
      setPlugins(data || []);
    } catch {}
  };

  useEffect(() => { load(); }, []);

  const handleRegister = async () => {
    setSchemaError('');
    let parsedSchema;
    try {
      parsedSchema = JSON.parse(schemaText);
    } catch {
      setSchemaError('Invalid JSON schema');
      return;
    }
    if (!name.trim() || !description.trim() || !endpoint.trim()) {
      setSchemaError('Name, description, and endpoint are required.');
      return;
    }
    setSaving(true);
    try {
      await registerPlugin({
        name: name.trim(),
        description: description.trim(),
        endpoint: endpoint.trim(),
        method,
        input_schema: parsedSchema,
        shared,
      });
      setName(''); setDescription(''); setEndpoint(''); setSchemaText(DEFAULT_SCHEMA); setShared(false);
      setShowForm(false);
      await load();
    } catch (e: any) {
      setSchemaError(e?.response?.data?.error || 'Failed to register plugin');
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async (id: string) => {
    if (!confirm('Delete this plugin?')) return;
    try {
      await deletePlugin(id);
      await load();
    } catch {}
  };

  const handleTest = async (p: Plugin) => {
    setTesting(prev => ({ ...prev, [p.id]: true }));
    setTestResult(prev => ({ ...prev, [p.id]: '' }));
    try {
      let input: any = {};
      try { input = JSON.parse(testInput[p.id] || '{}'); } catch {}
      const result = await testPlugin(p.id, input);
      setTestResult(prev => ({ ...prev, [p.id]: JSON.stringify(result, null, 2) }));
    } catch (e: any) {
      setTestResult(prev => ({ ...prev, [p.id]: e?.response?.data?.error || String(e) }));
    } finally {
      setTesting(prev => ({ ...prev, [p.id]: false }));
    }
  };

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>Plugins ({plugins.length})</h3>
        <button
          onClick={() => setShowForm(!showForm)}
          className="text-xs px-3 py-1.5 rounded"
          style={{ background: 'var(--accent-color)', color: 'white' }}
        >
          {showForm ? 'Cancel' : '+ Register Plugin'}
        </button>
      </div>

      {showForm && (
        <div className="rounded-lg p-4 space-y-3" style={{ background: 'var(--bg-tertiary)' }}>
          <h4 className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>Register New Plugin</h4>

          <input
            value={name}
            onChange={e => setName(e.target.value)}
            placeholder="Plugin name (e.g. weather)"
            className="w-full px-3 py-2 rounded text-sm"
            style={{ background: 'var(--bg-primary)', color: 'var(--text-primary)', border: '1px solid var(--border-color)' }}
          />
          <input
            value={description}
            onChange={e => setDescription(e.target.value)}
            placeholder="Description (shown to AI)"
            className="w-full px-3 py-2 rounded text-sm"
            style={{ background: 'var(--bg-primary)', color: 'var(--text-primary)', border: '1px solid var(--border-color)' }}
          />
          <div className="flex gap-2">
            <input
              value={endpoint}
              onChange={e => setEndpoint(e.target.value)}
              placeholder="https://api.example.com/tool"
              className="flex-1 px-3 py-2 rounded text-sm"
              style={{ background: 'var(--bg-primary)', color: 'var(--text-primary)', border: '1px solid var(--border-color)' }}
            />
            <select
              value={method}
              onChange={e => setMethod(e.target.value)}
              className="px-3 py-2 rounded text-sm"
              style={{ background: 'var(--bg-primary)', color: 'var(--text-primary)', border: '1px solid var(--border-color)' }}
            >
              <option>POST</option>
              <option>GET</option>
              <option>PUT</option>
            </select>
          </div>

          <div>
            <label className="block text-xs mb-1" style={{ color: 'var(--text-secondary)' }}>JSON Input Schema</label>
            <textarea
              value={schemaText}
              onChange={e => setSchemaText(e.target.value)}
              rows={6}
              className="w-full px-3 py-2 rounded text-xs font-mono resize-y"
              style={{ background: 'var(--bg-primary)', color: 'var(--text-primary)', border: '1px solid var(--border-color)' }}
            />
          </div>

          <label className="flex items-center gap-2 text-sm cursor-pointer" style={{ color: 'var(--text-secondary)' }}>
            <input type="checkbox" checked={shared} onChange={e => setShared(e.target.checked)} />
            Share with all users
          </label>

          {schemaError && <p className="text-red-400 text-sm">{schemaError}</p>}

          <button
            onClick={handleRegister}
            disabled={saving}
            className="px-4 py-2 rounded text-sm font-medium disabled:opacity-50"
            style={{ background: 'var(--accent-color)', color: 'white' }}
          >
            {saving ? 'Registering...' : 'Register Plugin'}
          </button>
        </div>
      )}

      {plugins.length === 0 ? (
        <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>No plugins registered yet.</p>
      ) : (
        <div className="space-y-3">
          {plugins.map(p => (
            <div key={p.id} className="rounded-lg p-3 space-y-2"
              style={{ background: 'var(--bg-tertiary)', border: '1px solid var(--border-color)' }}>
              <div className="flex items-center justify-between">
                <div>
                  <div className="flex items-center gap-2">
                    <span className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>{p.name}</span>
                    <span className="text-xs px-1.5 py-0.5 rounded"
                      style={{ background: p.enabled ? '#22c55e20' : '#ef444420', color: p.enabled ? '#22c55e' : '#ef4444' }}>
                      {p.enabled ? 'enabled' : 'disabled'}
                    </span>
                    {p.shared && <span className="text-xs px-1.5 py-0.5 rounded" style={{ background: 'var(--bg-primary)', color: 'var(--text-secondary)' }}>shared</span>}
                  </div>
                  <p className="text-xs mt-0.5" style={{ color: 'var(--text-secondary)' }}>{p.description}</p>
                  <p className="text-xs font-mono mt-0.5" style={{ color: 'var(--text-secondary)' }}>{p.method} {p.endpoint}</p>
                </div>
                <button
                  onClick={() => handleDelete(p.id)}
                  className="text-red-400 hover:text-red-300 text-xs px-2 py-1 rounded"
                >Delete</button>
              </div>

              <div className="space-y-1">
                <label className="text-xs" style={{ color: 'var(--text-secondary)' }}>Test input (JSON):</label>
                <div className="flex gap-2">
                  <input
                    value={testInput[p.id] || ''}
                    onChange={e => setTestInput(prev => ({ ...prev, [p.id]: e.target.value }))}
                    placeholder='{"query": "test"}'
                    className="flex-1 px-2 py-1 text-xs font-mono rounded"
                    style={{ background: 'var(--bg-primary)', color: 'var(--text-primary)', border: '1px solid var(--border-color)' }}
                  />
                  <button
                    onClick={() => handleTest(p)}
                    disabled={testing[p.id]}
                    className="text-xs px-3 py-1 rounded disabled:opacity-50"
                    style={{ background: 'var(--bg-primary)', color: 'var(--text-primary)', border: '1px solid var(--border-color)' }}
                  >
                    {testing[p.id] ? '...' : 'Test'}
                  </button>
                </div>
                {testResult[p.id] && (
                  <pre className="text-xs p-2 rounded overflow-auto max-h-24"
                    style={{ background: 'var(--bg-primary)', color: 'var(--text-primary)' }}>
                    {testResult[p.id]}
                  </pre>
                )}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
