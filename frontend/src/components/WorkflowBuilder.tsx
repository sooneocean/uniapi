import { useState, useEffect } from 'react';

interface Step {
  name: string;
  model: string;
  system_prompt: string;
  user_prompt: string;
  max_tokens: number;
}

interface Workflow {
  id: string;
  user_id: string;
  name: string;
  description: string;
  steps: Step[];
  shared: boolean;
  run_count: number;
}

interface StepResult {
  step_name: string;
  model: string;
  output: string;
  tokens_in: number;
  tokens_out: number;
  latency_ms: number;
}

interface RunResult {
  workflow_name: string;
  steps: StepResult[];
  final_output: string;
  total_cost: number;
}

const emptyStep = (): Step => ({
  name: 'Step',
  model: '',
  system_prompt: '',
  user_prompt: '{{input}}',
  max_tokens: 2048,
});

function insertPlaceholder(text: string, placeholder: string, selStart: number): string {
  return text.slice(0, selStart) + placeholder + text.slice(selStart);
}

export default function WorkflowBuilder() {
  const [workflows, setWorkflows] = useState<Workflow[]>([]);
  const [selected, setSelected] = useState<Workflow | null>(null);
  const [editing, setEditing] = useState(false);
  const [editWf, setEditWf] = useState<Partial<Workflow>>({ name: '', description: '', steps: [emptyStep()], shared: false });
  const [showRun, setShowRun] = useState(false);
  const [runInput, setRunInput] = useState('');
  const [running, setRunning] = useState(false);
  const [runResult, setRunResult] = useState<RunResult | null>(null);
  const [runError, setRunError] = useState('');
  const [expandedSteps, setExpandedSteps] = useState<Set<number>>(new Set());
  const [loading, setLoading] = useState(false);

  const load = async () => {
    try {
      const resp = await fetch('/api/workflows', { credentials: 'include' });
      if (resp.ok) setWorkflows(await resp.json());
    } catch {}
  };

  useEffect(() => { load(); }, []);

  const handleCreate = () => {
    setEditWf({ name: '', description: '', steps: [emptyStep()], shared: false });
    setSelected(null);
    setEditing(true);
  };

  const handleEdit = (wf: Workflow) => {
    setEditWf({ ...wf, steps: wf.steps ? [...wf.steps.map(s => ({ ...s }))] : [emptyStep()] });
    setSelected(wf);
    setEditing(true);
  };

  const handleSave = async () => {
    setLoading(true);
    try {
      const csrfMatch = document.cookie.match(/csrf_token=([^;]+)/);
      const csrf = csrfMatch ? csrfMatch[1] : '';
      const method = selected ? 'PUT' : 'POST';
      const url = selected ? `/api/workflows/${selected.id}` : '/api/workflows';
      const resp = await fetch(url, {
        method,
        credentials: 'include',
        headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrf },
        body: JSON.stringify(editWf),
      });
      if (resp.ok) {
        setEditing(false);
        await load();
      }
    } catch {}
    setLoading(false);
  };

  const handleDelete = async (wf: Workflow) => {
    if (!confirm(`Delete workflow "${wf.name}"?`)) return;
    const csrfMatch = document.cookie.match(/csrf_token=([^;]+)/);
    const csrf = csrfMatch ? csrfMatch[1] : '';
    await fetch(`/api/workflows/${wf.id}`, {
      method: 'DELETE', credentials: 'include',
      headers: { 'X-CSRF-Token': csrf },
    });
    await load();
    if (selected?.id === wf.id) setSelected(null);
  };

  const handleRun = async () => {
    if (!selected || !runInput.trim()) return;
    setRunning(true);
    setRunResult(null);
    setRunError('');
    setExpandedSteps(new Set());
    try {
      const csrfMatch = document.cookie.match(/csrf_token=([^;]+)/);
      const csrf = csrfMatch ? csrfMatch[1] : '';
      const resp = await fetch(`/api/workflows/${selected.id}/run`, {
        method: 'POST', credentials: 'include',
        headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrf },
        body: JSON.stringify({ input: runInput }),
      });
      if (resp.ok) {
        const result: RunResult = await resp.json();
        setRunResult(result);
        // Expand all steps by default
        setExpandedSteps(new Set(result.steps.map((_, i) => i)));
      } else {
        const err = await resp.json();
        setRunError(err.error || 'Run failed');
      }
    } catch (e: any) {
      setRunError(String(e));
    }
    setRunning(false);
  };

  const updateStep = (idx: number, field: keyof Step, value: string | number) => {
    setEditWf(prev => {
      const steps = [...(prev.steps || [])];
      steps[idx] = { ...steps[idx], [field]: value };
      return { ...prev, steps };
    });
  };

  const addStep = () => {
    setEditWf(prev => {
      const steps = [...(prev.steps || [])];
      const n = steps.length + 1;
      steps.push({ ...emptyStep(), name: `Step ${n}`, user_prompt: `{{step_${n - 1}}}` });
      return { ...prev, steps };
    });
  };

  const removeStep = (idx: number) => {
    setEditWf(prev => {
      const steps = [...(prev.steps || [])].filter((_, i) => i !== idx);
      return { ...prev, steps };
    });
  };

  const toggleExpand = (idx: number) => {
    setExpandedSteps(prev => {
      const next = new Set(prev);
      if (next.has(idx)) next.delete(idx); else next.add(idx);
      return next;
    });
  };

  const insertIntoPrompt = (stepIdx: number, placeholder: string) => {
    const ta = document.getElementById(`user-prompt-${stepIdx}`) as HTMLTextAreaElement | null;
    const pos = ta?.selectionStart ?? (editWf.steps?.[stepIdx]?.user_prompt?.length ?? 0);
    updateStep(stepIdx, 'user_prompt', insertPlaceholder(editWf.steps?.[stepIdx]?.user_prompt ?? '', placeholder, pos));
  };

  const inputStyle = {
    background: 'var(--bg-primary)',
    color: 'var(--text-primary)',
    border: '1px solid var(--border-color)',
  };

  return (
    <div className="flex h-full" style={{ minHeight: '500px' }}>
      {/* Sidebar */}
      <div className="w-56 flex-shrink-0 flex flex-col border-r" style={{ borderColor: 'var(--border-color)' }}>
        <div className="px-3 py-3 border-b flex items-center justify-between" style={{ borderColor: 'var(--border-color)' }}>
          <span className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>Workflows</span>
          <button
            onClick={handleCreate}
            className="text-xs px-2 py-1 rounded"
            style={{ background: 'var(--accent-color)', color: 'white' }}
          >+ New</button>
        </div>
        <div className="flex-1 overflow-y-auto">
          {workflows.map(wf => (
            <div
              key={wf.id}
              onClick={() => { setSelected(wf); setEditing(false); setRunResult(null); }}
              className="group flex items-center justify-between px-3 py-2 cursor-pointer"
              style={{
                background: selected?.id === wf.id ? 'var(--bg-tertiary)' : 'transparent',
                color: 'var(--text-primary)',
              }}
            >
              <div className="flex-1 min-w-0">
                <p className="text-sm truncate">{wf.name}</p>
                {wf.shared && <span className="text-xs" style={{ color: 'var(--text-secondary)' }}>shared</span>}
              </div>
              <button
                className="opacity-0 group-hover:opacity-100 text-red-400 text-xs ml-1"
                onClick={e => { e.stopPropagation(); handleDelete(wf); }}
              >✕</button>
            </div>
          ))}
          {workflows.length === 0 && (
            <p className="text-xs px-3 py-4" style={{ color: 'var(--text-secondary)' }}>No workflows yet</p>
          )}
        </div>
      </div>

      {/* Main area */}
      <div className="flex-1 overflow-y-auto px-4 py-4">
        {editing ? (
          <div className="space-y-4">
            <div className="flex items-center justify-between">
              <h2 className="text-base font-semibold" style={{ color: 'var(--text-primary)' }}>
                {selected ? 'Edit Workflow' : 'New Workflow'}
              </h2>
              <div className="flex gap-2">
                <button onClick={() => setEditing(false)} className="text-xs px-3 py-1.5 rounded" style={{ background: 'var(--bg-tertiary)', color: 'var(--text-secondary)' }}>Cancel</button>
                <button onClick={handleSave} disabled={loading} className="text-xs px-3 py-1.5 rounded" style={{ background: 'var(--accent-color)', color: 'white' }}>
                  {loading ? 'Saving...' : 'Save'}
                </button>
              </div>
            </div>

            <div className="space-y-2">
              <input
                value={editWf.name ?? ''}
                onChange={e => setEditWf(p => ({ ...p, name: e.target.value }))}
                placeholder="Workflow name"
                className="w-full px-3 py-2 text-sm rounded"
                style={inputStyle}
              />
              <input
                value={editWf.description ?? ''}
                onChange={e => setEditWf(p => ({ ...p, description: e.target.value }))}
                placeholder="Description (optional)"
                className="w-full px-3 py-2 text-sm rounded"
                style={inputStyle}
              />
              <label className="flex items-center gap-2 text-sm" style={{ color: 'var(--text-secondary)' }}>
                <input type="checkbox" checked={editWf.shared ?? false} onChange={e => setEditWf(p => ({ ...p, shared: e.target.checked }))} />
                Shared (visible to all users)
              </label>
            </div>

            <div className="space-y-3">
              {(editWf.steps ?? []).map((step, idx) => (
                <div key={idx} className="rounded-lg p-3 space-y-2" style={{ background: 'var(--bg-tertiary)', border: '1px solid var(--border-color)' }}>
                  <div className="flex items-center justify-between">
                    <span className="text-xs font-semibold uppercase tracking-wide" style={{ color: 'var(--text-secondary)' }}>Step {idx + 1}</span>
                    {(editWf.steps?.length ?? 0) > 1 && (
                      <button onClick={() => removeStep(idx)} className="text-xs text-red-400">Remove</button>
                    )}
                  </div>
                  <div className="grid grid-cols-2 gap-2">
                    <input
                      value={step.name}
                      onChange={e => updateStep(idx, 'name', e.target.value)}
                      placeholder="Step name"
                      className="px-2 py-1.5 text-xs rounded"
                      style={inputStyle}
                    />
                    <input
                      value={step.model}
                      onChange={e => updateStep(idx, 'model', e.target.value)}
                      placeholder="Model (e.g. gpt-4o)"
                      className="px-2 py-1.5 text-xs rounded"
                      style={inputStyle}
                    />
                  </div>
                  <textarea
                    value={step.system_prompt}
                    onChange={e => updateStep(idx, 'system_prompt', e.target.value)}
                    placeholder="System prompt (optional)"
                    rows={2}
                    className="w-full px-2 py-1.5 text-xs rounded resize-none"
                    style={inputStyle}
                  />
                  <div>
                    <div className="flex gap-1 mb-1 flex-wrap">
                      <span className="text-xs" style={{ color: 'var(--text-secondary)' }}>Insert:</span>
                      <button onClick={() => insertIntoPrompt(idx, '{{input}}')} className="text-xs px-1.5 py-0.5 rounded" style={{ background: 'var(--bg-primary)', color: 'var(--accent-color)', border: '1px solid var(--border-color)' }}>{'{{input}}'}</button>
                      {Array.from({ length: idx }, (_, i) => (
                        <button key={i} onClick={() => insertIntoPrompt(idx, `{{step_${i + 1}}}`)} className="text-xs px-1.5 py-0.5 rounded" style={{ background: 'var(--bg-primary)', color: 'var(--accent-color)', border: '1px solid var(--border-color)' }}>{`{{step_${i + 1}}}`}</button>
                      ))}
                    </div>
                    <textarea
                      id={`user-prompt-${idx}`}
                      value={step.user_prompt}
                      onChange={e => updateStep(idx, 'user_prompt', e.target.value)}
                      placeholder="User prompt — use {{input}} or {{step_N}}"
                      rows={3}
                      className="w-full px-2 py-1.5 text-xs rounded resize-none"
                      style={inputStyle}
                    />
                  </div>
                  <div className="flex items-center gap-2">
                    <span className="text-xs" style={{ color: 'var(--text-secondary)' }}>Max tokens:</span>
                    <input
                      type="number"
                      value={step.max_tokens}
                      onChange={e => updateStep(idx, 'max_tokens', parseInt(e.target.value) || 2048)}
                      className="w-24 px-2 py-1 text-xs rounded"
                      style={inputStyle}
                    />
                  </div>
                </div>
              ))}
              <button onClick={addStep} className="text-xs px-3 py-1.5 rounded w-full" style={{ background: 'var(--bg-tertiary)', color: 'var(--text-primary)', border: '1px dashed var(--border-color)' }}>
                + Add Step
              </button>
            </div>
          </div>
        ) : selected ? (
          <div className="space-y-4">
            <div className="flex items-center justify-between">
              <div>
                <h2 className="text-base font-semibold" style={{ color: 'var(--text-primary)' }}>{selected.name}</h2>
                {selected.description && <p className="text-xs mt-0.5" style={{ color: 'var(--text-secondary)' }}>{selected.description}</p>}
                <p className="text-xs mt-1" style={{ color: 'var(--text-secondary)' }}>{selected.steps?.length ?? 0} step(s) · Run {selected.run_count} time(s)</p>
              </div>
              <div className="flex gap-2">
                <button onClick={() => handleEdit(selected)} className="text-xs px-3 py-1.5 rounded" style={{ background: 'var(--bg-tertiary)', color: 'var(--text-primary)' }}>Edit</button>
                <button onClick={() => setShowRun(true)} className="text-xs px-3 py-1.5 rounded" style={{ background: 'var(--accent-color)', color: 'white' }}>Run</button>
              </div>
            </div>

            {/* Steps preview */}
            <div className="space-y-2">
              {selected.steps?.map((step, idx) => (
                <div key={idx} className="rounded-lg p-3" style={{ background: 'var(--bg-tertiary)', border: '1px solid var(--border-color)' }}>
                  <div className="flex items-center gap-2">
                    <span className="text-xs font-semibold" style={{ color: 'var(--accent-color)' }}>{idx + 1}</span>
                    <span className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>{step.name}</span>
                    <span className="text-xs px-1.5 py-0.5 rounded" style={{ background: 'var(--bg-primary)', color: 'var(--text-secondary)' }}>{step.model}</span>
                  </div>
                  <p className="text-xs mt-1 font-mono" style={{ color: 'var(--text-secondary)' }}>{step.user_prompt}</p>
                </div>
              ))}
            </div>

            {/* Run result */}
            {runResult && (
              <div className="space-y-2">
                <h3 className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>Last Run Results</h3>
                {runResult.steps.map((sr, idx) => (
                  <div key={idx} className="rounded-lg overflow-hidden" style={{ border: '1px solid var(--border-color)' }}>
                    <button
                      onClick={() => toggleExpand(idx)}
                      className="w-full flex items-center justify-between px-3 py-2 text-left"
                      style={{ background: 'var(--bg-tertiary)' }}
                    >
                      <div className="flex items-center gap-2">
                        <span className="text-xs font-semibold" style={{ color: 'var(--accent-color)' }}>{idx + 1}</span>
                        <span className="text-sm" style={{ color: 'var(--text-primary)' }}>{sr.step_name}</span>
                        <span className="text-xs" style={{ color: 'var(--text-secondary)' }}>{sr.model}</span>
                      </div>
                      <div className="flex items-center gap-3 text-xs" style={{ color: 'var(--text-secondary)' }}>
                        <span>{sr.latency_ms}ms</span>
                        <span>{sr.tokens_in + sr.tokens_out} tok</span>
                        <span>{expandedSteps.has(idx) ? '▲' : '▼'}</span>
                      </div>
                    </button>
                    {expandedSteps.has(idx) && (
                      <div className="px-3 py-2" style={{ background: 'var(--bg-primary)' }}>
                        <pre className="text-xs whitespace-pre-wrap" style={{ color: 'var(--text-primary)' }}>{sr.output}</pre>
                      </div>
                    )}
                  </div>
                ))}
                <div className="rounded-lg p-3" style={{ background: 'var(--bg-tertiary)', border: '1px solid var(--border-color)' }}>
                  <p className="text-xs font-semibold mb-1" style={{ color: 'var(--text-secondary)' }}>Final Output</p>
                  <pre className="text-sm whitespace-pre-wrap" style={{ color: 'var(--text-primary)' }}>{runResult.final_output}</pre>
                </div>
              </div>
            )}
          </div>
        ) : (
          <div className="flex items-center justify-center h-full" style={{ color: 'var(--text-secondary)' }}>
            <p className="text-sm">Select a workflow or create a new one</p>
          </div>
        )}
      </div>

      {/* Run dialog */}
      {showRun && selected && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60">
          <div className="rounded-xl shadow-2xl w-full max-w-lg mx-4 flex flex-col" style={{ background: 'var(--bg-secondary)', maxHeight: '80vh' }}>
            <div className="flex items-center justify-between px-4 py-3 border-b" style={{ borderColor: 'var(--border-color)' }}>
              <h3 className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>Run: {selected.name}</h3>
              <button onClick={() => { setShowRun(false); setRunResult(null); setRunError(''); }} style={{ color: 'var(--text-secondary)' }}>&times;</button>
            </div>
            <div className="flex-1 overflow-y-auto px-4 py-3 space-y-3">
              <textarea
                value={runInput}
                onChange={e => setRunInput(e.target.value)}
                placeholder="Enter input for the workflow..."
                rows={4}
                className="w-full px-3 py-2 text-sm rounded resize-none"
                style={{ background: 'var(--bg-primary)', color: 'var(--text-primary)', border: '1px solid var(--border-color)' }}
              />
              {runError && <p className="text-xs text-red-400">{runError}</p>}
              {running && (
                <div className="space-y-2">
                  {selected.steps?.map((step, idx) => (
                    <div key={idx} className="flex items-center gap-2 text-xs" style={{ color: 'var(--text-secondary)' }}>
                      <span className="animate-spin">⟳</span>
                      <span>{step.name}...</span>
                    </div>
                  ))}
                </div>
              )}
              {runResult && !running && (
                <div className="space-y-2">
                  {runResult.steps.map((sr, idx) => (
                    <div key={idx} className="rounded overflow-hidden text-xs" style={{ border: '1px solid var(--border-color)' }}>
                      <button
                        onClick={() => toggleExpand(idx)}
                        className="w-full flex items-center justify-between px-2 py-1.5"
                        style={{ background: 'var(--bg-tertiary)', color: 'var(--text-primary)' }}
                      >
                        <span>{sr.step_name}</span>
                        <span style={{ color: 'var(--text-secondary)' }}>{sr.latency_ms}ms · {expandedSteps.has(idx) ? '▲' : '▼'}</span>
                      </button>
                      {expandedSteps.has(idx) && (
                        <pre className="px-2 py-1.5 text-xs whitespace-pre-wrap" style={{ background: 'var(--bg-primary)', color: 'var(--text-primary)' }}>{sr.output}</pre>
                      )}
                    </div>
                  ))}
                  <div className="rounded p-2" style={{ background: 'var(--bg-tertiary)', border: '1px solid var(--border-color)' }}>
                    <p className="text-xs font-semibold mb-1" style={{ color: 'var(--text-secondary)' }}>Final Output</p>
                    <pre className="text-xs whitespace-pre-wrap" style={{ color: 'var(--text-primary)' }}>{runResult.final_output}</pre>
                  </div>
                </div>
              )}
            </div>
            <div className="px-4 py-3 border-t flex justify-end gap-2" style={{ borderColor: 'var(--border-color)' }}>
              <button onClick={() => { setShowRun(false); setRunResult(null); setRunError(''); }} className="text-xs px-3 py-1.5 rounded" style={{ background: 'var(--bg-tertiary)', color: 'var(--text-primary)' }}>Close</button>
              <button onClick={handleRun} disabled={running || !runInput.trim()} className="text-xs px-4 py-1.5 rounded disabled:opacity-50" style={{ background: 'var(--accent-color)', color: 'white' }}>
                {running ? 'Running...' : 'Run'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
