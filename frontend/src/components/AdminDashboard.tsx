import { useState, useEffect } from 'react';
import { getDashboard, downloadBackup } from '../api/client';

interface DashboardData {
  users: number;
  conversations: number;
  messages: number;
  active_providers: number;
  today: {
    requests: number;
    cost: number;
    tokens_in: number;
    tokens_out: number;
  };
  top_models: { model: string; requests: number; cost: number }[];
  recent_audit: {
    id: string;
    username: string;
    action: string;
    resource: string;
    details: string;
    created_at: string;
  }[];
}

function formatTokens(n: number): string {
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + 'M';
  if (n >= 1_000) return (n / 1_000).toFixed(1) + 'K';
  return String(n);
}

function timeAgo(dateStr: string): string {
  const diff = Date.now() - new Date(dateStr).getTime();
  const minutes = Math.floor(diff / 60000);
  if (minutes < 1) return 'just now';
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  return `${Math.floor(hours / 24)}d ago`;
}

interface StatCardProps {
  label: string;
  value: number | string;
}

function StatCard({ label, value }: StatCardProps) {
  return (
    <div className="flex flex-col items-center justify-center p-4 rounded-lg border border-gray-600" style={{ background: 'var(--bg-primary)' }}>
      <span className="text-2xl font-bold text-white">{typeof value === 'number' ? value.toLocaleString() : value}</span>
      <span className="text-xs text-gray-400 mt-1">{label}</span>
    </div>
  );
}

export default function AdminDashboard() {
  const [data, setData] = useState<DashboardData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [backupLoading, setBackupLoading] = useState(false);

  useEffect(() => {
    getDashboard()
      .then(setData)
      .catch(() => setError('Failed to load dashboard'))
      .finally(() => setLoading(false));
  }, []);

  const handleBackup = async () => {
    setBackupLoading(true);
    try {
      await downloadBackup();
    } catch {
      // ignore
    } finally {
      setBackupLoading(false);
    }
  };

  if (loading) {
    return <div className="text-gray-400 text-sm py-8 text-center">Loading dashboard...</div>;
  }

  if (error || !data) {
    return <div className="text-red-400 text-sm py-8 text-center">{error || 'No data'}</div>;
  }

  return (
    <div className="space-y-6">
      {/* Stat cards */}
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
        <StatCard label="Users" value={data.users} />
        <StatCard label="Conversations" value={data.conversations} />
        <StatCard label="Messages" value={data.messages} />
        <StatCard label="Active Providers" value={data.active_providers} />
      </div>

      {/* Today's usage */}
      <div className="rounded-lg border border-gray-600 p-4" style={{ background: 'var(--bg-primary)' }}>
        <h3 className="text-sm font-semibold text-gray-300 mb-3">Today's Usage</h3>
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 text-sm">
          <div>
            <span className="text-gray-500">Requests</span>
            <p className="text-white font-medium">{data.today.requests.toLocaleString()}</p>
          </div>
          <div>
            <span className="text-gray-500">Cost</span>
            <p className="text-white font-medium">${data.today.cost.toFixed(4)}</p>
          </div>
          <div>
            <span className="text-gray-500">Tokens In</span>
            <p className="text-white font-medium">{formatTokens(data.today.tokens_in)}</p>
          </div>
          <div>
            <span className="text-gray-500">Tokens Out</span>
            <p className="text-white font-medium">{formatTokens(data.today.tokens_out)}</p>
          </div>
        </div>
      </div>

      {/* Top models */}
      {data.top_models.length > 0 && (
        <div className="rounded-lg border border-gray-600 p-4" style={{ background: 'var(--bg-primary)' }}>
          <h3 className="text-sm font-semibold text-gray-300 mb-3">Top Models Today</h3>
          <div className="space-y-2">
            {data.top_models.map((m, i) => (
              <div key={m.model} className="flex items-center justify-between text-sm">
                <span className="text-gray-400 w-5">{i + 1}.</span>
                <span className="flex-1 text-white truncate">{m.model}</span>
                <span className="text-gray-400 ml-4">{m.requests} reqs</span>
                <span className="text-gray-400 ml-4 w-20 text-right">${m.cost.toFixed(4)}</span>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Recent activity */}
      {data.recent_audit.length > 0 && (
        <div className="rounded-lg border border-gray-600 p-4" style={{ background: 'var(--bg-primary)' }}>
          <h3 className="text-sm font-semibold text-gray-300 mb-3">Recent Activity</h3>
          <div className="space-y-2">
            {data.recent_audit.map((e) => (
              <div key={e.id} className="flex items-center justify-between text-sm">
                <span className="text-white flex-1 truncate">
                  <span className="text-blue-400">{e.username || 'system'}</span>
                  {' '}{e.action.replace(/_/g, ' ')}
                  {e.details ? <span className="text-gray-400"> "{e.details}"</span> : null}
                </span>
                <span className="text-gray-500 ml-4 flex-shrink-0">{timeAgo(e.created_at)}</span>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Backup */}
      <div className="flex justify-end pt-2">
        <button
          onClick={handleBackup}
          disabled={backupLoading}
          className="px-4 py-2 bg-gray-700 border border-gray-600 text-gray-300 rounded-lg text-sm hover:bg-gray-600 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
        >
          {backupLoading ? 'Preparing...' : 'Download Backup'}
        </button>
      </div>
    </div>
  );
}
