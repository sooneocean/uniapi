import { useState, useEffect } from 'react';
import { getScheduledTasks, createScheduledTask, deleteScheduledTask, getScheduledTaskResult } from '../api/client';
import { useToast } from './Toast';

interface ScheduledTask {
  id: string;
  model: string;
  prompt: string;
  system_prompt: string;
  run_at: string;
  last_run: string;
  result: string;
  status: 'pending' | 'running' | 'completed' | 'failed';
  created_at: string;
}

const statusColors: Record<string, string> = {
  pending: 'bg-yellow-900 text-yellow-300',
  running: 'bg-blue-900 text-blue-300',
  completed: 'bg-green-900 text-green-300',
  failed: 'bg-red-900 text-red-300',
};

export default function ScheduledTasks() {
  const { addToast } = useToast();
  const [tasks, setTasks] = useState<ScheduledTask[]>([]);
  const [loading, setLoading] = useState(true);
  const [expandedResult, setExpandedResult] = useState<string | null>(null);
  const [resultContent, setResultContent] = useState<Record<string, string>>({});

  // Form state
  const [showForm, setShowForm] = useState(false);
  const [model, setModel] = useState('');
  const [prompt, setPrompt] = useState('');
  const [systemPrompt, setSystemPrompt] = useState('');
  const [runAt, setRunAt] = useState('');
  const [submitting, setSubmitting] = useState(false);

  const load = async () => {
    setLoading(true);
    try {
      const data = await getScheduledTasks();
      setTasks(Array.isArray(data) ? data : []);
    } catch {
      setTasks([]);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    load();
  }, []);

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!model || !prompt || !runAt) return;
    setSubmitting(true);
    try {
      await createScheduledTask({ model, prompt, system_prompt: systemPrompt, run_at: new Date(runAt).toISOString() });
      setShowForm(false);
      setModel('');
      setPrompt('');
      setSystemPrompt('');
      setRunAt('');
      addToast('success', 'Task scheduled');
      await load();
    } catch {
      addToast('error', 'Failed to create scheduled task');
    } finally {
      setSubmitting(false);
    }
  };

  const handleDelete = async (id: string) => {
    if (!confirm('Delete this scheduled task?')) return;
    try {
      await deleteScheduledTask(id);
      addToast('success', 'Task deleted');
      await load();
    } catch {
      addToast('error', 'Failed to delete task');
    }
  };

  const handleViewResult = async (id: string) => {
    if (expandedResult === id) {
      setExpandedResult(null);
      return;
    }
    setExpandedResult(id);
    if (!resultContent[id]) {
      try {
        const data = await getScheduledTaskResult(id);
        setResultContent((prev) => ({ ...prev, [id]: data.result || '(no result)' }));
      } catch {
        setResultContent((prev) => ({ ...prev, [id]: 'Failed to load result' }));
      }
    }
  };

  // Default runAt to 1 hour from now
  const defaultRunAt = () => {
    const d = new Date(Date.now() + 3600000);
    return d.toISOString().slice(0, 16);
  };

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h2 className="text-white font-semibold">Scheduled Tasks</h2>
        <button
          className="px-3 py-1.5 bg-blue-600 hover:bg-blue-500 text-white text-sm rounded-lg transition-colors"
          onClick={() => { setShowForm(true); setRunAt(defaultRunAt()); }}
        >
          + Schedule Task
        </button>
      </div>

      {showForm && (
        <div className="bg-gray-700 rounded-xl p-4 border border-gray-600">
          <h3 className="text-white font-medium mb-3">New Scheduled Task</h3>
          <form onSubmit={handleCreate} className="space-y-3">
            <div>
              <label className="block text-gray-300 text-sm mb-1">Model</label>
              <input
                className="w-full bg-gray-800 text-white border border-gray-600 rounded-lg px-3 py-2 text-sm focus:outline-none focus:border-blue-500"
                placeholder="e.g. gpt-4o, claude-3-5-sonnet"
                value={model}
                onChange={(e) => setModel(e.target.value)}
                required
              />
            </div>
            <div>
              <label className="block text-gray-300 text-sm mb-1">Prompt</label>
              <textarea
                className="w-full bg-gray-800 text-white border border-gray-600 rounded-lg px-3 py-2 text-sm focus:outline-none focus:border-blue-500 resize-none"
                rows={3}
                placeholder="What should the AI do?"
                value={prompt}
                onChange={(e) => setPrompt(e.target.value)}
                required
              />
            </div>
            <div>
              <label className="block text-gray-300 text-sm mb-1">System Prompt (optional)</label>
              <textarea
                className="w-full bg-gray-800 text-white border border-gray-600 rounded-lg px-3 py-2 text-sm focus:outline-none focus:border-blue-500 resize-none"
                rows={2}
                placeholder="Optional system prompt..."
                value={systemPrompt}
                onChange={(e) => setSystemPrompt(e.target.value)}
              />
            </div>
            <div>
              <label className="block text-gray-300 text-sm mb-1">Run At</label>
              <input
                type="datetime-local"
                className="w-full bg-gray-800 text-white border border-gray-600 rounded-lg px-3 py-2 text-sm focus:outline-none focus:border-blue-500"
                value={runAt}
                onChange={(e) => setRunAt(e.target.value)}
                required
              />
            </div>
            <div className="flex gap-2 justify-end">
              <button
                type="button"
                className="px-3 py-1.5 bg-gray-600 hover:bg-gray-500 text-gray-200 text-sm rounded-lg transition-colors"
                onClick={() => setShowForm(false)}
              >
                Cancel
              </button>
              <button
                type="submit"
                disabled={submitting}
                className="px-3 py-1.5 bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white text-sm rounded-lg transition-colors"
              >
                {submitting ? 'Scheduling...' : 'Schedule'}
              </button>
            </div>
          </form>
        </div>
      )}

      {loading && (
        <div className="text-gray-400 text-sm text-center py-4">Loading...</div>
      )}

      {!loading && tasks.length === 0 && (
        <div className="text-gray-500 text-sm text-center py-8">
          No scheduled tasks. Create one to run an AI prompt at a specific time.
        </div>
      )}

      <div className="space-y-2">
        {tasks.map((task) => (
          <div key={task.id} className="bg-gray-700 rounded-xl border border-gray-600 overflow-hidden">
            <div className="p-4">
              <div className="flex items-start justify-between gap-3">
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2 mb-1">
                    <span className={`text-xs px-2 py-0.5 rounded-full font-medium ${statusColors[task.status] ?? 'bg-gray-800 text-gray-400'}`}>
                      {task.status}
                    </span>
                    <span className="text-gray-400 text-xs">{task.model}</span>
                  </div>
                  <p className="text-white text-sm truncate">{task.prompt}</p>
                  <div className="flex items-center gap-3 mt-1">
                    {task.run_at && (
                      <span className="text-gray-500 text-xs">
                        Run at: {new Date(task.run_at).toLocaleString()}
                      </span>
                    )}
                    {task.last_run && (
                      <span className="text-gray-500 text-xs">
                        Last run: {new Date(task.last_run).toLocaleString()}
                      </span>
                    )}
                  </div>
                </div>
                <div className="flex items-center gap-2 flex-shrink-0">
                  {(task.status === 'completed' || task.status === 'failed') && (
                    <button
                      className="text-xs text-blue-400 hover:text-blue-300 underline"
                      onClick={() => handleViewResult(task.id)}
                    >
                      {expandedResult === task.id ? 'Hide' : 'Result'}
                    </button>
                  )}
                  <button
                    className="text-xs text-red-400 hover:text-red-300"
                    onClick={() => handleDelete(task.id)}
                  >
                    Delete
                  </button>
                </div>
              </div>
            </div>
            {expandedResult === task.id && (
              <div className="px-4 pb-4 border-t border-gray-600 pt-3">
                <p className="text-gray-300 text-sm whitespace-pre-wrap leading-relaxed max-h-48 overflow-y-auto">
                  {resultContent[task.id] ?? 'Loading...'}
                </p>
              </div>
            )}
          </div>
        ))}
      </div>
    </div>
  );
}
