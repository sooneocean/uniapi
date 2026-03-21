import { useState, useEffect } from 'react';
import { getAPIKeys, sendPlaygroundRequest } from '../api/client';

const ENDPOINTS = [
  { method: 'POST', path: '/v1/chat/completions', label: 'POST /v1/chat/completions' },
  { method: 'GET', path: '/v1/models', label: 'GET /v1/models' },
  { method: 'POST', path: '/v1/compare', label: 'POST /v1/compare' },
];

const TEMPLATES: Record<string, any> = {
  'POST /v1/chat/completions': {
    model: 'gpt-4o',
    messages: [{ role: 'user', content: 'Hello!' }],
    stream: false,
  },
  'GET /v1/models': null,
  'POST /v1/compare': {
    prompt: 'Explain quantum computing in one sentence.',
    models: ['gpt-4o', 'claude-3-5-sonnet-20241022'],
    system_prompt: '',
  },
};

interface PlaygroundResult {
  status: number;
  statusText: string;
  elapsed: number;
  body: string;
  headers: Record<string, string>;
}

interface Props {
  onBack: () => void;
}

export default function APIPlayground({ onBack }: Props) {
  const [selectedEndpointIdx, setSelectedEndpointIdx] = useState(0);
  const [apiKey, setApiKey] = useState('');
  const [requestBody, setRequestBody] = useState('');
  const [result, setResult] = useState<PlaygroundResult | null>(null);
  const [loading, setLoading] = useState(false);
  const [copied, setCopied] = useState(false);

  const endpoint = ENDPOINTS[selectedEndpointIdx];

  // Load API key on mount
  useEffect(() => {
    getAPIKeys().then((keys: any[]) => {
      if (keys && keys.length > 0) {
        // keys may have a 'key' or 'token' field
        const k = keys[0];
        setApiKey(k.key || k.token || '');
      }
    }).catch(() => {});
  }, []);

  // Set template body when endpoint changes
  useEffect(() => {
    const tmpl = TEMPLATES[endpoint.label];
    if (tmpl) {
      setRequestBody(JSON.stringify(tmpl, null, 2));
    } else {
      setRequestBody('');
    }
    setResult(null);
  }, [selectedEndpointIdx]);

  const handleFormatBody = () => {
    if (!requestBody.trim()) return;
    try {
      const parsed = JSON.parse(requestBody);
      setRequestBody(JSON.stringify(parsed, null, 2));
    } catch {
      // leave as-is if invalid JSON
    }
  };

  const handleSend = async () => {
    setLoading(true);
    setResult(null);
    try {
      let body: any = undefined;
      if (endpoint.method !== 'GET' && requestBody.trim()) {
        try {
          body = JSON.parse(requestBody);
        } catch {
          alert('Invalid JSON in request body');
          setLoading(false);
          return;
        }
      }
      const res = await sendPlaygroundRequest(endpoint.method, endpoint.path, apiKey, body);
      setResult(res);
    } catch (e: any) {
      setResult({
        status: 0,
        statusText: 'Network Error',
        elapsed: 0,
        body: e?.message || 'Request failed',
        headers: {},
      });
    } finally {
      setLoading(false);
    }
  };

  const getCurl = () => {
    const baseUrl = window.location.origin;
    const url = `${baseUrl}${endpoint.path}`;
    const parts = [`curl -X ${endpoint.method} '${url}'`];
    if (apiKey) parts.push(`  -H 'Authorization: Bearer ${apiKey}'`);
    if (endpoint.method !== 'GET' && requestBody.trim()) {
      parts.push(`  -H 'Content-Type: application/json'`);
      // Inline single-line JSON for brevity
      let bodyStr = requestBody;
      try {
        bodyStr = JSON.stringify(JSON.parse(requestBody));
      } catch {}
      parts.push(`  -d '${bodyStr}'`);
    }
    return parts.join(' \\\n');
  };

  const handleCopyCurl = () => {
    navigator.clipboard.writeText(getCurl()).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    });
  };

  const prettyBody = (() => {
    if (!result) return '';
    try {
      return JSON.stringify(JSON.parse(result.body), null, 2);
    } catch {
      return result.body;
    }
  })();

  const statusColor = result
    ? result.status >= 200 && result.status < 300
      ? 'text-green-400'
      : 'text-red-400'
    : '';

  return (
    <div className="flex flex-col h-screen w-screen overflow-hidden" style={{ background: 'var(--bg-primary)', color: 'var(--text-primary)' }}>
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-gray-700" style={{ background: 'var(--bg-secondary)' }}>
        <div className="flex items-center gap-3">
          <button
            onClick={onBack}
            className="text-gray-400 hover:text-white transition-colors px-2 py-1 rounded hover:bg-gray-700"
          >
            ← Back
          </button>
          <h1 className="text-white font-semibold text-lg">API Playground</h1>
        </div>
      </div>

      {/* Content */}
      <div className="flex-1 overflow-y-auto p-4 md:p-6 max-w-4xl mx-auto w-full">
        {/* Endpoint + API Key row */}
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-4">
          <div>
            <label className="block text-xs text-gray-400 mb-1">Endpoint</label>
            <select
              value={selectedEndpointIdx}
              onChange={(e) => setSelectedEndpointIdx(Number(e.target.value))}
              className="w-full bg-gray-700 text-gray-100 border border-gray-600 rounded-lg px-3 py-2 text-sm focus:outline-none focus:border-blue-500"
            >
              {ENDPOINTS.map((ep, i) => (
                <option key={i} value={i}>{ep.label}</option>
              ))}
            </select>
          </div>
          <div>
            <label className="block text-xs text-gray-400 mb-1">API Key</label>
            <input
              type="text"
              value={apiKey}
              onChange={(e) => setApiKey(e.target.value)}
              placeholder="uniapi-sk-xxxxxxxxx"
              className="w-full bg-gray-700 text-gray-100 border border-gray-600 rounded-lg px-3 py-2 text-sm focus:outline-none focus:border-blue-500 font-mono"
            />
          </div>
        </div>

        {/* Template button */}
        <div className="mb-2 flex items-center gap-2">
          <button
            onClick={() => {
              const tmpl = TEMPLATES[endpoint.label];
              if (tmpl) setRequestBody(JSON.stringify(tmpl, null, 2));
              else setRequestBody('');
            }}
            className="text-xs px-3 py-1 bg-gray-700 hover:bg-gray-600 text-gray-300 rounded-lg border border-gray-600 transition-colors"
          >
            Fill Template
          </button>
        </div>

        {/* Request Body */}
        {endpoint.method !== 'GET' && (
          <div className="mb-4">
            <label className="block text-xs text-gray-400 mb-1">Request Body</label>
            <textarea
              value={requestBody}
              onChange={(e) => setRequestBody(e.target.value)}
              onBlur={handleFormatBody}
              rows={12}
              className="w-full bg-gray-800 text-gray-100 border border-gray-600 rounded-lg px-3 py-2 text-sm font-mono focus:outline-none focus:border-blue-500 resize-y"
              placeholder="{}"
            />
          </div>
        )}

        {/* Send button */}
        <button
          onClick={handleSend}
          disabled={loading}
          className="w-full py-2 bg-blue-600 hover:bg-blue-500 disabled:opacity-50 disabled:cursor-not-allowed text-white font-medium rounded-lg transition-colors mb-6"
        >
          {loading ? 'Sending...' : 'Send Request'}
        </button>

        {/* Response */}
        {result && (
          <div className="mb-6">
            <div className="flex items-center gap-3 mb-2">
              <span className="text-xs text-gray-400">Response:</span>
              <span className={`text-sm font-semibold ${statusColor}`}>
                {result.status} {result.statusText}
              </span>
              <span className="text-xs text-gray-500">({result.elapsed}ms)</span>
            </div>
            <textarea
              readOnly
              value={prettyBody}
              rows={16}
              className="w-full bg-gray-800 text-gray-100 border border-gray-600 rounded-lg px-3 py-2 text-sm font-mono focus:outline-none resize-y"
            />
          </div>
        )}

        {/* cURL */}
        <div>
          <div className="flex items-center gap-2 mb-1">
            <span className="text-xs text-gray-400">cURL:</span>
            <button
              onClick={handleCopyCurl}
              className="text-xs px-2 py-0.5 bg-gray-700 hover:bg-gray-600 text-gray-300 rounded border border-gray-600 transition-colors"
            >
              {copied ? 'Copied!' : 'Copy cURL'}
            </button>
          </div>
          <pre className="bg-gray-800 border border-gray-600 rounded-lg px-3 py-2 text-xs text-gray-300 font-mono overflow-x-auto whitespace-pre-wrap break-all">
            {getCurl()}
          </pre>
        </div>
      </div>
    </div>
  );
}
